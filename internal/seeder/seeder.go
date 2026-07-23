package seeder

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/dstryaaa/meeting-room-booker/internal/models"
	"github.com/dstryaaa/meeting-room-booker/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// Seeder - структура для заполнения тестовыми данными
type Seeder struct {
	db          *sql.DB
	userRepo    *repository.UserRepository
	roomRepo    *repository.RoomRepository
	bookingRepo *repository.BookingRepository
}

// NewSeeder - конструктор
func NewSeeder(
	db *sql.DB,
	userRepo *repository.UserRepository,
	roomRepo *repository.RoomRepository,
	bookingRepo *repository.BookingRepository,
) *Seeder {
	return &Seeder{
		db:          db,
		userRepo:    userRepo,
		roomRepo:    roomRepo,
		bookingRepo: bookingRepo,
	}
}

// IsEmpty - проверяет, есть ли уже данные в БД
// Если есть хотя бы один пользователь или бронирование - считаем, что данные уже есть
func (s *Seeder) IsEmpty(ctx context.Context) (bool, error) {
	var count int

	// Проверяем, есть ли пользователи
	err := s.db.QueryRowContext(ctx, "select count(*) from users").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("Failde to check users count: %w", err)
	}
	if count > 0 {
		return false, nil
	}

	// Проверяем, есть ли бронирования
	err = s.db.QueryRowContext(ctx, "select count(*) from bookings").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("Failde to check bookings count: %w", err)
	}
	if count > 0 {
		return false, nil
	}

	return true, nil
}

// SeedRooms - создает расширенный набор комнат
func (s *Seeder) SeedRooms(ctx context.Context) error {
	rooms := []struct {
		name        string
		capacity    int
		description string
		projector   bool
		whiteboard  bool
	}{
		// Базовые комнаты
		{"Conference A", 12, "Большая переговорная с проектором", true, true},
		{"Conference B", 8, "Средняя комната для встреч", true, false},
		{"Conference C", 6, "Малая переговорная", false, true},
		{"Коворкинг", 20, "Открытое пространство для работы", false, false},

		// Дополнительные комнаты для разнообразия
		{"Executive Room", 4, "VIP переговорная для руководства", true, true},
		{"Training Room", 16, "Комната для тренингов и презентаций", true, true},
		{"Focus Room", 2, "Маленькая комната для концентрации", false, false},
		{"Meeting Pod", 4, "Звукоизолированная капсула", false, true},
	}

	for _, room := range rooms {
		// Проверяем, существует ли уже комната
		var exists bool
		err := s.db.QueryRowContext(ctx, "select exists(select 1 from rooms where name = $1)", room.name).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check room existence: %w", err)
		}

		if !exists {
			_, err := s.db.ExecContext(ctx, `
				INSERT INTO rooms (name, capacity, description, has_projector, has_whiteboard)
				VALUES ($1, $2, $3, $4, $5)
			`, room.name, room.capacity, room.description, room.projector, room.whiteboard)
			if err != nil {
				return fmt.Errorf("failed to create room %s: %w", room.name, err)
			}
			log.Printf("✅ Добавлена комната: %s", room.name)
		}
	}
	return nil
}

// SeedUsers - создает тестовых пользователей
func (s *Seeder) SeedUsers(ctx context.Context) error {
	users := []struct {
		email    string
		password string
		fullName string
	}{
		{"alice@example.com", "password123", "Алиса Иванова"},
		{"bob@example.com", "password123", "Боб Смит"},
		{"charlie@example.com", "password123", "Чарли Браун"},
		{"diana@example.com", "password123", "Диана Принс"},
	}

	for _, user := range users {
		// Проверяем, существует ли пользователь
		existing, err := s.userRepo.FindByEmail(ctx, user.email)
		if err != nil {
			return fmt.Errorf("failed to check user existence: %w", err)
		}

		if existing != nil {
			continue
		}

		// Хешируем пароль
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}

		// Создаем пользователя
		newUser := &models.User{
			Email:        user.email,
			PasswordHash: string(hashedPassword),
			FullName:     user.fullName,
		}

		if err := s.userRepo.Create(ctx, newUser); err != nil {
			return fmt.Errorf("failed to create user %s: %w", user.email, err)
		}
		log.Printf("✅ Добавлен пользователь: %s (%s)", user.fullName, user.email)
	}
	return nil
}

// SeedBookings - создает тестовые бронирования
func (s *Seeder) SeedBookings(ctx context.Context) error {
	// 1. Получаем всех пользователей
	users, err := s.getAllUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get users: %w", err)
	}
	if len(users) == 0 {
		return fmt.Errorf("no users found, run SeedUsers first")
	}

	// 2. Получаем все комнаты
	rooms, err := s.roomRepo.GetAllActive(ctx)
	if err != nil {
		return fmt.Errorf("failed to get rooms: %w", err)
	}
	if len(rooms) == 0 {
		return fmt.Errorf("no rooms found, run SeedRooms first")
	}

	// 3. Генерируем бронирования на ближайшие дни
	now := time.Now()

	// Даты: сегодня, завтра, послезавтра, через 3 дня, через 7 дней
	dates := []time.Time{
		now,
		now.AddDate(0, 0, 1),
		now.AddDate(0, 0, 2),
		now.AddDate(0, 0, 3),
		now.AddDate(0, 0, 7),
	}

	// Временные слоты (30-минутные интервалы)
	timeSlots := []string{
		"09:00", "09:30", "10:00", "10:30",
		"11:00", "11:30", "12:00", "12:30",
		"13:00", "13:30", "14:00", "14:30",
		"15:00", "15:30", "16:00", "16:30",
		"17:00", "17:30",
	}

	purposes := []string{
		"Еженедельная встреча команды",
		"Планирование спринта",
		"Демо для клиента",
		"Ретроспектива",
		"Интервью с кандидатом",
		"Работа над проектом",
		"Обсуждение дизайна",
		"Code review сессия",
	}

	// Создаем бронирования (не больше 20, чтобы не перегружать БД)
	bookingsCreated := 0
	maxBookings := 20

	for _, date := range dates {
		// На каждой дате создаем 2-4 бронирования
		bookingsPerDay := 2 + bookingsCreated%3

		for i := 0; i < bookingsPerDay && bookingsCreated < maxBookings; i++ {
			// Выбираем случайного пользователя
			user := users[i%len(users)]

			// Выбираем случайную комнату
			room := rooms[i%len(rooms)]

			// Выбираем случайное время
			timeIdx := (i*3 + bookingsCreated) % len(timeSlots)
			startTime := timeSlots[timeIdx]

			// Конец времени = +30 минут
			endTime := s.addMinutes(startTime, 30)

			// Выбираем случайную цель
			purpose := purposes[bookingsCreated%len(purposes)]

			// Проверяем, не забронировано ли уже это время
			exists, err := s.bookingExists(ctx, room.ID, date.Format("2006-01-02"), startTime)
			if err != nil {
				log.Printf("⚠️ Ошибка проверки бронирования: %v", err)
				continue
			}
			if exists {
				continue
			}

			// Создаем бронирование
			booking := &models.Booking{
				UserID:      user.ID,
				RoomID:      room.ID,
				BookingDate: date.Format("2006-01-02"),
				StartTime:   startTime + ":00",
				EndTime:     endTime + ":00",
				Purpose:     purpose,
			}

			if err := s.bookingRepo.CreateWithLock(ctx, booking); err != nil {
				// Если ошибка "уже забронировано" - просто пропускаем
				if err.Error() == "this time slot is already booked" {
					continue
				}
				log.Printf("⚠️ Ошибка создания бронирования: %v", err)
				continue
			}

			log.Printf("✅ Создано бронирование: %s забронировал %s в %s на %s",
				user.FullName, room.Name, startTime, date.Format("2006-01-02"))
			bookingsCreated++
		}
	}

	log.Printf("✅ Всего создано бронирований: %d", bookingsCreated)
	return nil
}

// addMinutes - добавляет минуты к строке времени
func (s *Seeder) addMinutes(timeStr string, minutes int) string {
	t, _ := time.Parse("15:04", timeStr)
	t = t.Add(time.Duration(minutes) * time.Minute)
	return t.Format("15:04")
}

// bookingExists - проверяет, существует ли бронирование на этот слот
func (s *Seeder) bookingExists(ctx context.Context, roomID int, date, startTime string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM bookings 
			WHERE room_id = $1 AND booking_date = $2 AND start_time = $3
		)
	`, roomID, date, startTime).Scan(&exists)
	return exists, err
}

// getAllUsers - получает всех пользователей из БД
func (s *Seeder) getAllUsers(ctx context.Context) ([]models.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, email, full_name, created_at FROM users ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(&user.ID, &user.Email, &user.FullName, &user.CreatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

// Run - запускает заполнение тестовыми данными
func (s *Seeder) Run(ctx context.Context) error {
	log.Println("🌱 Начинаем заполнение тестовыми данными...")

	// 1. Проверяем, есть ли уже данные
	empty, err := s.IsEmpty(ctx)
	if err != nil {
		return fmt.Errorf("failed to check if database is empty: %w", err)
	}

	if !empty {
		log.Println("ℹ️ База данных уже содержит данные, пропускаем seeding")
		return nil
	}

	// 2. Создаем комнаты
	log.Println("📋 Создание комнат...")
	if err := s.SeedRooms(ctx); err != nil {
		return fmt.Errorf("failed to seed rooms: %w", err)
	}

	// 3. Создаем пользователей
	log.Println("👤 Создание пользователей...")
	if err := s.SeedUsers(ctx); err != nil {
		return fmt.Errorf("failed to seed users: %w", err)
	}

	// 4. Создаем бронирования
	log.Println("📅 Создание бронирований...")
	if err := s.SeedBookings(ctx); err != nil {
		return fmt.Errorf("failed to seed bookings: %w", err)
	}

	log.Println("✅ Seeding завершен успешно!")
	return nil
}
