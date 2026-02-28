package game

import "fmt"

const (
	MinPartySize = 1
	MaxPartySize = 3
)

// PartyRequest is the input for creating a new party
type PartyRequest struct {
	Name  string         `json:"name"`
	Class CharacterClass `json:"class"`
}

// NewParty creates a party from the given character requests
func NewParty(requests []PartyRequest) (*Party, error) {
	if len(requests) < MinPartySize || len(requests) > MaxPartySize {
		return nil, fmt.Errorf("party must have %d-%d characters, got %d", MinPartySize, MaxPartySize, len(requests))
	}

	party := &Party{
		ID:         generateID(),
		Characters: make([]*Character, 0, len(requests)),
		Formation:  make([]string, 0, len(requests)),
	}

	names := make(map[string]bool)
	for _, req := range requests {
		if names[req.Name] {
			return nil, fmt.Errorf("duplicate character name: %s", req.Name)
		}
		names[req.Name] = true

		char, err := NewCharacter(req.Name, req.Class)
		if err != nil {
			return nil, fmt.Errorf("failed to create character %q: %w", req.Name, err)
		}

		party.Characters = append(party.Characters, char)
		party.Formation = append(party.Formation, char.ID)
	}

	return party, nil
}

// GetCharacter returns a character by ID, or nil if not found
func (p *Party) GetCharacter(id string) *Character {
	for _, c := range p.Characters {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// GetCharacterByName returns a character by name, or nil if not found
func (p *Party) GetCharacterByName(name string) *Character {
	for _, c := range p.Characters {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// AliveCharacters returns all living party members
func (p *Party) AliveCharacters() []*Character {
	alive := make([]*Character, 0)
	for _, c := range p.Characters {
		if c.IsAlive {
			alive = append(alive, c)
		}
	}
	return alive
}

// IsWiped returns true if all party members are dead
func (p *Party) IsWiped() bool {
	return len(p.AliveCharacters()) == 0
}

// PointCharacter returns the first living character in formation (the one who triggers traps)
func (p *Party) PointCharacter() *Character {
	for _, id := range p.Formation {
		c := p.GetCharacter(id)
		if c != nil && c.IsAlive {
			return c
		}
	}
	return nil
}

// FirstNonThief returns the first living non-thief character in formation order
func (p *Party) FirstNonThief() *Character {
	for _, id := range p.Formation {
		c := p.GetCharacter(id)
		if c != nil && c.IsAlive && c.Class != ClassThief {
			return c
		}
	}
	return nil
}

// HasClass returns true if the party has a living member of the given class
func (p *Party) HasClass(class CharacterClass) bool {
	for _, c := range p.Characters {
		if c.IsAlive && c.Class == class {
			return true
		}
	}
	return false
}

// GetByClass returns the first living character of the given class, or nil
func (p *Party) GetByClass(class CharacterClass) *Character {
	for _, c := range p.Characters {
		if c.IsAlive && c.Class == class {
			return c
		}
	}
	return nil
}

// SetFormation updates the marching order. All IDs must be valid party member IDs.
func (p *Party) SetFormation(order []string) error {
	if len(order) != len(p.Characters) {
		return fmt.Errorf("formation must include all %d party members", len(p.Characters))
	}

	valid := make(map[string]bool)
	for _, c := range p.Characters {
		valid[c.ID] = true
	}

	seen := make(map[string]bool)
	for _, id := range order {
		if !valid[id] {
			return fmt.Errorf("invalid character ID in formation: %s", id)
		}
		if seen[id] {
			return fmt.Errorf("duplicate character ID in formation: %s", id)
		}
		seen[id] = true
	}

	p.Formation = order
	return nil
}

// MoveAllToRoom moves all living party members to the given room
func (p *Party) MoveAllToRoom(roomID string) {
	for _, c := range p.Characters {
		if c.IsAlive {
			c.CurrentRoomID = roomID
		}
	}
}
