package game

import (
	"math/rand"
	"sort"
)

const (
	GridWidth            = 6
	GridHeight           = 6
	MonsterMovementRange = 2
)

// charArmorBonus returns the armor bonus for a character based on their equipped armor.
func charArmorBonus(c *Character, items map[string]*Item) int {
	if c.EquippedArmorID == nil {
		return 0
	}
	if armor, ok := items[*c.EquippedArmorID]; ok {
		return armor.Armor
	}
	return 0
}

// InitCombat creates a new CombatState from the party and monsters in the room.
// If sneakResult is non-nil and successful, the thief starts hidden with a surprise round.
func InitCombat(party *Party, monsters []*Monster, previousRoomID string, sneakResult *SneakResult, items map[string]*Item, rng *rand.Rand) *CombatState {
	cs := &CombatState{
		Combatants:     make([]*Combatant, 0),
		CurrentTurnIdx: 0,
		RoundNumber:    1,
		Engagements:    make(map[string]string),
		IsActive:       true,
		PreviousRoomID: previousRoomID,
	}

	// Initialize grid
	for y := 0; y < GridHeight; y++ {
		for x := 0; x < GridWidth; x++ {
			cs.Grid[y][x] = &GridCell{X: x, Y: y}
		}
	}

	// Place party characters in row 0, centered
	aliveChars := party.AliveCharacters()
	startX := (GridWidth - len(aliveChars)) / 2
	for i, c := range aliveChars {
		x := startX + i
		combatant := &Combatant{
			ID:           c.ID,
			Name:         c.Name,
			IsPlayerChar: true,
			CharacterID:  c.ID,
			HP:           c.HP,
			MaxHP:        c.MaxHP,
			GridX:        x,
			GridY:        0,
			IsAlive:      true,
			ArmorBonus:   charArmorBonus(c, items),
		}

		// Check if thief started hidden
		if sneakResult != nil && sneakResult.Success && c.ID == sneakResult.SneakerID {
			combatant.IsHidden = true
		}

		cs.Combatants = append(cs.Combatants, combatant)
		cs.Grid[0][x].OccupantID = &c.ID
	}

	// Place monsters in row 5, centered
	startX = (GridWidth - len(monsters)) / 2
	if startX < 0 {
		startX = 0
	}
	for i, m := range monsters {
		x := startX + i
		if x >= GridWidth {
			x = GridWidth - 1
		}
		// Find an unoccupied cell in the back rows
		placed := false
		for _, py := range []int{5, 4} {
			for px := x; px < GridWidth; px++ {
				if cs.Grid[py][px].OccupantID == nil {
					combatant := &Combatant{
						ID:           m.ID,
						Name:         m.Name,
						IsPlayerChar: false,
						CharacterID:  m.ID,
						HP:           m.HP,
						MaxHP:        m.MaxHP,
						GridX:        px,
						GridY:        py,
						IsAlive:      true,
					}
					cs.Combatants = append(cs.Combatants, combatant)
					cs.Grid[py][px].OccupantID = &m.ID
					placed = true
					break
				}
			}
			if placed {
				break
			}
		}
	}

	// Roll initiative
	for _, c := range cs.Combatants {
		roll := rng.Intn(20) + 1
		dexBonus := 0
		initBonus := 0

		if c.IsPlayerChar {
			// Look up character stats
			char := party.GetCharacter(c.CharacterID)
			if char != nil {
				dexBonus = char.Dexterity / 2
				initBonus = char.GetInitiativeBonus()
			}
		} else {
			// Look up monster stats
			for _, m := range monsters {
				if m.ID == c.CharacterID {
					dexBonus = m.Dexterity / 2
					break
				}
			}
		}

		c.Initiative = roll + dexBonus + initBonus
	}

	// Sort by initiative (descending). Ties: players first, then alphabetical.
	sort.SliceStable(cs.Combatants, func(i, j int) bool {
		if cs.Combatants[i].Initiative != cs.Combatants[j].Initiative {
			return cs.Combatants[i].Initiative > cs.Combatants[j].Initiative
		}
		// Tie: players first
		if cs.Combatants[i].IsPlayerChar != cs.Combatants[j].IsPlayerChar {
			return cs.Combatants[i].IsPlayerChar
		}
		return cs.Combatants[i].Name < cs.Combatants[j].Name
	})

	// If there's a surprise round (thief sneaked), put the thief first
	if sneakResult != nil && sneakResult.Success {
		for i, c := range cs.Combatants {
			if c.ID == sneakResult.SneakerID {
				// Move to front
				entry := cs.Combatants[i]
				cs.Combatants = append(cs.Combatants[:i], cs.Combatants[i+1:]...)
				cs.Combatants = append([]*Combatant{entry}, cs.Combatants...)
				break
			}
		}
	}

	return cs
}

// InitScoutCombat creates a combat state with only the scouting thief on the grid.
// Monsters are placed but frozen (0 initiative). The thief gets initiative 100 (guaranteed first)
// and starts hidden.
func InitScoutCombat(thief *Character, monsters []*Monster, previousRoomID string, items map[string]*Item, rng *rand.Rand) *CombatState {
	cs := &CombatState{
		Combatants:     make([]*Combatant, 0),
		CurrentTurnIdx: 0,
		RoundNumber:    1,
		Engagements:    make(map[string]string),
		IsActive:       true,
		PreviousRoomID: previousRoomID,
		IsScoutPhase:   true,
		ScoutID:        thief.ID,
	}

	// Initialize grid
	for y := 0; y < GridHeight; y++ {
		for x := 0; x < GridWidth; x++ {
			cs.Grid[y][x] = &GridCell{X: x, Y: y}
		}
	}

	// Place thief in row 0, centered, hidden
	startX := GridWidth / 2
	thiefCombatant := &Combatant{
		ID:           thief.ID,
		Name:         thief.Name,
		IsPlayerChar: true,
		CharacterID:  thief.ID,
		HP:           thief.HP,
		MaxHP:        thief.MaxHP,
		GridX:        startX,
		GridY:        0,
		IsAlive:      true,
		IsHidden:     true,
		ArmorBonus:   charArmorBonus(thief, items),
		Initiative:   100, // guaranteed first
	}
	cs.Combatants = append(cs.Combatants, thiefCombatant)
	cs.Grid[0][startX].OccupantID = &thief.ID

	// Place monsters in rows 4-5, centered, with 0 initiative (frozen)
	monsterStartX := (GridWidth - len(monsters)) / 2
	if monsterStartX < 0 {
		monsterStartX = 0
	}
	for i, m := range monsters {
		x := monsterStartX + i
		if x >= GridWidth {
			x = GridWidth - 1
		}
		placed := false
		for _, py := range []int{5, 4} {
			for px := x; px < GridWidth; px++ {
				if cs.Grid[py][px].OccupantID == nil {
					combatant := &Combatant{
						ID:           m.ID,
						Name:         m.Name,
						IsPlayerChar: false,
						CharacterID:  m.ID,
						HP:           m.HP,
						MaxHP:        m.MaxHP,
						GridX:        px,
						GridY:        py,
						IsAlive:      true,
						Initiative:   0, // frozen
					}
					cs.Combatants = append(cs.Combatants, combatant)
					cs.Grid[py][px].OccupantID = &m.ID
					placed = true
					break
				}
			}
			if placed {
				break
			}
		}
	}

	// Sort: thief first (initiative 100), then monsters (initiative 0)
	sort.SliceStable(cs.Combatants, func(i, j int) bool {
		return cs.Combatants[i].Initiative > cs.Combatants[j].Initiative
	})

	return cs
}

// AddPartyToCombat places remaining alive party members into an existing scout combat,
// re-rolls initiative for everyone, and clears the scout phase flags.
func AddPartyToCombat(cs *CombatState, party *Party, monsters []*Monster, items map[string]*Item, rng *rand.Rand) {
	// Place remaining alive party members (skip the scout who is already in combat)
	newMembers := make([]*Character, 0)
	for _, c := range party.AliveCharacters() {
		if c.ID == cs.ScoutID {
			continue
		}
		newMembers = append(newMembers, c)
	}

	// Find open cells in rows 0-1 for new party members
	for _, c := range newMembers {
		placed := false
		for _, py := range []int{0, 1} {
			for px := 0; px < GridWidth; px++ {
				if cs.Grid[py][px].OccupantID == nil {
					combatant := &Combatant{
						ID:           c.ID,
						Name:         c.Name,
						IsPlayerChar: true,
						CharacterID:  c.ID,
						HP:           c.HP,
						MaxHP:        c.MaxHP,
						GridX:        px,
						GridY:        py,
						IsAlive:      true,
						ArmorBonus:   charArmorBonus(c, items),
					}
					cs.Combatants = append(cs.Combatants, combatant)
					cs.Grid[py][px].OccupantID = &c.ID
					placed = true
					break
				}
			}
			if placed {
				break
			}
		}
	}

	// Re-roll initiative for ALL combatants
	for _, c := range cs.Combatants {
		roll := rng.Intn(20) + 1
		dexBonus := 0
		initBonus := 0

		if c.IsPlayerChar {
			char := party.GetCharacter(c.CharacterID)
			if char != nil {
				dexBonus = char.Dexterity / 2
				initBonus = char.GetInitiativeBonus()
			}
		} else {
			for _, m := range monsters {
				if m.ID == c.CharacterID {
					dexBonus = m.Dexterity / 2
					break
				}
			}
		}

		c.Initiative = roll + dexBonus + initBonus
	}

	// Sort by initiative (descending), ties: players first, then alphabetical
	sort.SliceStable(cs.Combatants, func(i, j int) bool {
		if cs.Combatants[i].Initiative != cs.Combatants[j].Initiative {
			return cs.Combatants[i].Initiative > cs.Combatants[j].Initiative
		}
		if cs.Combatants[i].IsPlayerChar != cs.Combatants[j].IsPlayerChar {
			return cs.Combatants[i].IsPlayerChar
		}
		return cs.Combatants[i].Name < cs.Combatants[j].Name
	})

	// Clear scout phase flags and reset turn state
	cs.IsScoutPhase = false
	cs.AwaitingScoutDecision = false
	cs.CurrentTurnIdx = 0
	cs.RoundNumber = 1
	for _, c := range cs.Combatants {
		c.HasMoved = false
		c.HasActed = false
	}
}

// GetCurrentCombatant returns the combatant whose turn it is
func (cs *CombatState) GetCurrentCombatant() *Combatant {
	if cs.CurrentTurnIdx >= len(cs.Combatants) {
		return nil
	}
	return cs.Combatants[cs.CurrentTurnIdx]
}

// AdvanceTurn moves to the next living combatant. Returns true if a new round starts.
func (cs *CombatState) AdvanceTurn() bool {
	if len(cs.Combatants) == 0 {
		cs.IsActive = false
		return false
	}

	newRound := false
	checked := 0
	for checked < len(cs.Combatants) {
		cs.CurrentTurnIdx++
		if cs.CurrentTurnIdx >= len(cs.Combatants) {
			// New round
			cs.CurrentTurnIdx = 0
			cs.RoundNumber++
			newRound = true
			// Reset per-turn flags, clear protection, and tick buffs
			for _, c := range cs.Combatants {
				c.HasMoved = false
				c.HasActed = false
				c.ProtectedBy = ""
				c.TickBuffs()
			}
		}

		current := cs.Combatants[cs.CurrentTurnIdx]
		if current.IsAlive {
			return newRound
		}
		checked++
	}

	// All combatants are dead
	cs.IsActive = false
	return newRound
}

// GetCombatant returns a combatant by ID
func (cs *CombatState) GetCombatant(id string) *Combatant {
	for _, c := range cs.Combatants {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// GetPlayerCombatants returns all player combatants
func (cs *CombatState) GetPlayerCombatants() []*Combatant {
	result := make([]*Combatant, 0)
	for _, c := range cs.Combatants {
		if c.IsPlayerChar {
			result = append(result, c)
		}
	}
	return result
}

// GetEnemyCombatants returns all non-player combatants
func (cs *CombatState) GetEnemyCombatants() []*Combatant {
	result := make([]*Combatant, 0)
	for _, c := range cs.Combatants {
		if !c.IsPlayerChar {
			result = append(result, c)
		}
	}
	return result
}

// AllEnemiesDead returns true if all enemies are dead
func (cs *CombatState) AllEnemiesDead() bool {
	for _, c := range cs.Combatants {
		if !c.IsPlayerChar && c.IsAlive {
			return false
		}
	}
	return true
}

// AllPlayersDead returns true if all players are dead
func (cs *CombatState) AllPlayersDead() bool {
	for _, c := range cs.Combatants {
		if c.IsPlayerChar && c.IsAlive {
			return false
		}
	}
	return true
}

// RemoveFromGrid clears a combatant's grid cell
func (cs *CombatState) RemoveFromGrid(c *Combatant) {
	if c.GridY >= 0 && c.GridY < GridHeight && c.GridX >= 0 && c.GridX < GridWidth {
		cell := cs.Grid[c.GridY][c.GridX]
		if cell.OccupantID != nil && *cell.OccupantID == c.ID {
			cell.OccupantID = nil
		}
	}
}

// PlaceOnGrid places a combatant on the grid
func (cs *CombatState) PlaceOnGrid(c *Combatant, x, y int) {
	if x < 0 || x >= GridWidth || y < 0 || y >= GridHeight {
		return
	}
	cs.RemoveFromGrid(c)
	c.GridX = x
	c.GridY = y
	cs.Grid[y][x].OccupantID = &c.ID
}

// IsCellOccupied returns true if a grid cell has an occupant
func (cs *CombatState) IsCellOccupied(x, y int) bool {
	if x < 0 || x >= GridWidth || y < 0 || y >= GridHeight {
		return true // out of bounds = blocked
	}
	cell := cs.Grid[y][x]
	return cell.OccupantID != nil || cell.IsBlocked
}

// ManhattanDistance returns the Manhattan distance between two points
func ManhattanDistance(x1, y1, x2, y2 int) int {
	dx := x1 - x2
	if dx < 0 {
		dx = -dx
	}
	dy := y1 - y2
	if dy < 0 {
		dy = -dy
	}
	return dx + dy
}

// ChebyshevDistance returns the Chebyshev (chess king) distance between two points.
// Used for adjacency checks (diagonal = 1).
func ChebyshevDistance(x1, y1, x2, y2 int) int {
	dx := x1 - x2
	if dx < 0 {
		dx = -dx
	}
	dy := y1 - y2
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

// IsAdjacent returns true if two positions are in adjacent cells (including diagonals)
func IsAdjacent(x1, y1, x2, y2 int) bool {
	return ChebyshevDistance(x1, y1, x2, y2) == 1
}

// --- Buff helpers ---

// AddBuff adds a temporary buff to a combatant
func (c *Combatant) AddBuff(buffType BuffType, value, rounds int, source string) {
	c.Buffs = append(c.Buffs, Buff{
		Type:       buffType,
		Value:      value,
		RoundsLeft: rounds,
		SourceName: source,
	})
}

// GetACBonus returns total AC bonus from armor and buffs (including defend action)
func (c *Combatant) GetACBonus() int {
	bonus := c.ArmorBonus
	for _, b := range c.Buffs {
		if b.Type == BuffACBonus {
			bonus += b.Value
		}
	}
	return bonus
}

// IsSleeping returns true if the combatant has an active sleep debuff
func (c *Combatant) IsSleeping() bool {
	for _, b := range c.Buffs {
		if b.Type == BuffSleep && b.RoundsLeft > 0 {
			return true
		}
	}
	return false
}

// TickBuffs decrements all buff durations and removes expired ones
func (c *Combatant) TickBuffs() []string {
	var expired []string
	active := make([]Buff, 0, len(c.Buffs))
	for _, b := range c.Buffs {
		b.RoundsLeft--
		if b.RoundsLeft <= 0 {
			expired = append(expired, b.SourceName)
		} else {
			active = append(active, b)
		}
	}
	c.Buffs = active
	return expired
}

// RemoveSleep removes sleep debuffs (e.g. when taking damage)
func (c *Combatant) RemoveSleep() {
	active := make([]Buff, 0, len(c.Buffs))
	for _, b := range c.Buffs {
		if b.Type != BuffSleep {
			active = append(active, b)
		}
	}
	c.Buffs = active
}
