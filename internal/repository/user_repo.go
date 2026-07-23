package repository

import (
	"context"
	"database/sql"

	"github.com/dstryaaa/meeting-room-booker/internal/models"
)

// UserRepository - слой для работы с таблицей users в БД
// Repository pattern: изолируем SQL запросы от бизнес-логики
type UserRepository struct {
	db *sql.DB
}

// Конструктор - создает новый экземпляр репозитория
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create - создает нового пользователя в БД
// Возвращает ID и created_at через RETURNING
func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (email, password_hash, full_name)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`
	// QueryRowContext - выполняет запрос и сканирует результат
	// Используем контекст для возможности отмены запроса
	return r.db.QueryRowContext(
		ctx,
		query,
		user.Email,
		user.PasswordHash,
		user.FullName,
	).Scan(&user.ID, &user.CreatedAt)
}

// FindByEmail - ищет пользователя по email
// Возвращает nil, nil если пользователь не найден
// (это лучше, чем возвращать ошибку "not found")
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, email, password_hash, full_name, created_at FROM users WHERE email = $1`

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FullName,
		&user.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}
