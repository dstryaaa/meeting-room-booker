package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/dstryaaa/meeting-room-booker/internal/middleware"
	"github.com/dstryaaa/meeting-room-booker/internal/models"
	"github.com/dstryaaa/meeting-room-booker/internal/service"
)

type BookingHandler struct {
	bookingService *service.BookingService
}

func NewBookingHandler(bookingService *service.BookingService) *BookingHandler {
	return &BookingHandler{bookingService: bookingService}
}

// CreateBooking - POST /api/bookings
func (h *BookingHandler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	// Получаем ID пользователя из контекста (добавлен middleware)
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Декодируем запрос
	var req models.CreateBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	// Валидация
	if req.RoomID == 0 || req.BookingDate == "" || req.StartTime == "" || req.EndTime == "" || req.Purpose == "" {
		http.Error(w, "Все поля обязательны", http.StatusBadRequest)
		return
	}

	// Создаем бронирование
	booking, err := h.bookingService.CreateBooking(r.Context(), userID, &req)
	if err != nil {
		// Проверяем, ошибка ли это "уже забронировано"
		if err.Error() == "this time slot is already booked" {
			http.Error(w, err.Error(), http.StatusConflict) // 409 Conflict
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(booking)
}

// GetSchedule - GET /api/rooms/{room_id}/schedule?date=2024-01-15
func (h *BookingHandler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	// Получаем room_id из URL
	// Пример: /api/rooms/5/schedule
	path := r.URL.Path
	var roomID int
	_, err := fmt.Sscanf(path, "/api/rooms/%d/schedule", &roomID)
	if err != nil {
		http.Error(w, "Неверный ID комнаты", http.StatusBadRequest)
		return
	}
	// Получаем дату из query параметра
	date := r.URL.Query().Get("date")
	if date == "" {
		http.Error(w, "Параметр date обязателен (YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	schedule, err := h.bookingService.GetDaySchedule(r.Context(), roomID, date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schedule)
}

// GetMyBookings - GET /api/bookings
func (h *BookingHandler) GetMyBookings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	// 1. Получаем ID пользователя из контекста
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Парсим параметры пагинации из query-строки
	pagination := &models.PaginationRequest{
		Page:  1,
		Limit: 10,
	}

	// page - номер страницы
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			pagination.Page = page
		}
	}

	// limit - количество записей на странице
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			pagination.Limit = limit
		}
	}

	filter := &models.BookingFilter{}

	// Фильтр по комнате
	if roomIDStr := r.URL.Query().Get("room_id"); roomIDStr != "" {
		if roomID, err := strconv.Atoi(roomIDStr); err == nil && roomID > 0 {
			filter.RoomID = &roomID
		}
	}

	// Фильтр по дате "не раньше"
	if dateFrom := r.URL.Query().Get("date_from"); dateFrom != "" {
		filter.DateFrom = &dateFrom
	}

	// Фильтр по дате "не позже"
	if dateTo := r.URL.Query().Get("date_to"); dateTo != "" {
		filter.DateTo = &dateTo
	}

	// Фильтр по статусу
	if status := r.URL.Query().Get("status"); status != "" {
		if status == "active" || status == "past" {
			filter.Status = &status
		}
	}

	bookings, paginationResp, err := h.bookingService.GetUserBookingsWithFilters(r.Context(), userID, filter, pagination)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"data":       bookings,
		"pagination": paginationResp,
	}
	json.NewEncoder(w).Encode(response)
}

// CancelBooking - DELETE /api/bookings/{id}
func (h *BookingHandler) CancelBooking(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Получаем booking_id из URL
	// Пример: /api/bookings/15
	path := r.URL.Path
	var bookingID int
	_, err := fmt.Sscanf(path, "/api/bookings/%d", &bookingID)
	if err != nil {
		http.Error(w, "Неверный ID бронирования", http.StatusBadRequest)
		return
	}

	if err := h.bookingService.CancelBooking(r.Context(), userID, bookingID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
