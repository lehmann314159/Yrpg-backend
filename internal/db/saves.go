package db

import (
	"database/sql"
	"fmt"
	"time"
)

// SavedGame represents a saved game record
type SavedGame struct {
	ID        string
	SessionID string
	StateJSON string
	SavedAt   time.Time
}

// SaveGame persists a game state as JSON
func (db *DB) SaveGame(id, sessionID, stateJSON string) error {
	// Upsert: replace if exists
	_, err := db.conn.Exec(
		`INSERT INTO saved_games (id, session_id, state_json, saved_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET state_json = ?, saved_at = ?`,
		id, sessionID, stateJSON, time.Now(), stateJSON, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to save game: %w", err)
	}
	return nil
}

// LoadGame retrieves a saved game by ID
func (db *DB) LoadGame(id string) (*SavedGame, error) {
	row := db.conn.QueryRow(
		`SELECT id, session_id, state_json, saved_at FROM saved_games WHERE id = ?`, id)

	sg := &SavedGame{}
	err := row.Scan(&sg.ID, &sg.SessionID, &sg.StateJSON, &sg.SavedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load game: %w", err)
	}
	return sg, nil
}

// LoadGameBySession retrieves the most recent save for a session
func (db *DB) LoadGameBySession(sessionID string) (*SavedGame, error) {
	row := db.conn.QueryRow(
		`SELECT id, session_id, state_json, saved_at FROM saved_games
		WHERE session_id = ? ORDER BY saved_at DESC LIMIT 1`, sessionID)

	sg := &SavedGame{}
	err := row.Scan(&sg.ID, &sg.SessionID, &sg.StateJSON, &sg.SavedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load game by session: %w", err)
	}
	return sg, nil
}

// ListSaves returns all saved games, most recent first
func (db *DB) ListSaves(limit int) ([]*SavedGame, error) {
	rows, err := db.conn.Query(
		`SELECT sg.id, sg.session_id, sg.saved_at, s.outcome
		FROM saved_games sg
		JOIN sessions s ON sg.session_id = s.id
		ORDER BY sg.saved_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list saves: %w", err)
	}
	defer rows.Close()

	saves := make([]*SavedGame, 0)
	for rows.Next() {
		sg := &SavedGame{}
		var outcome *string
		if err := rows.Scan(&sg.ID, &sg.SessionID, &sg.SavedAt, &outcome); err != nil {
			return nil, fmt.Errorf("failed to scan save: %w", err)
		}
		saves = append(saves, sg)
	}
	return saves, nil
}

// DeleteSave removes a saved game
func (db *DB) DeleteSave(id string) error {
	_, err := db.conn.Exec(`DELETE FROM saved_games WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete save: %w", err)
	}
	return nil
}
