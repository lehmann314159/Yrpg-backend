package game

import "testing"

func TestSerializeDeserialize_RoundTrip(t *testing.T) {
	gs := NewGameState("test-session")

	party, _ := NewParty([]PartyRequest{
		{Name: "Fighter", Class: ClassFighter},
		{Name: "Mage", Class: ClassMagicUser},
	})
	gs.Party = party
	gs.Dungeon = &Dungeon{ID: "d1", Seed: 42, Depth: 3}

	room := &Room{ID: "r1", DungeonID: "d1", Name: "Entry", X: 0, Y: 0}
	gs.AddRoom(room)
	gs.MarkRoomVisited("r1")
	gs.AddConnection(&RoomConnection{ID: "c1", RoomID: "r1", Direction: "north", ConnectedRoomID: "r2"})

	monster := &Monster{ID: "m1", Name: "Goblin", HP: 10, MaxHP: 10, RoomID: "r1", IsAlive: true}
	gs.AddMonster(monster)

	roomID := "r1"
	charID := party.Characters[0].ID
	gs.AddItem(&Item{ID: "i1", Name: "Sword", Type: ItemWeapon, RoomID: &roomID})
	gs.AddItem(&Item{ID: "i2", Name: "Shield", Type: ItemArmor, CharacterID: &charID})

	gs.AddTrap(&Trap{ID: "t1", RoomID: "r1", Damage: 10, Difficulty: 15})
	gs.TurnContext.TurnNumber = 7

	// Serialize
	data, err := gs.SerializeState()
	if err != nil {
		t.Fatalf("SerializeState: %v", err)
	}
	if data == "" {
		t.Fatal("serialized data should not be empty")
	}

	// Deserialize
	gs2, err := DeserializeState(data)
	if err != nil {
		t.Fatalf("DeserializeState: %v", err)
	}

	// Verify core fields
	if gs2.SessionID != "test-session" {
		t.Errorf("SessionID = %q, want test-session", gs2.SessionID)
	}
	if gs2.Party == nil || len(gs2.Party.Characters) != 2 {
		t.Error("party should have 2 characters")
	}
	if gs2.Dungeon == nil || gs2.Dungeon.Seed != 42 {
		t.Error("dungeon should be restored")
	}
	if gs2.TurnContext.TurnNumber != 7 {
		t.Errorf("TurnNumber = %d, want 7", gs2.TurnContext.TurnNumber)
	}

	// Verify indexes rebuilt
	if gs2.GetRoomAt(0, 0) == nil {
		t.Error("room coordinate index should be rebuilt")
	}
	if len(gs2.GetRoomMonsters("r1")) != 1 {
		t.Error("monster room index should be rebuilt")
	}
	if len(gs2.GetRoomItems("r1")) != 1 {
		t.Error("item room index should be rebuilt")
	}
	if len(gs2.GetCharacterInventory(charID)) != 1 {
		t.Error("item character index should be rebuilt")
	}
	if len(gs2.GetRoomTraps("r1")) != 1 {
		t.Error("trap room index should be rebuilt")
	}
	if !gs2.IsRoomVisited("r1") {
		t.Error("visited rooms should be restored")
	}
}

func TestDeserializeState_InvalidJSON(t *testing.T) {
	_, err := DeserializeState("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
