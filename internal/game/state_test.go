package game

import "testing"

func TestNewGameState(t *testing.T) {
	gs := NewGameState("session-1")

	if gs.SessionID != "session-1" {
		t.Errorf("SessionID = %q, want session-1", gs.SessionID)
	}
	if gs.Mode != ModeExploration {
		t.Errorf("Mode = %q, want exploration", gs.Mode)
	}
	if gs.Rooms == nil || gs.RoomsByCoord == nil || gs.Monsters == nil ||
		gs.MonstersByRoom == nil || gs.Items == nil || gs.ItemsByRoom == nil ||
		gs.ItemsByChar == nil || gs.Traps == nil || gs.TrapsByRoom == nil ||
		gs.VisitedRooms == nil {
		t.Error("all maps should be initialized")
	}
	if gs.TurnContext == nil {
		t.Error("TurnContext should be initialized")
	}
}

func TestIsInitialized(t *testing.T) {
	gs := NewGameState("s1")

	if gs.IsInitialized() {
		t.Error("should not be initialized without party and dungeon")
	}

	gs.Party = &Party{ID: "p1"}
	if gs.IsInitialized() {
		t.Error("should not be initialized without dungeon")
	}

	gs.Dungeon = &Dungeon{ID: "d1"}
	if !gs.IsInitialized() {
		t.Error("should be initialized with both party and dungeon")
	}
}

func TestInCombat(t *testing.T) {
	gs := NewGameState("s1")

	if gs.InCombat() {
		t.Error("exploration mode should not be in combat")
	}

	gs.Mode = ModeCombat
	gs.Combat = &CombatState{IsActive: true}
	if !gs.InCombat() {
		t.Error("combat mode with active combat should return true")
	}

	gs.Combat.IsActive = false
	if gs.InCombat() {
		t.Error("inactive combat should return false")
	}
}

func TestGetCurrentRoom(t *testing.T) {
	gs := NewGameState("s1")

	if gs.GetCurrentRoom() != nil {
		t.Error("no party should return nil")
	}

	party, _ := NewParty([]PartyRequest{{Name: "Hero", Class: ClassFighter}})
	gs.Party = party

	room := &Room{ID: "r1", X: 0, Y: 0}
	gs.AddRoom(room)
	party.MoveAllToRoom("r1")

	current := gs.GetCurrentRoom()
	if current == nil || current.ID != "r1" {
		t.Error("should return the room the party is in")
	}
}

func TestAddRoom_GetRoomAt(t *testing.T) {
	gs := NewGameState("s1")

	room := &Room{ID: "r1", X: 2, Y: 3}
	gs.AddRoom(room)

	if gs.Rooms["r1"] == nil {
		t.Error("room should be in Rooms map")
	}
	if gs.GetRoomAt(2, 3) == nil {
		t.Error("room should be found by coordinates")
	}
	if gs.GetRoomAt(0, 0) != nil {
		t.Error("should return nil for empty coordinates")
	}
}

func TestAddMonster_GetRoomMonsters_HasMonstersInRoom(t *testing.T) {
	gs := NewGameState("s1")

	m := &Monster{ID: "m1", RoomID: "r1", IsAlive: true}
	gs.AddMonster(m)

	monsters := gs.GetRoomMonsters("r1")
	if len(monsters) != 1 {
		t.Errorf("monster count = %d, want 1", len(monsters))
	}
	if !gs.HasMonstersInRoom("r1") {
		t.Error("should have monsters in room")
	}

	// Dead monster should not appear
	m.IsAlive = false
	monsters = gs.GetRoomMonsters("r1")
	if len(monsters) != 0 {
		t.Errorf("dead monster count = %d, want 0", len(monsters))
	}
	if gs.HasMonstersInRoom("r1") {
		t.Error("should not have alive monsters")
	}

	// Empty room
	if gs.HasMonstersInRoom("r2") {
		t.Error("empty room should have no monsters")
	}
}

func TestAddItem_GetRoomItems_GetCharacterInventory(t *testing.T) {
	gs := NewGameState("s1")

	roomID := "r1"
	charID := "c1"

	roomItem := &Item{ID: "i1", Name: "Sword", RoomID: &roomID}
	charItem := &Item{ID: "i2", Name: "Shield", CharacterID: &charID}

	gs.AddItem(roomItem)
	gs.AddItem(charItem)

	roomItems := gs.GetRoomItems("r1")
	if len(roomItems) != 1 || roomItems[0].ID != "i1" {
		t.Errorf("room items = %v, want [i1]", roomItems)
	}

	inv := gs.GetCharacterInventory("c1")
	if len(inv) != 1 || inv[0].ID != "i2" {
		t.Errorf("inventory = %v, want [i2]", inv)
	}

	// Empty
	if len(gs.GetRoomItems("r2")) != 0 {
		t.Error("empty room should have no items")
	}
	if len(gs.GetCharacterInventory("c2")) != 0 {
		t.Error("empty char should have no items")
	}
}

func TestAddTrap_GetRoomTraps(t *testing.T) {
	gs := NewGameState("s1")

	trap := &Trap{ID: "t1", RoomID: "r1"}
	gs.AddTrap(trap)

	traps := gs.GetRoomTraps("r1")
	if len(traps) != 1 || traps[0].ID != "t1" {
		t.Errorf("traps = %v, want [t1]", traps)
	}

	if len(gs.GetRoomTraps("r2")) != 0 {
		t.Error("empty room should have no traps")
	}
}

func TestMarkRoomVisited_IsRoomVisited(t *testing.T) {
	gs := NewGameState("s1")

	if gs.IsRoomVisited("r1") {
		t.Error("room should not be visited initially")
	}

	gs.MarkRoomVisited("r1")
	if !gs.IsRoomVisited("r1") {
		t.Error("room should be visited after marking")
	}
}

func TestResetTurnContext(t *testing.T) {
	gs := NewGameState("s1")
	gs.TurnContext.TurnNumber = 5
	gs.TurnContext.TurnsInRoom = 3
	gs.TurnContext.ConsecutiveCombat = 2
	gs.TurnContext.DefeatedMonsters = []string{"m1"}

	gs.ResetTurnContext()

	if gs.TurnContext.TurnNumber != 6 {
		t.Errorf("TurnNumber = %d, want 6", gs.TurnContext.TurnNumber)
	}
	if gs.TurnContext.TurnsInRoom != 3 {
		t.Errorf("TurnsInRoom = %d, want 3 (preserved)", gs.TurnContext.TurnsInRoom)
	}
	if gs.TurnContext.ConsecutiveCombat != 2 {
		t.Errorf("ConsecutiveCombat = %d, want 2 (preserved)", gs.TurnContext.ConsecutiveCombat)
	}
	if gs.TurnContext.DefeatedMonsters != nil {
		t.Error("DefeatedMonsters should be nil/reset")
	}
}

func TestAddConnection_GetRoomExits(t *testing.T) {
	gs := NewGameState("s1")

	gs.AddConnection(&RoomConnection{ID: "c1", RoomID: "r1", Direction: "north", ConnectedRoomID: "r2"})
	gs.AddConnection(&RoomConnection{ID: "c2", RoomID: "r1", Direction: "east", ConnectedRoomID: "r3"})

	exits := gs.GetRoomExits("r1")
	if len(exits) != 2 {
		t.Errorf("exit count = %d, want 2", len(exits))
	}
	if exits["north"] != "r2" {
		t.Errorf("north exit = %q, want r2", exits["north"])
	}
	if exits["east"] != "r3" {
		t.Errorf("east exit = %q, want r3", exits["east"])
	}

	// No exits
	exits = gs.GetRoomExits("r99")
	if len(exits) != 0 {
		t.Error("unknown room should have no exits")
	}
}

func TestIsRoomAdjacent(t *testing.T) {
	gs := NewGameState("s1")

	// Create rooms at (0,0) and (1,0) with a connection between them
	room1 := &Room{ID: "r1", X: 0, Y: 0}
	room2 := &Room{ID: "r2", X: 1, Y: 0}
	room3 := &Room{ID: "r3", X: 2, Y: 0}
	gs.AddRoom(room1)
	gs.AddRoom(room2)
	gs.AddRoom(room3)

	gs.AddConnection(&RoomConnection{ID: "c1", RoomID: "r1", Direction: "east", ConnectedRoomID: "r2"})
	gs.AddConnection(&RoomConnection{ID: "c2", RoomID: "r2", Direction: "east", ConnectedRoomID: "r3"})

	// Mark r1 as visited
	gs.MarkRoomVisited("r1")

	// r2 should be adjacent to visited r1 (r1 has exit to r2)
	if !gs.IsRoomAdjacent("r2") {
		t.Error("r2 should be adjacent to visited r1")
	}

	// r3 is not adjacent to any visited room
	if gs.IsRoomAdjacent("r3") {
		t.Error("r3 should not be adjacent (r2 not visited)")
	}

	// Unknown room
	if gs.IsRoomAdjacent("unknown") {
		t.Error("unknown room should not be adjacent")
	}
}
