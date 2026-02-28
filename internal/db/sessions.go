package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Session represents a game session record
type Session struct {
	ID               string
	Seed             int64
	StartedAt        time.Time
	EndedAt          *time.Time
	Outcome          *string
	TurnsTotal       int
	RoomsExplored    int
	MonstersDefeated int
	TrapsEncountered int
	CharactersLost   int
	DungeonDepth     int
}

// CreateSession inserts a new session record
func (db *DB) CreateSession(id string, seed int64, depth int) error {
	_, err := db.conn.Exec(
		`INSERT INTO sessions (id, seed, dungeon_depth) VALUES (?, ?, ?)`,
		id, seed, depth,
	)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

// EndSession marks a session as ended with outcome and summary stats
func (db *DB) EndSession(id string, outcome string, turns, rooms, monsters, traps, charsLost int) error {
	now := time.Now()
	_, err := db.conn.Exec(
		`UPDATE sessions SET
			ended_at = ?, outcome = ?, turns_total = ?,
			rooms_explored = ?, monsters_defeated = ?,
			traps_encountered = ?, characters_lost = ?
		WHERE id = ?`,
		now, outcome, turns, rooms, monsters, traps, charsLost, id,
	)
	if err != nil {
		return fmt.Errorf("failed to end session: %w", err)
	}
	return nil
}

// UpdateSessionStats updates running counters on a session
func (db *DB) UpdateSessionStats(id string, turns, rooms, monsters, traps, charsLost int) error {
	_, err := db.conn.Exec(
		`UPDATE sessions SET
			turns_total = ?, rooms_explored = ?,
			monsters_defeated = ?, traps_encountered = ?,
			characters_lost = ?
		WHERE id = ?`,
		turns, rooms, monsters, traps, charsLost, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update session stats: %w", err)
	}
	return nil
}

// GetSession retrieves a session by ID
func (db *DB) GetSession(id string) (*Session, error) {
	row := db.conn.QueryRow(`SELECT id, seed, started_at, ended_at, outcome,
		turns_total, rooms_explored, monsters_defeated, traps_encountered,
		characters_lost, dungeon_depth FROM sessions WHERE id = ?`, id)

	s := &Session{}
	err := row.Scan(&s.ID, &s.Seed, &s.StartedAt, &s.EndedAt, &s.Outcome,
		&s.TurnsTotal, &s.RoomsExplored, &s.MonstersDefeated,
		&s.TrapsEncountered, &s.CharactersLost, &s.DungeonDepth)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return s, nil
}

// ListSessions returns recent sessions, most recent first
func (db *DB) ListSessions(limit int) ([]*Session, error) {
	rows, err := db.conn.Query(
		`SELECT id, seed, started_at, ended_at, outcome,
			turns_total, rooms_explored, monsters_defeated, traps_encountered,
			characters_lost, dungeon_depth
		FROM sessions ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]*Session, 0)
	for rows.Next() {
		s := &Session{}
		if err := rows.Scan(&s.ID, &s.Seed, &s.StartedAt, &s.EndedAt, &s.Outcome,
			&s.TurnsTotal, &s.RoomsExplored, &s.MonstersDefeated,
			&s.TrapsEncountered, &s.CharactersLost, &s.DungeonDepth); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate sessions: %w", err)
	}
	return sessions, nil
}
