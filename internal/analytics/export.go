package analytics

import (
	"database/sql"
	"fmt"
)

// Exporter provides analytics queries for RAG export
type Exporter struct {
	conn *sql.DB
}

// NewExporter creates a new analytics exporter
func NewExporter(conn *sql.DB) *Exporter {
	return &Exporter{conn: conn}
}

// SessionSummary is a high-level session overview
type SessionSummary struct {
	SessionID        string  `json:"session_id"`
	Outcome          string  `json:"outcome"`
	TurnsTotal       int     `json:"turns_total"`
	RoomsExplored    int     `json:"rooms_explored"`
	MonstersDefeated int     `json:"monsters_defeated"`
	TrapsEncountered int     `json:"traps_encountered"`
	CharactersLost   int     `json:"characters_lost"`
	PartyComposition string  `json:"party_composition"` // e.g. "fighter,thief,magic_user"
	Duration         *string `json:"duration,omitempty"`
}

// ClassPerformance aggregates stats across sessions for a class
type ClassPerformance struct {
	Class            string  `json:"class"`
	TotalKills       int     `json:"total_kills"`
	TotalDamageDealt int     `json:"total_damage_dealt"`
	TotalDamageTaken int     `json:"total_damage_taken"`
	TotalItemsUsed   int     `json:"total_items_used"`
	TrapsDisarmed    int     `json:"traps_disarmed"`
	SuccessfulSneaks int     `json:"successful_sneaks"`
	SurvivalRate     float64 `json:"survival_rate"` // percent alive at end
	AvgKillsPerGame  float64 `json:"avg_kills_per_game"`
	GameCount        int     `json:"game_count"`
}

// GetSessionSummaries returns summaries of all sessions
func (e *Exporter) GetSessionSummaries(limit int) ([]*SessionSummary, error) {
	rows, err := e.conn.Query(`
		SELECT
			s.id,
			COALESCE(s.outcome, 'in_progress'),
			s.turns_total,
			s.rooms_explored,
			s.monsters_defeated,
			s.traps_encountered,
			s.characters_lost,
			GROUP_CONCAT(sc.class) as party_comp
		FROM sessions s
		LEFT JOIN session_characters sc ON sc.session_id = s.id
		GROUP BY s.id
		ORDER BY s.started_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get session summaries: %w", err)
	}
	defer rows.Close()

	summaries := make([]*SessionSummary, 0)
	for rows.Next() {
		ss := &SessionSummary{}
		var partyComp *string
		if err := rows.Scan(&ss.SessionID, &ss.Outcome, &ss.TurnsTotal,
			&ss.RoomsExplored, &ss.MonstersDefeated, &ss.TrapsEncountered,
			&ss.CharactersLost, &partyComp); err != nil {
			return nil, fmt.Errorf("failed to scan summary: %w", err)
		}
		if partyComp != nil {
			ss.PartyComposition = *partyComp
		}
		summaries = append(summaries, ss)
	}
	return summaries, nil
}

// GetClassPerformance returns aggregated stats by class
func (e *Exporter) GetClassPerformance() ([]*ClassPerformance, error) {
	rows, err := e.conn.Query(`
		SELECT
			sc.class,
			SUM(sc.kills) as total_kills,
			SUM(sc.damage_dealt) as total_dmg_dealt,
			SUM(sc.damage_taken) as total_dmg_taken,
			SUM(sc.items_used) as total_items,
			SUM(sc.traps_disarmed) as total_traps,
			SUM(sc.successful_sneaks) as total_sneaks,
			CAST(SUM(CASE WHEN sc.final_status = 'alive' THEN 1 ELSE 0 END) AS FLOAT)
				/ CAST(COUNT(*) AS FLOAT) * 100 as survival_rate,
			CAST(SUM(sc.kills) AS FLOAT) / CAST(COUNT(DISTINCT sc.session_id) AS FLOAT) as avg_kills,
			COUNT(DISTINCT sc.session_id) as game_count
		FROM session_characters sc
		GROUP BY sc.class`)
	if err != nil {
		return nil, fmt.Errorf("failed to get class performance: %w", err)
	}
	defer rows.Close()

	results := make([]*ClassPerformance, 0)
	for rows.Next() {
		cp := &ClassPerformance{}
		if err := rows.Scan(&cp.Class, &cp.TotalKills, &cp.TotalDamageDealt,
			&cp.TotalDamageTaken, &cp.TotalItemsUsed, &cp.TrapsDisarmed,
			&cp.SuccessfulSneaks, &cp.SurvivalRate, &cp.AvgKillsPerGame,
			&cp.GameCount); err != nil {
			return nil, fmt.Errorf("failed to scan class performance: %w", err)
		}
		results = append(results, cp)
	}
	return results, nil
}

// GetOutcomesByPartyComposition returns win/loss rates by party makeup
func (e *Exporter) GetOutcomesByPartyComposition() ([]map[string]interface{}, error) {
	rows, err := e.conn.Query(`
		SELECT
			party_comp,
			COUNT(*) as total_games,
			SUM(CASE WHEN outcome = 'victory' THEN 1 ELSE 0 END) as wins,
			SUM(CASE WHEN outcome = 'party_wipe' THEN 1 ELSE 0 END) as losses,
			CAST(SUM(CASE WHEN outcome = 'victory' THEN 1 ELSE 0 END) AS FLOAT)
				/ CAST(COUNT(*) AS FLOAT) * 100 as win_rate
		FROM (
			SELECT
				s.id,
				s.outcome,
				GROUP_CONCAT(sc.class ORDER BY sc.class) as party_comp
			FROM sessions s
			JOIN session_characters sc ON sc.session_id = s.id
			WHERE s.outcome IS NOT NULL
			GROUP BY s.id
		)
		GROUP BY party_comp
		ORDER BY total_games DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to get outcomes by composition: %w", err)
	}
	defer rows.Close()

	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		var comp string
		var total, wins, losses int
		var winRate float64
		if err := rows.Scan(&comp, &total, &wins, &losses, &winRate); err != nil {
			return nil, fmt.Errorf("failed to scan composition stats: %w", err)
		}
		results = append(results, map[string]interface{}{
			"party_composition": comp,
			"total_games":       total,
			"wins":              wins,
			"losses":            losses,
			"win_rate":          winRate,
		})
	}
	return results, nil
}

// GetScrollStats returns scroll success/failure/backfire rates by class
func (e *Exporter) GetScrollStats() ([]map[string]interface{}, error) {
	rows, err := e.conn.Query(`
		SELECT
			actor_class,
			event_subtype,
			COUNT(*) as count
		FROM events
		WHERE event_type = 'item' AND event_subtype LIKE 'scroll_%'
		GROUP BY actor_class, event_subtype
		ORDER BY actor_class, event_subtype`)
	if err != nil {
		return nil, fmt.Errorf("failed to get scroll stats: %w", err)
	}
	defer rows.Close()

	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		var class, subtype string
		var count int
		if err := rows.Scan(&class, &subtype, &count); err != nil {
			return nil, fmt.Errorf("failed to scan scroll stats: %w", err)
		}
		results = append(results, map[string]interface{}{
			"class":   class,
			"outcome": subtype,
			"count":   count,
		})
	}
	return results, nil
}

// GetTrapStats returns trap encounter outcomes
func (e *Exporter) GetTrapStats() ([]map[string]interface{}, error) {
	rows, err := e.conn.Query(`
		SELECT
			event_subtype,
			actor_class,
			COUNT(*) as count
		FROM events
		WHERE event_type = 'trap'
		GROUP BY event_subtype, actor_class
		ORDER BY event_subtype, actor_class`)
	if err != nil {
		return nil, fmt.Errorf("failed to get trap stats: %w", err)
	}
	defer rows.Close()

	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		var subtype, class string
		var count int
		if err := rows.Scan(&subtype, &class, &count); err != nil {
			return nil, fmt.Errorf("failed to scan trap stats: %w", err)
		}
		results = append(results, map[string]interface{}{
			"outcome": subtype,
			"class":   class,
			"count":   count,
		})
	}
	return results, nil
}

// GetFlanking Stats returns flanking attack frequency and average damage
func (e *Exporter) GetFlankingStats() (map[string]interface{}, error) {
	row := e.conn.QueryRow(`
		SELECT
			COUNT(*) as total_attacks,
			SUM(CASE WHEN json_extract(details, '$.was_flanking') = 1 THEN 1 ELSE 0 END) as flanking_attacks,
			AVG(CASE WHEN json_extract(details, '$.was_flanking') = 1 THEN json_extract(details, '$.damage') END) as avg_flanking_dmg,
			AVG(CASE WHEN json_extract(details, '$.was_flanking') = 0 THEN json_extract(details, '$.damage') END) as avg_normal_dmg
		FROM events
		WHERE event_type = 'combat' AND event_subtype IN ('attack_hit', 'flanking_attack')`)

	var totalAttacks, flankingAttacks int
	var avgFlankingDmg, avgNormalDmg *float64
	if err := row.Scan(&totalAttacks, &flankingAttacks, &avgFlankingDmg, &avgNormalDmg); err != nil {
		return nil, fmt.Errorf("failed to get flanking stats: %w", err)
	}

	return map[string]interface{}{
		"total_attacks":    totalAttacks,
		"flanking_attacks": flankingAttacks,
		"avg_flanking_dmg": avgFlankingDmg,
		"avg_normal_dmg":   avgNormalDmg,
	}, nil
}
