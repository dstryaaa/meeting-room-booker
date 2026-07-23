package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Глобальная переменная для секретного ключа
// Она будет инициализирована при старте приложения
var jwtSecret []byte

// InitJWT - вызывается в main для установки секрета
func InitJWT(secret string) {
	jwtSecret = []byte(secret)
}

// Claims - структура того, что мы храним в токене
type Claims struct {
	UserID int    `json:"user_id"`
	Email  string `json:"email"`
	// Встроенная структура из библиотеки jwt
	// содержит стандартные поля: ExpiresAt, IssuedAt, Issuer и т.д.
	jwt.RegisteredClaims
}

// GenerateToken - создает JWT токен для пользователя
func GenerateToken(userId int, email string) (string, error) {
	claims := Claims{
		UserID: userId,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// Создаем токен с алгоритмом HS256 (симметричное шифрование)
	// HS256 означает, что мы используем один секретный ключ
	// и для создания, и для проверки токена
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Подписываем токен нашим секретом
	// Получается строка вида: "eyJhbGciOiJIUzI1NiIs..."
	return token.SignedString(jwtSecret)
}

// ValidateToken - проверяет, что токен валидный и возвращает данные пользователя
func ValidateToken(tokenString string) (*Claims, error) {
	// Парсим токен, используя нашу структуру Claims
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Возвращаем секрет для проверки подписи
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	// Проверяем, что токен валидный и преобразуем к нашему типу
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
