package middleware

import "net/http"

// CORS - middleware для обработки CORS заголовков
// Добавляет разрешения для кросс-доменных запросов
func CORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Устанавливаем заголовки CORS для всех ответов

		// Access-Control-Allow-Origin - какой домен разрешен
		// "*" - разрешить все домены (для разработки)
		// В продакшене лучше указать конкретный домен
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Access-Control-Allow-Methods - какие HTTP методы разрешены
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		// Access-Control-Allow-Headers - какие заголовки разрешены
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Access-Control-Expose-Headers - какие заголовки видны клиенту
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type")

		// 2. Если это preflight-запрос (OPTIONS), отвечаем сразу
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent) // 204 No Content
			return
		}

		// 3. Передаем управление следующему хендлеру
		next.ServeHTTP(w, r)
	}
}
