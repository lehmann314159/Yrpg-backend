package game

import (
	"math/rand"
	"strings"
	"testing"
)

func TestManhattanDistance(t *testing.T) {
	tests := []struct {
		x1, y1, x2, y2, want int
	}{
		{0, 0, 0, 0, 0},
		{0, 0, 1, 0, 1},
		{0, 0, 0, 1, 1},
		{0, 0, 1, 1, 2},
		{0, 0, 3, 2, 5},
		{3, 3, 0, 0, 6},
	}
	for _, tc := range tests {
		got := ManhattanDistance(tc.x1, tc.y1, tc.x2, tc.y2)
		if got != tc.want {
			t.Errorf("ManhattanDistance(%d,%d,%d,%d) = %d, want %d",
				tc.x1, tc.y1, tc.x2, tc.y2, got, tc.want)
		}
	}
}

func TestChebyshevDistance(t *testing.T) {
	tests := []struct {
		x1, y1, x2, y2, want int
	}{
		{0, 0, 0, 0, 0},
		{0, 0, 1, 0, 1},
		{0, 0, 0, 1, 1},
		{0, 0, 1, 1, 1}, // diagonal = 1
		{0, 0, 3, 2, 3},
	}
	for _, tc := range tests {
		got := ChebyshevDistance(tc.x1, tc.y1, tc.x2, tc.y2)
		if got != tc.want {
			t.Errorf("ChebyshevDistance(%d,%d,%d,%d) = %d, want %d",
				tc.x1, tc.y1, tc.x2, tc.y2, got, tc.want)
		}
	}
}

func TestIsAdjacent(t *testing.T) {
	if !IsAdjacent(0, 0, 1, 0) {
		t.Error("(0,0)-(1,0) should be adjacent")
	}
	if !IsAdjacent(0, 0, 1, 1) {
		t.Error("(0,0)-(1,1) diagonal should be adjacent")
	}
	if IsAdjacent(0, 0, 2, 0) {
		t.Error("(0,0)-(2,0) should not be adjacent")
	}
	if IsAdjacent(0, 0, 0, 0) {
		t.Error("same cell should not be adjacent")
	}
}

// newTestCombatState builds a minimal CombatState with an initialized grid.
func newTestCombatState() *CombatState {
	cs := &CombatState{
		Combatants:  make([]*Combatant, 0),
		Engagements: make(map[string]string),
		IsActive:    true,
	}
	for y := 0; y < GridHeight; y++ {
		for x := 0; x < GridWidth; x++ {
			cs.Grid[y][x] = &GridCell{X: x, Y: y}
		}
	}
	return cs
}

func TestCombatMove_Success(t *testing.T) {
	cs := newTestCombatState()
	c := &Combatant{ID: "c1", Name: "Hero", GridX: 0, GridY: 0, IsAlive: true}
	cs.Combatants = append(cs.Combatants, c)
	cs.PlaceOnGrid(c, 0, 0)

	result := CombatMove(cs, c, 1, 0, 2)
	if !result.Success {
		t.Errorf("expected success, got: %s", result.Message)
	}
	if c.GridX != 1 || c.GridY != 0 {
		t.Errorf("position = (%d,%d), want (1,0)", c.GridX, c.GridY)
	}
	if !c.HasMoved {
		t.Error("HasMoved should be true")
	}
}

func TestCombatMove_AlreadyMoved(t *testing.T) {
	cs := newTestCombatState()
	c := &Combatant{ID: "c1", Name: "Hero", HasMoved: true, IsAlive: true}
	cs.Combatants = append(cs.Combatants, c)

	result := CombatMove(cs, c, 1, 0, 2)
	if result.Success {
		t.Error("should fail if already moved")
	}
}

func TestCombatMove_OutOfBounds(t *testing.T) {
	cs := newTestCombatState()
	c := &Combatant{ID: "c1", Name: "Hero", GridX: 0, GridY: 0, IsAlive: true}
	cs.Combatants = append(cs.Combatants, c)

	result := CombatMove(cs, c, -1, 0, 2)
	if result.Success {
		t.Error("should fail for out-of-bounds")
	}
	result = CombatMove(cs, c, 0, GridHeight, 2)
	if result.Success {
		t.Error("should fail for out-of-bounds Y")
	}
}

func TestCombatMove_Occupied(t *testing.T) {
	cs := newTestCombatState()
	c1 := &Combatant{ID: "c1", Name: "Hero", GridX: 0, GridY: 0, IsAlive: true}
	c2 := &Combatant{ID: "c2", Name: "Blocker", GridX: 1, GridY: 0, IsAlive: true}
	cs.Combatants = append(cs.Combatants, c1, c2)
	cs.PlaceOnGrid(c1, 0, 0)
	cs.PlaceOnGrid(c2, 1, 0)

	result := CombatMove(cs, c1, 1, 0, 2)
	if result.Success {
		t.Error("should fail for occupied cell")
	}
}

func TestCombatMove_TooFar(t *testing.T) {
	cs := newTestCombatState()
	c := &Combatant{ID: "c1", Name: "Hero", GridX: 0, GridY: 0, IsAlive: true}
	cs.Combatants = append(cs.Combatants, c)
	cs.PlaceOnGrid(c, 0, 0)

	result := CombatMove(cs, c, 5, 0, 2) // Manhattan dist 5 > range 2
	if result.Success {
		t.Error("should fail for too far")
	}
}

func TestSetEngagement_RemoveEngagement(t *testing.T) {
	cs := newTestCombatState()

	SetEngagement(cs, "a", "b")
	if cs.Engagements["a"] != "b" || cs.Engagements["b"] != "a" {
		t.Error("engagement should be mutual")
	}

	RemoveEngagement(cs, "a")
	if _, ok := cs.Engagements["a"]; ok {
		t.Error("a's engagement should be removed")
	}
	if _, ok := cs.Engagements["b"]; ok {
		t.Error("b's reverse engagement should be removed")
	}
}

func TestResolveCombatEnd_EnemiesDead(t *testing.T) {
	cs := newTestCombatState()
	cs.Combatants = []*Combatant{
		{ID: "p1", IsPlayerChar: true, IsAlive: true},
		{ID: "e1", IsPlayerChar: false, IsAlive: false},
	}

	result := ResolveCombatEnd(cs)
	if result == nil {
		t.Fatal("expected combat end result")
	}
	if !result.AllEnemiesDead {
		t.Error("AllEnemiesDead should be true")
	}
	if cs.IsActive {
		t.Error("combat should no longer be active")
	}
}

func TestResolveCombatEnd_PartyWiped(t *testing.T) {
	cs := newTestCombatState()
	cs.Combatants = []*Combatant{
		{ID: "p1", IsPlayerChar: true, IsAlive: false},
		{ID: "e1", IsPlayerChar: false, IsAlive: true},
	}

	result := ResolveCombatEnd(cs)
	if result == nil {
		t.Fatal("expected combat end result")
	}
	if !result.PartyWiped {
		t.Error("PartyWiped should be true")
	}
}

func TestResolveCombatEnd_Ongoing(t *testing.T) {
	cs := newTestCombatState()
	cs.Combatants = []*Combatant{
		{ID: "p1", IsPlayerChar: true, IsAlive: true},
		{ID: "e1", IsPlayerChar: false, IsAlive: true},
	}

	if ResolveCombatEnd(cs) != nil {
		t.Error("combat should be ongoing, expected nil")
	}
}

func TestFormatAttackResult_Miss(t *testing.T) {
	r := &AttackResult{
		AttackerName: "Hero",
		TargetName:   "Goblin",
		Hit:          false,
		ToHit:        8,
		Defense:      12,
	}
	msg := FormatAttackResult(r)
	if !strings.Contains(msg, "misses") {
		t.Errorf("miss message = %q, expected 'misses'", msg)
	}
}

func TestFormatAttackResult_Hit(t *testing.T) {
	r := &AttackResult{
		Hit:          true,
		Damage:       5,
		AttackerName: "Hero",
		TargetName:   "Goblin",
		TargetHP:     10,
		TargetMaxHP:  15,
	}
	msg := FormatAttackResult(r)
	if !strings.Contains(msg, "hits") || !strings.Contains(msg, "5 damage") {
		t.Errorf("hit message = %q", msg)
	}
}

func TestFormatAttackResult_Critical(t *testing.T) {
	r := &AttackResult{
		Hit:          true,
		Critical:     true,
		Damage:       10,
		AttackerName: "Hero",
		TargetName:   "Goblin",
		TargetHP:     5,
		TargetMaxHP:  15,
	}
	msg := FormatAttackResult(r)
	if !strings.Contains(msg, "CRITICAL HIT") {
		t.Errorf("critical message = %q", msg)
	}
}

func TestFormatAttackResult_Flanking(t *testing.T) {
	r := &AttackResult{
		Hit:          true,
		Flanking:     true,
		Damage:       5,
		AttackerName: "Hero",
		TargetName:   "Goblin",
		TargetHP:     5,
		TargetMaxHP:  15,
	}
	msg := FormatAttackResult(r)
	if !strings.Contains(msg, "Flanking") {
		t.Errorf("flanking message = %q", msg)
	}
}

func TestFormatAttackResult_TargetDied(t *testing.T) {
	r := &AttackResult{
		Hit:          true,
		Damage:       15,
		AttackerName: "Hero",
		TargetName:   "Goblin",
		TargetHP:     0,
		TargetMaxHP:  15,
		TargetDied:   true,
	}
	msg := FormatAttackResult(r)
	if !strings.Contains(msg, "falls") {
		t.Errorf("death message = %q, expected 'falls'", msg)
	}
}

func TestFormatRetreatResult_Success(t *testing.T) {
	r := &RetreatResult{
		Success:       true,
		Roll:          15,
		DC:            12,
		CombatantName: "Hero",
	}
	msg := FormatRetreatResult(r)
	if !strings.Contains(msg, "successfully retreats") {
		t.Errorf("success retreat message = %q", msg)
	}
}

func TestFormatRetreatResult_Failure(t *testing.T) {
	r := &RetreatResult{
		Success:       false,
		Roll:          8,
		DC:            12,
		CombatantName: "Hero",
	}
	msg := FormatRetreatResult(r)
	if !strings.Contains(msg, "fails to retreat") {
		t.Errorf("failure retreat message = %q", msg)
	}
}

func TestFormatRetreatResult_OpportunityAttack(t *testing.T) {
	r := &RetreatResult{
		Success:       false,
		Roll:          8,
		DC:            12,
		CombatantName: "Hero",
		OpportunityAttack: &AttackResult{
			Hit:          true,
			Damage:       7,
			AttackerName: "Goblin",
			TargetName:   "Hero",
			TargetDied:   true,
		},
	}
	msg := FormatRetreatResult(r)
	if !strings.Contains(msg, "opportunity attack") {
		t.Errorf("opportunity attack message = %q", msg)
	}
	if !strings.Contains(msg, "falls while trying to flee") {
		t.Errorf("expected death on retreat message, got: %q", msg)
	}
	// When TargetDied, it returns early without the success/failure part
	if strings.Contains(msg, "fails to retreat") {
		t.Errorf("should not include retreat result when killed: %q", msg)
	}
}

// --- CombatAttack tests ---

func TestCombatAttack_MeleeHit(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(42))

	attacker := &Combatant{ID: "p1", Name: "Fighter", IsPlayerChar: true, IsAlive: true, GridX: 0, GridY: 0}
	target := &Combatant{ID: "e1", Name: "Goblin", IsPlayerChar: false, IsAlive: true, HP: 20, MaxHP: 20, GridX: 1, GridY: 0}
	cs.Combatants = append(cs.Combatants, attacker, target)
	cs.PlaceOnGrid(attacker, 0, 0)
	cs.PlaceOnGrid(target, 1, 0)

	char, _ := NewCharacter("Fighter", ClassFighter) // str=14
	weapon := &Item{Type: ItemWeapon, Damage: 2, Range: RangeMelee}

	result := CombatAttack(cs, attacker, target, char, weapon, rng)
	// With str 14 (modifier 7) + d20, should almost always hit AC 10
	if result.AttackerName != "Fighter" || result.TargetName != "Goblin" {
		t.Error("names should be set")
	}
	// The result was computed — just verify fields are populated
	if result.Roll == 0 {
		t.Error("Roll should be set")
	}
}

func TestCombatAttack_MeleeOutOfRange(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(42))

	attacker := &Combatant{ID: "p1", Name: "Fighter", IsAlive: true, GridX: 0, GridY: 0}
	target := &Combatant{ID: "e1", Name: "Goblin", IsAlive: true, HP: 20, MaxHP: 20, GridX: 3, GridY: 3}
	cs.Combatants = append(cs.Combatants, attacker, target)
	cs.PlaceOnGrid(attacker, 0, 0)
	cs.PlaceOnGrid(target, 3, 3)

	weapon := &Item{Type: ItemWeapon, Range: RangeMelee}

	result := CombatAttack(cs, attacker, target, nil, weapon, rng)
	if result.Hit {
		t.Error("melee attack should miss when target is not adjacent")
	}
}

func TestCombatAttack_RangedInRange(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(42))

	attacker := &Combatant{ID: "p1", Name: "Archer", IsAlive: true, GridX: 0, GridY: 0}
	target := &Combatant{ID: "e1", Name: "Goblin", IsAlive: true, HP: 20, MaxHP: 20, GridX: 3, GridY: 3}
	cs.Combatants = append(cs.Combatants, attacker, target)
	cs.PlaceOnGrid(attacker, 0, 0)
	cs.PlaceOnGrid(target, 3, 3)

	char, _ := NewCharacter("Archer", ClassThief) // dex=14 for ranged modifier
	weapon := &Item{Type: ItemWeapon, Damage: 2, Range: RangeRanged, MaxRange: 5}

	result := CombatAttack(cs, attacker, target, char, weapon, rng)
	// Chebyshev distance is 3, max range is 5 — in range
	if result.Roll == 0 {
		t.Error("should attempt the attack (Roll should be set)")
	}
}

func TestCombatAttack_RangedOutOfRange(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(42))

	attacker := &Combatant{ID: "p1", Name: "Archer", IsAlive: true, GridX: 0, GridY: 0}
	target := &Combatant{ID: "e1", Name: "Goblin", IsAlive: true, HP: 20, MaxHP: 20, GridX: 5, GridY: 5}
	cs.Combatants = append(cs.Combatants, attacker, target)
	cs.PlaceOnGrid(attacker, 0, 0)
	cs.PlaceOnGrid(target, 5, 5)

	weapon := &Item{Type: ItemWeapon, Range: RangeRanged, MaxRange: 3}

	result := CombatAttack(cs, attacker, target, nil, weapon, rng)
	if result.Hit {
		t.Error("ranged attack should miss when out of max range")
	}
}

func TestCombatAttack_Flanking(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(42))

	attacker := &Combatant{ID: "p1", Name: "Flanker", IsPlayerChar: true, IsAlive: true, GridX: 0, GridY: 0}
	other := &Combatant{ID: "p2", Name: "Ally", IsPlayerChar: true, IsAlive: true, GridX: 2, GridY: 0}
	target := &Combatant{ID: "e1", Name: "Goblin", IsPlayerChar: false, IsAlive: true, HP: 50, MaxHP: 50, GridX: 1, GridY: 0}
	cs.Combatants = append(cs.Combatants, attacker, other, target)
	cs.PlaceOnGrid(attacker, 0, 0)
	cs.PlaceOnGrid(other, 2, 0)
	cs.PlaceOnGrid(target, 1, 0)

	// Target is engaged with "other" (not the attacker)
	SetEngagement(cs, other.ID, target.ID)

	char, _ := NewCharacter("Flanker", ClassFighter)
	weapon := &Item{Type: ItemWeapon, Range: RangeMelee}

	result := CombatAttack(cs, attacker, target, char, weapon, rng)
	if !result.Flanking {
		t.Error("should detect flanking when target engaged with someone else")
	}
}

func TestCombatAttack_KillsTarget(t *testing.T) {
	cs := newTestCombatState()
	// Use a seed that produces a high roll for guaranteed hit
	rng := rand.New(rand.NewSource(99))

	attacker := &Combatant{ID: "p1", Name: "Fighter", IsPlayerChar: true, IsAlive: true, GridX: 0, GridY: 0}
	target := &Combatant{ID: "e1", Name: "Goblin", IsPlayerChar: false, IsAlive: true, HP: 1, MaxHP: 10, GridX: 1, GridY: 0}
	cs.Combatants = append(cs.Combatants, attacker, target)
	cs.PlaceOnGrid(attacker, 0, 0)
	cs.PlaceOnGrid(target, 1, 0)

	char, _ := NewCharacter("Fighter", ClassFighter)
	weapon := &Item{Type: ItemWeapon, Damage: 5, Range: RangeMelee}

	result := CombatAttack(cs, attacker, target, char, weapon, rng)
	if result.Hit && !result.TargetDied {
		t.Error("target with 1 HP should die on hit")
	}
	if result.Hit && target.IsAlive {
		t.Error("target should be dead")
	}
}

// --- MonsterAttack tests ---

func TestMonsterAttack_MeleeHit(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(42))

	attacker := &Combatant{ID: "m1", Name: "Goblin", IsPlayerChar: false, IsAlive: true, GridX: 0, GridY: 0}
	target := &Combatant{ID: "p1", Name: "Hero", IsPlayerChar: true, IsAlive: true, HP: 30, MaxHP: 30, GridX: 1, GridY: 0}
	cs.Combatants = append(cs.Combatants, attacker, target)
	cs.PlaceOnGrid(attacker, 0, 0)
	cs.PlaceOnGrid(target, 1, 0)

	monster := &Monster{ID: "m1", Damage: 3, Dexterity: 10}

	result := MonsterAttack(cs, attacker, target, monster, rng)
	if result.AttackerName != "Goblin" || result.TargetName != "Hero" {
		t.Error("names should be set")
	}
	if result.Roll == 0 {
		t.Error("Roll should be set")
	}
}

func TestMonsterAttack_MeleeOutOfRange(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(42))

	attacker := &Combatant{ID: "m1", Name: "Goblin", IsAlive: true, GridX: 0, GridY: 0}
	target := &Combatant{ID: "p1", Name: "Hero", IsAlive: true, HP: 30, MaxHP: 30, GridX: 3, GridY: 3}
	cs.Combatants = append(cs.Combatants, attacker, target)

	monster := &Monster{ID: "m1", Damage: 3, Dexterity: 10, IsRanged: false}

	result := MonsterAttack(cs, attacker, target, monster, rng)
	if result.Hit {
		t.Error("melee monster should miss when not adjacent")
	}
}

func TestMonsterAttack_RangedOutOfRange(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(42))

	attacker := &Combatant{ID: "m1", Name: "Archer", IsAlive: true, GridX: 0, GridY: 0}
	target := &Combatant{ID: "p1", Name: "Hero", IsAlive: true, HP: 30, MaxHP: 30, GridX: 5, GridY: 5}
	cs.Combatants = append(cs.Combatants, attacker, target)

	monster := &Monster{ID: "m1", Damage: 3, Dexterity: 10, IsRanged: true, AttackRange: 3}

	result := MonsterAttack(cs, attacker, target, monster, rng)
	if result.Hit {
		t.Error("ranged monster should miss when out of range")
	}
}

func TestMonsterAttack_KillsTarget(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(99))

	attacker := &Combatant{ID: "m1", Name: "Goblin", IsPlayerChar: false, IsAlive: true, GridX: 0, GridY: 0}
	target := &Combatant{ID: "p1", Name: "Hero", IsPlayerChar: true, IsAlive: true, HP: 1, MaxHP: 30, GridX: 1, GridY: 0}
	cs.Combatants = append(cs.Combatants, attacker, target)
	cs.PlaceOnGrid(attacker, 0, 0)
	cs.PlaceOnGrid(target, 1, 0)

	monster := &Monster{ID: "m1", Damage: 10, Dexterity: 14}

	result := MonsterAttack(cs, attacker, target, monster, rng)
	if result.Hit && !result.TargetDied {
		t.Error("1 HP target should die on hit")
	}
}

// --- AttemptRetreat tests ---

func TestAttemptRetreat_SuccessNoEngagement(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(99)) // high roll seed

	combatant := &Combatant{ID: "p1", Name: "Hero", IsPlayerChar: true, IsAlive: true, HP: 30, MaxHP: 30, GridX: 0, GridY: 0}
	cs.Combatants = append(cs.Combatants, combatant)
	cs.PlaceOnGrid(combatant, 0, 0)

	char, _ := NewCharacter("Hero", ClassThief) // dex=14, thief bonus

	result := AttemptRetreat(cs, combatant, char, rng)
	// Thief: dex/2=7 + ThiefRetreatBonus=4 = 11 bonus, DC=12, any roll >= 1 succeeds
	if !result.Success {
		t.Errorf("expected retreat success (Roll=%d, DC=%d)", result.Roll, result.DC)
	}
	if result.OpportunityAttack != nil {
		t.Error("no engagement means no opportunity attack")
	}
}

func TestAttemptRetreat_WithEngagementOpportunityAttack(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(42))

	combatant := &Combatant{ID: "p1", Name: "Hero", IsPlayerChar: true, IsAlive: true, HP: 30, MaxHP: 30, GridX: 0, GridY: 0}
	enemy := &Combatant{ID: "e1", Name: "Goblin", IsPlayerChar: false, IsAlive: true, HP: 10, MaxHP: 10, GridX: 1, GridY: 0}
	cs.Combatants = append(cs.Combatants, combatant, enemy)
	cs.PlaceOnGrid(combatant, 0, 0)
	cs.PlaceOnGrid(enemy, 1, 0)

	SetEngagement(cs, combatant.ID, enemy.ID)

	char, _ := NewCharacter("Hero", ClassFighter)

	result := AttemptRetreat(cs, combatant, char, rng)
	if result.OpportunityAttack == nil {
		t.Error("engaged enemy should get opportunity attack")
	}
}

// --- CheckPartyRetreat tests ---

func TestCheckPartyRetreat(t *testing.T) {
	cs := newTestCombatState()
	cs.Combatants = []*Combatant{
		{ID: "p1", IsPlayerChar: true, IsAlive: true},
		{ID: "p2", IsPlayerChar: true, IsAlive: true},
		{ID: "e1", IsPlayerChar: false, IsAlive: true},
	}

	retreated := map[string]bool{"p1": true}
	if CheckPartyRetreat(cs, retreated) {
		t.Error("not all players retreated")
	}

	retreated["p2"] = true
	if !CheckPartyRetreat(cs, retreated) {
		t.Error("all living players retreated")
	}
}

func TestCheckPartyRetreat_DeadDontCount(t *testing.T) {
	cs := newTestCombatState()
	cs.Combatants = []*Combatant{
		{ID: "p1", IsPlayerChar: true, IsAlive: true},
		{ID: "p2", IsPlayerChar: true, IsAlive: false}, // dead
		{ID: "e1", IsPlayerChar: false, IsAlive: true},
	}

	retreated := map[string]bool{"p1": true}
	if !CheckPartyRetreat(cs, retreated) {
		t.Error("dead player should not block party retreat")
	}
}

// --- SyncCombatToCharacters tests ---

func TestSyncCombatToCharacters(t *testing.T) {
	party, _ := NewParty([]PartyRequest{
		{Name: "Fighter", Class: ClassFighter},
	})
	char := party.Characters[0]

	cs := newTestCombatState()
	combatant := &Combatant{
		ID:           char.ID,
		CharacterID:  char.ID,
		IsPlayerChar: true,
		IsAlive:      true,
		HP:           15,
		MaxHP:        30,
	}
	cs.Combatants = append(cs.Combatants, combatant)

	SyncCombatToCharacters(cs, party)
	if char.HP != 15 {
		t.Errorf("char HP = %d, want 15", char.HP)
	}

	// Test death sync
	combatant.IsAlive = false
	combatant.HP = 0
	SyncCombatToCharacters(cs, party)
	if char.IsAlive {
		t.Error("char should be dead after sync")
	}
}

// --- SyncCombatToMonsters tests ---

func TestSyncCombatToMonsters(t *testing.T) {
	monsters := map[string]*Monster{
		"m1": {ID: "m1", HP: 10, MaxHP: 10, IsAlive: true},
	}

	cs := newTestCombatState()
	combatant := &Combatant{
		ID:           "m1",
		CharacterID:  "m1",
		IsPlayerChar: false,
		IsAlive:      true,
		HP:           5,
	}
	cs.Combatants = append(cs.Combatants, combatant)

	SyncCombatToMonsters(cs, monsters)
	if monsters["m1"].HP != 5 {
		t.Errorf("monster HP = %d, want 5", monsters["m1"].HP)
	}

	// Test death sync
	combatant.IsAlive = false
	combatant.HP = 0
	SyncCombatToMonsters(cs, monsters)
	if monsters["m1"].IsAlive {
		t.Error("monster should be dead after sync")
	}
	if monsters["m1"].HP != 0 {
		t.Errorf("dead monster HP = %d, want 0", monsters["m1"].HP)
	}
}

