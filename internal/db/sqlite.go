package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"
)

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	dir := path
	if idx := strings.LastIndexByte(path, '/'); idx >= 0 {
		dir = path[:idx]
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	// Remove stale WAL/SHM files from other SQLite implementations
	for _, ext := range []string{"-wal", "-shm"} {
		os.Remove(path + ext)
	}

	dsn := path + "?_pragma=journal_mode(DELETE)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := s.seed(); err != nil {
		return nil, fmt.Errorf("seed: %w", err)
	}
	go s.cleanupLoop()
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			password_change_required INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			expires_at DATETIME NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);
	`)
	return err
}

func (s *Store) seed() error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(
		"INSERT INTO users (username, password_hash, password_change_required) VALUES (?, ?, 1)",
		"admin", string(hash),
	); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "\n*** Default credentials: admin / admin -- CHANGE IMMEDIATELY ***\n\n")
	return nil
}

func (s *Store) Authenticate(username, password string) (int64, error) {
	var id int64
	var hash string
	err := s.db.QueryRow(
		"SELECT id, password_hash FROM users WHERE username = ?", username,
	).Scan(&id, &hash)
	if err != nil {
		return 0, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) RequiresPasswordChange(userID int64) (bool, error) {
	var required bool
	err := s.db.QueryRow(
		"SELECT password_change_required FROM users WHERE id = ?", userID,
	).Scan(&required)
	return required, err
}

func (s *Store) ChangePassword(userID int64, oldPassword, newPassword string) error {
	// Verify old password
	var hash string
	if err := s.db.QueryRow(
		"SELECT password_hash FROM users WHERE id = ?", userID,
	).Scan(&hash); err != nil {
		return fmt.Errorf("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(oldPassword)); err != nil {
		return fmt.Errorf("current password is incorrect")
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		"UPDATE users SET password_hash = ?, password_change_required = 0 WHERE id = ?",
		string(newHash), userID,
	)
	return err
}

func (s *Store) CreateSession(userID int64) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)
	expires := time.Now().Add(24 * time.Hour)
	_, err := s.db.Exec(
		"INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)",
		id, userID, expires.Format(time.RFC3339),
	)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) ValidateSession(sessionID string) (int64, error) {
	var userID int64
	var expiresStr string
	err := s.db.QueryRow(
		"SELECT user_id, expires_at FROM sessions WHERE id = ?", sessionID,
	).Scan(&userID, &expiresStr)
	if err != nil {
		return 0, err
	}
	expires, err := time.Parse(time.RFC3339, expiresStr)
	if err != nil {
		return 0, err
	}
	if time.Now().After(expires) {
		_ = s.DeleteSession(sessionID)
		return 0, fmt.Errorf("session expired")
	}
	return userID, nil
}

func (s *Store) DeleteSession(sessionID string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
	return err
}

func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		_, _ = s.db.Exec(
			"DELETE FROM sessions WHERE expires_at < ?",
			time.Now().Format(time.RFC3339),
		)
	}
}
