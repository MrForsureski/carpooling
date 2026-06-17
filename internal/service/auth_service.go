package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"office_trip/internal/config"
	"office_trip/internal/dto"
	"office_trip/internal/model"
	"office_trip/internal/repository"
)

type AuthService struct {
	userRepo repository.UserRepository
	cfg      *config.Config
}

func NewAuthService(userRepo repository.UserRepository, cfg *config.Config) *AuthService {
	return &AuthService{userRepo: userRepo, cfg: cfg}
}

//register создание нового пользователя
func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest) (*model.User, error) {
	// извлечение домена из email
	parts := strings.Split(req.Email, "@")
	if len(parts) != 2 {
		return nil, model.ErrEmailDomainNotAllowed
	}
	domain := parts[1]

	// проверка домена на доступность
	company, err := s.userRepo.GetCompanyByEmailDomain(ctx, domain)
	if err != nil {
		return nil, model.ErrEmailDomainNotAllowed
	}

	//проверка существования пользователя
	if _, err := s.userRepo.GetByEmail(ctx, req.Email); err == nil {
		return nil, model.ErrEmailAlreadyExists
	}

	//Хэширование пароля
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, err
	}

	// Генерация verify token
	verifyToken, err := generateToken(32)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user := &model.User{
		ID:            uuid.New(),
		CompanyID:     company.ID,
		Email:         req.Email,
		PasswordHash:  string(hash),
		FullName:      req.FullName,
		Role:          model.RoleUnverified,
		EmailVerified: false,
		VerifyToken:   &verifyToken,
		VerifyTokenAt: &now,
		IsActive:      true,
	}
	if req.Phone != "" {
		user.Phone = &req.Phone
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

//Login проверка учетных данных и выдача токенов
func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, string, error) {
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, "", model.ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, "", model.ErrInvalidCredentials
	}

	if s.cfg.RequireEmailVerification && !user.EmailVerified {
		return nil, "", model.ErrEmailNotVerified
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, "", model.ErrInvalidCredentials
	}

	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, "", err
	}

	refreshToken, err := generateToken(32)
	if err != nil {
		return nil, "", err
	}

	//хэширование refresh token для хранения
	refreshHash, err := bcrypt.GenerateFromPassword([]byte(refreshToken), 10)
	if err != nil {
		return nil, "", err
	}
	hashStr := string(refreshHash)
	now := time.Now()
	if err := s.userRepo.UpdateRefreshToken(ctx, user.ID, &hashStr, &now); err != nil {
		return nil, "", err
	}

	return &dto.AuthResponse{
		AccessToken: accessToken,
		ExpiresIn:   int(s.cfg.JWTAccessExpiry.Seconds()),
		User: dto.UserInfo{
			ID:       user.ID,
			Email:    user.Email,
			FullName: user.FullName,
			Role:     user.Role,
		},
	}, refreshToken, nil
}

// verifyemail подтверждение email по токену
func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	user, err := s.userRepo.GetByVerifyToken(ctx, token)
	if err != nil {
		return model.ErrInvalidToken
	}

	//Проверка срока действия токена
	if user.VerifyTokenAt != nil && time.Since(*user.VerifyTokenAt) > 24*time.Hour {
		return model.ErrInvalidToken
	}

	return s.userRepo.UpdateEmailVerified(ctx, user.ID)
}

//logout инвалидация refresh token
func (s *AuthService) Logout(ctx context.Context, userID uuid.UUID) error {
	return s.userRepo.UpdateRefreshToken(ctx, userID, nil, nil)
}

// Refreshaccesstoken обновление access token через refresh token
func (s *AuthService) RefreshAccessToken(ctx context.Context, userID uuid.UUID, refreshToken string) (string, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", model.ErrUserNotFound
	}

	if user.RefreshToken == nil {
		return "", model.ErrInvalidToken
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.RefreshToken), []byte(refreshToken)); err != nil {
		return "", model.ErrInvalidToken
	}

	// проверка срока refresh token
	if user.RefreshTokenAt != nil && time.Since(*user.RefreshTokenAt) > s.cfg.JWTRefreshExpiry {
		return "", model.ErrInvalidToken
	}

	return s.generateAccessToken(user)
}

//validateaccesstoken проверка jwt и получение claims
func (s *AuthService) ValidateAccessToken(tokenStr string) (uuid.UUID, model.Role, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, model.ErrInvalidToken
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return uuid.Nil, "", model.ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, "", model.ErrInvalidToken
	}

	subStr, _ := claims["sub"].(string)
	userID, err := uuid.Parse(subStr)
	if err != nil {
		return uuid.Nil, "", model.ErrInvalidToken
	}

	role, _ := claims["role"].(string)
	return userID, model.Role(role), nil
}

func (s *AuthService) generateAccessToken(user *model.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":  user.ID.String(),
		"role": string(user.Role),
		"exp":  time.Now().Add(s.cfg.JWTAccessExpiry).Unix(),
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

// generatetoken генерация случайного hex токена
func generateToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

//quickverifyfordev немедленная верификация пользователя для разработки
func (s *AuthService) QuickVerifyForDev(ctx context.Context, userID uuid.UUID) error {
	return s.userRepo.UpdateEmailVerified(ctx, userID)
}

//getuserbyid получение пользователя по id
func (s *AuthService) GetUserByID(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}

func (s *AuthService) ListUsers(ctx context.Context, limit, offset int) ([]*model.User, int, error) {
	return s.userRepo.List(ctx, limit, offset)
}

func (s *AuthService) UpdateRole(ctx context.Context, userID uuid.UUID, role model.Role) error {
	return s.userRepo.UpdateRole(ctx, userID, role)
}

//getverifytoken получение verify token пользователя
func (s *AuthService) GetVerifyToken(user *model.User) string {
	if user.VerifyToken != nil {
		return *user.VerifyToken
	}
	return ""
}

// Geterrors получение списка ошибок
func (s *AuthService) IsEmailDomainError(err error) bool {
	return errors.Is(err, model.ErrEmailDomainNotAllowed)
}
