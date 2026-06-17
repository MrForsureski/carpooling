package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"office_trip/internal/dto"
	mw "office_trip/internal/middleware"
	"office_trip/internal/model"
)

// adminlistusers получение списка пользователей
func (h *Handler) adminListUsers(w http.ResponseWriter, r *http.Request) {
	currentUser, ok := h.getAuthUser(w, r)
	if !ok {
		return
	}

	limit, offset := pagination(r)
	users, total, err := h.authSvc.ListUsers(r.Context(), limit, offset)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Ошибка получения пользователей")
		return
	}

	if acceptsHTML(r) {
		data := map[string]any{
			"Title":   "Пользователи",
			"AppName": h.cfg.AppName,
			"Users":   users,
			"Total":   total,
			"User":    currentUser,
			"Year":    time.Now().Year(),
		}
		h.renderTemplate(w, "admin_users.html", data)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{"users": users, "total": total})
}

//adminupdaterole изменение роли пользователя
func (h *Handler) adminUpdateRole(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_id", "Неверный ID пользователя")
		return
	}

	var req dto.UpdateRoleRequest
	if err := h.decodeJSON(r, &req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_body", "Неверный формат запроса")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		h.respondError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := h.authSvc.UpdateRole(r.Context(), userID, req.Role); err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Ошибка обновления роли")
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "Роль обновлена"})
}

// adminlisttrips получение списка поездок
func (h *Handler) adminListTrips(w http.ResponseWriter, r *http.Request) {
	limit, offset := pagination(r)
	trips, total, err := h.tripSvc.ListAllTrips(r.Context(), limit, offset)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Ошибка получения поездок")
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{"trips": trips, "total": total})
}

// Admincanceltrip отмена поездки админом
func (h *Handler) adminCancelTrip(w http.ResponseWriter, r *http.Request) {
	tripID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_id", "Неверный ID поездки")
		return
	}

	userIDStr, _ := mw.GetUserID(r)
	userID, _ := uuid.Parse(userIDStr)

	if err := h.tripSvc.CancelTrip(r.Context(), userID, tripID, true); err != nil {
		switch {
		case errors.Is(err, model.ErrTripNotScheduled):
			h.respondError(w, http.StatusBadRequest, "trip_not_available", "Поездка уже отменена или завершена")
		default:
			h.respondError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка")
		}
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "Поездка отменена"})
}

// Adminstats получение админ статистики
func (h *Handler) adminStats(w http.ResponseWriter, r *http.Request) {
	_, totalUsers, _ := h.authSvc.ListUsers(r.Context(), 1, 0)
	_, totalTrips, _ := h.tripSvc.ListAllTrips(r.Context(), 1, 0)

	h.respondJSON(w, http.StatusOK, map[string]any{
		"total_users": totalUsers,
		"total_trips": totalTrips,
	})
}

// Adminpage получение админ страницы
func (h *Handler) adminPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.getAuthUser(w, r)
	if !ok {
		return
	}

	_, totalUsers, _ := h.authSvc.ListUsers(r.Context(), 1, 0)
	_, totalTrips, _ := h.tripSvc.ListAllTrips(r.Context(), 1, 0)

	data := map[string]any{
		"Title":      "Панель администратора",
		"AppName":    h.cfg.AppName,
		"User":       user,
		"TotalUsers": totalUsers,
		"TotalTrips": totalTrips,
		"Year":       time.Now().Year(),
	}
	h.renderTemplate(w, "admin.html", data)
}
