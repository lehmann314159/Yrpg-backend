package game

import (
	"fmt"
	"strings"
	"sync"
)

// GameState holds all in-memory game state
type GameState struct {
	mu sync.RWMutex

	SessionID string
	Mode      CombatMode
	Party     *Party
	Dungeon   *Dungeon

	// World state (indexed for O(1) lookup)
	Rooms          map[string]*Room             // room ID -> Room
	RoomsByCoord   map[string]*Room             // "x,y" -> Room
	Connections    map[string][]*RoomConnection  // room ID -> connections
	Monsters       map[string]*Monster          // monster ID -> Monster
	MonstersByRoom map[string]map[string]bool   // room ID -> monster IDs
	Items          map[string]*Item             // item ID -> Item
	ItemsByRoom    map[string]map[string]bool   // room ID -> item IDs
	ItemsByChar    map[string]map[string]bool   // character ID -> item IDs
	Traps          map[string]*Trap             // trap ID -> Trap
	TrapsByRoom    map[string][]string          // room ID -> trap IDs
	VisitedRooms   map[string]bool              // room ID -> visited

	// Combat (nil when in exploration mode)
	Combat *CombatState

	// Game status
	GameOver bool
	Victory  bool

	// Per-turn tracking
	TurnContext *TurnContext
}

// Lock acquires a write lock on the game state
func (gs *GameState) Lock() { gs.mu.Lock() }

// Unlock releases the write lock on the game state
func (gs *GameState) Unlock() { gs.mu.Unlock() }

// RLock acquires a read lock on the game state
func (gs *GameState) RLock() { gs.mu.RLock() }

// RUnlock releases the read lock on the game state
func (gs *GameState) RUnlock() { gs.mu.RUnlock() }

// NewGameState creates an empty, initialized game state
func NewGameState(sessionID string) *GameState {
	return &GameState{
		SessionID:      sessionID,
		Mode:           ModeExploration,
		Rooms:          make(map[string]*Room),
		RoomsByCoord:   make(map[string]*Room),
		Connections:    make(map[string][]*RoomConnection),
		Monsters:       make(map[string]*Monster),
		MonstersByRoom: make(map[string]map[string]bool),
		Items:          make(map[string]*Item),
		ItemsByRoom:    make(map[string]map[string]bool),
		ItemsByChar:    make(map[string]map[string]bool),
		Traps:          make(map[string]*Trap),
		TrapsByRoom:    make(map[string][]string),
		VisitedRooms:   make(map[string]bool),
		TurnContext:    &TurnContext{},
	}
}

// IsInitialized returns true if a game has been started
func (gs *GameState) IsInitialized() bool {
	return gs.Party != nil && gs.Dungeon != nil
}

// InCombat returns true if the game is in combat mode
func (gs *GameState) InCombat() bool {
	return gs.Mode == ModeCombat && gs.Combat != nil && gs.Combat.IsActive
}

// --- Room accessors ---

// GetCurrentRoom returns the room the party is in (based on point character)
func (gs *GameState) GetCurrentRoom() *Room {
	if gs.Party == nil {
		return nil
	}
	point := gs.Party.PointCharacter()
	if point == nil {
		return nil
	}
	return gs.Rooms[point.CurrentRoomID]
}

// GetRoomExits returns the exits available from a room as direction -> room ID
func (gs *GameState) GetRoomExits(roomID string) map[string]string {
	exits := make(map[string]string)
	for _, conn := range gs.Connections[roomID] {
		exits[conn.Direction] = conn.ConnectedRoomID
	}
	return exits
}

// GetRoomAt returns the room at the given coordinates
func (gs *GameState) GetRoomAt(x, y int) *Room {
	return gs.RoomsByCoord[fmt.Sprintf("%d,%d", x, y)]
}

// --- Monster accessors ---

// GetRoomMonsters returns all alive monsters in a room
func (gs *GameState) GetRoomMonsters(roomID string) []*Monster {
	monsters := make([]*Monster, 0)
	monsterIDs, ok := gs.MonstersByRoom[roomID]
	if !ok {
		return monsters
	}
	for monsterID := range monsterIDs {
		if m, exists := gs.Monsters[monsterID]; exists && m.IsAlive {
			monsters = append(monsters, m)
		}
	}
	return monsters
}

// HasMonstersInRoom returns true if there are alive monsters in the room
func (gs *GameState) HasMonstersInRoom(roomID string) bool {
	return len(gs.GetRoomMonsters(roomID)) > 0
}

// --- Item accessors ---

// GetRoomItems returns all items in a room
func (gs *GameState) GetRoomItems(roomID string) []*Item {
	items := make([]*Item, 0)
	itemIDs, ok := gs.ItemsByRoom[roomID]
	if !ok {
		return items
	}
	for itemID := range itemIDs {
		if item, exists := gs.Items[itemID]; exists {
			items = append(items, item)
		}
	}
	return items
}

// GetCharacterInventory returns all items carried by a character
func (gs *GameState) GetCharacterInventory(charID string) []*Item {
	items := make([]*Item, 0)
	itemIDs, ok := gs.ItemsByChar[charID]
	if !ok {
		return items
	}
	for itemID := range itemIDs {
		if item, exists := gs.Items[itemID]; exists {
			items = append(items, item)
		}
	}
	return items
}

// --- Trap accessors ---

// GetRoomTraps returns all traps in a room
func (gs *GameState) GetRoomTraps(roomID string) []*Trap {
	traps := make([]*Trap, 0)
	trapIDs, ok := gs.TrapsByRoom[roomID]
	if !ok {
		return traps
	}
	for _, trapID := range trapIDs {
		if trap, exists := gs.Traps[trapID]; exists {
			traps = append(traps, trap)
		}
	}
	return traps
}

// --- Add entities (with index management) ---

// AddRoom adds a room and updates coordinate index
func (gs *GameState) AddRoom(room *Room) {
	gs.Rooms[room.ID] = room
	gs.RoomsByCoord[fmt.Sprintf("%d,%d", room.X, room.Y)] = room
}

// AddConnection adds a room connection
func (gs *GameState) AddConnection(conn *RoomConnection) {
	gs.Connections[conn.RoomID] = append(gs.Connections[conn.RoomID], conn)
}

// AddMonster adds a monster and updates room index
func (gs *GameState) AddMonster(monster *Monster) {
	gs.Monsters[monster.ID] = monster
	if gs.MonstersByRoom[monster.RoomID] == nil {
		gs.MonstersByRoom[monster.RoomID] = make(map[string]bool)
	}
	gs.MonstersByRoom[monster.RoomID][monster.ID] = true
}

// AddItem adds an item and updates room/character indexes
func (gs *GameState) AddItem(item *Item) {
	gs.Items[item.ID] = item
	if item.RoomID != nil {
		if gs.ItemsByRoom[*item.RoomID] == nil {
			gs.ItemsByRoom[*item.RoomID] = make(map[string]bool)
		}
		gs.ItemsByRoom[*item.RoomID][item.ID] = true
	}
	if item.CharacterID != nil {
		if gs.ItemsByChar[*item.CharacterID] == nil {
			gs.ItemsByChar[*item.CharacterID] = make(map[string]bool)
		}
		gs.ItemsByChar[*item.CharacterID][item.ID] = true
	}
}

// AddTrap adds a trap and updates room index
func (gs *GameState) AddTrap(trap *Trap) {
	gs.Traps[trap.ID] = trap
	gs.TrapsByRoom[trap.RoomID] = append(gs.TrapsByRoom[trap.RoomID], trap.ID)
}

// --- Visited rooms ---

// MarkRoomVisited marks a room as visited
func (gs *GameState) MarkRoomVisited(roomID string) {
	gs.VisitedRooms[roomID] = true
}

// IsRoomVisited returns true if a room has been visited
func (gs *GameState) IsRoomVisited(roomID string) bool {
	return gs.VisitedRooms[roomID]
}

// --- Turn context ---

// ResetTurnContext clears per-turn tracking at the start of each action
func (gs *GameState) ResetTurnContext() {
	gs.TurnContext = &TurnContext{
		TurnNumber:        gs.TurnContext.TurnNumber + 1,
		TurnsInRoom:       gs.TurnContext.TurnsInRoom,
		ConsecutiveCombat: gs.TurnContext.ConsecutiveCombat,
	}
}

// --- Map rendering ---

// IsRoomAdjacent returns true if a room is adjacent to a visited room via a connection
func (gs *GameState) IsRoomAdjacent(roomID string) bool {
	room := gs.Rooms[roomID]
	if room == nil {
		return false
	}

	adjacentCoords := []struct{ x, y int }{
		{room.X - 1, room.Y},
		{room.X + 1, room.Y},
		{room.X, room.Y - 1},
		{room.X, room.Y + 1},
	}

	for _, coord := range adjacentCoords {
		adjRoom := gs.GetRoomAt(coord.x, coord.y)
		if adjRoom != nil && gs.IsRoomVisited(adjRoom.ID) {
			exits := gs.GetRoomExits(adjRoom.ID)
			for _, connectedID := range exits {
				if connectedID == roomID {
					return true
				}
			}
		}
	}
	return false
}

// RenderMap generates a box-drawing map of the dungeon
func (gs *GameState) RenderMap(gridSize int) string {
	currentRoom := gs.GetCurrentRoom()
	if currentRoom == nil {
		return "No map available"
	}

	var sb strings.Builder

	sb.WriteString("┌")
	for x := 0; x < gridSize; x++ {
		sb.WriteString("───")
		if x < gridSize-1 {
			sb.WriteString("┬")
		}
	}
	sb.WriteString("┐\n")

	for y := gridSize - 1; y >= 0; y-- {
		sb.WriteString("│")
		for x := 0; x < gridSize; x++ {
			room := gs.GetRoomAt(x, y)
			cell := "   "

			if room != nil {
				if room.ID == currentRoom.ID {
					cell = " @ "
				} else if room.IsExit && gs.IsRoomVisited(room.ID) {
					cell = " E "
				} else if gs.IsRoomVisited(room.ID) {
					cell = " # "
				} else if gs.IsRoomAdjacent(room.ID) {
					cell = " ? "
				}
			}

			sb.WriteString(cell)
			if x < gridSize-1 {
				sb.WriteString("│")
			}
		}
		sb.WriteString("│\n")

		if y > 0 {
			sb.WriteString("├")
			for x := 0; x < gridSize; x++ {
				sb.WriteString("───")
				if x < gridSize-1 {
					sb.WriteString("┼")
				}
			}
			sb.WriteString("┤\n")
		}
	}

	sb.WriteString("└")
	for x := 0; x < gridSize; x++ {
		sb.WriteString("───")
		if x < gridSize-1 {
			sb.WriteString("┴")
		}
	}
	sb.WriteString("┘\n")

	sb.WriteString("\n@ = Party  # = Explored  ? = Adjacent  E = Exit")

	return sb.String()
}
