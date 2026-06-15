package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"office_trip/internal/config"
	mw "office_trip/internal/middleware"
	"office_trip/internal/model"
	"office_trip/internal/service"
)

// Handler — основная структура с зависимостями
type Handler struct {
	authSvc   *service.AuthService
	tripSvc   *service.TripService
	officeSvc *service.OfficeService
	cfg       *config.Config
	validate  *validator.Validate
	templates map[string]*template.Template
}

// Deps — зависимости для создания Handler
type Deps struct {
	AuthSvc   *service.AuthService
	TripSvc   *service.TripService
	OfficeSvc *service.OfficeService
	Config    *config.Config
}

func New(deps Deps) *Handler {
	h := &Handler{
		authSvc:   deps.AuthSvc,
		tripSvc:   deps.TripSvc,
		officeSvc: deps.OfficeSvc,
		cfg:       deps.Config,
		validate:  validator.New(),
	}

	// Загружаем шаблоны
	h.loadTemplates()

	return h
}

func (h *Handler) loadTemplates() {
	// Template функции
	moscow, _ := time.LoadLocation("Europe/Moscow")
	if moscow == nil {
		moscow = time.FixedZone("MSK", 3*60*60)
	}
	funcMap := template.FuncMap{
		"addSeconds": func(t time.Time, seconds int) time.Time {
			return t.Add(time.Duration(seconds) * time.Second)
		},
		"formatTime": func(t time.Time) string {
			return t.In(moscow).Format("15:04")
		},
		"formatDate": func(t time.Time) string {
			months := []string{"янв", "фев", "мар", "апр", "май", "июн", "июл", "авг", "сен", "окт", "ноя", "дек"}
			tLocal := t.In(moscow)
			return fmt.Sprintf("%d %s", tLocal.Day(), months[tLocal.Month()-1])
		},
		"formatDuration": func(seconds int) string {
			h := seconds / 3600
			m := (seconds % 3600) / 60
			if h > 0 {
				return fmt.Sprintf("%dч %dм", h, m)
			}
			return fmt.Sprintf("%dм", m)
		},
		"formatDistance": func(meters float64) string {
			if meters == 0 {
				return "—"
			}
			if meters < 1000 {
				return fmt.Sprintf("%.0f м", meters)
			}
			return fmt.Sprintf("%.1f км", meters/1000)
		},
		"formatDistanceInt": func(meters int) string {
			if meters == 0 {
				return "—"
			}
			if meters < 1000 {
				return fmt.Sprintf("%d м", meters)
			}
			return fmt.Sprintf("%.1f км", float64(meters)/1000)
		},
		"now": func() string {
			return time.Now().Format("2006-01-02")
		},
		"isTripPast": func(departAt time.Time, durationSeconds int) bool {
			return departAt.Add(time.Duration(durationSeconds) * time.Second).Before(time.Now())
		},
		"slice": func(s string, start, end int) string {
			if len(s) == 0 {
				return ""
			}
			runes := []rune(s)
			if start < 0 {
				start = 0
			}
			if end > len(runes) {
				end = len(runes)
			}
			if start > end {
				return ""
			}
			return string(runes[start:end])
		},
	}

	h.templates = make(map[string]*template.Template)
	pages := []string{
		"web/templates/auth/login.html",
		"web/templates/auth/register.html",
		"web/templates/trips/trips_list.html",
		"web/templates/trips/trip_detail.html",
		"web/templates/trips/create_trip.html",
		"web/templates/trips/my_trips.html",
		"web/templates/map/map.html",
		"web/templates/admin/admin.html",
		"web/templates/admin/admin_users.html",
		"web/templates/admin/admin_offices.html",
		"web/templates/profile/profile.html",
	}

	for _, page := range pages {
		name := filepath.Base(page)
		t := template.New(name).Funcs(funcMap)
		_, err := t.ParseFiles(
			"web/templates/layout/base.html",
			"web/templates/layout/nav.html",
			page,
		)
		if err != nil {
			slog.Error("failed to parse template", "page", page, "error", err)
			continue
		}
		h.templates[name] = t
	}
}

// Router создаёт и возвращает Chi роутер со всеми маршрутами
func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()

	// Глобальный middleware
	r.Use(middleware.Recoverer)
	r.Use(mw.Logging)

	// Статические файлы
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Health check (без авторизации)
	r.Get("/health", h.healthCheck)

	// Auth маршруты (без JWT)
	r.Group(func(r chi.Router) {
		r.Post("/auth/register", h.register)
		r.Post("/auth/login", h.login)
		r.Post("/auth/logout", h.logout)
		r.Post("/auth/refresh", h.refresh)
		r.Get("/auth/verify", h.verifyEmail)

		// Страницы авторизации
		r.Get("/login", h.loginPage)
		r.Get("/register", h.registerPage)
	})

	// Защищённые маршруты (требуют JWT)
	r.Group(func(r chi.Router) {
		r.Use(mw.Auth(h.cfg.JWTSecret))

		// Офисы
		r.Get("/offices", h.listOffices)
		r.Get("/offices/{id}", h.getOffice)

		// Поиск и просмотр поездок
		r.Get("/trips", h.listTrips)
		r.Get("/trips/my", h.myTrips)
		r.Get("/trips/joined", h.joinedTrips)
		r.Get("/trips/{id}", h.getTrip)

		// Route preview (для карты)
		r.Get("/api/route-preview", h.routePreview)

		// Страницы
		r.Get("/map", h.mapPage)
		r.Get("/profile", h.profilePage)

		// Присоединение/выход из поездки
		r.Post("/trips/{id}/join", h.joinTrip)
		r.Post("/trips/{id}/leave", h.leaveTrip)

		// Для водителей и admin
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireRole("driver", "admin"))
			r.Post("/trips", h.createTrip)
			r.Post("/trips/{id}/cancel", h.cancelTrip)
			r.Get("/trips/new", h.createTripPage)
		})

		// Admin маршруты
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireRole("admin"))
			r.Get("/admin/users", h.adminListUsers)
			r.Put("/admin/users/{id}/role", h.adminUpdateRole)
			r.Get("/admin/trips", h.adminListTrips)
			r.Post("/admin/trips/{id}/cancel", h.adminCancelTrip)
			r.Get("/admin/stats", h.adminStats)

			r.Post("/admin/offices", h.createOffice)
			r.Put("/admin/offices/{id}", h.updateOffice)
			r.Delete("/admin/offices/{id}", h.deactivateOffice)
			r.Get("/admin/offices/{id}/zone", h.getZone)
			r.Post("/admin/offices/{id}/zones", h.createZone)
			r.Put("/admin/offices/{id}/zones/{zid}", h.updateZone)

			r.Get("/admin", h.adminPage)
			r.Get("/admin/offices", h.adminOfficesPage)
		})
	})

	return r
}

// healthCheck — /health endpoint
func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	h.respondJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "office-trip-carpooling",
	})
}

// --- Вспомогательные методы ---

func (h *Handler) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

func (h *Handler) respondError(w http.ResponseWriter, status int, code, message string) {
	h.respondJSON(w, status, map[string]string{
		"error":   code,
		"message": message,
	})
}

func (h *Handler) renderTemplate(w http.ResponseWriter, name string, data any) {
	if h.templates == nil {
		http.Error(w, "templates not loaded", http.StatusInternalServerError)
		return
	}
	t, ok := h.templates[name]
	if !ok {
		slog.Error("template not found in cache", "template", name)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("template error", "template", name, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) decodeJSON(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

// pagination парсит limit/offset из query параметров
func pagination(r *http.Request) (limit, offset int) {
	limit = 20
	offset = 0
	// Простая реализация — можно расширить
	return limit, offset
}

// getAuthUser возвращает авторизованного пользователя или редиректит/очищает сессию при ошибке
func (h *Handler) getAuthUser(w http.ResponseWriter, r *http.Request) (*model.User, bool) {
	userIDStr, ok := mw.GetUserID(r)
	if !ok {
		h.clearAuthAndRedirect(w, r)
		return nil, false
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.clearAuthAndRedirect(w, r)
		return nil, false
	}
	user, err := h.authSvc.GetUserByID(r.Context(), userID)
	if err != nil || user == nil {
		h.clearAuthAndRedirect(w, r)
		return nil, false
	}
	return user, true
}

func (h *Handler) clearAuthAndRedirect(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/auth/refresh",
		MaxAge:   -1,
		HttpOnly: true,
	})
	if r.Header.Get("Accept") == "" || strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	} else {
		h.respondError(w, http.StatusUnauthorized, "unauthorized", "Требуется авторизация")
	}
}
