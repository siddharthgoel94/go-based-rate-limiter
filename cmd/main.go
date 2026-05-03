package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"api-gateway/internal/auth"
	"api-gateway/internal/db"
	"api-gateway/internal/handlers"
	"api-gateway/internal/middleware"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system env vars")
	}

	// Init PostgreSQL
	db.Init()
	defer db.DB.Close()

	// Init Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:      os.Getenv("REDIS_ADDR"),
		Password:  os.Getenv("REDIS_PASSWORD"),
		TLSConfig: &tls.Config{}, // Upstash requires TLS
	})

	// Set up routes
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	// Public routes (no JWT) — wrap only with logger + rate limiter
	publicMux := http.NewServeMux()
	publicMux.HandleFunc("/health", handlers.HealthHandler)
	publicMux.HandleFunc("/token", handlers.TokenHandler)

	// Protected routes — full middleware chain
	limiter := middleware.NewRateLimiter(redisClient, 10, 60) // 10 req/min for testing

	handler := middleware.Logger(
		limiter.Middleware(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/health" || r.URL.Path == "/token" {
					publicMux.ServeHTTP(w, r)
					return
				}
				auth.JWTMiddleware(mux).ServeHTTP(w, r)
			}),
		),
	)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("API Gateway running on :%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}
