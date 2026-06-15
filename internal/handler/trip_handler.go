package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"office_trip/internal/dto"
	mw "office_trip/internal/middleware"
	"office_trip/internal/model"
)

// listTrips — GET /trips?office_id=&date=&lat=&lng=
func (h *Handler) listTrips(w http.ResponseWriter, r *http.Request) {
	user, ok := h.getAuthUser(w, r)
	if !ok {
		return
	}

	q := r.URL.Query()

	officeIDStr := q.Get("office_id")
	var officeID uuid.UUID
	var err error
	if officeIDStr != "" {
		officeID, err = uuid.Parse(officeIDStr)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid_office_id", "Неверный office_id")
			return
		}
	}

	dateStr := q.Get("date")
	var date time.Time
	if dateStr != "" {
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid_date", "Формат даты: YYYY-MM-DD")
			return
		}
	} else {
		date = time.Now()
		dateStr = date.Format("2006-01-02")
	}

	lat, _ := strconv.ParseFloat(q.Get("lat"), 64)
	lng, _ := strconv.ParseFloat(q.Get("lng"), 64)

	limit, offset := pagination(r)
	trips, total, err := h.tripSvc.SearchTrips(r.Context(), officeID, date, lat, lng, limit, offset)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Ошибка поиска поездок")
		return
	}

	// HTML или JSON в зависимости от Accept
	if isHTMX(r) || acceptsHTML(r) {
		offices, _ := h.officeSvc.ListOffices(r.Context(), user.CompanyID)

		data := map[string]any{
			"Title":    "Поиск поездок",
			"AppName":  h.cfg.AppName,
			"Trips":    trips,
			"Total":    total,
			"OfficeID": officeIDStr,
			"Offices":  offices,
			"User":     user,
			"Date":     dateStr,
			"Lat":      lat,
			"Lng":      lng,
			"Year":     time.Now().Year(),
		}
		h.renderTemplate(w, "trips_list.html", data)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{
		"trips": trips,
		"total": total,
		"page":  1,
	})
}

// getTrip — GET /trips/:id
func (h *Handler) getTrip(w http.ResponseWriter, r *http.Request) {
	user, ok := h.getAuthUser(w, r)
	if !ok {
		return
	}

	tripID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_id", "Неверный ID поездки")
		return
	}

	trip, err := h.tripSvc.GetTrip(r.Context(), tripID)
	if err != nil {
		h.respondError(w, http.StatusNotFound, "not_found", "Поездка не найдена")
		return
	}

	passengers, _ := h.tripSvc.GetPassengers(r.Context(), tripID)

	isPassenger := false
	// Находим остановку текущего пользователя
	for _, p := range passengers {
		if p.UserID == user.ID {
			isPassenger = true
			if p.StopID != nil {
				stops := trip.Stops
				if len(stops) == 0 {
					stops, _ = h.tripSvc.GetStopsByTripID(r.Context(), trip.ID)
				}
				for _, s := range stops {
					if s.ID == *p.StopID {
						trip.UserPickupStop = s
						break
					}
				}
			}
			break
		}
	}

	if acceptsHTML(r) {
		data := map[string]any{
			"Title":       "Поездка",
			"AppName":     h.cfg.AppName,
			"Trip":        trip,
			"Passengers":  passengers,
			"CurrentUser": user.ID.String(),
			"User":        user,
			"Year":        time.Now().Year(),
			"IsPassenger": isPassenger,
		}
		h.renderTemplate(w, "trip_detail.html", data)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{
		"trip":       trip,
		"passengers": passengers,
	})
}

// createTrip — POST /trips
func (h *Handler) createTrip(w http.ResponseWriter, r *http.Request) {
	userIDStr, _ := mw.GetUserID(r)
	driverID, _ := uuid.Parse(userIDStr)

	var req dto.CreateTripRequest
	if err := h.decodeJSON(r, &req); err != nil {
		// Попробуем прочитать как form data (HTMX)
		if err := r.ParseForm(); err == nil {
			req.OfficeID = r.FormValue("office_id")
			req.OriginAddress = r.FormValue("origin_address")
			req.DepartAt = r.FormValue("depart_at")
			req.OriginLat, _ = strconv.ParseFloat(r.FormValue("origin_lat"), 64)
			req.OriginLng, _ = strconv.ParseFloat(r.FormValue("origin_lng"), 64)
			req.SeatsTotal, _ = strconv.Atoi(r.FormValue("seats_total"))
		} else {
			h.respondError(w, http.StatusBadRequest, "invalid_body", "Неверный формат запроса")
			return
		}
	}

	if err := h.validate.Struct(req); err != nil {
		h.respondError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	trip, err := h.tripSvc.CreateTrip(r.Context(), driverID, req)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrTooSoonToCreate):
			h.respondError(w, http.StatusBadRequest, "too_soon", "Поездка должна быть создана минимум за 60 минут до выезда")
		case errors.Is(err, model.ErrOriginOutsideZone):
			h.respondError(w, http.StatusBadRequest, "origin_outside_zone", "Точка старта находится вне допустимой зоны для этого офиса")
		case errors.Is(err, model.ErrDriverConflict):
			h.respondError(w, http.StatusConflict, "scheduling_conflict", "У вас уже есть поездка в это время (±2 часа)")
		default:
			h.respondError(w, http.StatusInternalServerError, "internal_error", "Ошибка создания поездки")
		}
		return
	}

	h.respondJSON(w, http.StatusCreated, map[string]any{
		"id":               trip.ID,
		"route_geojson":    trip.RouteGeoJSON,
		"duration_seconds": trip.DurationSeconds,
		"distance_meters":  trip.DistanceMeters,
		"message":          "Поездка создана",
	})
}

// joinTrip — POST /trips/:id/join
func (h *Handler) joinTrip(w http.ResponseWriter, r *http.Request) {
	tripID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_id", "Неверный ID поездки")
		return
	}

	userIDStr, _ := mw.GetUserID(r)
	passengerID, _ := uuid.Parse(userIDStr)

	var req dto.JoinTripRequest
	if err := h.decodeJSON(r, &req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_body", "Неверный формат запроса")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		h.respondError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	passenger, err := h.tripSvc.JoinTrip(r.Context(), passengerID, tripID, req)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrNoSeatsAvailable):
			h.respondError(w, http.StatusConflict, "no_seats_available", "Нет свободных мест")
		case errors.Is(err, model.ErrJoinTooLate):
			h.respondError(w, http.StatusBadRequest, "join_too_late", "Слишком поздно присоединяться к поездке")
		case errors.Is(err, model.ErrSchedulingConflict):
			h.respondError(w, http.StatusConflict, "scheduling_conflict", "Конфликт с другой поездкой")
		case errors.Is(err, model.ErrCannotJoinOwnTrip):
			h.respondError(w, http.StatusBadRequest, "cannot_join_own_trip", "Водитель не может быть пассажиром своей поездки")
		case errors.Is(err, model.ErrAlreadyInTrip):
			h.respondError(w, http.StatusConflict, "already_in_trip", "Вы уже участвуете в этой поездке")
		case errors.Is(err, model.ErrTripNotScheduled):
			h.respondError(w, http.StatusBadRequest, "trip_not_scheduled", "Поездка недоступна для записи")
		default:
			var errFar *model.ErrTooFarFromRoute
			var errDetour *model.ErrDetourExceeded
			switch {
			case errors.As(err, &errFar):
				h.respondError(w, http.StatusBadRequest, "too_far_from_route", err.Error())
			case errors.As(err, &errDetour):
				h.respondError(w, http.StatusBadRequest, "detour_exceeded", err.Error())
			default:
				h.respondError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка")
			}
		}
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{
		"message":        "Вы присоединились к поездке",
		"detour_seconds": passenger.DetourSeconds,
	})
}

// leaveTrip — POST /trips/:id/leave
func (h *Handler) leaveTrip(w http.ResponseWriter, r *http.Request) {
	tripID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_id", "Неверный ID поездки")
		return
	}

	userIDStr, _ := mw.GetUserID(r)
	userID, _ := uuid.Parse(userIDStr)

	if err := h.tripSvc.LeaveTrip(r.Context(), userID, tripID); err != nil {
		switch {
		case errors.Is(err, model.ErrCancelTooLate):
			h.respondError(w, http.StatusBadRequest, "cancel_too_late", "Слишком поздно отказываться от поездки")
		case errors.Is(err, model.ErrTripNotScheduled):
			h.respondError(w, http.StatusBadRequest, "trip_not_scheduled", "Поездка уже недоступна")
		default:
			h.respondError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка")
		}
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "Вы покинули поездку"})
}

// cancelTrip — POST /trips/:id/cancel
func (h *Handler) cancelTrip(w http.ResponseWriter, r *http.Request) {
	tripID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_id", "Неверный ID поездки")
		return
	}

	userIDStr, _ := mw.GetUserID(r)
	userID, _ := uuid.Parse(userIDStr)
	isAdmin := mw.GetUserRole(r) == "admin"

	if err := h.tripSvc.CancelTrip(r.Context(), userID, tripID, isAdmin); err != nil {
		switch {
		case errors.Is(err, model.ErrForbidden):
			h.respondError(w, http.StatusForbidden, "forbidden", "Вы не можете отменить эту поездку")
		case errors.Is(err, model.ErrTripNotScheduled):
			h.respondError(w, http.StatusBadRequest, "trip_not_available", "Поездка уже отменена или завершена")
		default:
			h.respondError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка")
		}
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "Поездка отменена"})
}

// myTrips — GET /trips/my
func (h *Handler) myTrips(w http.ResponseWriter, r *http.Request) {
	user, ok := h.getAuthUser(w, r)
	if !ok {
		return
	}

	limit, offset := pagination(r)
	tripsLimit := limit
	tripsOffset := offset
	if acceptsHTML(r) {
		tripsLimit = 10000
		tripsOffset = 0
	}
	var trips []*model.Trip
	var total int
	var err error
	if user.Role == "admin" {
		trips, total, err = h.tripSvc.ListAllTrips(r.Context(), tripsLimit, tripsOffset)
	} else {
		trips, total, err = h.tripSvc.ListMyTrips(r.Context(), user.ID, tripsLimit, tripsOffset)
	}
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Ошибка получения поездок")
		return
	}

	// Загружаем поездки, в которых пользователь участвует как пассажир
	joinedTrips, joinedTotal, _ := h.tripSvc.ListJoinedTrips(r.Context(), user.ID, tripsLimit, tripsOffset)
	// Подгружаем остановки для каждой поездки пассажира
	for _, t := range joinedTrips {
		stops, _ := h.tripSvc.GetStopsByTripID(r.Context(), t.ID)
		t.Stops = stops

		// Находим остановку текущего пользователя
		passengers, _ := h.tripSvc.GetPassengers(r.Context(), t.ID)
		for _, p := range passengers {
			if p.UserID == user.ID && p.StopID != nil {
				for _, s := range stops {
					if s.ID == *p.StopID {
						t.UserPickupStop = s
						break
					}
				}
				break
			}
		}
	}

	if acceptsHTML(r) {
		data := map[string]any{
			"Title":       "Мои поездки",
			"AppName":     h.cfg.AppName,
			"Trips":       trips,
			"Total":       total,
			"JoinedTrips": joinedTrips,
			"JoinedTotal": joinedTotal,
			"User":        user,
			"Year":        time.Now().Year(),
		}
		h.renderTemplate(w, "my_trips.html", data)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{"trips": trips, "total": total, "joined_trips": joinedTrips})
}

// joinedTrips — GET /trips/joined
func (h *Handler) joinedTrips(w http.ResponseWriter, r *http.Request) {
	userIDStr, _ := mw.GetUserID(r)
	userID, _ := uuid.Parse(userIDStr)

	limit, offset := pagination(r)
	trips, total, err := h.tripSvc.ListJoinedTrips(r.Context(), userID, limit, offset)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Ошибка получения поездок")
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{"trips": trips, "total": total})
}

// routePreview — GET /api/route-preview?origin_lat=&origin_lng=&office_id=
func (h *Handler) routePreview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lat, _ := strconv.ParseFloat(q.Get("origin_lat"), 64)
	lng, _ := strconv.ParseFloat(q.Get("origin_lng"), 64)
	officeID, err := uuid.Parse(q.Get("office_id"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_office_id", "Неверный office_id")
		return
	}

	stopsLats := q["stops_lat"]
	stopsLngs := q["stops_lng"]
	var stopCoords []float64
	for i := 0; i < len(stopsLats) && i < len(stopsLngs); i++ {
		slat, err1 := strconv.ParseFloat(stopsLats[i], 64)
		slng, err2 := strconv.ParseFloat(stopsLngs[i], 64)
		if err1 == nil && err2 == nil {
			stopCoords = append(stopCoords, slat, slng)
		}
	}

	route, err := h.tripSvc.GetRoutePreview(r.Context(), lat, lng, officeID, stopCoords...)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "routing_error", "Не удалось построить маршрут")
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{
		"route_geojson":    route.GeoJSON,
		"duration_seconds": route.DurationSeconds,
		"distance_meters":  route.DistanceMeters,
	})
}

// createTripPage — GET /trips/new
func (h *Handler) createTripPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.getAuthUser(w, r)
	if !ok {
		return
	}

	// Получаем список офисов для компании пользователя
	offices, _ := h.officeSvc.ListOffices(r.Context(), user.CompanyID)

	data := map[string]any{
		"Title":   "Создать поездку",
		"AppName": h.cfg.AppName,
		"Offices": offices,
		"User":    user,
		"Year":    time.Now().Year(),
	}
	h.renderTemplate(w, "create_trip.html", data)
}

// mapPage — GET /map
func (h *Handler) mapPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.getAuthUser(w, r)
	if !ok {
		return
	}

	offices, _ := h.officeSvc.ListOffices(r.Context(), user.CompanyID)

	data := map[string]any{
		"Title":   "Карта",
		"AppName": h.cfg.AppName,
		"Offices": offices,
		"User":    user,
		"Year":    time.Now().Year(),
	}
	h.renderTemplate(w, "map.html", data)
}

// isHTMX проверяет, что запрос пришёл от HTMX
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// acceptsHTML проверяет Accept заголовок
func acceptsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return accept == "" || contains(accept, "text/html")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

