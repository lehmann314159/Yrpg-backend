package game

import "testing"

func makeTestParty(t *testing.T) *Party {
	t.Helper()
	p, err := NewParty([]PartyRequest{
		{Name: "Fighter", Class: ClassFighter},
		{Name: "Thief", Class: ClassThief},
		{Name: "Mage", Class: ClassMagicUser},
	})
	if err != nil {
		t.Fatalf("NewParty: %v", err)
	}
	return p
}

func TestNewParty_Valid(t *testing.T) {
	p := makeTestParty(t)
	if len(p.Characters) != 3 {
		t.Errorf("got %d characters, want 3", len(p.Characters))
	}
	if len(p.Formation) != 3 {
		t.Errorf("got %d formation entries, want 3", len(p.Formation))
	}
	if p.ID == "" {
		t.Error("party ID should not be empty")
	}
}

func TestNewParty_SingleCharacter(t *testing.T) {
	p, err := NewParty([]PartyRequest{{Name: "Solo", Class: ClassFighter}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Characters) != 1 {
		t.Errorf("got %d characters, want 1", len(p.Characters))
	}
}

func TestNewParty_RejectsEmpty(t *testing.T) {
	_, err := NewParty([]PartyRequest{})
	if err == nil {
		t.Error("expected error for empty party")
	}
}

func TestNewParty_RejectsTooMany(t *testing.T) {
	_, err := NewParty([]PartyRequest{
		{Name: "A", Class: ClassFighter},
		{Name: "B", Class: ClassFighter},
		{Name: "C", Class: ClassFighter},
		{Name: "D", Class: ClassFighter},
	})
	if err == nil {
		t.Error("expected error for >3 party members")
	}
}

func TestNewParty_RejectsDuplicateNames(t *testing.T) {
	_, err := NewParty([]PartyRequest{
		{Name: "Dupe", Class: ClassFighter},
		{Name: "Dupe", Class: ClassThief},
	})
	if err == nil {
		t.Error("expected error for duplicate names")
	}
}

func TestNewParty_RejectsInvalidClass(t *testing.T) {
	_, err := NewParty([]PartyRequest{
		{Name: "Bob", Class: "bard"},
	})
	if err == nil {
		t.Error("expected error for invalid class")
	}
}

func TestGetCharacter(t *testing.T) {
	p := makeTestParty(t)

	c := p.GetCharacter(p.Characters[0].ID)
	if c == nil {
		t.Fatal("expected to find character by ID")
	}
	if c.Name != "Fighter" {
		t.Errorf("got %q, want Fighter", c.Name)
	}

	if p.GetCharacter("nonexistent") != nil {
		t.Error("expected nil for unknown ID")
	}
}

func TestGetCharacterByName(t *testing.T) {
	p := makeTestParty(t)

	c := p.GetCharacterByName("Thief")
	if c == nil {
		t.Fatal("expected to find Thief")
	}
	if c.Class != ClassThief {
		t.Errorf("class = %s, want thief", c.Class)
	}

	if p.GetCharacterByName("Nobody") != nil {
		t.Error("expected nil for unknown name")
	}
}

func TestAliveCharacters(t *testing.T) {
	p := makeTestParty(t)

	alive := p.AliveCharacters()
	if len(alive) != 3 {
		t.Errorf("all alive: got %d, want 3", len(alive))
	}

	p.Characters[1].TakeDamage(999) // kill one
	alive = p.AliveCharacters()
	if len(alive) != 2 {
		t.Errorf("one dead: got %d, want 2", len(alive))
	}
}

func TestIsWiped(t *testing.T) {
	p := makeTestParty(t)

	if p.IsWiped() {
		t.Error("party should not be wiped")
	}

	for _, c := range p.Characters {
		c.TakeDamage(999)
	}
	if !p.IsWiped() {
		t.Error("party should be wiped")
	}
}

func TestPointCharacter(t *testing.T) {
	p := makeTestParty(t)

	point := p.PointCharacter()
	if point == nil {
		t.Fatal("expected a point character")
	}
	// First in formation
	if point.ID != p.Formation[0] {
		t.Errorf("point character ID = %s, want %s", point.ID, p.Formation[0])
	}

	// Kill the first, point should be second
	point.TakeDamage(999)
	point2 := p.PointCharacter()
	if point2 == nil {
		t.Fatal("expected point character after first dies")
	}
	if point2.ID != p.Formation[1] {
		t.Errorf("point character after death = %s, want %s", point2.ID, p.Formation[1])
	}
}

func TestHasClass(t *testing.T) {
	p := makeTestParty(t)

	if !p.HasClass(ClassThief) {
		t.Error("party should have a thief")
	}
	if !p.HasClass(ClassFighter) {
		t.Error("party should have a fighter")
	}

	// Kill thief
	thief := p.GetCharacterByName("Thief")
	thief.TakeDamage(999)
	if p.HasClass(ClassThief) {
		t.Error("dead thief should not count")
	}
}

func TestGetByClass(t *testing.T) {
	p := makeTestParty(t)

	fighter := p.GetByClass(ClassFighter)
	if fighter == nil || fighter.Class != ClassFighter {
		t.Error("expected to get the fighter")
	}

	// Kill fighter
	fighter.TakeDamage(999)
	if p.GetByClass(ClassFighter) != nil {
		t.Error("dead fighter should not be returned")
	}
}

func TestSetFormation(t *testing.T) {
	p := makeTestParty(t)

	// Valid reorder
	reversed := []string{p.Formation[2], p.Formation[1], p.Formation[0]}
	if err := p.SetFormation(reversed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Formation[0] != reversed[0] {
		t.Error("formation not updated")
	}

	// Wrong count
	if err := p.SetFormation([]string{p.Characters[0].ID}); err == nil {
		t.Error("expected error for wrong count")
	}

	// Invalid ID
	if err := p.SetFormation([]string{"bad", p.Characters[1].ID, p.Characters[2].ID}); err == nil {
		t.Error("expected error for invalid ID")
	}

	// Duplicate ID
	if err := p.SetFormation([]string{p.Characters[0].ID, p.Characters[0].ID, p.Characters[2].ID}); err == nil {
		t.Error("expected error for duplicate ID")
	}
}

func TestFirstNonThief(t *testing.T) {
	p := makeTestParty(t) // Fighter, Thief, Mage

	// First non-thief should be Fighter (first in formation)
	got := p.FirstNonThief()
	if got == nil {
		t.Fatal("expected a non-thief character")
	}
	if got.Class == ClassThief {
		t.Errorf("expected non-thief, got class=%s", got.Class)
	}
	if got.Name != "Fighter" {
		t.Errorf("expected Fighter (first in formation), got %s", got.Name)
	}

	// Kill the fighter — should return Mage
	got.TakeDamage(999)
	got2 := p.FirstNonThief()
	if got2 == nil {
		t.Fatal("expected Mage after fighter dies")
	}
	if got2.Name != "Mage" {
		t.Errorf("expected Mage, got %s", got2.Name)
	}

	// All-thief party returns nil
	allThieves, _ := NewParty([]PartyRequest{
		{Name: "T1", Class: ClassThief},
		{Name: "T2", Class: ClassThief},
	})
	if allThieves.FirstNonThief() != nil {
		t.Error("expected nil for all-thief party")
	}
}

func TestMoveAllToRoom(t *testing.T) {
	p := makeTestParty(t)
	p.Characters[1].TakeDamage(999) // kill one

	p.MoveAllToRoom("room-1")

	for _, c := range p.Characters {
		if c.IsAlive && c.CurrentRoomID != "room-1" {
			t.Errorf("alive char %s room = %s, want room-1", c.Name, c.CurrentRoomID)
		}
		if !c.IsAlive && c.CurrentRoomID == "room-1" {
			t.Errorf("dead char %s should not be moved", c.Name)
		}
	}
}
