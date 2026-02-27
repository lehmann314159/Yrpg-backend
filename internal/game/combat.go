package game

import (
	"fmt"
	"math/rand"
)

// Combat constants
const (
	BaseDefense     = 10
	CriticalRoll    = 20 // natural 20 = critical hit
	MinDamage       = 1
	DefendACBonus   = 4
	ThiefRetreatBonus = 4
	RetreatDC       = 12
)

// AttackResult holds the outcome of a single attack
type AttackResult struct {
	Hit         bool
	Critical    bool
	Damage      int
	Roll        int
	ToHit       int
	Defense     int
	Flanking    bool
	AttackerID  string
	AttackerName string
	TargetID    string
	TargetName  string
	TargetHP    int
	TargetMaxHP int
	TargetDied  bool
}

// MoveResult holds the outcome of a grid movement
type MoveResult struct {
	Success    bool
	CombatantID string
	FromX, FromY int
	ToX, ToY     int
	Message    string
}

// RetreatResult holds the outcome of a retreat attempt
type RetreatResult struct {
	Success          bool
	Roll             int
	DC               int
	CombatantID      string
	CombatantName    string
	OpportunityAttack *AttackResult // if engaged, enemy gets a free attack
}

// CombatEndResult describes how combat ended
type CombatEndResult struct {
	AllEnemiesDead bool
	PartyWiped     bool
	PartyRetreated bool
	LootDrops      []*Item
	Message        string
}

// --- Movement ---

// CombatMove moves a combatant on the grid within their movement range.
// Uses Manhattan distance for movement cost.
func CombatMove(cs *CombatState, combatant *Combatant, destX, destY int, movementRange int) *MoveResult {
	result := &MoveResult{
		CombatantID: combatant.ID,
		FromX:       combatant.GridX,
		FromY:       combatant.GridY,
	}

	if combatant.HasMoved {
		result.Message = fmt.Sprintf("%s has already moved this turn.", combatant.Name)
		return result
	}

	// Bounds check
	if destX < 0 || destX >= GridWidth || destY < 0 || destY >= GridHeight {
		result.Message = "Destination is out of bounds."
		return result
	}

	// Check if cell is occupied
	if cs.IsCellOccupied(destX, destY) {
		result.Message = "That cell is occupied."
		return result
	}

	// Check movement range (Manhattan distance)
	dist := ManhattanDistance(combatant.GridX, combatant.GridY, destX, destY)
	if dist > movementRange {
		result.Message = fmt.Sprintf("Too far! %s can move %d cells, destination is %d away.",
			combatant.Name, movementRange, dist)
		return result
	}

	// Execute move
	cs.PlaceOnGrid(combatant, destX, destY)
	combatant.HasMoved = true
	result.Success = true
	result.ToX = destX
	result.ToY = destY
	result.Message = fmt.Sprintf("%s moves to (%d,%d).", combatant.Name, destX, destY)
	return result
}

// --- Attacks ---

// CombatAttack executes an attack from one combatant to another.
// Handles melee/ranged range checking, engagement, flanking advantage, and damage.
func CombatAttack(cs *CombatState, attacker *Combatant, target *Combatant,
	attackerChar *Character, weapon *Item, rng *rand.Rand) *AttackResult {

	result := &AttackResult{
		AttackerID:   attacker.ID,
		AttackerName: attacker.Name,
		TargetID:     target.ID,
		TargetName:   target.Name,
	}

	// Determine if melee or ranged
	isRanged := false
	maxRange := 1
	if weapon != nil && weapon.Range == RangeRanged {
		isRanged = true
		maxRange = weapon.MaxRange
	}

	// Check range
	dist := ChebyshevDistance(attacker.GridX, attacker.GridY, target.GridX, target.GridY)
	if !isRanged {
		// Melee: must be adjacent
		if dist > 1 {
			result.Hit = false
			return result
		}
	} else {
		if dist > maxRange {
			result.Hit = false
			return result
		}
	}

	// Check for flanking advantage
	// Attacking a target that is engaged with someone else (not the attacker) = advantage
	flanking := false
	if engagedWith, ok := cs.Engagements[target.ID]; ok && engagedWith != attacker.ID {
		flanking = true
	}
	result.Flanking = flanking

	// Attack roll: d20 + modifier
	roll1 := rng.Intn(20) + 1
	roll2 := roll1
	if flanking || attacker.IsHidden {
		// Advantage: roll twice, take better
		roll2 = rng.Intn(20) + 1
		if roll2 > roll1 {
			roll1 = roll2
		}
	}

	// Attack modifier
	modifier := 0
	if attackerChar != nil {
		if isRanged {
			modifier = attackerChar.Dexterity / 2
		} else {
			modifier = attackerChar.Strength / 2
		}
	} else {
		// Monster attacking — use dexterity approximation
		modifier = 0
	}

	toHit := roll1 + modifier
	result.Roll = roll1
	result.ToHit = toHit

	// Target defense (base + buff AC bonus)
	defense := BaseDefense + target.GetACBonus()
	result.Defense = defense

	isCritical := roll1 == CriticalRoll

	if toHit >= defense || isCritical {
		result.Hit = true
		result.Critical = isCritical

		// Damage: d6 + modifier + weapon bonus
		numDice := 1
		if isCritical {
			numDice = 2
		}
		dmg := 0
		for i := 0; i < numDice; i++ {
			dmg += rng.Intn(6) + 1
		}
		weaponDmg := 0
		if weapon != nil {
			weaponDmg = weapon.Damage
		}
		if attackerChar != nil {
			if isRanged {
				dmg += attackerChar.Dexterity/2 + weaponDmg
			} else {
				dmg += attackerChar.Strength/2 + weaponDmg
			}
		}
		if dmg < MinDamage {
			dmg = MinDamage
		}
		result.Damage = dmg

		// Damage wakes sleeping targets
		target.RemoveSleep()

		// Apply damage
		target.HP -= dmg
		if target.HP <= 0 {
			target.HP = 0
			target.IsAlive = false
			result.TargetDied = true
			cs.RemoveFromGrid(target)
			// Clean up engagements
			RemoveEngagement(cs, target.ID)
		}

		result.TargetHP = target.HP
		result.TargetMaxHP = target.MaxHP

		// Create engagement (melee only)
		if !isRanged && target.IsAlive {
			SetEngagement(cs, attacker.ID, target.ID)
		}

		// Attacking breaks hidden
		attacker.IsHidden = false
	} else {
		result.Hit = false
		result.TargetHP = target.HP
		result.TargetMaxHP = target.MaxHP
		// Attacking breaks hidden even on miss
		attacker.IsHidden = false
	}

	return result
}

// MonsterAttack executes a monster's attack against a player combatant.
func MonsterAttack(cs *CombatState, attacker *Combatant, target *Combatant,
	monster *Monster, rng *rand.Rand) *AttackResult {

	result := &AttackResult{
		AttackerID:   attacker.ID,
		AttackerName: attacker.Name,
		TargetID:     target.ID,
		TargetName:   target.Name,
	}

	// Check range
	dist := ChebyshevDistance(attacker.GridX, attacker.GridY, target.GridX, target.GridY)
	if monster.IsRanged {
		if dist > monster.AttackRange {
			result.Hit = false
			return result
		}
	} else {
		if dist > 1 {
			result.Hit = false
			return result
		}
	}

	// Attack roll
	roll := rng.Intn(20) + 1
	toHit := roll + monster.Dexterity/2
	result.Roll = roll
	result.ToHit = toHit

	// Target defense (base + buff AC bonus)
	defense := BaseDefense + target.GetACBonus()
	result.Defense = defense

	isCritical := roll == CriticalRoll

	if toHit >= defense || isCritical {
		result.Hit = true
		result.Critical = isCritical

		numDice := 1
		if isCritical {
			numDice = 2
		}
		dmg := 0
		for i := 0; i < numDice; i++ {
			dmg += rng.Intn(6) + 1
		}
		dmg += monster.Damage
		if dmg < MinDamage {
			dmg = MinDamage
		}
		result.Damage = dmg

		// Damage wakes sleeping targets
		target.RemoveSleep()

		target.HP -= dmg
		if target.HP <= 0 {
			target.HP = 0
			target.IsAlive = false
			result.TargetDied = true
			cs.RemoveFromGrid(target)
			RemoveEngagement(cs, target.ID)
		}

		result.TargetHP = target.HP
		result.TargetMaxHP = target.MaxHP

		// Create engagement (melee only)
		if !monster.IsRanged && target.IsAlive {
			SetEngagement(cs, attacker.ID, target.ID)
		}
	} else {
		result.Hit = false
		result.TargetHP = target.HP
		result.TargetMaxHP = target.MaxHP
	}

	return result
}

// --- Engagement ---

// SetEngagement creates a mutual engagement between attacker and target
func SetEngagement(cs *CombatState, attackerID, targetID string) {
	cs.Engagements[attackerID] = targetID
	cs.Engagements[targetID] = attackerID
}

// RemoveEngagement removes all engagements involving the given combatant
func RemoveEngagement(cs *CombatState, combatantID string) {
	// Find who they're engaged with and remove the reverse
	if engagedWith, ok := cs.Engagements[combatantID]; ok {
		if cs.Engagements[engagedWith] == combatantID {
			delete(cs.Engagements, engagedWith)
		}
	}
	delete(cs.Engagements, combatantID)
}

// --- Defend ---

// CombatDefend puts a combatant in full defense mode for the round.
// The defense bonus is applied when calculating defense in attacks.
// We track this via a simple convention: HasActed = true and we check for it.
// For a cleaner implementation, we'd add a DefendingThisRound field.
// For now, callers should check and apply DefendACBonus.

// --- Retreat ---

// AttemptRetreat has a combatant attempt to flee combat.
func AttemptRetreat(cs *CombatState, combatant *Combatant, char *Character, rng *rand.Rand) *RetreatResult {
	result := &RetreatResult{
		CombatantID:   combatant.ID,
		CombatantName: combatant.Name,
		DC:            RetreatDC,
	}

	// Opportunity attack from engaged enemy
	if engagedWith, ok := cs.Engagements[combatant.ID]; ok {
		engaged := cs.GetCombatant(engagedWith)
		if engaged != nil && engaged.IsAlive {
			// Simple opportunity attack (monster attacks the retreating character)
			oaRoll := rng.Intn(20) + 1
			oaDmg := rng.Intn(6) + 1 + 2 // simplified monster damage
			oaResult := &AttackResult{
				AttackerID:   engaged.ID,
				AttackerName: engaged.Name,
				TargetID:     combatant.ID,
				TargetName:   combatant.Name,
				Roll:         oaRoll,
			}
			if oaRoll >= BaseDefense+combatant.GetACBonus() {
				oaResult.Hit = true
				oaResult.Damage = oaDmg
				combatant.HP -= oaDmg
				if combatant.HP <= 0 {
					combatant.HP = 0
					combatant.IsAlive = false
					oaResult.TargetDied = true
					cs.RemoveFromGrid(combatant)
				}
				oaResult.TargetHP = combatant.HP
			} else {
				oaResult.Hit = false
				oaResult.TargetHP = combatant.HP
			}
			result.OpportunityAttack = oaResult
		}
	}

	// If killed by opportunity attack, retreat fails
	if !combatant.IsAlive {
		result.Success = false
		return result
	}

	// Retreat roll: d20 + dex/2 vs DC
	roll := rng.Intn(20) + 1
	bonus := 0
	if char != nil {
		bonus = char.Dexterity / 2
		if char.Class == ClassThief {
			bonus += ThiefRetreatBonus
		}
	}
	total := roll + bonus
	result.Roll = total

	if total >= RetreatDC {
		result.Success = true
		RemoveEngagement(cs, combatant.ID)
		cs.RemoveFromGrid(combatant)
	} else {
		result.Success = false
	}

	return result
}

// CheckPartyRetreat checks if all living player combatants have successfully retreated.
func CheckPartyRetreat(cs *CombatState, retreated map[string]bool) bool {
	for _, c := range cs.GetPlayerCombatants() {
		if c.IsAlive && !retreated[c.ID] {
			return false
		}
	}
	return true
}

// --- Combat Resolution ---

// ResolveCombatEnd checks if combat should end and returns the result.
func ResolveCombatEnd(cs *CombatState) *CombatEndResult {
	if cs.AllEnemiesDead() {
		cs.IsActive = false
		return &CombatEndResult{
			AllEnemiesDead: true,
			Message:        "All enemies defeated! Combat over.",
		}
	}
	if cs.AllPlayersDead() {
		cs.IsActive = false
		return &CombatEndResult{
			PartyWiped: true,
			Message:    "Your party has been wiped out...",
		}
	}
	return nil
}

// SyncCombatToCharacters updates party character HP from combat state.
func SyncCombatToCharacters(cs *CombatState, party *Party) {
	for _, combatant := range cs.GetPlayerCombatants() {
		char := party.GetCharacter(combatant.CharacterID)
		if char != nil {
			char.HP = combatant.HP
			if !combatant.IsAlive {
				char.TakeDamage(char.HP + 1) // ensure death is recorded with DiedAt
			}
		}
	}
}

// SyncCombatToMonsters updates monster HP from combat state.
func SyncCombatToMonsters(cs *CombatState, monsters map[string]*Monster) {
	for _, combatant := range cs.GetEnemyCombatants() {
		monster, ok := monsters[combatant.CharacterID]
		if ok {
			monster.HP = combatant.HP
			if !combatant.IsAlive {
				monster.IsAlive = false
				monster.HP = 0
			}
		}
	}
}

// --- Formatting ---

// FormatAttackResult returns a human-readable attack description
func FormatAttackResult(r *AttackResult) string {
	if !r.Hit {
		return fmt.Sprintf("%s attacks %s but misses! (Roll: %d vs AC %d)",
			r.AttackerName, r.TargetName, r.ToHit, r.Defense)
	}

	prefix := ""
	if r.Critical {
		prefix = "CRITICAL HIT! "
	} else if r.Flanking {
		prefix = "Flanking! "
	}

	msg := fmt.Sprintf("%s%s hits %s for %d damage!",
		prefix, r.AttackerName, r.TargetName, r.Damage)
	if r.TargetDied {
		msg += fmt.Sprintf(" %s falls!", r.TargetName)
	} else {
		msg += fmt.Sprintf(" (%s HP: %d/%d)", r.TargetName, r.TargetHP, r.TargetMaxHP)
	}
	return msg
}

// FormatRetreatResult returns a human-readable retreat description
func FormatRetreatResult(r *RetreatResult) string {
	var msg string
	if r.OpportunityAttack != nil && r.OpportunityAttack.Hit {
		msg += fmt.Sprintf("%s takes an opportunity attack from %s for %d damage! ",
			r.CombatantName, r.OpportunityAttack.AttackerName, r.OpportunityAttack.Damage)
		if r.OpportunityAttack.TargetDied {
			msg += fmt.Sprintf("%s falls while trying to flee!", r.CombatantName)
			return msg
		}
	}

	if r.Success {
		msg += fmt.Sprintf("%s successfully retreats! (Roll: %d vs DC %d)", r.CombatantName, r.Roll, r.DC)
	} else {
		msg += fmt.Sprintf("%s fails to retreat! (Roll: %d vs DC %d)", r.CombatantName, r.Roll, r.DC)
	}
	return msg
}
