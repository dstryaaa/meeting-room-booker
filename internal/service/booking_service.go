package service

import (
	"context"
	"errors"
	"time"

	"github.com/dstryaaa/meeting-room-booker/internal/models"
	"github.com/dstryaaa/meeting-room-booker/internal/repository"
)

type BookingService struct {
	bookingRepo *repository.BookingRepository
	roomRepo    *repository.RoomRepository
}

func NewBookingService(bookingRepo *repository.BookingRepository, roomRepo *repository.RoomRepository) *BookingService {
	return &BookingService{bookingRepo: bookingRepo, roomRepo: roomRepo}
}

// CreateBooking - создает новое бронирование
func (s *BookingService) CreateBooking(ctx context.Context, userID int, req *models.CreateBookingRequest) (*models.Booking, error) {
	// 1. Проверяем, существует ли комната
	room, err := s.roomRepo.GetByID(ctx, req.RoomID)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, errors.New("room not found")
	}

	// 2. Проверяем, что дата не в прошлом
	bookingDate, err := time.Parse("2006-01-02", req.BookingDate)
	if err != nil {
		return nil, errors.New("invalid date format, use YYYY-MM-DD")
	}
	today := time.Now().Format("006-01-02")
	if bookingDate.Format("2006-01-02") < today {
		return nil, errors.New("cannot book in the past")
	}

	// 3. Проверяем, что время в рабочем диапазоне (09:00 - 18:00)
	startTime, err := time.Parse("15:04", req.StartTime)
	if err != nil {
		return nil, errors.New("invalid start time format, use HH:MM")
	}
	endTime, err := time.Parse("15:04", req.EndTime)
	if err != nil {
		return nil, errors.New("invalid end time format, use HH:MM")
	}

	// Проверяем часы
	if startTime.Hour() < 9 || startTime.Hour() >= 18 {
		return nil, errors.New("booking must be between 09:00 and 18:00")
	}
	if endTime.Hour() < 9 || endTime.Hour() > 18 {
		return nil, errors.New("booking must be between 09:00 and 18:00")
	}

	// Проверяем, что начало раньше конца
	if startTime.After(endTime) || startTime.Equal(endTime) {
		return nil, errors.New("start time must be before end time")
	}

	// 4. Проверяем, что слот 30-минутный
	// Мы разрешаем бронировать только получасовые слоты
	diff := endTime.Sub(startTime)
	if diff.Minutes() != 30 {
		return nil, errors.New("slot must be exactly 30 minutes")
	}

	// 5. Создаем бронирование
	booking := &models.Booking{
		UserID:      userID,
		RoomID:      req.RoomID,
		BookingDate: req.BookingDate,
		StartTime:   req.StartTime + ":00", // Добавляем секунды для БД
		EndTime:     req.EndTime + ":00",
		Purpose:     req.Purpose,
	}

	// Repository сам обработает race condition через FOR UPDATE
	if err := s.bookingRepo.CreateWithLock(ctx, booking); err != nil {
		return nil, err
	}

	return booking, nil
}

// GetDaySchedule - получает расписание на день
func (s *BookingService) GetDaySchedule(ctx context.Context, roomID int, date string) (*models.DaySchedule, error) {
	// Проверяем, что комната существует
	room, err := s.roomRepo.GetByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, errors.New("room not found")
	}

	// Проверяем формат даты
	_, err = time.Parse("2006-01-02", date)
	if err != nil {
		return nil, errors.New("invalid date format, use YYYY-MM-DD")
	}

	slots, err := s.bookingRepo.GetDaySchedule(ctx, roomID, date)
	if err != nil {
		return nil, err
	}

	return &models.DaySchedule{
		Date:  date,
		Slots: slots,
	}, nil
}

// GetUserBookings - получает все бронирования пользователя
func (s *BookingService) GetUserBookings(ctx context.Context, userID int) ([]models.Booking, error) {
	return s.bookingRepo.GetUserBookings(ctx, userID)
}

// CancelBooking - отменяет бронирование
func (s *BookingService) CancelBooking(ctx context.Context, userID, bookingID int) error {
	return s.bookingRepo.Cancel(ctx, userID, bookingID)
}

// GetUserBookingsWithFilters - получает бронирования пользователя с пагинацией и фильтрами
func (s *BookingService) GetUserBookingsWithFilters(
	ctx context.Context,
	userID int,
	filter *models.BookingFilter,
	pagination *models.PaginationRequest,
) ([]models.Booking, *models.PaginationResponce, error) {
	// Получаем данные из репозитория
	bookings, total, err := s.bookingRepo.GetUserBookingswithFilters(ctx, userID, filter, pagination)
	if err != nil {
		return nil, nil, err
	}

	// Определяем значения по умолчанию
	page := 1
	limit := 10
	if pagination != nil {
		if pagination.Page > 0 {
			page = pagination.Page
		}
		if pagination.Limit > 0 && pagination.Limit < 100 {
			limit = pagination.Limit
		}
	}

	// Рассчитываем количество страниц
	totalPages := (total + limit - 1) / limit

	paginationResp := &models.PaginationResponce{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}

	return bookings, paginationResp, nil
}
