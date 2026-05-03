package handlers

import (
	"encoding/json"
	"net/http"

	"api-gateway/internal/auth"
	"api-gateway/internal/db"
)

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", HealthHandler)
	mux.HandleFunc("/token", TokenHandler)   // public — no JWT needed
	mux.HandleFunc("/api/data", DataHandler) // protected — needs JWT
	mux.HandleFunc("/api/logs", LogsHandler) // protected — needs JWT
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func TokenHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, `{"error":"user_id required"}`, http.StatusBadRequest)
		return
	}
	token, err := auth.GenerateToken(userID)
	if err != nil {
		http.Error(w, `{"error":"could not generate token"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func DataHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(auth.UserIDKey)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "success",
		"user_id": userID,
		"data":    []string{"item1", "item2", "item3"},
	})
}

func LogsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query(
		`SELECT client_id, path, method, status_code, latency_ms, created_at 
		 FROM request_logs ORDER BY created_at DESC LIMIT 20`)
	if err != nil {
		http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var logs []map[string]interface{}
	for rows.Next() {
		var clientID, path, method, createdAt string
		var statusCode, latencyMs int
		rows.Scan(&clientID, &path, &method, &statusCode, &latencyMs, &createdAt)
		logs = append(logs, map[string]interface{}{
			"client_id":   clientID,
			"path":        path,
			"method":      method,
			"status_code": statusCode,
			"latency_ms":  latencyMs,
			"created_at":  createdAt,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}
