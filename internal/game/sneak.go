package game

import (
	"fmt"
	"math/rand"
)

// SneakResult holds the outcome of a sneak check
type SneakResult struct {
	Success    bool
	SneakerID  string
	SneakerName string
	Roll       int
	DC         int
	IsHidden   bool // if successful, sneaker starts combat hidden
}

// Sneak bonuses
const (
	ThiefSneakBonus = 2
)

// CheckRoomSneak performs a sneak check when entering a room with enemies.
// Only a thief can attempt this.
func CheckRoomSneak(thief *Character, roomDifficulty int, rng *rand.Rand) *SneakResult {
	if thief.Class != ClassThief || !thief.IsAlive {
		return &SneakResult{
			Success:     false,
			SneakerID:   thief.ID,
			SneakerName: thief.Name,
		}
	}

	dc := roomDifficulty + 10

	roll := rng.Intn(20) + 1 // d20
	bonus := thief.Dexterity/2 + ThiefSneakBonus
	total := roll + bonus

	result := &SneakResult{
		SneakerID:   thief.ID,
		SneakerName: thief.Name,
		Roll:        total,
		DC:          dc,
	}

	if total >= dc {
		result.Success = true
		result.IsHidden = true
	}

	return result
}

// CombatHideResult holds the outcome of a combat hide attempt
type CombatHideResult struct {
	Success     bool
	SneakerID   string
	SneakerName string
	Roll        int
	DC          int
}

// AttemptCombatHide has a thief attempt to hide during combat.
// DC = highest enemy perception (10 + enemy dexterity/2)
func AttemptCombatHide(thief *Character, enemies []*Monster, rng *rand.Rand) *CombatHideResult {
	if thief.Class != ClassThief || !thief.IsAlive {
		return &CombatHideResult{
			Success:     false,
			SneakerID:   thief.ID,
			SneakerName: thief.Name,
		}
	}

	// Find highest enemy perception
	dc := 10
	for _, e := range enemies {
		if !e.IsAlive {
			continue
		}
		perception := 10 + e.Dexterity/2
		if perception > dc {
			dc = perception
		}
	}

	roll := rng.Intn(20) + 1 // d20
	bonus := thief.Dexterity/2 + ThiefSneakBonus
	total := roll + bonus

	result := &CombatHideResult{
		SneakerID:   thief.ID,
		SneakerName: thief.Name,
		Roll:        total,
		DC:          dc,
	}

	result.Success = total >= dc
	return result
}

// FormatSneakResult returns a human-readable description of a sneak check
func FormatSneakResult(result *SneakResult) string {
	if result.Success {
		return fmt.Sprintf("%s moves silently through the shadows... (Roll: %d vs DC %d) — Hidden!",
			result.SneakerName, result.Roll, result.DC)
	}
	return fmt.Sprintf("%s tries to sneak but is detected! (Roll: %d vs DC %d)",
		result.SneakerName, result.Roll, result.DC)
}

// FormatCombatHideResult returns a human-readable description of a combat hide attempt
func FormatCombatHideResult(result *CombatHideResult) string {
	if result.Success {
		return fmt.Sprintf("%s slips into the shadows! (Roll: %d vs DC %d) — Hidden!",
			result.SneakerName, result.Roll, result.DC)
	}
	return fmt.Sprintf("%s fails to hide! (Roll: %d vs DC %d)",
		result.SneakerName, result.Roll, result.DC)
}