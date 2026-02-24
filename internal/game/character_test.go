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

func TestTakeDamage(t *testing.T) {
	c, _ := NewCharacter("Fighter", ClassFighter) // HP=30

	c.TakeDamage(10)
	if c.HP != 20 {
		t.Errorf("HP after 10 damage = %d, want 20", c.HP)
	}
	if !c.IsAlive {
		t.Error("should still be alive")
	}

	// Kill
	c.TakeDamage(25)
	if c.HP != 0 {
		t.Errorf("HP after lethal damage = %d, want 0", c.HP)
	}
	if c.IsAlive {
		t.Error("should be dead")
	}
	if c.DiedAt == nil {
		t.Error("DiedAt should be set")
	}
}

func TestTakeDamage_ExactlyZero(t *testing.T) {
	c, _ := NewCharacter("Fighter", ClassFighter) // HP=30
	c.TakeDamage(30)
	if c.HP != 0 {
		t.Errorf("HP = %d, want 0", c.HP)
	}
	if c.IsAlive {
		t.Error("should be dead at 0 HP")
	}
}

func TestHeal(t *testing.T) {
	c, _ := NewCharacter("Fighter", ClassFighter) // HP=30, MaxHP=30
	c.TakeDamage(20)                              // HP=10

	c.Heal(5)
	if c.HP != 15 {
		t.Errorf("HP after heal = %d, want 15", c.HP)
	}

	// Cap at MaxHP
	c.Heal(100)
	if c.HP != 30 {
		t.Errorf("HP after over-heal = %d, want 30", c.HP)
	}
}

func TestResurrect(t *testing.T) {
	c, _ := NewCharacter("Fighter", ClassFighter) // HP=30
	c.TakeDamage(50)                              // dead, HP=0

	c.Resurrect(15)
	if !c.IsAlive {
		t.Error("should be alive after resurrect")
	}
	if c.HP != 15 {
		t.Errorf("HP = %d, want 15", c.HP)
	}
	if c.DiedAt != nil {
		t.Error("DiedAt should be nil after resurrect")
	}

	// Resurrect capped at MaxHP
	c.TakeDamage(50)
	c.Resurrect(999)
	if c.HP != 30 {
		t.Errorf("HP after over-resurrect = %d, want 30", c.HP)
	}
}

func TestGetMovementRange(t *testing.T) {
	fighter, _ := NewCharacter("F", ClassFighter)
	thief, _ := NewCharacter("T", ClassThief)
	mage, _ := NewCharacter("M", ClassMagicUser)

	if fighter.GetMovementRange() != 2 {
		t.Errorf("fighter movement = %d, want 2", fighter.GetMovementRange())
	}
	if thief.GetMovementRange() != 4 {
		t.Errorf("thief movement = %d, want 4", thief.GetMovementRange())
	}
	if mage.GetMovementRange() != 2 {
		t.Errorf("mage movement = %d, want 2", mage.GetMovementRange())
	}
}

func TestGetInitiativeBonus(t *testing.T) {
	fighter, _ := NewCharacter("F", ClassFighter)
	thief, _ := NewCharacter("T", ClassThief)
	mage, _ := NewCharacter("M", ClassMagicUser)

	if fighter.GetInitiativeBonus() != 0 {
		t.Errorf("fighter initiative bonus = %d, want 0", fighter.GetInitiativeBonus())
	}
	if thief.GetInitiativeBonus() != 2 {
		t.Errorf("thief initiative bonus = %d, want 2", thief.GetInitiativeBonus())
	}
	if mage.GetInitiativeBonus() != 0 {
		t.Errorf("mage initiative bonus = %d, want 0", mage.GetInitiativeBonus())
	}
}

func TestCanEquip(t *testing.T) {
	fighter, _ := NewCharacter("F", ClassFighter)
	thief, _ := NewCharacter("T", ClassThief)

	weapon := &Item{Type: ItemWeapon}
	armor := &Item{Type: ItemArmor}
	consumable := &Item{Type: ItemConsumable}
	restrictedWeapon := &Item{Type: ItemWeapon, ClassRestriction: []CharacterClass{ClassThief}}

	// Weapon and armor with no restriction — anyone can equip
	if !fighter.CanEquip(weapon) {
		t.Error("fighter should equip unrestricted weapon")
	}
	if !fighter.CanEquip(armor) {
		t.Error("fighter should equip unrestricted armor")
	}

	// Consumable — cannot equip
	if fighter.CanEquip(consumable) {
		t.Error("should not equip consumable")
	}

	// Class restriction
	if fighter.CanEquip(restrictedWeapon) {
		t.Error("fighter should not equip thief-only weapon")
	}
	if !thief.CanEquip(restrictedWeapon) {
		t.Error("thief should equip thief-only weapon")
	}
}

func TestHealthStatus(t *testing.T) {
	c, _ := NewCharacter("F", ClassFighter) // HP=30, MaxHP=30

	if c.HealthStatus() != "Healthy" {
		t.Errorf("full HP status = %q, want Healthy", c.HealthStatus())
	}

	c.HP = 23 // 23/30 = 76.7% > 75%
	if c.HealthStatus() != "Healthy" {
		t.Errorf("76%% HP status = %q, want Healthy", c.HealthStatus())
	}

	c.HP = 22 // 22/30 = 73.3% <= 75%
	if c.HealthStatus() != "Wounded" {
		t.Errorf("73%% HP status = %q, want Wounded", c.HealthStatus())
	}

	c.HP = 12 // 12/30 = 40% <= 40%
	if c.HealthStatus() != "Critical" {
		t.Errorf("40%% HP status = %q, want Critical", c.HealthStatus())
	}

	c.TakeDamage(100)
	if c.HealthStatus() != "Dead" {
		t.Errorf("dead status = %q, want Dead", c.HealthStatus())
	}
}
