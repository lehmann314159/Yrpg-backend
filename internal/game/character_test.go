package game

import "testing"

func TestNewCharacter_MagicUser(t *testing.T) {
	c, err := NewCharacter("Merlin", ClassMagicUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Intelligence != 16 {
		t.Errorf("Intelligence = %d, want 16", c.Intelligence)
	}

	// CalcMaxSlots(16) = 3
	if c.MaxSpellSlots != 3 {
		t.Errorf("MaxSpellSlots = %d, want 3", c.MaxSpellSlots)
	}
	if c.SpellSlots != 3 {
		t.Errorf("SpellSlots = %d, want 3", c.SpellSlots)
	}

	if len(c.KnownSpells) != 2 {
		t.Fatalf("KnownSpells length = %d, want 2", len(c.KnownSpells))
	}

	spells := map[string]bool{}
	for _, s := range c.KnownSpells {
		spells[s] = true
	}
	if !spells["heal"] {
		t.Error("magic_user should know heal")
	}
	if !spells["fireball"] {
		t.Error("magic_user should know fireball")
	}
}

func TestNewCharacter_Fighter(t *testing.T) {
	c, err := NewCharacter("Conan", ClassFighter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.MaxSpellSlots != 0 {
		t.Errorf("MaxSpellSlots = %d, want 0", c.MaxSpellSlots)
	}
	if c.SpellSlots != 0 {
		t.Errorf("SpellSlots = %d, want 0", c.SpellSlots)
	}
	if len(c.KnownSpells) != 0 {
		t.Errorf("KnownSpells should be empty, got %v", c.KnownSpells)
	}
}

func TestNewCharacter_Thief(t *testing.T) {
	c, err := NewCharacter("Bilbo", ClassThief)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.MaxSpellSlots != 0 {
		t.Errorf("MaxSpellSlots = %d, want 0", c.MaxSpellSlots)
	}
	if c.SpellSlots != 0 {
		t.Errorf("SpellSlots = %d, want 0", c.SpellSlots)
	}
	if len(c.KnownSpells) != 0 {
		t.Errorf("KnownSpells should be empty, got %v", c.KnownSpells)
	}
}
