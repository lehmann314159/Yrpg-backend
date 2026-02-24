package game

import "testing"

func TestBuildSnapshot_MagicUserSpellFields(t *testing.T) {
	gs := NewGameState("test-session")
	party, err := NewParty([]PartyRequest{
		{Name: "Gandalf", Class: ClassMagicUser},
		{Name: "Conan", Class: ClassFighter},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gs.Party = party
	gs.Dungeon = &Dungeon{ID: "d1", Depth: 1}

	room := &Room{ID: "room1", Name: "Test Room", X: 0, Y: 0}
	gs.AddRoom(room)
	party.MoveAllToRoom(room.ID)

	snap := gs.BuildSnapshot()
	if snap.Party == nil {
		t.Fatal("snapshot party is nil")
	}

	// Find magic_user and fighter views
	var mageView, fighterView *CharacterView
	for _, cv := range snap.Party.Characters {
		switch cv.Name {
		case "Gandalf":
			mageView = cv
		case "Conan":
			fighterView = cv
		}
	}

	if mageView == nil {
		t.Fatal("magic_user view not found")
	}
	if fighterView == nil {
		t.Fatal("fighter view not found")
	}

	// Magic user should have spell fields populated
	if mageView.MaxSpellSlots != 3 {
		t.Errorf("mage MaxSpellSlots = %d, want 3", mageView.MaxSpellSlots)
	}
	if mageView.SpellSlots != 3 {
		t.Errorf("mage SpellSlots = %d, want 3", mageView.SpellSlots)
	}
	if len(mageView.KnownSpells) != 2 {
		t.Errorf("mage KnownSpells length = %d, want 2", len(mageView.KnownSpells))
	}
	if mageView.Class != "magic_user" {
		t.Errorf("mage Class = %s, want magic_user", mageView.Class)
	}

	// Fighter should have zero spell fields
	if fighterView.MaxSpellSlots != 0 {
		t.Errorf("fighter MaxSpellSlots = %d, want 0", fighterView.MaxSpellSlots)
	}
	if fighterView.SpellSlots != 0 {
		t.Errorf("fighter SpellSlots = %d, want 0", fighterView.SpellSlots)
	}
	if len(fighterView.KnownSpells) != 0 {
		t.Errorf("fighter KnownSpells should be empty, got %v", fighterView.KnownSpells)
	}
}
