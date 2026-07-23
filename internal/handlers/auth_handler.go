package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/dstryaaa/meeting-room-booker/internal/models"
	"github.com/dstryaaa/meeting-room-booker/internal/service"
)

// AuthHandler - HTTP слой для аутентификации
// Отвечает за прием запросов, валидацию и отправку ответов
type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Register - обработчик POST /api/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	// Проверяем метод
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	// Декодируем JSON из тела запроса
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	// Базовая валидация (более сложную можно добавить позже)
	if req.Email == "" || req.Password == "" || req.FullName == "" {
		http.Error(w, "Все поля обязательны", http.StatusBadRequest)
		return
	}

	// Вызываем бизнес-логику
	resp, err := h.authService.Register(r.Context(), &req)
	if err != nil {
		// Разные ошибки - разные HTTP статусы
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Успешный ответ с кодом 201 Created
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// Login - обработчик POST /api/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	var req models.LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email и пароль обязательны", http.StatusBadRequest)
		return
	}

	resp, err := h.authService.Login(r.Context(), &req)
	if err != nil {
		// Неверные данные - 401 Unauthorized
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
