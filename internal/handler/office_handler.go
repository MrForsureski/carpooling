package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"office_trip/internal/dto"
	mw "office_trip/internal/middleware"
	"office_trip/internal/model"
)

// listOffices — GET /offices
func (h *Handler) listOffices(w http.ResponseWriter, r *http.Request) {
	userIDStr, _ := mw.GetUserID(r)
	userID, _ := uuid.Parse(userIDStr)

	user, err := h.authSvc.GetUserByID(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Не авторизован")
		return
	}

	offices, err := h.officeSvc.ListOffices(r.Context(), user.CompanyID)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Ошибка получения офисов")
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{"offices": offices})
}

// getOffice — GET /offices/:id
func (h *Handler) getOffice(w http.ResponseWriter, r *http.Request) {
	officeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_id", "Неверный ID офиса")
		return
	}

	office, err := h.officeSvc.GetOffice(r.Context(), officeID)
	if err != nil {
		h.respondError(w, http.StatusNotFound, "not_found", "Офис не найден")
		return
	}

	h.respondJSON(w, http.StatusOK, office)
}

// createOffice — POST /admin/offices
func (h *Handler) createOffice(w http.ResponseWriter, r *http.Request) {
	userIDStr, _ := mw.GetUserID(r)
	userID, _ := uuid.Parse(userIDStr)

	user, err := h.authSvc.GetUserByID(r.Context(), userID)
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Не авторизован")
		return
	}

	var req dto.CreateOfficeRequest
	if err := h.decodeJSON(r, &req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_body", "Неверный формат запроса")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		h.respondError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	office, err := h.officeSvc.CreateOffice(r.Context(), user.CompanyID, req)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Ошибка создания офиса")
		return
	}

	h.respondJSON(w, http.StatusCreated, office)
}

// updateOffice — PUT /admin/offices/:id
func (h *Handler) updateOffice(w http.ResponseWriter, r *http.Request) {
	officeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_id", "Неверный ID офиса")
		return
	}

	var req dto.CreateOfficeRequest
	if err := h.decodeJSON(r, &req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_body", "Неверный формат запроса")
		return
	}

	office, err := h.officeSvc.UpdateOffice(r.Context(), officeID, req)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Ошибка обновления офиса")
		return
	}

	h.respondJSON(w, http.StatusOK, office)
}

// deactivateOffice — DELETE /admin/offices/:id
func (h *Handler) deactivateOffice(w http.ResponseWriter, r *http.Request) {
	officeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_id", "Неверный ID офиса")
		return
	}

	if err := h.officeSvc.DeactivateOffice(r.Context(), officeID); err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Ошибка деактивации офиса")
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "Офис деактивирован"})
}

// createZone — POST /admin/offices/:id/zones
func (h *Handler) createZone(w http.ResponseWriter, r *http.Request) {
	officeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_id", "Неверный ID офиса")
		return
	}

	var req dto.CreateZoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_body", "Неверный формат запроса")
		return
	}

	zone, err := h.officeSvc.CreateZone(r.Context(), officeID, req)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Ошибка создания зоны")
		return
	}

	h.respondJSON(w, http.StatusCreated, zone)
}

// updateZone — PUT /admin/offices/:id/zones/:zid
func (h *Handler) updateZone(w http.ResponseWriter, r *http.Request) {
	officeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_id", "Неверный ID офиса")
		return
	}
	zoneID, err := uuid.Parse(chi.URLParam(r, "zid"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_zone_id", "Неверный ID зоны")
		return
	}

	var req dto.UpdateZoneRequest
	if err := h.decodeJSON(r, &req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_body", "Неверный формат запроса")
		return
	}

	if err := h.officeSvc.UpdateZone(r.Context(), officeID, zoneID, req); err != nil {
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Ошибка обновления зоны")
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "Зона обновлена"})
}

// adminOfficesPage — GET /admin/offices
func (h *Handler) adminOfficesPage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.getAuthUser(w, r)
	if !ok {
		return
	}

	offices, _ := h.officeSvc.ListOffices(r.Context(), user.CompanyID)

	data := map[string]any{
		"Title":   "Управление офисами",
		"AppName": h.cfg.AppName,
		"Offices": offices,
		"User":    user,
		"Year":    time.Now().Year(),
	}
	h.renderTemplate(w, "admin_offices.html", data)
}

// getZone — GET /admin/offices/:id/zone
func (h *Handler) getZone(w http.ResponseWriter, r *http.Request) {
	officeID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_id", "Неверный ID офиса")
		return
	}

	zone, err := h.officeSvc.GetZone(r.Context(), officeID)
	if err != nil {
		if errors.Is(err, model.ErrZoneNotFound) {
			h.respondJSON(w, http.StatusOK, nil)
			return
		}
		h.respondError(w, http.StatusInternalServerError, "internal_error", "Ошибка получения зоны")
		return
	}

	h.respondJSON(w, http.StatusOK, zone)
}
