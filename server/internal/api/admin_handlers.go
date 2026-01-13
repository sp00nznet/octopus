package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sp00nznet/octopus/internal/db"
)

// Environment variable handlers
func (s *Server) listEnvVariables(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT id, name, value, description, is_secret, created_at, updated_at
		FROM env_variables
		ORDER BY name
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	var envVars []db.EnvVariable
	for rows.Next() {
		var ev db.EnvVariable
		err := rows.Scan(&ev.ID, &ev.Name, &ev.Value, &ev.Description, &ev.IsSecret, &ev.CreatedAt, &ev.UpdatedAt)
		if err != nil {
			continue
		}
		// Mask secret values
		if ev.IsSecret {
			ev.Value = "********"
		}
		envVars = append(envVars, ev)
	}

	respondJSON(w, http.StatusOK, envVars)
}

func (s *Server) createEnvVariable(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Value       string `json:"value"`
		Description string `json:"description"`
		IsSecret    bool   `json:"is_secret"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	result, err := s.db.Exec(`
		INSERT INTO env_variables (name, value, description, is_secret)
		VALUES (?, ?, ?, ?)
	`, req.Name, req.Value, req.Description, req.IsSecret)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create environment variable")
		return
	}

	// Log activity
	username := r.Context().Value("username").(string)
	s.logActivity(username, "create_env_var", "env_variable", 0, req.Name, r.RemoteAddr)

	id, _ := result.LastInsertId()
	respondJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (s *Server) updateEnvVariable(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req struct {
		Name        string `json:"name"`
		Value       string `json:"value"`
		Description string `json:"description"`
		IsSecret    bool   `json:"is_secret"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	_, err := s.db.Exec(`
		UPDATE env_variables SET name=?, value=?, description=?, is_secret=?, updated_at=?
		WHERE id=?
	`, req.Name, req.Value, req.Description, req.IsSecret, time.Now(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update environment variable")
		return
	}

	// Log activity
	username := r.Context().Value("username").(string)
	s.logActivity(username, "update_env_var", "env_variable", 0, req.Name, r.RemoteAddr)

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) deleteEnvVariable(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	// Get name for logging
	var name string
	s.db.QueryRow("SELECT name FROM env_variables WHERE id = ?", id).Scan(&name)

	_, err := s.db.Exec("DELETE FROM env_variables WHERE id = ?", id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete environment variable")
		return
	}

	// Log activity
	username := r.Context().Value("username").(string)
	s.logActivity(username, "delete_env_var", "env_variable", 0, name, r.RemoteAddr)

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Activity log handlers
func (s *Server) listActivityLogs(w http.ResponseWriter, r *http.Request) {
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "100"
	}

	rows, err := s.db.Query(`
		SELECT a.id, a.user_id, a.action, a.entity_type, a.entity_id, a.details, a.ip_address, a.created_at,
			COALESCE(u.username, 'system') as username
		FROM activity_logs a
		LEFT JOIN users u ON a.user_id = u.id
		ORDER BY a.created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	type ActivityLogWithUser struct {
		db.ActivityLog
		Username string `json:"username"`
	}

	var logs []ActivityLogWithUser
	for rows.Next() {
		var log ActivityLogWithUser
		err := rows.Scan(&log.ID, &log.UserID, &log.Action, &log.EntityType, &log.EntityID,
			&log.Details, &log.IPAddress, &log.CreatedAt, &log.Username)
		if err != nil {
			continue
		}
		logs = append(logs, log)
	}

	respondJSON(w, http.StatusOK, logs)
}

// User management handlers
func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT id, username, email, display_name, is_admin, created_at, last_login
		FROM users
		ORDER BY username
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	type User struct {
		ID          int64      `json:"id"`
		Username    string     `json:"username"`
		Email       string     `json:"email"`
		DisplayName string     `json:"display_name"`
		IsAdmin     bool       `json:"is_admin"`
		CreatedAt   time.Time  `json:"created_at"`
		LastLogin   *time.Time `json:"last_login,omitempty"`
	}

	var users []User
	for rows.Next() {
		var u User
		err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.IsAdmin, &u.CreatedAt, &u.LastLogin)
		if err != nil {
			continue
		}
		users = append(users, u)
	}

	respondJSON(w, http.StatusOK, users)
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	type User struct {
		ID          int64      `json:"id"`
		Username    string     `json:"username"`
		Email       string     `json:"email"`
		DisplayName string     `json:"display_name"`
		IsAdmin     bool       `json:"is_admin"`
		CreatedAt   time.Time  `json:"created_at"`
		LastLogin   *time.Time `json:"last_login,omitempty"`
	}

	var u User
	err := s.db.QueryRow(`
		SELECT id, username, email, display_name, is_admin, created_at, last_login
		FROM users WHERE id = ?
	`, id).Scan(&u.ID, &u.Username, &u.Email, &u.DisplayName, &u.IsAdmin, &u.CreatedAt, &u.LastLogin)
	if err == sql.ErrNoRows {
		respondError(w, http.StatusNotFound, "User not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Database error")
		return
	}
	respondJSON(w, http.StatusOK, u)
}

func (s *Server) toggleUserAdmin(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req struct {
		IsAdmin bool `json:"is_admin"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	_, err := s.db.Exec(`UPDATE users SET is_admin = ? WHERE id = ?`, req.IsAdmin, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update user")
		return
	}

	// Log activity
	username := r.Context().Value("username").(string)
	action := "grant_admin"
	if !req.IsAdmin {
		action = "revoke_admin"
	}
	s.logActivity(username, action, "user", 0, "", r.RemoteAddr)

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// Helper to log activity
func (s *Server) logActivity(username, action, entityType string, entityID int64, details, ipAddress string) {
	// Get user ID
	var userID int64
	s.db.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&userID)

	s.db.Exec(`
		INSERT INTO activity_logs (user_id, action, entity_type, entity_id, details, ip_address)
		VALUES (?, ?, ?, ?, ?, ?)
	`, userID, action, entityType, entityID, details, ipAddress)
}
