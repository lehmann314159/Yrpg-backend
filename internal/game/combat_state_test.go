package game

import (
	"math/rand"
	"testing"
)

func TestInitCombat(t *testing.T) {
	party, err := NewParty([]PartyRequest{
		{Name: "Fighter", Class: ClassFighter},
		{Name: "Thief", Class: ClassThief},
	})
	if err != nil {
		t.Fatalf("NewParty: %v", err)
	}

	monsters := []*Monster{
		{ID: "m1", Name: "Goblin", HP: 10, MaxHP: 10, Dexterity: 8, IsAlive: true},
	}

	rng := rand.New(rand.NewSource(42))
	cs := InitCombat(party, monsters, "prev-room", nil, rng)

	if !cs.IsActive {
		t.Error("combat should be active")
	}
	if cs.PreviousRoomID != "prev-room" {
		t.Errorf("PreviousRoomID = %q, want prev-room", cs.PreviousRoomID)
	}

	// Should have 3 combatants (2 party + 1 monster)
	if len(cs.Combatants) != 3 {
		t.Fatalf("combatant count = %d, want 3", len(cs.Combatants))
	}

	// Party should be in row 0
	players := cs.GetPlayerCombatants()
	for _, p := range players {
		if p.GridY != 0 {
			t.Errorf("player %s at Y=%d, want 0", p.Name, p.GridY)
		}
	}

	// Monster should be in row 4 or 5
	enemies := cs.GetEnemyCombatants()
	for _, e := range enemies {
		if e.GridY < 4 {
			t.Errorf("enemy %s at Y=%d, want >= 4", e.Name, e.GridY)
		}
	}

	// All should have initiative > 0
	for _, c := range cs.Combatants {
		if c.Initiative <= 0 {
			t.Errorf("combatant %s initiative = %d, want > 0", c.Name, c.Initiative)
		}
	}
}

func TestGetCurrentCombatant(t *testing.T) {
	cs := newTestCombatState()
	cs.Combatants = []*Combatant{
		{ID: "a", Name: "A", IsAlive: true},
		{ID: "b", Name: "B", IsAlive: true},
	}
	cs.CurrentTurnIdx = 0

	current := cs.GetCurrentCombatant()
	if current == nil || current.ID != "a" {
		t.Errorf("current = %v, want a", current)
	}

	cs.CurrentTurnIdx = 10 // OOB
	if cs.GetCurrentCombatant() != nil {
		t.Error("expected nil for OOB index")
	}
}

func TestGetCombatant(t *testing.T) {
	cs := newTestCombatState()
	cs.Combatants = []*Combatant{
		{ID: "a", Name: "A"},
		{ID: "b", Name: "B"},
	}

	if cs.GetCombatant("a") == nil {
		t.Error("expected to find combatant a")
	}
	if cs.GetCombatant("missing") != nil {
		t.Error("expected nil for unknown ID")
	}
}

func TestGetPlayerCombatants_GetEnemyCombatants(t *testing.T) {
	cs := newTestCombatState()
	cs.Combatants = []*Combatant{
		{ID: "p1", IsPlayerChar: true},
		{ID: "p2", IsPlayerChar: true},
		{ID: "e1", IsPlayerChar: false},
	}

	players := cs.GetPlayerCombatants()
	if len(players) != 2 {
		t.Errorf("player count = %d, want 2", len(players))
	}
	enemies := cs.GetEnemyCombatants()
	if len(enemies) != 1 {
		t.Errorf("enemy count = %d, want 1", len(enemies))
	}
}

func TestAllEnemiesDead(t *testing.T) {
	cs := newTestCombatState()
	cs.Combatants = []*Combatant{
		{ID: "p1", IsPlayerChar: true, IsAlive: true},
		{ID: "e1", IsPlayerChar: false, IsAlive: true},
	}

	if cs.AllEnemiesDead() {
		t.Error("enemies are alive, should be false")
	}

	cs.Combatants[1].IsAlive = false
	if !cs.AllEnemiesDead() {
		t.Error("all enemies dead, should be true")
	}
}

func TestAllPlayersDead(t *testing.T) {
	cs := newTestCombatState()
	cs.Combatants = []*Combatant{
		{ID: "p1", IsPlayerChar: true, IsAlive: true},
		{ID: "e1", IsPlayerChar: false, IsAlive: true},
	}

	if cs.AllPlayersDead() {
		t.Error("player alive, should be false")
	}

	cs.Combatants[0].IsAlive = false
	if !cs.AllPlayersDead() {
		t.Error("all players dead, should be true")
	}
}

func TestPlaceOnGrid_RemoveFromGrid(t *testing.T) {
	cs := newTestCombatState()
	c := &Combatant{ID: "c1", GridX: 0, GridY: 0}

	cs.PlaceOnGrid(c, 2, 3)
	if c.GridX != 2 || c.GridY != 3 {
		t.Errorf("position = (%d,%d), want (2,3)", c.GridX, c.GridY)
	}
	if cs.Grid[3][2].OccupantID == nil || *cs.Grid[3][2].OccupantID != "c1" {
		t.Error("grid cell should have occupant c1")
	}

	// Move to new position clears old cell
	cs.PlaceOnGrid(c, 4, 4)
	if cs.Grid[3][2].OccupantID != nil {
		t.Error("old cell should be cleared")
	}
	if cs.Grid[4][4].OccupantID == nil || *cs.Grid[4][4].OccupantID != "c1" {
		t.Error("new cell should have occupant")
	}

	cs.RemoveFromGrid(c)
	if cs.Grid[4][4].OccupantID != nil {
		t.Error("cell should be empty after remove")
	}
}

func TestIsCellOccupied(t *testing.T) {
	cs := newTestCombatState()
	c := &Combatant{ID: "c1"}
	cs.PlaceOnGrid(c, 1, 1)

	if !cs.IsCellOccupied(1, 1) {
		t.Error("cell (1,1) should be occupied")
	}
	if cs.IsCellOccupied(0, 0) {
		t.Error("cell (0,0) should be empty")
	}
	// Out of bounds = blocked
	if !cs.IsCellOccupied(-1, 0) {
		t.Error("out of bounds should be treated as occupied")
	}
	if !cs.IsCellOccupied(GridWidth, 0) {
		t.Error("out of bounds should be treated as occupied")
	}
}

func TestIsCellOccupied_Blocked(t *testing.T) {
	cs := newTestCombatState()
	cs.Grid[2][2].IsBlocked = true

	if !cs.IsCellOccupied(2, 2) {
		t.Error("blocked cell should be occupied")
	}
}

func TestAddBuff_GetACBonus(t *testing.T) {
	c := &Combatant{ID: "c1"}

	if c.GetACBonus() != 0 {
		t.Error("initial AC bonus should be 0")
	}

	c.AddBuff(BuffACBonus, 4, 2, "Shield")
	if c.GetACBonus() != 4 {
		t.Errorf("AC bonus = %d, want 4", c.GetACBonus())
	}

	c.AddBuff(BuffACBonus, 2, 1, "Defend")
	if c.GetACBonus() != 6 {
		t.Errorf("stacked AC bonus = %d, want 6", c.GetACBonus())
	}
}

func TestIsSleeping_RemoveSleep(t *testing.T) {
	c := &Combatant{ID: "c1"}

	if c.IsSleeping() {
		t.Error("should not be sleeping initially")
	}

	c.AddBuff(BuffSleep, 0, 3, "Sleep Spell")
	if !c.IsSleeping() {
		t.Error("should be sleeping after sleep buff")
	}

	c.RemoveSleep()
	if c.IsSleeping() {
		t.Error("should not be sleeping after RemoveSleep")
	}
}

func TestTickBuffs(t *testing.T) {
	c := &Combatant{ID: "c1"}
	c.AddBuff(BuffACBonus, 4, 2, "Shield")
	c.AddBuff(BuffACBonus, 2, 1, "Defend")

	expired := c.TickBuffs()
	// "Defend" (1 round) should expire
	if len(expired) != 1 || expired[0] != "Defend" {
		t.Errorf("expired = %v, want [Defend]", expired)
	}
	if len(c.Buffs) != 1 {
		t.Errorf("remaining buffs = %d, want 1", len(c.Buffs))
	}
	if c.GetACBonus() != 4 {
		t.Errorf("AC bonus after tick = %d, want 4", c.GetACBonus())
	}

	// Tick again — Shield expires
	expired = c.TickBuffs()
	if len(expired) != 1 || expired[0] != "Shield" {
		t.Errorf("expired = %v, want [Shield]", expired)
	}
	if len(c.Buffs) != 0 {
		t.Errorf("remaining buffs = %d, want 0", len(c.Buffs))
	}
}

func TestAdvanceTurn(t *testing.T) {
	cs := newTestCombatState()
	cs.RoundNumber = 1
	cs.CurrentTurnIdx = 0
	cs.Combatants = []*Combatant{
		{ID: "a", Name: "A", IsAlive: true},
		{ID: "b", Name: "B", IsAlive: true},
		{ID: "c", Name: "C", IsAlive: false}, // dead, should be skipped
	}

	newRound := cs.AdvanceTurn()
	if newRound {
		t.Error("should not be a new round yet")
	}
	if cs.CurrentTurnIdx != 1 {
		t.Errorf("turn index = %d, want 1", cs.CurrentTurnIdx)
	}

	// Advance past dead combatant C, wraps to new round, lands on A
	newRound = cs.AdvanceTurn()
	if !newRound {
		t.Error("should be a new round after wrapping")
	}
	if cs.RoundNumber != 2 {
		t.Errorf("round = %d, want 2", cs.RoundNumber)
	}
	if cs.CurrentTurnIdx != 0 {
		t.Errorf("turn index = %d, want 0 (first alive)", cs.CurrentTurnIdx)
	}
}

func TestAdvanceTurn_AllDead(t *testing.T) {
	cs := newTestCombatState()
	cs.RoundNumber = 1
	cs.CurrentTurnIdx = 0
	cs.Combatants = []*Combatant{
		{ID: "a", IsAlive: false},
		{ID: "b", IsAlive: false},
	}

	cs.AdvanceTurn()
	if cs.IsActive {
		t.Error("combat should become inactive when all combatants dead")
	}
}

func TestInitScoutCombat(t *testing.T) {
	thief, err := NewCharacter("Shadow", ClassThief)
	if err != nil {
		t.Fatalf("NewCharacter: %v", err)
	}

	monsters := []*Monster{
		{ID: "m1", Name: "Goblin", HP: 10, MaxHP: 10, Dexterity: 8, IsAlive: true},
		{ID: "m2", Name: "Orc", HP: 20, MaxHP: 20, Dexterity: 10, IsAlive: true},
	}

	rng := rand.New(rand.NewSource(42))
	cs := InitScoutCombat(thief, monsters, "prev-room", rng)

	if !cs.IsActive {
		t.Error("combat should be active")
	}
	if !cs.IsScoutPhase {
		t.Error("should be scout phase")
	}
	if cs.ScoutID != thief.ID {
		t.Errorf("ScoutID = %q, want %q", cs.ScoutID, thief.ID)
	}
	if cs.PreviousRoomID != "prev-room" {
		t.Errorf("PreviousRoomID = %q, want prev-room", cs.PreviousRoomID)
	}

	// Should have 3 combatants (1 thief + 2 monsters)
	if len(cs.Combatants) != 3 {
		t.Fatalf("combatant count = %d, want 3", len(cs.Combatants))
	}

	// Thief should be first (initiative 100)
	if cs.Combatants[0].ID != thief.ID {
		t.Error("thief should be first combatant")
	}
	if cs.Combatants[0].Initiative != 100 {
		t.Errorf("thief initiative = %d, want 100", cs.Combatants[0].Initiative)
	}
	if !cs.Combatants[0].IsHidden {
		t.Error("thief should start hidden")
	}

	// Thief should be in row 0
	thiefCombatant := cs.GetCombatant(thief.ID)
	if thiefCombatant.GridY != 0 {
		t.Errorf("thief GridY = %d, want 0", thiefCombatant.GridY)
	}

	// Monsters should be in rows 4-5 with 0 initiative
	for _, c := range cs.GetEnemyCombatants() {
		if c.GridY < 4 {
			t.Errorf("monster %s at Y=%d, want >= 4", c.Name, c.GridY)
		}
		if c.Initiative != 0 {
			t.Errorf("monster %s initiative = %d, want 0 (frozen)", c.Name, c.Initiative)
		}
	}
}

func TestAddPartyToCombat(t *testing.T) {
	party, err := NewParty([]PartyRequest{
		{Name: "Shadow", Class: ClassThief},
		{Name: "Conan", Class: ClassFighter},
		{Name: "Gandalf", Class: ClassMagicUser},
	})
	if err != nil {
		t.Fatalf("NewParty: %v", err)
	}

	thief := party.GetByClass(ClassThief)
	monsters := []*Monster{
		{ID: "m1", Name: "Goblin", HP: 10, MaxHP: 10, Dexterity: 8, IsAlive: true},
	}

	rng := rand.New(rand.NewSource(42))
	cs := InitScoutCombat(thief, monsters, "prev-room", rng)

	// Verify only 2 combatants before adding party (thief + 1 monster)
	if len(cs.Combatants) != 2 {
		t.Fatalf("initial combatant count = %d, want 2", len(cs.Combatants))
	}

	// Add party
	AddPartyToCombat(cs, party, monsters, rng)

	// Should now have 4 combatants (3 party + 1 monster)
	if len(cs.Combatants) != 4 {
		t.Fatalf("combatant count after AddPartyToCombat = %d, want 4", len(cs.Combatants))
	}

	// Scout phase flags should be cleared
	if cs.IsScoutPhase {
		t.Error("IsScoutPhase should be false after AddPartyToCombat")
	}
	if cs.AwaitingScoutDecision {
		t.Error("AwaitingScoutDecision should be false after AddPartyToCombat")
	}
	if cs.CurrentTurnIdx != 0 {
		t.Errorf("CurrentTurnIdx = %d, want 0", cs.CurrentTurnIdx)
	}
	if cs.RoundNumber != 1 {
		t.Errorf("RoundNumber = %d, want 1", cs.RoundNumber)
	}

	// All combatants should have re-rolled initiative (non-zero for players)
	for _, c := range cs.GetPlayerCombatants() {
		if c.Initiative <= 0 {
			t.Errorf("player %s initiative = %d, want > 0", c.Name, c.Initiative)
		}
	}

	// Monster should have non-zero initiative now (re-rolled)
	for _, c := range cs.GetEnemyCombatants() {
		if c.Initiative <= 0 {
			t.Errorf("monster %s initiative = %d, want > 0 after re-roll", c.Name, c.Initiative)
		}
	}

	// All combatants should have HasMoved/HasActed reset
	for _, c := range cs.Combatants {
		if c.HasMoved {
			t.Errorf("combatant %s HasMoved should be false", c.Name)
		}
		if c.HasActed {
			t.Errorf("combatant %s HasActed should be false", c.Name)
		}
	}
}

func TestAddPartyToCombat_SoloThief(t *testing.T) {
	// Solo party (thief only) — signal_party adds 0 additional combatants
	party, err := NewParty([]PartyRequest{
		{Name: "Shadow", Class: ClassThief},
	})
	if err != nil {
		t.Fatalf("NewParty: %v", err)
	}

	thief := party.GetByClass(ClassThief)
	monsters := []*Monster{
		{ID: "m1", Name: "Goblin", HP: 10, MaxHP: 10, Dexterity: 8, IsAlive: true},
	}

	rng := rand.New(rand.NewSource(42))
	cs := InitScoutCombat(thief, monsters, "prev-room", rng)

	AddPartyToCombat(cs, party, monsters, rng)

	// Should still have 2 combatants (thief + monster, no one else to add)
	if len(cs.Combatants) != 2 {
		t.Fatalf("combatant count = %d, want 2", len(cs.Combatants))
	}
	if cs.IsScoutPhase {
		t.Error("scout phase should be cleared")
	}
}
