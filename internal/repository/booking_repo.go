package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dstryaaa/meeting-room-booker/internal/models"
)

type BookingRepository struct {
	db *sql.DB
}

func NewBookingrepository(db *sql.DB) *BookingRepository {
	return &BookingRepository{db: db}
}

// CreateWithLock - создает бронирование с использованием блокировки FOR UPDATE
// Это главный метод, который защищает от двойного бронирования
func (r *BookingRepository) CreateWithLock(ctx context.Context, booking *models.Booking) error {
	// 1. Начинаем транзакцию
	// Транзакция нужна, чтобы все операции были атомарными
	// Либо все выполнится, либо ничего
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction %w", err)
	}

	// 2. Важно! Если что-то пойдет не так, откатываем транзакцию
	// defer сработает при выходе из функции
	defer tx.Rollback()

	// 3. Проверяем, что слот свободен с БЛОКИРОВКОЙ строки
	// "FOR UPDATE" - это ключевая часть!
	// Когда мы выполняем SELECT ... FOR UPDATE:
	// - БД блокирует найденные строки
	// - Другие транзакции не могут изменить эти строки
	// - Блокировка снимается только после COMMIT или ROLLBACK
	checkquery := `
		select id from bookings
		where room_id = $1 and booking_date = $2 and start_time = $3
		for update
	`

	var existingID int
	err = tx.QueryRowContext(ctx, checkquery,
		booking.RoomID,
		booking.BookingDate,
		booking.StartTime,
	).Scan(&existingID)

	// 4. Анализируем результат проверки
	if err != nil && err != sql.ErrNoRows {
		// Если ошибка не "нет строк", значит что-то пошло не так
		return fmt.Errorf("Failed to check availability: %w", err)
	}

	if err == nil {
		// Если мы нашли запись, значит слот уже занят
		// Возвращаем ошибку, транзакция откатится (defer Rollback)
		return fmt.Errorf("this time slot is already booked")
	}

	// 5. Создаем бронирование
	insertQuery := `
		insert into bookings (user_id, room_id, booking_date, start_time, end_time, purpose)
		values ($1, $2, $3, $4, $5, $6)
		returning id, created_at
	`
	err = tx.QueryRowContext(
		ctx,
		insertQuery,
		booking.UserID,
		booking.RoomID,
		booking.BookingDate,
		booking.StartTime,
		booking.EndTime,
		booking.Purpose,
	).Scan(&booking.ID, &booking.CreatedAt)

	if err != nil {
		return fmt.Errorf("Failed to create booking: %w", err)
	}

	// 6. Фиксируем транзакцию
	// Только после COMMIT блокировка снимается и изменения становятся видимыми
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("Failed to commit transaction: %w", err)
	}

	return nil
}

// GetDaySchedule - получает расписание на день для комнаты
func (r *BookingRepository) GetDaySchedule(ctx context.Context, roomID int, date string) ([]models.TimeSlot, error) {
	// 1. Генерируем все возможные слоты с 09:00 до 18:00
	// Это 18 слотов по 30 минут (9:00, 9:30, 10:00, ... 17:30)
	var allSlots []models.TimeSlot
	startHour, endHour := 9, 18

	for hour := startHour; hour < endHour; hour++ {
		for minute := 0; minute < 60; minute += 30 {
			// Форматируем время начала
			startTime := fmt.Sprintf("%02d:%02d:00", hour, minute)

			// Вычисляем время окончания (+30 минут)
			endMinute := minute + 30
			endHourTemp := hour
			if endMinute >= 60 {
				endHourTemp = hour + 1
				endMinute = 0
			}
			endTime := fmt.Sprintf("%02d:%02d:00", endHourTemp, endMinute)

			// Добавляем слот (пока все свободны)
			allSlots = append(allSlots, models.TimeSlot{
				StartTime: startTime[:5],
				EndTime:   endTime[:5],
				IsBooked:  false,
			})
		}
	}
	// 2. Получаем все бронирования на этот день для комнаты
	query := `
		select id, user_id, start_time, end_time, purpose
		from bookings
		where room_id = $1 and booking_date = $2
		order by start_time
	`

	rows, err := r.db.QueryContext(ctx, query, roomID, date)
	if err != nil {
		return nil, fmt.Errorf("Failde to get bookings: %w", err)
	}
	defer rows.Close()

	// 3. Создаем карту для быстрого поиска забронированных слотов
	// Ключ - время начала (например "10:00")
	bookedSlots := make(map[string]models.TimeSlot)
	for rows.Next() {
		var slot models.TimeSlot
		var bookingID int
		var userID int
		var startTime, endTime, purpose string

		err := rows.Scan(&bookingID, &userID, &startTime, &endTime, &purpose)
		if err != nil {
			return nil, err
		}

		// Сохраняем только время (без секунд)
		slot = models.TimeSlot{
			StartTime: startTime[:5],
			EndTime:   endTime[:5],
			IsBooked:  true,
			BookingID: &bookingID,
			UserID:    &userID,
			Purpose:   purpose,
		}
		bookedSlots[startTime[:5]] = slot
	}

	// 4. Объединяем все слоты с бронированиями
	// Проходим по всем возможным слотам и заменяем забронированные
	for i := range allSlots {
		if booked, exists := bookedSlots[allSlots[i].StartTime]; exists {
			allSlots[i] = booked
		}
	}

	return allSlots, nil
}

// GetUserBookings - получает все бронирования пользователя
func (r *BookingRepository) GetUserBookings(ctx context.Context, userID int) ([]models.Booking, error) {
	query := `
		SELECT id, user_id, room_id, booking_date, start_time, end_time, purpose, created_at
		FROM bookings
		WHERE user_id = $1
		ORDER BY booking_date DESC, start_time DESC
		LIMIT 50
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []models.Booking
	for rows.Next() {
		var booking models.Booking
		var bookingDate time.Time
		var startTime, endTime string

		err := rows.Scan(
			&booking.ID,
			&booking.UserID,
			&booking.RoomID,
			&bookingDate,
			&startTime,
			&endTime,
			&booking.Purpose,
			&booking.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Форматируем дату
		booking.BookingDate = bookingDate.Format("2006-01-02")

		// Обрезаем секунды из времени (было "10:00:00" → стало "10:00")
		if len(startTime) >= 5 {
			booking.StartTime = startTime[:5]
		} else {
			booking.StartTime = startTime
		}
		if len(endTime) >= 5 {
			booking.EndTime = endTime[:5]
		} else {
			booking.EndTime = endTime
		}

		bookings = append(bookings, booking)
	}

	return bookings, nil
}

// Cancel - отменяет бронирование
// Проверяет, что бронирование принадлежит пользователю
func (r *BookingRepository) Cancel(ctx context.Context, userID, bookingID int) error {
	query := `
		delete from bookings
		where id = $1 and user_id = $2
	`

	result, err := r.db.ExecContext(ctx, query, bookingID, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	// Если ничего не удалилось - либо нет такого бронирования,
	// либо оно принадлежит другому пользователю
	if rowsAffected == 0 {
		return fmt.Errorf("booking not found or you don't have permission to cancel it")
	}

	return nil
}

// GetUserBookingsWithFilters - получает бронирования пользователя с пагинацией и фильтрами
func (r *BookingRepository) GetUserBookingswithFilters(
	ctx context.Context,
	userID int,
	filter *models.BookingFilter,
	pagination *models.PaginationRequest,
) ([]models.Booking, int, error) {
	baseQuery := `
		from bookings
		where user_id = $1
	`

	// 2. Счетчик аргументов для SQL (нужен для построения запроса с динамическими фильтрами)
	args := []interface{}{userID}
	argIndex := 2

	// 3. Добавляем фильтры (если они указаны)
	if filter != nil {
		// Фильтр по комнате
		if filter.RoomID != nil {
			baseQuery += fmt.Sprintf((" and room_id = $%d"), argIndex)
			args = append(args, *filter.RoomID)
			argIndex++
		}

		// Фильтр по дате "не раньше"
		if filter.DateFrom != nil && *filter.DateFrom != "" {
			baseQuery += fmt.Sprintf(" and booking_date >= $%d", argIndex)
			args = append(args, *filter.DateFrom)
			argIndex++
		}

		// Фильтр по дате "не позже"
		if filter.DateTo != nil && *filter.DateTo != "" {
			baseQuery += fmt.Sprintf(" and booking_date <= $%d", argIndex)
			args = append(args, *filter.DateTo)
			argIndex++
		}

		// Фильтр по статусу
		if filter.Status != nil {
			switch *filter.Status {
			case "active":
				// Активные бронирования (сегодня или в будущем)
				today := time.Now().Format("2006-01-02")
				baseQuery += fmt.Sprintf("and booking_date >= $%d", argIndex)
				args = append(args, today)
				argIndex++
			case "past":
				// Прошедшие бронирования
				today := time.Now().Format("2006-01-02")
				baseQuery += fmt.Sprintf("and booking_date <= $%d", argIndex)
				args = append(args, today)
				argIndex++
			}
		}
	}

	// 4. Считаем общее количество записей (без пагинации)
	countQuery := `select count(*) ` + baseQuery
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count bookings: %w", err)
	}

	if total == 0 {
		return []models.Booking{}, 0, nil
	}

	// 5. Добавляем пагинацию
	// По умолчанию: страница 1, лимит 10
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

	offset := (page - 1) * limit

	// 6. Добавляем ORDER BY, LIMIT и OFFSET к запросу
	query := `
		SELECT id, user_id, room_id, booking_date, start_time, end_time, purpose, created_at
	` + baseQuery + `
		order by booking_date desc, start_time desc
		limit $` + fmt.Sprintf("%d", argIndex) + ` offset $` + fmt.Sprintf("%d", argIndex+1)

	args = append(args, limit, offset)

	// 7. Выполняем запрос
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query bookings: %w", err)
	}
	defer rows.Close()

	// 8. Парсим результаты
	var bookings []models.Booking
	for rows.Next() {
		var booking models.Booking
		var bookingDate time.Time
		var startTime, endTime string

		err := rows.Scan(
			&booking.ID,
			&booking.UserID,
			&booking.RoomID,
			&bookingDate,
			&startTime,
			&endTime,
			&booking.Purpose,
			&booking.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		booking.BookingDate = bookingDate.Format("2006-01-02")
		booking.StartTime = startTime[:5]
		booking.EndTime = endTime[:5]

		bookings = append(bookings, booking)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return bookings, total, nil
}
