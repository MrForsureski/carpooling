package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"office_trip/internal/dto"
	mw "office_trip/internal/middleware"
	"office_trip/internal/model"
)

// register регистрация пользователя
func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := h.decodeJSON(r, &req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_body", "Неверный формат запроса")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		h.respondError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	user, err := h.authSvc.Register(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrEmailDomainNotAllowed):
			h.respondError(w, http.StatusBadRequest, "email_domain_not_allowed",
				"Регистрация с этим email-доменом недоступна")
		case errors.Is(err, model.ErrEmailAlreadyExists):
			h.respondError(w, http.StatusConflict, "email_already_exists",
				"Пользователь с таким email уже зарегистрирован")
		default:
			h.respondError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка")
		}
		return
	}

	//разработка верификация без отправки email
	if h.cfg.Environment == "development" {
		_ = h.authSvc.QuickVerifyForDev(r.Context(), user.ID)
	}

	h.respondJSON(w, http.StatusCreated, map[string]any{
		"id":      user.ID,
		"email":   user.Email,
		"message": "Регистрация успешна. Проверьте почту для подтверждения email",
	})
}

// login авторизация пользователя
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := h.decodeJSON(r, &req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_body", "Неверный формат запроса")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		h.respondError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	authResp, refreshToken, err := h.authSvc.Login(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrInvalidCredentials):
			h.respondError(w, http.StatusUnauthorized, "invalid_credentials", "Неверный email или пароль")
		case errors.Is(err, model.ErrEmailNotVerified):
			h.respondError(w, http.StatusForbidden, "email_not_verified", "Подтвердите email перед входом")
		default:
			slog.Error("login failed", "error", err)
			h.respondError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка")
		}
		return
	}

	//установка refresh token в httponly cookie
	secure := h.cfg.Environment == "production"
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/auth/refresh",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.cfg.JWTRefreshExpiry.Seconds()),
	})

	// Установка access token в cookie для прямой навигации в браузере
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    authResp.AccessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.cfg.JWTAccessExpiry.Seconds()),
	})

	h.respondJSON(w, http.StatusOK, authResp)
}

// Logout выход из системы
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	userIDStr, ok := mw.GetUserID(r)
	if !ok {
		http.SetCookie(w, &http.Cookie{
			Name:    "refresh_token",
			Value:   "",
			Path:    "/auth/refresh",
			MaxAge:  -1,
			HttpOnly: true,
		})
		http.SetCookie(w, &http.Cookie{
			Name:    "access_token",
			Value:   "",
			Path:    "/",
			MaxAge:  -1,
			HttpOnly: true,
		})
		if acceptsHTML(r) || isHTMX(r) {
			if isHTMX(r) {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusOK)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		h.respondJSON(w, http.StatusOK, map[string]string{"message": "Выход выполнен"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err == nil {
		_ = h.authSvc.Logout(r.Context(), userID)
	}

	http.SetCookie(w, &http.Cookie{
		Name:    "refresh_token",
		Value:   "",
		Path:    "/auth/refresh",
		MaxAge:  -1,
		HttpOnly: true,
	})
	http.SetCookie(w, &http.Cookie{
		Name:    "access_token",
		Value:   "",
		Path:    "/",
		MaxAge:  -1,
		HttpOnly: true,
	})

	if acceptsHTML(r) || isHTMX(r) {
		if isHTMX(r) {
			w.Header().Set("HX-Redirect", "/login")
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "Выход выполнен"})
}

//refresh обновление токенов
func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "missing_refresh_token", "Refresh token не найден")
		return
	}

	//Получение user id из тела запроса или из cookie
	var body struct {
		UserID string `json:"user_id"`
	}
	_ = h.decodeJSON(r, &body)

	userID, err := uuid.Parse(body.UserID)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_user_id", "Неверный user_id")
		return
	}

	newToken, err := h.authSvc.RefreshAccessToken(r.Context(), userID, cookie.Value)
	if err != nil {
		h.respondError(w, http.StatusUnauthorized, "invalid_refresh_token", "Refresh token недействителен")
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]any{
		"access_token": newToken,
		"expires_in":   int(h.cfg.JWTAccessExpiry.Seconds()),
	})
}

// Verifyemail подтверждение email
func (h *Handler) verifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Неверный токен", http.StatusBadRequest)
		return
	}

	if err := h.authSvc.VerifyEmail(r.Context(), token); err != nil {
		http.Error(w, "Токен недействителен или истёк", http.StatusBadRequest)
		return
	}

	//редирект на страницу логина
	http.Redirect(w, r, "/login?verified=1", http.StatusFound)
}

//Loginpage получение страницы входа
func (h *Handler) loginPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title":    "Вход",
		"AppName":  h.cfg.AppName,
		"Verified": r.URL.Query().Get("verified") == "1",
		"Year":     time.Now().Year(),
	}
	h.renderTemplate(w, "login.html", data)
}

// registerpage получение страницы регистрации
func (h *Handler) registerPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Title":   "Регистрация",
		"AppName": h.cfg.AppName,
		"Year":    time.Now().Year(),
	}
	h.renderTemplate(w, "register.html", data)
}

//Profilepage получение страницы профиля
func (h *Handler) profilePage(w http.ResponseWriter, r *http.Request) {
	user, ok := h.getAuthUser(w, r)
	if !ok {
		return
	}

	//статистика поездок водитель создал пассажир записался
	var tripCount int
	if user.Role == "driver" || user.Role == "admin" {
		_, total, err := h.tripSvc.ListMyTrips(r.Context(), user.ID, 1, 0)
		if err == nil {
			tripCount = total
		}
	} else {
		_, total, err := h.tripSvc.ListJoinedTrips(r.Context(), user.ID, 1, 0)
		if err == nil {
			tripCount = total
		}
	}

	data := map[string]any{
		"Title":     "Профиль",
		"AppName":   h.cfg.AppName,
		"User":      user,
		"TripCount": tripCount,
		"Year":      time.Now().Year(),
	}
	h.renderTemplate(w, "profile.html", data)
}
