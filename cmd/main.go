package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dstryaaa/meeting-room-booker/internal/config"
	"github.com/dstryaaa/meeting-room-booker/internal/handlers"
	"github.com/dstryaaa/meeting-room-booker/internal/middleware"
	"github.com/dstryaaa/meeting-room-booker/internal/models"
	"github.com/dstryaaa/meeting-room-booker/internal/repository"
	"github.com/dstryaaa/meeting-room-booker/internal/seeder"
	"github.com/dstryaaa/meeting-room-booker/internal/service"
	"github.com/dstryaaa/meeting-room-booker/internal/utils"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	// 1. Загружаем конфиг
	cfg := config.Load()

	// 2. Инициализируем JWT секретом
	// Теперь все функции в utils/jwt.go могут работать
	utils.InitJWT(cfg.JWTSecret)

	// 3. Подключаемся к БД через pgx
	db, err := sql.Open("pgx", cfg.GetConnectionString())
	if err != nil {
		log.Fatalf("Ошибка подключенияя: %v", err)
	}
	defer db.Close()

	// 4. Проверяем подключение (с таймаутом)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		log.Fatalf("Не могу пинговать БД: %v", err)
	}

	fmt.Println("Подключение к PostgreSQL (pgx) успешно!")

	// 5. Выполняем миграцию (создаем таблицы)
	migrationSQL, err := os.ReadFile("migrations/001_init.up.sql")
	if err != nil {
		log.Fatalf("Не могу прочитать миграцию: %v", err)
	}

	_, err = db.ExecContext(ctx, string(migrationSQL))
	if err != nil {
		log.Printf("Ошибка выполнения миграции: %v", err)
		log.Println("Возможно, таблицы уже созданы")
	} else {
		fmt.Println("Миграция выполнена успешно!")
	}

	// 6. Инициализируем Repository слой
	// Repository знает только о БД
	userRepo := repository.NewUserRepository(db)
	roomRepo := repository.NewRoomRepository(db)
	bookingRepo := repository.NewBookingrepository(db)

	// Инициализация сеялки
	seeder := seeder.NewSeeder(db, userRepo, roomRepo, bookingRepo)

	// Заполняем тестовыми данными (если БД пустая)
	if err := seeder.Run(ctx); err != nil {
		log.Printf("⚠️ Ошибка заполнения тестовыми данными: %v", err)
	}

	// 7. Инициализируем Service слой
	// Service использует Repository
	authService := service.NewAuthService(userRepo)
	bookingService := service.NewBookingService(bookingRepo, roomRepo)

	// 8. Инициализируем Handler слой
	// Handler использует Service
	authHandler := handlers.NewAuthHandler(authService)
	bookingHandler := handlers.NewBookingHandler(bookingService)

	// 9. Создаем роутер
	mux := http.NewServeMux()

	// 10. Регистрируем эндпоинты

	// Health-check - для мониторинга
	mux.HandleFunc("/health", middleware.LoggingMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	}))

	// Комнаты - GET запрос без авторизации
	// (пока что любой может смотреть комнаты)
	mux.HandleFunc("/rooms", middleware.LoggingMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
			return
		}

		rows, err := db.QueryContext(r.Context(), `
			SELECT id, name, capacity, description, has_projector, has_whiteboard, is_active 
			FROM rooms WHERE is_active = true
		`)
		if err != nil {
			log.Printf("❌ Ошибка запроса к БД: %v", err)
			http.Error(w, "Ошибка БД", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var rooms []models.Room
		for rows.Next() {
			var room models.Room
			err := rows.Scan(
				&room.ID, &room.Name, &room.Capacity,
				&room.Description, &room.HasProjector,
				&room.HasWhiteboard, &room.IsActive,
			)
			if err != nil {
				log.Printf("❌ Ошибка сканирования: %v", err)
				http.Error(w, "ошибка чтения данных", http.StatusInternalServerError)
				return
			}
			rooms = append(rooms, room)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rooms)
	}))

	// Auth эндпоинты
	mux.HandleFunc("/api/register", middleware.LoggingMiddleware(authHandler.Register))
	mux.HandleFunc("/api/login", middleware.LoggingMiddleware(authHandler.Login))

	// Booking эндпоинты
	mux.HandleFunc("/api/bookings", func(w http.ResponseWriter, r *http.Request) {
		handler := middleware.LoggingMiddleware

		switch r.Method {
		case http.MethodGet:
			handler(middleware.AuthMiddleware(bookingHandler.GetMyBookings))(w, r)
		case http.MethodPost:
			handler(middleware.AuthMiddleware(bookingHandler.CreateBooking))(w, r)
		default:
			http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/bookings/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			middleware.AuthMiddleware(bookingHandler.CancelBooking)(w, r)
		} else {
			http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		}
	})

	// Публичный эндпоинт для расписания (можно смотреть без авторизации)
	mux.HandleFunc("/api/rooms/", bookingHandler.GetSchedule) // GET /api/rooms/{id}/schedule

	meHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
			return
		}

		// Получаем данные пользователя из контекста
		// Мы знаем, что они там есть, потому что middleware отработал
		userID, _ := middleware.GetUserID(r.Context())
		email, _ := middleware.GetUserEmail(r.Context())

		w.Header().Set("content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user_id":    userID,
			"user_email": email,
			"message":    "You are authenticated",
		})
	}

	// Применяем middleware к хендлеру
	// Теперь /api/me требует валидный JWT токен
	mux.HandleFunc("/api/me",
		middleware.LoggingMiddleware(
			middleware.AuthMiddleware(
				middleware.LoggingMiddleware(meHandler),
			),
		))

	log.Printf("🚀 Сервер запущен на http://localhost:%s", cfg.Port)
	log.Println("📋 Доступные эндпоинты:")
	log.Println("  GET  /health                          - публичный")
	log.Println("  GET  /rooms                          - публичный")
	log.Println("  POST /api/register                  - публичный")
	log.Println("  POST /api/login                     - публичный")
	log.Println("  GET  /api/me                        - 🔒 защищенный")
	log.Println("  GET  /api/bookings                  - 🔒 мои бронирования")
	log.Println("  POST /api/bookings                  - 🔒 создать бронирование")
	log.Println("  DELETE /api/bookings/{id}           - 🔒 отменить бронирование")
	log.Println("  GET  /api/rooms/{id}/schedule?date= - публичный (расписание)")

	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatalf("Ошибка запуска сервера: %v", err)
	}

}
