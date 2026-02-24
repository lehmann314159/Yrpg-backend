package game

import (
	"math/rand"
	"strings"
	"testing"
)

func TestTriggerTrap(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	victim, _ := NewCharacter("Victim", ClassFighter) // HP=30

	trap := &Trap{
		ID:          "t1",
		RoomID:      "r1",
		Description: "Spike trap",
		Damage:      10,
		Difficulty:  15,
	}

	result := TriggerTrap(trap, victim, rng)
	if !result.Triggered {
		t.Fatal("trap should trigger")
	}
	if result.Damage != 10 {
		t.Errorf("damage = %d, want 10", result.Damage)
	}
	if victim.HP != 20 {
		t.Errorf("victim HP = %d, want 20", victim.HP)
	}
	if result.VictimDied {
		t.Error("victim should not be dead")
	}
	if !trap.IsTriggered {
		t.Error("trap should be marked as triggered")
	}
}

func TestTriggerTrap_Kills(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	victim, _ := NewCharacter("Victim", ClassFighter) // HP=30

	trap := &Trap{
		ID:     "t1",
		Damage: 50, // lethal
	}

	result := TriggerTrap(trap, victim, rng)
	if !result.Triggered {
		t.Fatal("trap should trigger")
	}
	if !result.VictimDied {
		t.Error("victim should be dead")
	}
	if victim.IsAlive {
		t.Error("victim should not be alive")
	}
}

func TestTriggerTrap_SkipsAlreadyTriggered(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	victim, _ := NewCharacter("Victim", ClassFighter)

	trap := &Trap{ID: "t1", Damage: 10, IsTriggered: true}
	result := TriggerTrap(trap, victim, rng)
	if result.Triggered {
		t.Error("already triggered trap should not trigger again")
	}
}

func TestTriggerTrap_SkipsDisarmed(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	victim, _ := NewCharacter("Victim", ClassFighter)

	trap := &Trap{ID: "t1", Damage: 10, IsDisarmed: true}
	result := TriggerTrap(trap, victim, rng)
	if result.Triggered {
		t.Error("disarmed trap should not trigger")
	}
}

func TestAttemptDisarm_Success(t *testing.T) {
	// Use a seed that gives a high roll so disarm succeeds
	// Thief: dex 14 -> bonus 7 + ThiefDisarmBonus(6) = 13 bonus
	// With a d20 roll, need roll + 13 >= difficulty
	rng := rand.New(rand.NewSource(99))
	thief, _ := NewCharacter("Thief", ClassThief)

	trap := &Trap{
		ID:           "t1",
		Description:  "Spike trap",
		Damage:       10,
		Difficulty:   10, // low DC, thief bonus alone almost guarantees success
		IsDiscovered: true,
	}

	result := AttemptDisarm(thief, trap, rng)
	if !result.Success {
		t.Errorf("expected disarm success (roll=%d, DC=%d)", result.Roll, result.DC)
	}
	if !trap.IsDisarmed {
		t.Error("trap should be marked disarmed")
	}
}

func TestAttemptDisarm_FailureTriggersOnDisarmer(t *testing.T) {
	// Use high difficulty so failure is certain with a low-dex character
	rng := rand.New(rand.NewSource(1))
	fighter, _ := NewCharacter("Fighter", ClassFighter) // dex 10 -> bonus 5

	trap := &Trap{
		ID:           "t1",
		Description:  "Deadly trap",
		Damage:       5,
		Difficulty:   30, // impossibly high
		IsDiscovered: true,
	}

	result := AttemptDisarm(fighter, trap, rng)
	if result.Success {
		t.Error("expected disarm failure with DC 30")
	}
	if result.TriggerResult == nil {
		t.Fatal("failure should trigger trap on disarmer")
	}
	if !result.TriggerResult.Triggered {
		t.Error("trigger result should show triggered")
	}
	if fighter.HP >= 30 {
		t.Error("fighter should have taken trap damage")
	}
}

func TestAttemptDisarm_RejectsUndiscovered(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	thief, _ := NewCharacter("Thief", ClassThief)

	trap := &Trap{
		ID:           "t1",
		Difficulty:   10,
		IsDiscovered: false,
	}

	result := AttemptDisarm(thief, trap, rng)
	if result.Success {
		t.Error("should not disarm undiscovered trap")
	}
	if result.TriggerResult != nil {
		t.Error("undiscovered trap should not trigger on failed disarm attempt")
	}
}

func TestFormatTrapDetection(t *testing.T) {
	detected := &TrapDetectionResult{
		Detected:     true,
		DetectorName: "Thief",
		Trap:         &Trap{Description: "Spike trap", Difficulty: 15},
	}
	msg := FormatTrapDetection(detected)
	if !strings.Contains(msg, "Thief") || !strings.Contains(msg, "Spike trap") {
		t.Errorf("detection message = %q", msg)
	}

	undetected := &TrapDetectionResult{Detected: false}
	if FormatTrapDetection(undetected) != "" {
		t.Error("undetected should return empty string")
	}
}

func TestFormatTrapTrigger(t *testing.T) {
	triggered := &TrapTriggerResult{
		Triggered:  true,
		Damage:     10,
		VictimName: "Fighter",
		Trap:       &Trap{Description: "Spike trap"},
	}
	msg := FormatTrapTrigger(triggered)
	if !strings.Contains(msg, "TRAP") || !strings.Contains(msg, "Fighter") {
		t.Errorf("trigger message = %q", msg)
	}

	// With death
	died := &TrapTriggerResult{
		Triggered:  true,
		Damage:     50,
		VictimName: "Fighter",
		VictimDied: true,
		Trap:       &Trap{Description: "Spike trap"},
	}
	msg = FormatTrapTrigger(died)
	if !strings.Contains(msg, "has fallen") {
		t.Errorf("death message = %q", msg)
	}

	// Not triggered
	notTriggered := &TrapTriggerResult{Triggered: false}
	if FormatTrapTrigger(notTriggered) != "" {
		t.Error("not triggered should return empty")
	}
}

func TestFormatDisarmResult(t *testing.T) {
	success := &TrapDisarmResult{
		Success:      true,
		DisarmerName: "Thief",
		Roll:         18,
		DC:           15,
		Trap:         &Trap{Description: "Spike trap"},
	}
	msg := FormatDisarmResult(success)
	if !strings.Contains(msg, "disarms") {
		t.Errorf("success disarm message = %q", msg)
	}

	failure := &TrapDisarmResult{
		Success:      false,
		DisarmerName: "Fighter",
		Roll:         8,
		DC:           15,
		Trap:         &Trap{Description: "Spike trap"},
		TriggerResult: &TrapTriggerResult{
			Triggered:  true,
			Damage:     10,
			VictimName: "Fighter",
			Trap:       &Trap{Description: "Spike trap"},
		},
	}
	msg = FormatDisarmResult(failure)
	if !strings.Contains(msg, "fails") {
		t.Errorf("failure disarm message = %q", msg)
	}
	if !strings.Contains(msg, "TRAP") {
		t.Errorf("failure should include trap trigger: %q", msg)
	}
}
