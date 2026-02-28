package db

import (
	"encoding/json"
	"fmt"
)

// LogEvent inserts a structured event into the events table
func (db *DB) LogEvent(sessionID string, turnNumber int, eventType, eventSubtype string,
	actorID, actorClass, targetID, roomID string, details interface{}) error {

	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("failed to marshal event details: %w", err)
	}

	_, err = db.conn.Exec(
		`INSERT INTO events (session_id, turn_number, event_type, event_subtype,
			actor_id, actor_class, target_id, room_id, details)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID, turnNumber, eventType, eventSubtype,
		actorID, actorClass, targetID, roomID, string(detailsJSON),
	)
	if err != nil {
		return fmt.Errorf("failed to log event: %w", err)
	}
	return nil
}

// SessionCharacter represents a character's per-session stats
type SessionCharacter struct {
	ID              string
	SessionID       string
	CharacterName   string
	Class           string
	FinalHP         *int
	FinalStatus     *string
	Kills           int
	DamageDealt     int
	DamageTaken     int
	ItemsUsed       int
	TrapsDisarmed   int
	SuccessfulSneaks int
}

// CreateSessionCharacter inserts a character record for a session
func (db *DB) CreateSessionCharacter(id, sessionID, name, class string) error {
	_, err := db.conn.Exec(
		`INSERT INTO session_characters (id, session_id, character_name, class)
		VALUES (?, ?, ?, ?)`,
		id, sessionID, name, class,
	)
	if err != nil {
		return fmt.Errorf("failed to create session character: %w", err)
	}
	return nil
}

// IncrementCharacterStat increments a single stat field for a session character
func (db *DB) IncrementCharacterStat(id, field string, amount int) error {
	// Whitelist allowed fields to prevent SQL injection
	allowed := map[string]bool{
		"kills": true, "damage_dealt": true, "damage_taken": true,
		"items_used": true, "traps_disarmed": true, "successful_sneaks": true,
	}
	if !allowed[field] {
		return fmt.Errorf("invalid stat field: %s", field)
	}
	_, err := db.conn.Exec(
		fmt.Sprintf(`UPDATE session_characters SET %s = %s + ? WHERE id = ?`, field, field),
		amount, id,
	)
	if err != nil {
		return fmt.Errorf("failed to increment %s: %w", field, err)
	}
	return nil
}

// FinalizeSessionCharacter sets the final HP and status for a character
func (db *DB) FinalizeSessionCharacter(id string, finalHP int, finalStatus string) error {
	_, err := db.conn.Exec(
		`UPDATE session_characters SET final_hp = ?, final_status = ? WHERE id = ?`,
		finalHP, finalStatus, id,
	)
	if err != nil {
		return fmt.Errorf("failed to finalize session character: %w", err)
	}
	return nil
}

// GetSessionEvents retrieves all events for a session, ordered by turn
func (db *DB) GetSessionEvents(sessionID string, limit int) ([]map[string]interface{}, error) {
	query := `SELECT id, turn_number, event_type, event_subtype,
		actor_id, actor_class, target_id, room_id, details, created_at
		FROM events WHERE session_id = ? ORDER BY id ASC`
	args := []interface{}{sessionID}

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}
	defer rows.Close()

	events := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int
		var turnNumber int
		var eventType, eventSubtype string
		var actorID, actorClass, targetID, roomID, detailsStr, createdAt string

		if err := rows.Scan(&id, &turnNumber, &eventType, &eventSubtype,
			&actorID, &actorClass, &targetID, &roomID, &detailsStr, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		var details interface{}
		_ = json.Unmarshal([]byte(detailsStr), &details) // best-effort; details may be empty string

		events = append(events, map[string]interface{}{
			"id":            id,
			"turn_number":   turnNumber,
			"event_type":    eventType,
			"event_subtype": eventSubtype,
			"actor_id":      actorID,
			"actor_class":   actorClass,
			"target_id":     targetID,
			"room_id":       roomID,
			"details":       details,
			"created_at":    createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate events: %w", err)
	}
	return events, nil
}

// GetSessionCharacters retrieves all characters for a session
func (db *DB) GetSessionCharacters(sessionID string) ([]*SessionCharacter, error) {
	rows, err := db.conn.Query(
		`SELECT id, session_id, character_name, class, final_hp, final_status,
			kills, damage_dealt, damage_taken, items_used, traps_disarmed, successful_sneaks
		FROM session_characters WHERE session_id = ?`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session characters: %w", err)
	}
	defer rows.Close()

	chars := make([]*SessionCharacter, 0)
	for rows.Next() {
		c := &SessionCharacter{}
		if err := rows.Scan(&c.ID, &c.SessionID, &c.CharacterName, &c.Class,
			&c.FinalHP, &c.FinalStatus, &c.Kills, &c.DamageDealt, &c.DamageTaken,
			&c.ItemsUsed, &c.TrapsDisarmed, &c.SuccessfulSneaks); err != nil {
			return nil, fmt.Errorf("failed to scan session character: %w", err)
		}
		chars = append(chars, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate session characters: %w", err)
	}
	return chars, nil
}

// CountEventsByType returns the count of events by type for a session
func (db *DB) CountEventsByType(sessionID string) (map[string]int, error) {
	rows, err := db.conn.Query(
		`SELECT event_type, COUNT(*) FROM events WHERE session_id = ? GROUP BY event_type`,
		sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to count events: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var eventType string
		var count int
		if err := rows.Scan(&eventType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan count: %w", err)
		}
		counts[eventType] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate event counts: %w", err)
	}
	return counts, nil
}
