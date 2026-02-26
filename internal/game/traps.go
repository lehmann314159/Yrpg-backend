package game

import (
	"fmt"
	"math/rand"
)

// TrapDetectionResult holds the outcome of a trap detection check
type TrapDetectionResult struct {
	Detected   bool
	DetectorID string
	DetectorName string
	Roll       int
	DC         int
	Trap       *Trap
}

// TrapTriggerResult holds the outcome of a trap triggering
type TrapTriggerResult struct {
	Triggered  bool
	Damage     int
	VictimID   string
	VictimName string
	Trap       *Trap
	VictimDied bool
}

// TrapDisarmResult holds the outcome of a disarm attempt
type TrapDisarmResult struct {
	Success     bool
	DisarmerID  string
	DisarmerName string
	Roll        int
	DC          int
	Trap        *Trap
	// If failed, trap triggers on the disarmer
	TriggerResult *TrapTriggerResult
}

// Trap detection bonuses by class
const (
	ThiefDetectionBonus    = 4
	MagicUserDetectionBonus = 2
	ThiefDisarmBonus       = 6
)

// CheckTrapDetection performs passive trap detection for all party members against a trap.
// Returns the best detection result.
func CheckTrapDetection(party *Party, trap *Trap, rng *rand.Rand) *TrapDetectionResult {
	if trap.IsTriggered || trap.IsDiscovered || trap.IsDisarmed {
		return &TrapDetectionResult{Detected: false, Trap: trap}
	}

	best := &TrapDetectionResult{Detected: false, Trap: trap, DC: trap.Difficulty}

	for _, c := range party.Characters {
		if !c.IsAlive {
			continue
		}

		roll := rng.Intn(20) + 1 // d20
		bonus := c.Dexterity / 2

		// Class bonuses
		switch c.Class {
		case ClassThief:
			bonus += ThiefDetectionBonus
		case ClassMagicUser:
			bonus += MagicUserDetectionBonus
		}

		total := roll + bonus
		if total >= trap.Difficulty {
			if !best.Detected || total > best.Roll+best.DC { // take best roll
				best.Detected = true
				best.DetectorID = c.ID
				best.DetectorName = c.Name
				best.Roll = total
			}
		}
	}

	if best.Detected {
		trap.IsDiscovered = true
	}

	return best
}

// CheckTrapDetectionSolo performs passive trap detection for a single character against a trap.
// Used when the thief scouts ahead alone.
func CheckTrapDetectionSolo(char *Character, trap *Trap, rng *rand.Rand) *TrapDetectionResult {
	if trap.IsTriggered || trap.IsDiscovered || trap.IsDisarmed {
		return &TrapDetectionResult{Detected: false, Trap: trap}
	}

	if !char.IsAlive {
		return &TrapDetectionResult{Detected: false, Trap: trap, DC: trap.Difficulty}
	}

	roll := rng.Intn(20) + 1 // d20
	bonus := char.Dexterity / 2

	switch char.Class {
	case ClassThief:
		bonus += ThiefDetectionBonus
	case ClassMagicUser:
		bonus += MagicUserDetectionBonus
	}

	total := roll + bonus
	result := &TrapDetectionResult{
		Detected:     total >= trap.Difficulty,
		DetectorID:   char.ID,
		DetectorName: char.Name,
		Roll:         total,
		DC:           trap.Difficulty,
		Trap:         trap,
	}

	if result.Detected {
		trap.IsDiscovered = true
	}

	return result
}

// TriggerTrap applies trap damage to the victim
func TriggerTrap(trap *Trap, victim *Character, rng *rand.Rand) *TrapTriggerResult {
	if trap.IsTriggered || trap.IsDisarmed {
		return &TrapTriggerResult{Triggered: false, Trap: trap}
	}

	trap.IsTriggered = true

	result := &TrapTriggerResult{
		Triggered:  true,
		Damage:     trap.Damage,
		VictimID:   victim.ID,
		VictimName: victim.Name,
		Trap:       trap,
	}

	victim.TakeDamage(trap.Damage)
	result.VictimDied = !victim.IsAlive

	return result
}

// AttemptDisarm has a character try to disarm a discovered trap
func AttemptDisarm(character *Character, trap *Trap, rng *rand.Rand) *TrapDisarmResult {
	if !trap.IsDiscovered || trap.IsTriggered || trap.IsDisarmed {
		return &TrapDisarmResult{
			Success:      false,
			DisarmerID:   character.ID,
			DisarmerName: character.Name,
			Trap:         trap,
		}
	}

	roll := rng.Intn(20) + 1 // d20
	bonus := character.Dexterity / 2

	if character.Class == ClassThief {
		bonus += ThiefDisarmBonus
	}

	total := roll + bonus

	result := &TrapDisarmResult{
		DisarmerID:   character.ID,
		DisarmerName: character.Name,
		Roll:         total,
		DC:           trap.Difficulty,
		Trap:         trap,
	}

	if total >= trap.Difficulty {
		result.Success = true
		trap.IsDisarmed = true
	} else {
		// Failure: trap triggers on the disarmer
		result.Success = false
		result.TriggerResult = TriggerTrap(trap, character, rng)
	}

	return result
}

// FormatTrapDetection returns a human-readable description of trap detection
func FormatTrapDetection(result *TrapDetectionResult) string {
	if result.Detected {
		return fmt.Sprintf("%s notices something: %s (DC %d)",
			result.DetectorName, result.Trap.Description, result.Trap.Difficulty)
	}
	return ""
}

// FormatTrapTrigger returns a human-readable description of a trap triggering
func FormatTrapTrigger(result *TrapTriggerResult) string {
	if !result.Triggered {
		return ""
	}
	msg := fmt.Sprintf("TRAP! %s — %s takes %d damage!",
		result.Trap.Description, result.VictimName, result.Damage)
	if result.VictimDied {
		msg += fmt.Sprintf(" %s has fallen!", result.VictimName)
	}
	return msg
}

// FormatDisarmResult returns a human-readable description of a disarm attempt
func FormatDisarmResult(result *TrapDisarmResult) string {
	if result.Success {
		return fmt.Sprintf("%s carefully disarms the trap. (Roll: %d vs DC %d)",
			result.DisarmerName, result.Roll, result.DC)
	}
	msg := fmt.Sprintf("%s fails to disarm the trap! (Roll: %d vs DC %d)",
		result.DisarmerName, result.Roll, result.DC)
	if result.TriggerResult != nil && result.TriggerResult.Triggered {
		msg += "\n" + FormatTrapTrigger(result.TriggerResult)
	}
	return msg
}