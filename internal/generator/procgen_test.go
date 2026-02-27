package generator

import (
	"testing"

	"github.com/lehmann314159/yrpg-backend/internal/game"
)

func testRoom(id string, difficulty int) *game.Room {
	// difficulty = x + y, so use x=difficulty, y=0
	return &game.Room{ID: id, DungeonID: "d1", Name: "Test Room", X: difficulty, Y: 0}
}

// --- spawnChestLoot ---

func TestSpawnChestLoot_AlwaysReturnsAtLeastOneItem(t *testing.T) {
	dg := NewDungeonGenerator(42)
	room := testRoom("r1", 2)

	for i := 0; i < 50; i++ {
		items := dg.spawnChestLoot(room, 2, "chest1")
		if len(items) < 1 {
			t.Fatalf("iteration %d: expected at least 1 item, got %d", i, len(items))
		}
		if len(items) > 2 {
			t.Fatalf("iteration %d: expected at most 2 items, got %d", i, len(items))
		}
	}
}

func TestSpawnChestLoot_ItemsLinkedToChestID(t *testing.T) {
	dg := NewDungeonGenerator(42)
	room := testRoom("r1", 2)
	chestID := "chest_abc"

	items := dg.spawnChestLoot(room, 2, chestID)
	for _, item := range items {
		if item.ChestTrapID == nil {
			t.Errorf("item %s has nil ChestTrapID", item.ID)
		} else if *item.ChestTrapID != chestID {
			t.Errorf("item %s ChestTrapID = %q, want %q", item.ID, *item.ChestTrapID, chestID)
		}
	}
}

func TestSpawnChestLoot_ItemsHaveRoomID(t *testing.T) {
	dg := NewDungeonGenerator(42)
	room := testRoom("r1", 2)

	items := dg.spawnChestLoot(room, 2, "chest1")
	for _, item := range items {
		if item.RoomID == nil {
			t.Errorf("item %s has nil RoomID", item.ID)
		} else if *item.RoomID != "r1" {
			t.Errorf("item %s RoomID = %q, want r1", item.ID, *item.RoomID)
		}
	}
}

func TestSpawnChestLoot_SecondItemChance(t *testing.T) {
	// Over many iterations, we should see some 1-item and some 2-item results
	oneItem := 0
	twoItem := 0

	for seed := int64(0); seed < 200; seed++ {
		dg := NewDungeonGenerator(seed)
		room := testRoom("r1", 3)
		items := dg.spawnChestLoot(room, 3, "chest1")
		switch len(items) {
		case 1:
			oneItem++
		case 2:
			twoItem++
		}
	}

	if oneItem == 0 {
		t.Error("expected some chests with exactly 1 item")
	}
	if twoItem == 0 {
		t.Error("expected some chests with 2 items")
	}
	// 35% second item chance, so roughly 65/35 split
	ratio := float64(twoItem) / float64(oneItem+twoItem)
	if ratio < 0.15 || ratio > 0.55 {
		t.Errorf("second item ratio = %.2f, expected roughly 0.35 (got %d/%d)", ratio, twoItem, oneItem+twoItem)
	}
}

// --- spawnTraps (chest integration) ---

func TestSpawnTraps_ChestHasTrapEntity(t *testing.T) {
	// Run many seeds to find one that spawns a chest (20% base chance)
	for seed := int64(0); seed < 200; seed++ {
		dg := NewDungeonGenerator(seed)
		room := testRoom("r1", 3)
		traps, items := dg.spawnTraps(room, 3)

		// Find chest traps
		var chestTraps []*game.Trap
		for _, trap := range traps {
			if trap.Location == game.TrapChest {
				chestTraps = append(chestTraps, trap)
			}
		}

		if len(chestTraps) == 0 {
			continue
		}

		// Every chest trap should have linked items
		chest := chestTraps[0]
		linkedItems := 0
		for _, item := range items {
			if item.ChestTrapID != nil && *item.ChestTrapID == chest.ID {
				linkedItems++
			}
		}
		if linkedItems == 0 {
			t.Fatalf("seed %d: chest trap %s has no linked items", seed, chest.ID)
		}

		// Chest should have correct fields
		if chest.RoomID != "r1" {
			t.Errorf("seed %d: chest RoomID = %q, want r1", seed, chest.RoomID)
		}
		return // success
	}
	t.Fatal("no chest spawned across 200 seeds")
}

func TestSpawnTraps_UntrappedChestIsBenign(t *testing.T) {
	// Find a seed that produces an untrapped chest (Damage == 0, IsDisarmed == true)
	for seed := int64(0); seed < 500; seed++ {
		dg := NewDungeonGenerator(seed)
		// difficulty 0: no chest trap templates are eligible (all MinDiff >= 1)
		// so every chest at difficulty 0 should be untrapped
		room := testRoom("r1", 0)
		traps, _ := dg.spawnTraps(room, 0)

		for _, trap := range traps {
			if trap.Location == game.TrapChest {
				if trap.Damage != 0 {
					t.Errorf("seed %d: untrapped chest Damage = %d, want 0", seed, trap.Damage)
				}
				if !trap.IsDisarmed {
					t.Errorf("seed %d: untrapped chest IsDisarmed = false, want true", seed)
				}
				if trap.Description != "A sturdy wooden chest." {
					t.Errorf("seed %d: untrapped chest Description = %q", seed, trap.Description)
				}
				return // success
			}
		}
	}
	t.Fatal("no untrapped chest spawned across 500 seeds at difficulty 0")
}

func TestSpawnTraps_TrappedChestHasDamage(t *testing.T) {
	// At high difficulty, chest trap templates are eligible
	for seed := int64(0); seed < 500; seed++ {
		dg := NewDungeonGenerator(seed)
		room := testRoom("r1", 5)
		traps, _ := dg.spawnTraps(room, 5)

		for _, trap := range traps {
			if trap.Location == game.TrapChest && trap.Damage > 0 {
				// Trapped chest should NOT be pre-disarmed
				if trap.IsDisarmed {
					t.Errorf("seed %d: trapped chest should not be IsDisarmed", seed)
				}
				if trap.Difficulty == 0 {
					t.Errorf("seed %d: trapped chest should have non-zero Difficulty", seed)
				}
				return // success
			}
		}
	}
	t.Fatal("no trapped chest spawned across 500 seeds at difficulty 5")
}

// --- PopulateRoom (chest items in output) ---

func TestPopulateRoom_ChestItemsIncluded(t *testing.T) {
	for seed := int64(0); seed < 200; seed++ {
		dg := NewDungeonGenerator(seed)
		room := testRoom("r1", 3)
		_, items, traps := dg.PopulateRoom(room, 3)

		// Find chest traps
		var chestIDs []string
		for _, trap := range traps {
			if trap.Location == game.TrapChest {
				chestIDs = append(chestIDs, trap.ID)
			}
		}

		if len(chestIDs) == 0 {
			continue
		}

		// Items slice should contain items linked to this chest
		chestID := chestIDs[0]
		linkedCount := 0
		for _, item := range items {
			if item.ChestTrapID != nil && *item.ChestTrapID == chestID {
				linkedCount++
			}
		}
		if linkedCount == 0 {
			t.Fatalf("seed %d: PopulateRoom returned chest trap but no linked items in items slice", seed)
		}
		return // success
	}
	t.Fatal("no chest spawned in PopulateRoom across 200 seeds")
}

func TestPopulateRoom_EntranceHasNoChest(t *testing.T) {
	dg := NewDungeonGenerator(42)
	room := &game.Room{ID: "r1", DungeonID: "d1", Name: "Entrance", X: 0, Y: 0, IsEntrance: true}
	_, _, traps := dg.PopulateRoom(room, 0)

	for _, trap := range traps {
		if trap.Location == game.TrapChest {
			t.Error("entrance room should not have chests")
		}
	}
}

func TestPopulateRoom_ExitHasNoChest(t *testing.T) {
	dg := NewDungeonGenerator(42)
	room := &game.Room{ID: "r1", DungeonID: "d1", Name: "Exit", X: 4, Y: 4, IsExit: true}
	_, _, traps := dg.PopulateRoom(room, 8)

	for _, trap := range traps {
		if trap.Location == game.TrapChest {
			t.Error("exit room should not have chests")
		}
	}
}

// --- GenerateDungeon (integration) ---

func TestGenerateDungeon_ChestItemsInOutput(t *testing.T) {
	dg := NewDungeonGenerator(42)
	_, _, _, _, items, traps := dg.GenerateDungeon(1)

	// Find all chest trap IDs
	chestIDs := make(map[string]bool)
	for _, trap := range traps {
		if trap.Location == game.TrapChest {
			chestIDs[trap.ID] = true
		}
	}

	if len(chestIDs) == 0 {
		t.Fatal("expected at least one chest in a full dungeon with seed 42")
	}

	// Every chest should have at least one linked item
	for chestID := range chestIDs {
		found := false
		for _, item := range items {
			if item.ChestTrapID != nil && *item.ChestTrapID == chestID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("chest %s has no linked items in GenerateDungeon output", chestID)
		}
	}
}
