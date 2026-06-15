package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"office_trip/internal/model"
)

type contextKey string

const (
	ContextUserID   contextKey = "user_id"
	ContextUserRole contextKey = "user_role"
)

// Auth проверяет JWT Bearer токен из заголовка Authorization
func Auth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tokenStr string
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && parts[0] == "Bearer" {
					tokenStr = parts[1]
				} else {
					if acceptsHTML(r) {
						http.Redirect(w, r, "/login", http.StatusSeeOther)
					} else {
						respondUnauthorized(w, "invalid_token_format", "Неверный формат токена")
					}
					return
				}
			}

			if tokenStr == "" {
				if cookie, err := r.Cookie("access_token"); err == nil {
					tokenStr = cookie.Value
				}
			}

			if tokenStr == "" {
				if acceptsHTML(r) {
					http.Redirect(w, r, "/login", http.StatusSeeOther)
				} else {
					respondUnauthorized(w, "missing_token", "Требуется авторизация")
				}
				return
			}

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, model.ErrInvalidToken
				}
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				if acceptsHTML(r) {
					// Очищаем куки
					http.SetCookie(w, &http.Cookie{
						Name:     "access_token",
						Value:    "",
						Path:     "/",
						MaxAge:   -1,
						HttpOnly: true,
					})
					http.Redirect(w, r, "/login", http.StatusSeeOther)
				} else {
					respondUnauthorized(w, "invalid_token", "Токен недействителен или истёк")
				}
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				if acceptsHTML(r) {
					http.Redirect(w, r, "/login", http.StatusSeeOther)
				} else {
					respondUnauthorized(w, "invalid_claims", "Неверные данные токена")
				}
				return
			}

			ctx := context.WithValue(r.Context(), ContextUserID, claims["sub"])
			ctx = context.WithValue(ctx, ContextUserRole, claims["role"])
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func acceptsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return accept == "" || strings.Contains(accept, "text/html")
}

// RequireRole проверяет, что у пользователя одна из разрешённых ролей
func RequireRole(roles ...model.Role) func(http.Handler) http.Handler {
	allowed := make(map[string]bool)
	for _, r := range roles {
		allowed[string(r)] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, _ := r.Context().Value(ContextUserRole).(string)
			if !allowed[role] {
				http.Error(w, `{"error":"forbidden","message":"Недостаточно прав"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GetUserID извлекает UUID пользователя из контекста
func GetUserID(r *http.Request) (string, bool) {
	id, ok := r.Context().Value(ContextUserID).(string)
	return id, ok && id != ""
}

// GetUserRole извлекает роль пользователя из контекста
func GetUserRole(r *http.Request) string {
	role, _ := r.Context().Value(ContextUserRole).(string)
	return role
}

func respondUnauthorized(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"` + code + `","message":"` + message + `"}`))
}
