package game

import (
	"math/rand"
	"strings"
	"testing"
)

func TestCheckRoomSneak_ThiefSuccess(t *testing.T) {
	thief, _ := NewCharacter("Shadow", ClassThief) // dex=14, bonus=7+2=9
	// Low difficulty so success is likely
	rng := rand.New(rand.NewSource(42))
	result := CheckRoomSneak(thief, 1, rng) // DC = 1+10 = 11, need roll+9 >= 11

	if result.SneakerID != thief.ID {
		t.Errorf("SneakerID = %s, want %s", result.SneakerID, thief.ID)
	}
	if result.SneakerName != "Shadow" {
		t.Errorf("SneakerName = %s, want Shadow", result.SneakerName)
	}
	// With dex bonus 9 and DC 11, any roll >= 2 succeeds
	if !result.Success {
		t.Errorf("expected success (Roll=%d, DC=%d)", result.Roll, result.DC)
	}
	if !result.IsHidden {
		t.Error("should be hidden on success")
	}
}

func TestCheckRoomSneak_NonThiefFails(t *testing.T) {
	fighter, _ := NewCharacter("Tank", ClassFighter)
	rng := rand.New(rand.NewSource(42))
	result := CheckRoomSneak(fighter, 1, rng)

	if result.Success {
		t.Error("non-thief should always fail sneak")
	}
}

func TestCheckRoomSneak_DeadThiefFails(t *testing.T) {
	thief, _ := NewCharacter("Shadow", ClassThief)
	thief.TakeDamage(999)
	rng := rand.New(rand.NewSource(42))
	result := CheckRoomSneak(thief, 1, rng)

	if result.Success {
		t.Error("dead thief should fail sneak")
	}
}

func TestAttemptCombatHide_ThiefSuccess(t *testing.T) {
	thief, _ := NewCharacter("Shadow", ClassThief) // dex=14 -> bonus=7+2=9
	enemies := []*Monster{
		{ID: "m1", Dexterity: 4, IsAlive: true}, // perception = 10+2=12
	}
	// Use seed that gives a high roll
	rng := rand.New(rand.NewSource(99))
	result := AttemptCombatHide(thief, enemies, rng)

	if result.SneakerID != thief.ID {
		t.Errorf("SneakerID = %s, want %s", result.SneakerID, thief.ID)
	}
	if result.DC != 12 {
		t.Errorf("DC = %d, want 12", result.DC)
	}
	// With bonus 9 and DC 12, need roll >= 3 to succeed
	if !result.Success {
		t.Errorf("expected success (Roll=%d, DC=%d)", result.Roll, result.DC)
	}
}

func TestAttemptCombatHide_NonThiefFails(t *testing.T) {
	fighter, _ := NewCharacter("Tank", ClassFighter)
	rng := rand.New(rand.NewSource(42))
	result := AttemptCombatHide(fighter, nil, rng)

	if result.Success {
		t.Error("non-thief should fail combat hide")
	}
}

func TestAttemptCombatHide_DeadThiefFails(t *testing.T) {
	thief, _ := NewCharacter("Shadow", ClassThief)
	thief.TakeDamage(999)
	rng := rand.New(rand.NewSource(42))
	result := AttemptCombatHide(thief, nil, rng)

	if result.Success {
		t.Error("dead thief should fail combat hide")
	}
}

func TestAttemptCombatHide_HighestEnemyPerception(t *testing.T) {
	thief, _ := NewCharacter("Shadow", ClassThief)
	enemies := []*Monster{
		{ID: "m1", Dexterity: 4, IsAlive: true},  // perception = 12
		{ID: "m2", Dexterity: 14, IsAlive: true},  // perception = 17
		{ID: "m3", Dexterity: 20, IsAlive: false}, // dead, skip
	}
	rng := rand.New(rand.NewSource(42))
	result := AttemptCombatHide(thief, enemies, rng)

	if result.DC != 17 {
		t.Errorf("DC = %d, want 17 (highest alive enemy perception)", result.DC)
	}
}

func TestFormatSneakResult(t *testing.T) {
	success := &SneakResult{
		Success:     true,
		SneakerName: "Shadow",
		Roll:        18,
		DC:          12,
	}
	msg := FormatSneakResult(success)
	if !strings.Contains(msg, "silently") || !strings.Contains(msg, "Hidden") {
		t.Errorf("success sneak message = %q", msg)
	}

	failure := &SneakResult{
		Success:     false,
		SneakerName: "Shadow",
		Roll:        8,
		DC:          12,
	}
	msg = FormatSneakResult(failure)
	if !strings.Contains(msg, "detected") {
		t.Errorf("failure sneak message = %q", msg)
	}
}

func TestFormatCombatHideResult(t *testing.T) {
	success := &CombatHideResult{
		Success:     true,
		SneakerName: "Shadow",
		Roll:        18,
		DC:          12,
	}
	msg := FormatCombatHideResult(success)
	if !strings.Contains(msg, "shadows") || !strings.Contains(msg, "Hidden") {
		t.Errorf("success hide message = %q", msg)
	}

	failure := &CombatHideResult{
		Success:     false,
		SneakerName: "Shadow",
		Roll:        8,
		DC:          12,
	}
	msg = FormatCombatHideResult(failure)
	if !strings.Contains(msg, "fails to hide") {
		t.Errorf("failure hide message = %q", msg)
	}
}
