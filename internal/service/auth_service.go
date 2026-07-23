package service

import (
	"context"
	"errors"
	"strings"

	"github.com/dstryaaa/meeting-room-booker/internal/models"
	"github.com/dstryaaa/meeting-room-booker/internal/repository"
	"github.com/dstryaaa/meeting-room-booker/internal/utils"
	"golang.org/x/crypto/bcrypt"
)

// AuthService - слой бизнес-логики для аутентификации
// Здесь мы описываем ЧТО делаем, а не КАК (это делает repository)
type AuthService struct {
	userRepo *repository.UserRepository
}

func NewAuthService(userRepo *repository.UserRepository) *AuthService {
	return &AuthService{userRepo: userRepo}
}

// Register - регистрация нового пользователя
func (s *AuthService) Register(ctx context.Context, req *models.RegisterRequest) (*models.AuthResponse, error) {
	// ШАГ 1: Проверяем, что email не занят
	// Приводим к нижнему регистру, чтобы "User@example.com" и "user@example.com" считались одинаковыми
	email := strings.ToLower(req.Email)
	existing, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("user with this email is already exists")
	}

	// ШАГ 2: Хешируем пароль
	// bcrypt.GenerateFromPassword создает хеш с солью (salt)
	// DefaultCost = 10 - это компромисс между безопасностью и скоростью
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// ШАГ 3: Создаем пользователя в БД
	user := &models.User{
		Email:        email,
		PasswordHash: string(hashedPassword),
		FullName:     req.FullName,
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// ШАГ 4: Генерируем JWT токен
	// Токен будет использоваться для всех последующих запросов
	token, err := utils.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	// Возвращаем ответ с токеном и данными пользователя
	return &models.AuthResponse{
		Token:    token,
		UserID:   user.ID,
		Email:    user.Email,
		FullName: user.FullName,
	}, nil
}

// Login - вход пользователя

func (s *AuthService) Login(ctx context.Context, req *models.LoginRequest) (*models.AuthResponse, error) {
	email := strings.ToLower(req.Email)
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("invalid credentials")
	}

	// ШАГ 2: Проверяем пароль
	// bcrypt.CompareHashAndPassword сравнивает хеш из БД с введенным паролем
	// Самое важное: мы НЕ расшифровываем хеш, а хешируем введенный пароль и сравниваем
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	// ШАГ 3: Генерируем токен
	token, err := utils.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		Token:    token,
		UserID:   user.ID,
		Email:    user.Email,
		FullName: user.FullName,
	}, nil
}
