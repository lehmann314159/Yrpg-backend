package game

import (
	"crypto/rand"
	"fmt"
	"time"
)

// ClassStatsTable maps each class to its base stats
var ClassStatsTable = map[CharacterClass]ClassStats{
	ClassFighter: {
		BaseHP:           30,
		BaseStrength:     14,
		BaseDexterity:    10,
		BaseIntelligence: 8,
		MovementRange:    2,
		InitiativeBonus:  0,
	},
	ClassMagicUser: {
		BaseHP:           16,
		BaseStrength:     8,
		BaseDexterity:    10,
		BaseIntelligence: 16,
		MovementRange:    2,
		InitiativeBonus:  0,
	},
	ClassThief: {
		BaseHP:           22,
		BaseStrength:     10,
		BaseDexterity:    14,
		BaseIntelligence: 10,
		MovementRange:    4,
		InitiativeBonus:  2,
	},
}

// ValidClasses is the set of allowed character classes
var ValidClasses = map[CharacterClass]bool{
	ClassFighter:   true,
	ClassMagicUser: true,
	ClassThief:     true,
}

// generateID creates a random hex ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// NewCharacter creates a new character of the given class with base stats
func NewCharacter(name string, class CharacterClass) (*Character, error) {
	if !ValidClasses[class] {
		return nil, fmt.Errorf("invalid class: %s", class)
	}
	if name == "" {
		return nil, fmt.Errorf("character name cannot be empty")
	}

	stats := ClassStatsTable[class]

	return &Character{
		ID:           generateID(),
		Name:         name,
		Class:        class,
		HP:           stats.BaseHP,
		MaxHP:        stats.BaseHP,
		Strength:     stats.BaseStrength,
		Dexterity:    stats.BaseDexterity,
		Intelligence: stats.BaseIntelligence,
		IsAlive:      true,
		CreatedAt:    time.Now(),
	}, nil
}

// TakeDamage applies damage to a character
func (c *Character) TakeDamage(damage int) {
	c.HP -= damage
	if c.HP <= 0 {
		c.HP = 0
		c.IsAlive = false
		now := time.Now()
		c.DiedAt = &now
	}
}

// Heal restores HP to a character, capped at MaxHP
func (c *Character) Heal(amount int) {
	c.HP += amount
	if c.HP > c.MaxHP {
		c.HP = c.MaxHP
	}
}

// Resurrect revives a dead character at the given HP
func (c *Character) Resurrect(hp int) {
	c.IsAlive = true
	c.DiedAt = nil
	c.HP = hp
	if c.HP > c.MaxHP {
		c.HP = c.MaxHP
	}
}

// GetMovementRange returns the character's movement range from their class
func (c *Character) GetMovementRange() int {
	stats, ok := ClassStatsTable[c.Class]
	if !ok {
		return 2
	}
	return stats.MovementRange
}

// GetInitiativeBonus returns the character's initiative bonus from their class
func (c *Character) GetInitiativeBonus() int {
	stats, ok := ClassStatsTable[c.Class]
	if !ok {
		return 0
	}
	return stats.InitiativeBonus
}

// CanEquip checks whether the character's class allows equipping the given item
func (c *Character) CanEquip(item *Item) bool {
	if item.Type != ItemWeapon && item.Type != ItemArmor {
		return false
	}
	if len(item.ClassRestriction) == 0 {
		return true
	}
	for _, allowed := range item.ClassRestriction {
		if allowed == c.Class {
			return true
		}
	}
	return false
}

// HealthStatus returns a human-readable status string
func (c *Character) HealthStatus() string {
	if !c.IsAlive {
		return "Dead"
	}
	pct := float64(c.HP) / float64(c.MaxHP)
	switch {
	case pct > 0.75:
		return "Healthy"
	case pct > 0.40:
		return "Wounded"
	default:
		return "Critical"
	}
}