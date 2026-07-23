package middleware

import (
	"log"
	"net/http"
	"time"
)

// LoggingMiddleware - middleware для логирования всех HTTP запросов
// Он оборачивает любой хендлер и добавляет логирование
func LoggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Запоминаем время начала обработки запроса
		start := time.Now()

		// 2. Создаем обертку для ResponseWriter, чтобы перехватить статус ответа
		// Стандартный ResponseWriter не дает нам узнать статус (200, 404, 500...)
		wrapped := &responseWriterWrapper{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// 3. Вызываем следующий хендлер (передаем управление дальше)
		next.ServeHTTP(wrapped, r)

		// 4. Замеряем время выполнения
		duration := time.Since(start)

		// 5. Получаем ID пользователя из контекста (если есть)
		userID := GetUserIDStr(r.Context())

		log.Printf("[%s] %s %s | %d | %v | UserID=%s",
			time.Now().Format("2006-01-02 15:04:05"),
			r.Method,
			r.URL.Path,
			wrapped.statusCode,
			duration,
			userID,
		)
	}
}

// responseWriterWrapper - обертка для http.ResponseWriter
// Нужна, чтобы перехватывать статус код ответа
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader - переопределяем метод, чтобы сохранить статус код
// Этот метод вызывается, когда хендлер вызывает w.WriteHeader(code)
func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write - переопределяем метод, чтобы если статус еще не установлен,
// то установить его в 200 OK (стандартное поведение)
func (w responseWriterWrapper) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}
