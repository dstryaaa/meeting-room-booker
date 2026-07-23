package models

import "time"

type User struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FullName     string    `json:"full_name"`
	CreatedAt    time.Time `json:"created_at"`
}

type Room struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Capacity      int    `json:"capacity"`
	Description   string `json:"description"`
	HasProjector  bool   `json:"has_projector"`
	HasWhiteboard bool   `json:"has_whiteboard"`
	IsActive      bool   `json:"is_active"`
}

type Booking struct {
	ID          int       `json:"id"`
	UserID      int       `json:"user_id"`
	RoomID      int       `json:"room_id"`
	BookingDate string    `json:"booking_date"`
	StartTime   string    `json:"start_time"`
	EndTime     string    `json:"end_time"`
	Purpose     string    `json:"purpose"`
	CreatedAt   time.Time `json:"created_at"`
}

// DTO (Data Transfer Object) для регистрации
// Это не модель БД, а структура для HTTP запроса
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

// DTO для логина
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Ответ с токеном - возвращается после успешной регистрации или логина
type AuthResponse struct {
	Token    string `json:"token"`
	UserID   int    `json:"user_id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
}

// DTO для создания бронирования
// Это то, что приходит от клиента
type CreateBookingRequest struct {
	RoomID      int    `json:"room_id"`
	BookingDate string `json:"booking_date"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	Purpose     string `json:"purpose"`
}

// Слот времени для календаря
// Используется для отображения расписания на день
type TimeSlot struct {
	StartTime string `json:"srart_time"`
	EndTime   string `json:"end_time"`
	IsBooked  bool   `json:"is_booked"`
	BookingID *int   `json:"booking_id,omitempty"`
	UserID    *int   `json:"user_id,omitempty"`
	Purpose   string `json:"purpose,omitempty"`
}

// Расписание на день для одной комнаты
type DaySchedule struct {
	Date  string     `json:"date"`
	Slots []TimeSlot `json:"slots"`
}

// Структуры для пагинации и фильтров

// PaginationRequest - параметры пагинации
type PaginationRequest struct {
	Page  int `json:"page"`  // Номер страницы (начинается с 1)
	Limit int `json:"limit"` // Количество записей на странице
}

// PaginationResponse - ответ с пагинацией
type PaginationResponce struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`       // Всего записей
	TotalPages int `json:"total_pages"` // Всего страниц
}

// BookingFilter - фильтры для бронирований
type BookingFilter struct {
	RoomID   *int    `json:"room_id"`   // ID комнаты (если указан)
	DateFrom *string `json:"date_from"` // Дата "не раньше" (YYYY-MM-DD)
	DateTo   *string `json:"date_to"`   // Дата "не позже" (YYYY-MM-DD)
	Status   *string `json:"status"`    // Статус (например "active", "past")
}
