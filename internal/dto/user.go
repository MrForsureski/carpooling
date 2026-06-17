package dto

import (
	"github.com/google/uuid"
	"office_trip/internal/model"
)

//registerrequest запрос на регистрацию
type RegisterRequest struct {
	Email    string `json:"email"     validate:"required,email"`
	Password string `json:"password"  validate:"required,min=8"`
	FullName string `json:"full_name" validate:"required,min=2,max=255"`
	Phone    string `json:"phone"     validate:"omitempty,max=20"`
}

// loginrequest запрос на логин
type LoginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// Authresponse ответ при успешном логине
type AuthResponse struct {
	AccessToken string   `json:"access_token"`
	ExpiresIn   int      `json:"expires_in"`
	User        UserInfo `json:"user"`
}

//userinfo публичная информация о пользователе
type UserInfo struct {
	ID       uuid.UUID  `json:"id"`
	Email    string     `json:"email"`
	FullName string     `json:"full_name"`
	Role     model.Role `json:"role"`
}

// Updaterolerequest запрос на изменение роли admin
type UpdateRoleRequest struct {
	Role model.Role `json:"role" validate:"required,oneof=unverified passenger driver admin"`
}
