package game

import "math/rand"

// SpellDefinition describes a castable spell
type SpellDefinition struct {
	ID         string
	Name       string
	ManaCost   int  // spell slot cost (always 1 for now)
	CombatOnly bool // true = can only be cast during combat
}

// SpellRegistry maps spell IDs to their definitions.
// Spell IDs match the ScrollEffect strings used in procgen item templates.
var SpellRegistry = map[string]*SpellDefinition{
	"heal":      {ID: "heal", Name: "Heal", ManaCost: 1, CombatOnly: false},
	"fireball":  {ID: "fireball", Name: "Fireball", ManaCost: 1, CombatOnly: true},
	"lightning": {ID: "lightning", Name: "Lightning Bolt", ManaCost: 1, CombatOnly: true},
	"shield":    {ID: "shield", Name: "Shield", ManaCost: 1, CombatOnly: true},
	"sleep":     {ID: "sleep", Name: "Sleep", ManaCost: 1, CombatOnly: true},
	"resurrect": {ID: "resurrect", Name: "Resurrect", ManaCost: 1, CombatOnly: false},
}

// CalcMaxSlots derives max spell slots from intelligence: intelligence / 4
func CalcMaxSlots(intelligence int) int {
	return intelligence / 4
}

// CanCast checks whether a character can cast a specific spell.
// Returns (ok, errorReason).
func CanCast(char *Character, spellID string) (bool, string) {
	if char.Class != ClassMagicUser {
		return false, "Only a magic_user can cast spells."
	}

	spell, exists := SpellRegistry[spellID]
	if !exists {
		return false, "Unknown spell: " + spellID
	}

	// Check the character knows the spell
	known := false
	for _, s := range char.KnownSpells {
		if s == spellID {
			known = true
			break
		}
	}
	if !known {
		return false, char.Name + " does not know the spell " + spell.Name + "."
	}

	if char.SpellSlots < spell.ManaCost {
		return false, char.Name + " has no spell slots remaining."
	}

	return true, ""
}

// SpendSlot decrements a character's available spell slots by 1
func SpendSlot(char *Character) {
	char.SpellSlots--
	if char.SpellSlots < 0 {
		char.SpellSlots = 0
	}
}

// RechargeSlots restores all spell slots to max
func RechargeSlots(char *Character) {
	char.SpellSlots = char.MaxSpellSlots
}

// LearnSpell adds a spell to a character's known spells if not already known.
// Returns true if the spell was newly learned.
func LearnSpell(char *Character, spellID string) bool {
	if _, exists := SpellRegistry[spellID]; !exists {
		return false
	}
	for _, s := range char.KnownSpells {
		if s == spellID {
			return false // already known
		}
	}
	char.KnownSpells = append(char.KnownSpells, spellID)
	return true
}

// --- Rest Mechanic ---

// CanRest checks if the party can rest (not in combat, current room is cleared)
func CanRest(state *GameState) (bool, string) {
	if state.InCombat() {
		return false, "Cannot rest during combat."
	}
	room := state.GetCurrentRoom()
	if room == nil {
		return false, "No current room."
	}
	if state.HasMonstersInRoom(room.ID) {
		return false, "Cannot rest — enemies are still in the room!"
	}
	return true, ""
}

// CalcRestHealing returns the HP to restore per character: ceiling of 33% of MaxHP
func CalcRestHealing(char *Character) int {
	return (char.MaxHP + 2) / 3
}

// CalcAmbushChance returns the ambush percentage (0-80).
// Base 30% + 2% per room depth, halved if thief is on watch.
func CalcAmbushChance(roomDepth int, hasThiefWatch bool) int {
	chance := 30 + (roomDepth * 2)
	if hasThiefWatch {
		chance /= 2
	}
	if chance > 80 {
		chance = 80
	}
	return chance
}

// RollAmbush returns true if an ambush occurs (d100 <= chance)
func RollAmbush(rng *rand.Rand, chance int) bool {
	roll := rng.Intn(100) + 1 // 1-100
	return roll <= chance
}