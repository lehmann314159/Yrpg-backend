package game

import (
	"math/rand"
	"testing"
)

func TestExecuteMonsterTurn_Sleeping(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(42))

	combatant := &Combatant{ID: "m1", Name: "Goblin", IsAlive: true, GridX: 3, GridY: 3}
	combatant.AddBuff(BuffSleep, 0, 3, "Sleep Spell")
	cs.Combatants = append(cs.Combatants, combatant)

	monster := &Monster{ID: "m1", Damage: 3, Dexterity: 10}

	action := ExecuteMonsterTurn(cs, combatant, monster, rng)
	if action.AttackResult != nil {
		t.Error("sleeping monster should not attack")
	}
	if action.MovedTo != nil {
		t.Error("sleeping monster should not move")
	}
	if action.Message == "" {
		t.Error("should have a message about sleeping")
	}
}

func TestExecuteMonsterTurn_NoTargets(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(42))

	combatant := &Combatant{ID: "m1", Name: "Goblin", IsAlive: true, GridX: 3, GridY: 3}
	cs.Combatants = append(cs.Combatants, combatant)
	cs.PlaceOnGrid(combatant, 3, 3)

	// No player combatants at all
	monster := &Monster{ID: "m1", Damage: 3, Dexterity: 10}

	action := ExecuteMonsterTurn(cs, combatant, monster, rng)
	if action.AttackResult != nil {
		t.Error("no targets means no attack")
	}
	if action.Message == "" {
		t.Error("should have a 'no targets' message")
	}
}

func TestExecuteMonsterTurn_MeleeAdjacentAttacks(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(42))

	monsterC := &Combatant{ID: "m1", Name: "Goblin", IsPlayerChar: false, IsAlive: true, HP: 10, MaxHP: 10, GridX: 1, GridY: 0}
	playerC := &Combatant{ID: "p1", Name: "Hero", IsPlayerChar: true, IsAlive: true, HP: 30, MaxHP: 30, GridX: 0, GridY: 0}
	cs.Combatants = append(cs.Combatants, monsterC, playerC)
	cs.PlaceOnGrid(monsterC, 1, 0)
	cs.PlaceOnGrid(playerC, 0, 0)

	monster := &Monster{ID: "m1", Damage: 3, Dexterity: 10, IsRanged: false}

	action := ExecuteMonsterTurn(cs, monsterC, monster, rng)
	if action.AttackResult == nil {
		t.Error("adjacent melee monster should attack")
	}
}

func TestExecuteMonsterTurn_MeleeMovesThenAttacks(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(42))

	monsterC := &Combatant{ID: "m1", Name: "Goblin", IsPlayerChar: false, IsAlive: true, HP: 10, MaxHP: 10, GridX: 3, GridY: 0}
	playerC := &Combatant{ID: "p1", Name: "Hero", IsPlayerChar: true, IsAlive: true, HP: 30, MaxHP: 30, GridX: 0, GridY: 0}
	cs.Combatants = append(cs.Combatants, monsterC, playerC)
	cs.PlaceOnGrid(monsterC, 3, 0)
	cs.PlaceOnGrid(playerC, 0, 0)

	monster := &Monster{ID: "m1", Damage: 3, Dexterity: 10, IsRanged: false}

	action := ExecuteMonsterTurn(cs, monsterC, monster, rng)
	// Monster at (3,0) is 3 Chebyshev away from player at (0,0)
	// With 2 moves, it can get to (1,0) — adjacent
	if action.MovedTo == nil {
		t.Error("monster should move toward player")
	}
	// After moving 2 cells toward (0,0), should be adjacent and attack
	if action.AttackResult == nil {
		t.Error("monster should attack after moving adjacent")
	}
}

func TestExecuteMonsterTurn_RangedAttacksInRange(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(42))

	monsterC := &Combatant{ID: "m1", Name: "Archer", IsPlayerChar: false, IsAlive: true, HP: 10, MaxHP: 10, GridX: 4, GridY: 0}
	playerC := &Combatant{ID: "p1", Name: "Hero", IsPlayerChar: true, IsAlive: true, HP: 30, MaxHP: 30, GridX: 0, GridY: 0}
	cs.Combatants = append(cs.Combatants, monsterC, playerC)
	cs.PlaceOnGrid(monsterC, 4, 0)
	cs.PlaceOnGrid(playerC, 0, 0)

	monster := &Monster{ID: "m1", Damage: 3, Dexterity: 10, IsRanged: true, AttackRange: 5}

	action := ExecuteMonsterTurn(cs, monsterC, monster, rng)
	if action.AttackResult == nil {
		t.Error("ranged monster should attack when in range")
	}
	if action.MovedTo != nil {
		t.Error("ranged monster in range should not need to move")
	}
}

func TestExecuteMonsterTurn_RangedMovesCloserThenAttacks(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(42))

	monsterC := &Combatant{ID: "m1", Name: "Archer", IsPlayerChar: false, IsAlive: true, HP: 10, MaxHP: 10, GridX: 5, GridY: 5}
	playerC := &Combatant{ID: "p1", Name: "Hero", IsPlayerChar: true, IsAlive: true, HP: 30, MaxHP: 30, GridX: 0, GridY: 0}
	cs.Combatants = append(cs.Combatants, monsterC, playerC)
	cs.PlaceOnGrid(monsterC, 5, 5)
	cs.PlaceOnGrid(playerC, 0, 0)

	monster := &Monster{ID: "m1", Damage: 3, Dexterity: 10, IsRanged: true, AttackRange: 4}

	action := ExecuteMonsterTurn(cs, monsterC, monster, rng)
	// Chebyshev dist from (5,5) to (0,0) is 5, range is 4 — needs to move closer
	if action.MovedTo == nil {
		t.Error("ranged monster out of range should move closer")
	}
}

func TestExecuteMonsterTurn_SkipsHiddenPlayers(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(42))

	monsterC := &Combatant{ID: "m1", Name: "Goblin", IsPlayerChar: false, IsAlive: true, HP: 10, MaxHP: 10, GridX: 1, GridY: 0}
	playerC := &Combatant{ID: "p1", Name: "Shadow", IsPlayerChar: true, IsAlive: true, HP: 20, MaxHP: 20, GridX: 0, GridY: 0, IsHidden: true}
	cs.Combatants = append(cs.Combatants, monsterC, playerC)
	cs.PlaceOnGrid(monsterC, 1, 0)
	cs.PlaceOnGrid(playerC, 0, 0)

	monster := &Monster{ID: "m1", Damage: 3, Dexterity: 10}

	action := ExecuteMonsterTurn(cs, monsterC, monster, rng)
	if action.AttackResult != nil {
		t.Error("monster should not target hidden players")
	}
}

func TestExecuteMonsterTurn_PrefersEngagedTarget(t *testing.T) {
	cs := newTestCombatState()
	rng := rand.New(rand.NewSource(42))

	monsterC := &Combatant{ID: "m1", Name: "Goblin", IsPlayerChar: false, IsAlive: true, HP: 10, MaxHP: 10, GridX: 1, GridY: 0}
	player1 := &Combatant{ID: "p1", Name: "Tank", IsPlayerChar: true, IsAlive: true, HP: 30, MaxHP: 30, GridX: 0, GridY: 0}
	player2 := &Combatant{ID: "p2", Name: "Mage", IsPlayerChar: true, IsAlive: true, HP: 16, MaxHP: 16, GridX: 2, GridY: 0}
	cs.Combatants = append(cs.Combatants, monsterC, player1, player2)
	cs.PlaceOnGrid(monsterC, 1, 0)
	cs.PlaceOnGrid(player1, 0, 0)
	cs.PlaceOnGrid(player2, 2, 0)

	// Monster engaged with player1
	SetEngagement(cs, monsterC.ID, player1.ID)

	monster := &Monster{ID: "m1", Damage: 3, Dexterity: 10}

	action := ExecuteMonsterTurn(cs, monsterC, monster, rng)
	if action.AttackResult == nil {
		t.Fatal("monster should attack")
	}
	if action.AttackResult.TargetID != "p1" {
		t.Errorf("should prefer engaged target, got %s", action.AttackResult.TargetID)
	}
}
