package game

// --- Frontend View Types ---

type CharacterView struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Class         string   `json:"class"`
	HP            int      `json:"hp"`
	MaxHP         int      `json:"maxHp"`
	Strength      int      `json:"strength"`
	Dexterity     int      `json:"dexterity"`
	Intelligence  int      `json:"intelligence"`
	SpellSlots    int      `json:"spellSlots"`
	MaxSpellSlots int      `json:"maxSpellSlots"`
	KnownSpells   []string `json:"knownSpells,omitempty"`
	IsAlive       bool        `json:"isAlive"`
	Status        string      `json:"status"`
	Inventory     []*ItemView `json:"inventory"`
}

type PartyView struct {
	Characters []*CharacterView `json:"characters"`
	Formation  []string         `json:"formation"`
}

type RoomView struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	IsEntrance   bool     `json:"isEntrance"`
	IsExit       bool     `json:"isExit"`
	X            int      `json:"x"`
	Y            int      `json:"y"`
	Exits        []string `json:"exits"`
	IsFirstVisit bool     `json:"isFirstVisit"`
}

type MonsterView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	HP          int    `json:"hp"`
	MaxHP       int    `json:"maxHp"`
	Damage      int    `json:"damage"`
	Threat      string `json:"threat"`
	IsDefeated  bool   `json:"isDefeated"`
	IsRanged    bool   `json:"isRanged"`
}

type ItemView struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Type             string   `json:"type"`
	Damage           int      `json:"damage,omitempty"`
	Armor            int      `json:"armor,omitempty"`
	Healing          int      `json:"healing,omitempty"`
	Range            string   `json:"range,omitempty"`
	Rarity           string   `json:"rarity"`
	ClassRestriction []string `json:"classRestriction,omitempty"`
	IsEquipped       bool     `json:"isEquipped"`
}

type TrapView struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	IsDisarmed  bool   `json:"isDisarmed"`
	Difficulty  int    `json:"difficulty"`
}

type CombatantView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	IsPlayerChar bool   `json:"isPlayerChar"`
	HP           int    `json:"hp"`
	MaxHP        int    `json:"maxHp"`
	GridX        int    `json:"gridX"`
	GridY        int    `json:"gridY"`
	HasMoved     bool   `json:"hasMoved"`
	HasActed     bool   `json:"hasActed"`
	IsAlive      bool   `json:"isAlive"`
	IsHidden     bool   `json:"isHidden"`
}

type CombatView struct {
	Grid           [6][6]string     `json:"grid"` // cell contents for display
	Combatants     []*CombatantView `json:"combatants"`
	CurrentTurnIdx int              `json:"currentTurnIdx"`
	RoundNumber    int              `json:"roundNumber"`
	IsActive       bool             `json:"isActive"`
}

type MapCell struct {
	X         int      `json:"x"`
	Y         int      `json:"y"`
	RoomID    string   `json:"roomId,omitempty"`
	Status    string   `json:"status"` // "unknown", "visited", "current", "adjacent", "exit"
	HasPlayer bool     `json:"hasPlayer"`
	Exits     []string `json:"exits,omitempty"`
}

// GameStateSnapshot is the complete game state for the frontend
type GameStateSnapshot struct {
	Mode        string          `json:"mode"` // "exploration" or "combat"
	Party       *PartyView      `json:"party,omitempty"`
	CurrentRoom *RoomView       `json:"currentRoom,omitempty"`
	Monsters    []*MonsterView  `json:"monsters,omitempty"`
	RoomItems   []*ItemView     `json:"roomItems,omitempty"`
	RoomTraps   []*TrapView     `json:"roomTraps,omitempty"` // only discovered traps
	Combat      *CombatView     `json:"combat,omitempty"`
	MapGrid     [][]MapCell     `json:"mapGrid,omitempty"`
	GameOver    bool            `json:"gameOver"`
	Victory     bool            `json:"victory"`
	TurnNumber  int             `json:"turnNumber"`
	Message     string          `json:"message,omitempty"`
	LastEvent   *GameEvent      `json:"lastEvent,omitempty"`
}

// --- Snapshot Builder ---

// BuildSnapshot creates a frontend-ready snapshot of the current game state
func (gs *GameState) BuildSnapshot() *GameStateSnapshot {
	snap := &GameStateSnapshot{
		Mode:       string(gs.Mode),
		GameOver:   gs.GameOver,
		Victory:    gs.Victory,
		TurnNumber: gs.TurnContext.TurnNumber,
		LastEvent:  gs.TurnContext.LastEvent,
	}

	// Party
	if gs.Party != nil {
		snap.Party = buildPartyView(gs.Party, gs)
	}

	// Current room
	currentRoom := gs.GetCurrentRoom()
	if currentRoom != nil {
		snap.CurrentRoom = buildRoomView(currentRoom, gs)

		// Monsters in room
		for _, m := range gs.GetRoomMonsters(currentRoom.ID) {
			snap.Monsters = append(snap.Monsters, buildMonsterView(m))
		}

		// Items in room
		for _, item := range gs.GetRoomItems(currentRoom.ID) {
			snap.RoomItems = append(snap.RoomItems, buildItemView(item))
		}

		// Discovered traps in room
		for _, trap := range gs.GetRoomTraps(currentRoom.ID) {
			if trap.IsDiscovered {
				snap.RoomTraps = append(snap.RoomTraps, buildTrapView(trap))
			}
		}
	}

	// Combat
	if gs.InCombat() {
		snap.Combat = buildCombatView(gs.Combat)
	}

	return snap
}

// --- View Builders ---

func buildPartyView(p *Party, gs *GameState) *PartyView {
	pv := &PartyView{
		Characters: make([]*CharacterView, 0, len(p.Characters)),
		Formation:  p.Formation,
	}
	for _, c := range p.Characters {
		cv := buildCharacterView(c)
		inv := gs.GetCharacterInventory(c.ID)
		cv.Inventory = make([]*ItemView, 0, len(inv))
		for _, item := range inv {
			cv.Inventory = append(cv.Inventory, buildItemView(item))
		}
		pv.Characters = append(pv.Characters, cv)
	}
	return pv
}

func buildCharacterView(c *Character) *CharacterView {
	return &CharacterView{
		ID:            c.ID,
		Name:          c.Name,
		Class:         string(c.Class),
		HP:            c.HP,
		MaxHP:         c.MaxHP,
		Strength:      c.Strength,
		Dexterity:     c.Dexterity,
		Intelligence:  c.Intelligence,
		SpellSlots:    c.SpellSlots,
		MaxSpellSlots: c.MaxSpellSlots,
		KnownSpells:   c.KnownSpells,
		IsAlive:       c.IsAlive,
		Status:        c.HealthStatus(),
	}
}

func buildRoomView(r *Room, gs *GameState) *RoomView {
	exits := make([]string, 0)
	for _, conn := range gs.Connections[r.ID] {
		exits = append(exits, conn.Direction)
	}
	return &RoomView{
		ID:           r.ID,
		Name:         r.Name,
		Description:  r.Description,
		IsEntrance:   r.IsEntrance,
		IsExit:       r.IsExit,
		X:            r.X,
		Y:            r.Y,
		Exits:        exits,
		IsFirstVisit: !gs.IsRoomVisited(r.ID),
	}
}

func buildMonsterView(m *Monster) *MonsterView {
	threat := "normal"
	switch {
	case m.MaxHP >= 30:
		threat = "deadly"
	case m.MaxHP >= 20:
		threat = "dangerous"
	case m.MaxHP <= 8:
		threat = "trivial"
	}
	return &MonsterView{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		HP:          m.HP,
		MaxHP:       m.MaxHP,
		Damage:      m.Damage,
		Threat:      threat,
		IsDefeated:  !m.IsAlive,
		IsRanged:    m.IsRanged,
	}
}

func buildItemView(item *Item) *ItemView {
	iv := &ItemView{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		Type:        string(item.Type),
		Rarity:      item.Rarity,
		IsEquipped:  item.IsEquipped,
	}
	if item.Damage > 0 {
		iv.Damage = item.Damage
	}
	if item.Armor > 0 {
		iv.Armor = item.Armor
	}
	if item.Healing > 0 {
		iv.Healing = item.Healing
	}
	if item.Range != "" {
		iv.Range = string(item.Range)
	}
	if len(item.ClassRestriction) > 0 {
		iv.ClassRestriction = make([]string, len(item.ClassRestriction))
		for i, c := range item.ClassRestriction {
			iv.ClassRestriction[i] = string(c)
		}
	}
	return iv
}

func buildTrapView(t *Trap) *TrapView {
	return &TrapView{
		ID:          t.ID,
		Description: t.Description,
		IsDisarmed:  t.IsDisarmed,
		Difficulty:  t.Difficulty,
	}
}

func buildCombatView(cs *CombatState) *CombatView {
	cv := &CombatView{
		CurrentTurnIdx: cs.CurrentTurnIdx,
		RoundNumber:    cs.RoundNumber,
		IsActive:       cs.IsActive,
	}

	// Build grid display
	for y := 0; y < 6; y++ {
		for x := 0; x < 6; x++ {
			cell := cs.Grid[y][x]
			if cell != nil && cell.OccupantID != nil {
				cv.Grid[y][x] = *cell.OccupantID
			} else if cell != nil && cell.IsBlocked {
				cv.Grid[y][x] = "blocked"
			} else {
				cv.Grid[y][x] = ""
			}
		}
	}

	// Build combatant views
	cv.Combatants = make([]*CombatantView, 0, len(cs.Combatants))
	for _, c := range cs.Combatants {
		cv.Combatants = append(cv.Combatants, &CombatantView{
			ID:           c.ID,
			Name:         c.Name,
			IsPlayerChar: c.IsPlayerChar,
			HP:           c.HP,
			MaxHP:        c.MaxHP,
			GridX:        c.GridX,
			GridY:        c.GridY,
			HasMoved:     c.HasMoved,
			HasActed:     c.HasActed,
			IsAlive:      c.IsAlive,
			IsHidden:     c.IsHidden,
		})
	}

	return cv
}
