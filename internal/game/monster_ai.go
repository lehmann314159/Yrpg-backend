package game

import (
	"math/rand"
)

// MonsterAIAction represents what the monster AI decided to do
type MonsterAIAction struct {
	MovedTo      *GridPos      // nil if didn't move
	AttackResult *AttackResult // nil if didn't attack
	Message      string
}

// ExecuteMonsterTurn runs the AI for a single monster combatant.
// Logic:
//  1. If ranged and a target is in range → attack nearest
//  2. If melee and adjacent to enemy → attack (prefer engaged target)
//  3. If melee and not adjacent → move toward nearest player, attack if now adjacent
func ExecuteMonsterTurn(cs *CombatState, combatant *Combatant, monster *Monster, rng *rand.Rand) *MonsterAIAction {
	action := &MonsterAIAction{}

	// Skip turn if sleeping
	if combatant.IsSleeping() {
		action.Message = combatant.Name + " is asleep and cannot act!"
		return action
	}

	// Find nearest alive player
	players := cs.GetPlayerCombatants()
	var nearest *Combatant
	nearestDist := 999

	for _, p := range players {
		if !p.IsAlive || p.IsHidden {
			continue
		}
		dist := ChebyshevDistance(combatant.GridX, combatant.GridY, p.GridX, p.GridY)
		if dist < nearestDist {
			nearestDist = dist
			nearest = p
		}
	}

	if nearest == nil {
		action.Message = combatant.Name + " has no targets."
		return action
	}

	if monster.IsRanged {
		return monsterRangedTurn(cs, combatant, monster, nearest, nearestDist, rng, action)
	}
	return monsterMeleeTurn(cs, combatant, monster, nearest, nearestDist, players, rng, action)
}

func monsterRangedTurn(cs *CombatState, combatant *Combatant, monster *Monster,
	nearest *Combatant, dist int, rng *rand.Rand, action *MonsterAIAction) *MonsterAIAction {

	if dist <= monster.AttackRange {
		// Attack
		result := MonsterAttack(cs, combatant, nearest, monster, rng)
		action.AttackResult = result
		action.Message = FormatAttackResult(result)
		combatant.HasActed = true
		return action
	}

	// Move closer, then try to attack
	moveToward(cs, combatant, nearest, rng)
	action.MovedTo = &GridPos{X: combatant.GridX, Y: combatant.GridY}

	newDist := ChebyshevDistance(combatant.GridX, combatant.GridY, nearest.GridX, nearest.GridY)
	if newDist <= monster.AttackRange {
		result := MonsterAttack(cs, combatant, nearest, monster, rng)
		action.AttackResult = result
		action.Message = FormatAttackResult(result)
		combatant.HasActed = true
	} else {
		action.Message = combatant.Name + " moves closer."
	}

	return action
}

func monsterMeleeTurn(cs *CombatState, combatant *Combatant, monster *Monster,
	nearest *Combatant, dist int, players []*Combatant, rng *rand.Rand, action *MonsterAIAction) *MonsterAIAction {

	// Check if already adjacent to someone
	var adjacentTarget *Combatant

	// Prefer the target we're already engaged with
	if engagedWith, ok := cs.Engagements[combatant.ID]; ok {
		engaged := cs.GetCombatant(engagedWith)
		if engaged != nil && engaged.IsAlive && !engaged.IsHidden {
			if IsAdjacent(combatant.GridX, combatant.GridY, engaged.GridX, engaged.GridY) {
				adjacentTarget = engaged
			}
		}
	}

	// Otherwise, find any adjacent player
	if adjacentTarget == nil {
		for _, p := range players {
			if !p.IsAlive || p.IsHidden {
				continue
			}
			if IsAdjacent(combatant.GridX, combatant.GridY, p.GridX, p.GridY) {
				adjacentTarget = p
				break
			}
		}
	}

	if adjacentTarget != nil {
		// Attack
		result := MonsterAttack(cs, combatant, adjacentTarget, monster, rng)
		action.AttackResult = result
		action.Message = FormatAttackResult(result)
		combatant.HasActed = true
		return action
	}

	// Move toward nearest player
	moveToward(cs, combatant, nearest, rng)
	action.MovedTo = &GridPos{X: combatant.GridX, Y: combatant.GridY}
	combatant.HasMoved = true

	// Check if now adjacent after moving
	if IsAdjacent(combatant.GridX, combatant.GridY, nearest.GridX, nearest.GridY) {
		result := MonsterAttack(cs, combatant, nearest, monster, rng)
		action.AttackResult = result
		action.Message = FormatAttackResult(result)
		combatant.HasActed = true
	} else {
		action.Message = combatant.Name + " moves closer."
	}

	return action
}

// moveToward moves a combatant one step toward the target using simple greedy movement.
func moveToward(cs *CombatState, combatant *Combatant, target *Combatant, rng *rand.Rand) {
	movesLeft := MonsterMovementRange

	for movesLeft > 0 {
		bestDist := ChebyshevDistance(combatant.GridX, combatant.GridY, target.GridX, target.GridY)
		var candidates []GridPos

		// Check all 8 neighbors
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				nx, ny := combatant.GridX+dx, combatant.GridY+dy
				if nx < 0 || nx >= GridWidth || ny < 0 || ny >= GridHeight {
					continue
				}
				if cs.IsCellOccupied(nx, ny) {
					continue
				}
				dist := ChebyshevDistance(nx, ny, target.GridX, target.GridY)
				if dist < bestDist {
					bestDist = dist
					candidates = []GridPos{{X: nx, Y: ny}}
				} else if dist == bestDist && len(candidates) > 0 {
					candidates = append(candidates, GridPos{X: nx, Y: ny})
				}
			}
		}

		// If we can't get closer, stop
		if len(candidates) == 0 {
			break
		}

		// Pick randomly among equally good moves
		chosen := candidates[rng.Intn(len(candidates))]
		cs.PlaceOnGrid(combatant, chosen.X, chosen.Y)
		movesLeft--

		// Stop if adjacent to target
		if IsAdjacent(combatant.GridX, combatant.GridY, target.GridX, target.GridY) {
			break
		}
	}
}
