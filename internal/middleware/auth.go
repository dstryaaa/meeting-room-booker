package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/dstryaaa/meeting-room-booker/internal/utils"
)

// Определяем типы для ключей контекста
// Это нужно, чтобы безопасно хранить данные в контексте запроса
type contextKey string

// Константы для ключей контекста
// Используем тип contextKey, а не string, чтобы избежать конфликтов
const UserIDKey contextKey = "userID"
const UserEmailKEy contextKey = "userEmail"

// AuthMiddleware - это функция, которая возвращает http.HandlerFunc
// То есть это "фабрика" middleware
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	// Возвращаем новую функцию, которая оборачивает исходный хендлер
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("🔐 AuthMiddleware: начал выполнение")
		// 1. Получаем токен из заголовка Authorization
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			log.Println("🔐 AuthMiddleware: нет заголовка Authorization")
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// 2. Проверяем формат "Bearer <token>"
		// Токен должен передаваться как: "Bearer eyJhbGciOiJIUzI1NiIs..."
		// Это стандарт, которого придерживаются все API
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			log.Println("🔐 AuthMiddleware: неправильный формат заголовка")
			http.Error(w, "Invalid authorization header format. Use: Bearer <token>", http.StatusUnauthorized)
			return
		}
		tokenString := parts[1]

		// 3. Валидируем токен
		claims, err := utils.ValidateToken(tokenString)
		if err != nil {
			log.Printf("🔐 AuthMiddleware: ошибка валидации токена: %v", err)
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		log.Printf("🔐 AuthMiddleware: токен валиден, UserID=%d", claims.UserID)

		// 4. Кладем данные пользователя в контекст запроса
		// Это ключевой момент: мы извлекаем данные из токена и делаем их доступными
		// для всех последующих хендлеров через r.Context()
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UserEmailKEy, claims.Email)

		userID, ok := ctx.Value(UserIDKey).(int)
		log.Printf("🔐 AuthMiddleware: userID из контекста ДО вызова next: %d, ok=%v", userID, ok)

		log.Println("🔐 AuthMiddleware: вызываю next.ServeHTTP()")

		// 5. Вызываем следующий хендлер с обновленным контекстом
		// Передаем управление дальше, но уже с контекстом, где есть данные пользователя
		next.ServeHTTP(w, r.WithContext(ctx))
		log.Println("🔐 AuthMiddleware: next.ServeHTTP() завершился")
	}
}

// Helper функции для получения данных пользователя из контекста
// Это удобно использовать в хендлерах

// GetUserID - извлекает ID пользователя из контекста запроса
func GetUserID(ctx context.Context) (int, bool) {
	userID, ok := ctx.Value(UserIDKey).(int)
	return userID, ok
}

func GetUserIDStr(ctx context.Context) string {

	log.Printf("🔍 GetUserIDStr: вызываю GetUserID")
	userID, ok := GetUserID(ctx)
	log.Printf("🔍 GetUserIDStr: результат: userID=%d, ok=%v", userID, ok)
	if !ok {
		return "anonymous"
	}
	return fmt.Sprintf("%d", userID)
}

// GetUserEmail - извлекает Email пользователя из контекста запроса
func GetUserEmail(ctx context.Context) (string, bool) {
	userEmail, ok := ctx.Value(UserEmailKEy).(string)
	return userEmail, ok
}
