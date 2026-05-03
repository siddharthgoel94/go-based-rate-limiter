package db

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func Init() {
	var err error
	DB, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("Failed to connect to PostgreSQL:", err)
	}

	if err = DB.Ping(); err != nil {
		log.Fatal("PostgreSQL not reachable:", err)
	}

	createTables()
	log.Println("PostgreSQL connected successfully")
}

func createTables() {
	query := `
	CREATE TABLE IF NOT EXISTS request_logs (
		id          SERIAL PRIMARY KEY,
		client_id   TEXT NOT NULL,
		path        TEXT NOT NULL,
		method      TEXT NOT NULL,
		status_code INT NOT NULL,
		latency_ms  INT NOT NULL,
		created_at  TIMESTAMPTZ DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_client_created 
		ON request_logs(client_id, created_at);`

	if _, err := DB.Exec(query); err != nil {
		log.Fatal("Failed to create tables:", err)
	}
	log.Println("Database tables ready")
}

func LogRequest(clientID, path, method string, statusCode, latencyMs int) {
	go func() {
		_, err := DB.Exec(
			`INSERT INTO request_logs 
				(client_id, path, method, status_code, latency_ms) 
			VALUES ($1,$2,$3,$4,$5)`,
			clientID, path, method, statusCode, latencyMs,
		)
		if err != nil {
			log.Println("Failed to log request:", err)
		}
	}()
}
