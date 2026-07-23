package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dstryaaa/meeting-room-booker/internal/models"
)

// RoomRepository - слой для работы с таблицей rooms в БД
// Отвечает за все запросы, связанные с комнатами
type RoomRepository struct {
	db *sql.DB
}

// NewRoomRepository - конструктор, создает новый экземпляр
func NewRoomRepository(db *sql.DB) *RoomRepository {
	return &RoomRepository{db: db}
}

// GetAllActive - получает все активные комнаты
// Используется для отображения списка комнат на фронтенде
func (r *RoomRepository) GetAllActive(ctx context.Context) ([]models.Room, error) {
	query := `
		SELECT id, name, capacity, description, has_projector, has_whiteboard, is_active
		FROM rooms
		WHERE is_active = true
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get rooms: %w", err)
	}

	var rooms []models.Room
	for rows.Next() {
		var room models.Room
		err := rows.Scan(
			&room.ID,
			&room.Name,
			&room.Capacity,
			&room.Description,
			&room.HasProjector,
			&room.HasWhiteboard,
			&room.IsActive,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan room: %w", err)
		}
		rooms = append(rooms, room)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return rooms, nil
}

// GetByID - получает комнату по ID
// Используется при создании бронирования, чтобы проверить, что комната существует
// Возвращает nil, nil если комната не найдена
func (r *RoomRepository) GetByID(ctx context.Context, id int) (*models.Room, error) {
	room := &models.Room{}
	query := `
		SELECT id, name, capacity, description, has_projector, has_whiteboard, is_active
		FROM rooms
		WHERE id = $1 AND is_active = true
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&room.ID,
		&room.Name,
		&room.Capacity,
		&room.Description,
		&room.HasProjector,
		&room.HasWhiteboard,
		&room.IsActive,
	)

	if err == sql.ErrNoRows {
		// Комната не найдена - возвращаем nil, nil (не ошибка)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get room by id: %w", err)
	}

	return room, nil
}

// SeedRooms - заполняет базу тестовыми комнатами
// Вызывается при первом запуске, чтобы были данные для работы
// Использует проверку "если нет - создай", чтобы не дублировать данные
func (r *RoomRepository) SeedRooms(ctx context.Context) error {
	rooms := []struct {
		name        string
		capacity    int
		description string
		projector   bool
		whiteboard  bool
	}{
		{"Conference A", 12, "Большая переговорная с проектором", true, true},
		{"Conference B", 8, "Средняя комната для встреч", true, false},
		{"Conference C", 6, "Малая переговорная", false, true},
		{"Коворкинг", 20, "Открытое пространство для работы", false, false},
	}

	for _, room := range rooms {
		// Проверяем, существует ли уже комната с таким именем
		var exists bool
		err := r.db.QueryRowContext(ctx,
			"SELECT EXISTS(SELECT 1 FROM rooms WHERE name = $1)", room.name,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check if room exists: %w", err)
		}

		// Если нет - создаем
		if !exists {
			_, err := r.db.ExecContext(ctx, `
				INSERT INTO rooms (name, capacity, description, has_projector, has_whiteboard)
				VALUES ($1, $2, $3, $4, $5)
			`, room.name, room.capacity, room.description, room.projector, room.whiteboard)
			if err != nil {
				return fmt.Errorf("failed to create room %s: %w", room.name, err)
			}
			fmt.Printf("✅ Добавлена комната: %s\n", room.name)
		}
	}
	return nil
}
