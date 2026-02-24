package game

import (
	"math/rand"
	"testing"
)

// --- Test helpers ---

func newTestMagicUser() *Character {
	c, _ := NewCharacter("Gandalf", ClassMagicUser)
	return c
}

func newTestGameState() *GameState {
	gs := NewGameState("test-session")
	party, _ := NewParty([]PartyRequest{
		{Name: "Gandalf", Class: ClassMagicUser},
		{Name: "Conan", Class: ClassFighter},
	})
	gs.Party = party
	gs.Dungeon = &Dungeon{ID: "d1", Depth: 3}

	room := &Room{ID: "room1", Name: "Test Room", X: 1, Y: 1}
	gs.AddRoom(room)
	party.MoveAllToRoom(room.ID)
	gs.MarkRoomVisited(room.ID)

	return gs
}

// --- CalcMaxSlots ---

func TestCalcMaxSlots(t *testing.T) {
	tests := []struct {
		intelligence int
		want         int
	}{
		{0, 0},
		{4, 0},
		{5, 1},
		{10, 2},
		{16, 3},
		{24, 4},
	}

	for _, tt := range tests {
		got := CalcMaxSlots(tt.intelligence)
		if got != tt.want {
			t.Errorf("CalcMaxSlots(%d) = %d, want %d", tt.intelligence, got, tt.want)
		}
	}
}

// --- CanCast ---

func TestCanCast(t *testing.T) {
	t.Run("rejects non-magic_user", func(t *testing.T) {
		fighter, _ := NewCharacter("Conan", ClassFighter)
		ok, _ := CanCast(fighter, "heal")
		if ok {
			t.Error("fighter should not be able to cast")
		}
	})

	t.Run("rejects unknown spell", func(t *testing.T) {
		mage := newTestMagicUser()
		ok, reason := CanCast(mage, "meteor")
		if ok {
			t.Error("should reject unknown spell")
		}
		if reason != "Unknown spell: meteor" {
			t.Errorf("unexpected reason: %s", reason)
		}
	})

	t.Run("rejects spell not known", func(t *testing.T) {
		mage := newTestMagicUser()
		// mage knows heal and fireball, not lightning
		ok, _ := CanCast(mage, "lightning")
		if ok {
			t.Error("should reject spell not in known list")
		}
	})

	t.Run("rejects when no slots", func(t *testing.T) {
		mage := newTestMagicUser()
		mage.SpellSlots = 0
		ok, reason := CanCast(mage, "heal")
		if ok {
			t.Error("should reject when no slots")
		}
		if reason != "Gandalf has no spell slots remaining." {
			t.Errorf("unexpected reason: %s", reason)
		}
	})

	t.Run("success", func(t *testing.T) {
		mage := newTestMagicUser()
		ok, reason := CanCast(mage, "heal")
		if !ok {
			t.Errorf("should succeed, got reason: %s", reason)
		}
	})
}

// --- SpendSlot ---

func TestSpendSlot(t *testing.T) {
	t.Run("decrements", func(t *testing.T) {
		mage := newTestMagicUser()
		before := mage.SpellSlots
		SpendSlot(mage)
		if mage.SpellSlots != before-1 {
			t.Errorf("SpellSlots = %d, want %d", mage.SpellSlots, before-1)
		}
	})

	t.Run("does not go below zero", func(t *testing.T) {
		mage := newTestMagicUser()
		mage.SpellSlots = 0
		SpendSlot(mage)
		if mage.SpellSlots != 0 {
			t.Errorf("SpellSlots = %d, want 0", mage.SpellSlots)
		}
	})
}

// --- RechargeSlots ---

func TestRechargeSlots(t *testing.T) {
	mage := newTestMagicUser()
	mage.SpellSlots = 0
	RechargeSlots(mage)
	if mage.SpellSlots != mage.MaxSpellSlots {
		t.Errorf("SpellSlots = %d, want %d", mage.SpellSlots, mage.MaxSpellSlots)
	}
}

// --- LearnSpell ---

func TestLearnSpell(t *testing.T) {
	t.Run("learns new spell", func(t *testing.T) {
		mage := newTestMagicUser()
		ok := LearnSpell(mage, "lightning")
		if !ok {
			t.Error("should return true for newly learned spell")
		}
		found := false
		for _, s := range mage.KnownSpells {
			if s == "lightning" {
				found = true
			}
		}
		if !found {
			t.Error("lightning should be in KnownSpells")
		}
	})

	t.Run("rejects duplicate", func(t *testing.T) {
		mage := newTestMagicUser()
		ok := LearnSpell(mage, "heal") // already known
		if ok {
			t.Error("should return false for duplicate")
		}
	})

	t.Run("rejects unknown spell ID", func(t *testing.T) {
		mage := newTestMagicUser()
		ok := LearnSpell(mage, "meteor")
		if ok {
			t.Error("should return false for unknown spell")
		}
	})
}

// --- CanRest ---

func TestCanRest(t *testing.T) {
	t.Run("blocked in combat", func(t *testing.T) {
		gs := newTestGameState()
		gs.Mode = ModeCombat
		gs.Combat = &CombatState{IsActive: true}
		ok, _ := CanRest(gs)
		if ok {
			t.Error("should be blocked in combat")
		}
	})

	t.Run("blocked with monsters in room", func(t *testing.T) {
		gs := newTestGameState()
		room := gs.GetCurrentRoom()
		gs.AddMonster(&Monster{
			ID:      "m1",
			Name:    "Goblin",
			RoomID:  room.ID,
			IsAlive: true,
		})
		ok, reason := CanRest(gs)
		if ok {
			t.Error("should be blocked with monsters")
		}
		if reason != "Cannot rest — enemies are still in the room!" {
			t.Errorf("unexpected reason: %s", reason)
		}
	})

	t.Run("success in cleared room", func(t *testing.T) {
		gs := newTestGameState()
		ok, reason := CanRest(gs)
		if !ok {
			t.Errorf("should succeed, got reason: %s", reason)
		}
	})
}

// --- CalcRestHealing ---

func TestCalcRestHealing(t *testing.T) {
	tests := []struct {
		maxHP int
		want  int
	}{
		{16, 6},  // (16+2)/3 = 6
		{30, 10}, // (30+2)/3 = 10 (integer, exact after +2)
		{22, 8},  // (22+2)/3 = 8
		{1, 1},   // (1+2)/3 = 1
	}

	for _, tt := range tests {
		c := &Character{MaxHP: tt.maxHP}
		got := CalcRestHealing(c)
		if got != tt.want {
			t.Errorf("CalcRestHealing(MaxHP=%d) = %d, want %d", tt.maxHP, got, tt.want)
		}
	}
}

// --- CalcAmbushChance ---

func TestCalcAmbushChance(t *testing.T) {
	tests := []struct {
		depth     int
		hasThief  bool
		want      int
	}{
		{0, false, 30},  // base 30
		{5, false, 40},  // 30 + 5*2 = 40
		{5, true, 20},   // 40 / 2 = 20
		{30, false, 80}, // 30 + 60 = 90, capped at 80
		{30, true, 45},  // (30+60)/2 = 45
	}

	for _, tt := range tests {
		got := CalcAmbushChance(tt.depth, tt.hasThief)
		if got != tt.want {
			t.Errorf("CalcAmbushChance(%d, %v) = %d, want %d", tt.depth, tt.hasThief, got, tt.want)
		}
	}
}

// --- RollAmbush ---

func TestRollAmbush(t *testing.T) {
	t.Run("chance 100 always true", func(t *testing.T) {
		rng := rand.New(rand.NewSource(42))
		for i := 0; i < 100; i++ {
			if !RollAmbush(rng, 100) {
				t.Fatal("chance=100 should always trigger ambush")
			}
		}
	})

	t.Run("chance 0 always false", func(t *testing.T) {
		rng := rand.New(rand.NewSource(42))
		for i := 0; i < 100; i++ {
			if RollAmbush(rng, 0) {
				t.Fatal("chance=0 should never trigger ambush")
			}
		}
	})
}
