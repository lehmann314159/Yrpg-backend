package game

import "testing"

func TestBuildSnapshot_MagicUserSpellFields(t *testing.T) {
	gs := NewGameState("test-session")
	party, err := NewParty([]PartyRequest{
		{Name: "Gandalf", Class: ClassMagicUser},
		{Name: "Conan", Class: ClassFighter},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gs.Party = party
	gs.Dungeon = &Dungeon{ID: "d1", Depth: 1}

	room := &Room{ID: "room1", Name: "Test Room", X: 0, Y: 0}
	gs.AddRoom(room)
	party.MoveAllToRoom(room.ID)

	snap := gs.BuildSnapshot()
	if snap.Party == nil {
		t.Fatal("snapshot party is nil")
	}

	// Find magic_user and fighter views
	var mageView, fighterView *CharacterView
	for _, cv := range snap.Party.Characters {
		switch cv.Name {
		case "Gandalf":
			mageView = cv
		case "Conan":
			fighterView = cv
		}
	}

	if mageView == nil {
		t.Fatal("magic_user view not found")
	}
	if fighterView == nil {
		t.Fatal("fighter view not found")
	}

	// Magic user should have spell fields populated
	if mageView.MaxSpellSlots != 3 {
		t.Errorf("mage MaxSpellSlots = %d, want 3", mageView.MaxSpellSlots)
	}
	if mageView.SpellSlots != 3 {
		t.Errorf("mage SpellSlots = %d, want 3", mageView.SpellSlots)
	}
	if len(mageView.KnownSpells) != 2 {
		t.Errorf("mage KnownSpells length = %d, want 2", len(mageView.KnownSpells))
	}
	if mageView.Class != "magic_user" {
		t.Errorf("mage Class = %s, want magic_user", mageView.Class)
	}

	// Fighter should have zero spell fields
	if fighterView.MaxSpellSlots != 0 {
		t.Errorf("fighter MaxSpellSlots = %d, want 0", fighterView.MaxSpellSlots)
	}
	if fighterView.SpellSlots != 0 {
		t.Errorf("fighter SpellSlots = %d, want 0", fighterView.SpellSlots)
	}
	if len(fighterView.KnownSpells) != 0 {
		t.Errorf("fighter KnownSpells should be empty, got %v", fighterView.KnownSpells)
	}
}

func TestBuildSnapshot_WithCombat(t *testing.T) {
	gs := NewGameState("s1")

	party, _ := NewParty([]PartyRequest{
		{Name: "Fighter", Class: ClassFighter},
	})
	gs.Party = party

	room := &Room{ID: "r1", Name: "Dungeon", X: 0, Y: 0}
	gs.AddRoom(room)
	party.MoveAllToRoom("r1")

	gs.AddMonster(&Monster{ID: "m1", Name: "Goblin", HP: 10, MaxHP: 10, RoomID: "r1", IsAlive: true})
	roomID := "r1"
	gs.AddItem(&Item{ID: "i1", Name: "Sword", Type: ItemWeapon, Damage: 3, Range: RangeMelee, Rarity: "common", RoomID: &roomID})
	gs.AddTrap(&Trap{ID: "t1", RoomID: "r1", Description: "Spike trap", IsDiscovered: true, Difficulty: 15})

	gs.Mode = ModeCombat
	cs := newTestCombatState()
	cs.Combatants = []*Combatant{
		{ID: party.Characters[0].ID, Name: "Fighter", IsPlayerChar: true, IsAlive: true, HP: 30, MaxHP: 30, GridX: 0, GridY: 0},
		{ID: "m1", Name: "Goblin", IsPlayerChar: false, IsAlive: true, HP: 10, MaxHP: 10, GridX: 5, GridY: 5},
	}
	cs.PlaceOnGrid(cs.Combatants[0], 0, 0)
	cs.PlaceOnGrid(cs.Combatants[1], 5, 5)
	cs.RoundNumber = 1
	gs.Combat = cs

	snap := gs.BuildSnapshot()

	if snap.Mode != "combat" {
		t.Errorf("Mode = %q, want combat", snap.Mode)
	}
	if snap.Party == nil || len(snap.Party.Characters) != 1 {
		t.Error("party should have 1 character")
	}
	if snap.CurrentRoom == nil || snap.CurrentRoom.Name != "Dungeon" {
		t.Error("current room should be Dungeon")
	}
	if len(snap.Monsters) != 1 {
		t.Errorf("monster count = %d, want 1", len(snap.Monsters))
	}
	if len(snap.RoomItems) != 1 {
		t.Errorf("item count = %d, want 1", len(snap.RoomItems))
	}
	if len(snap.RoomTraps) != 1 {
		t.Errorf("trap count = %d, want 1 (discovered)", len(snap.RoomTraps))
	}
	if snap.Combat == nil {
		t.Fatal("combat view should be set")
	}
	if len(snap.Combat.Combatants) != 2 {
		t.Errorf("combatant count = %d, want 2", len(snap.Combat.Combatants))
	}
}

func TestBuildSnapshot_UndiscoveredTrapHidden(t *testing.T) {
	gs := NewGameState("s1")
	party, _ := NewParty([]PartyRequest{{Name: "Fighter", Class: ClassFighter}})
	gs.Party = party
	room := &Room{ID: "r1", X: 0, Y: 0}
	gs.AddRoom(room)
	party.MoveAllToRoom("r1")
	gs.AddTrap(&Trap{ID: "t1", RoomID: "r1", IsDiscovered: false})

	snap := gs.BuildSnapshot()
	if len(snap.RoomTraps) != 0 {
		t.Error("undiscovered trap should not appear in snapshot")
	}
}

func TestBuildMonsterView_ThreatLevels(t *testing.T) {
	tests := []struct {
		maxHP int
		want  string
	}{
		{5, "trivial"},
		{8, "trivial"},
		{15, "normal"},
		{20, "dangerous"},
		{30, "deadly"},
		{50, "deadly"},
	}

	for _, tc := range tests {
		m := &Monster{ID: "m1", MaxHP: tc.maxHP, IsAlive: true}
		mv := buildMonsterView(m)
		if mv.Threat != tc.want {
			t.Errorf("MaxHP=%d: threat=%q, want %q", tc.maxHP, mv.Threat, tc.want)
		}
	}
}

func TestBuildItemView(t *testing.T) {
	item := &Item{
		ID:               "i1",
		Name:             "Magic Sword",
		Description:      "A glowing blade",
		Type:             ItemWeapon,
		Damage:           5,
		Armor:            2,
		Healing:          1,
		Range:            RangeMelee,
		Rarity:           "rare",
		ClassRestriction: []CharacterClass{ClassFighter},
		IsEquipped:       true,
	}

	iv := buildItemView(item)
	if iv.Name != "Magic Sword" {
		t.Errorf("Name = %q", iv.Name)
	}
	if iv.Damage != 5 {
		t.Errorf("Damage = %d, want 5", iv.Damage)
	}
	if iv.Armor != 2 {
		t.Errorf("Armor = %d, want 2", iv.Armor)
	}
	if iv.Healing != 1 {
		t.Errorf("Healing = %d, want 1", iv.Healing)
	}
	if iv.Range != "melee" {
		t.Errorf("Range = %q, want melee", iv.Range)
	}
	if len(iv.ClassRestriction) != 1 || iv.ClassRestriction[0] != "fighter" {
		t.Errorf("ClassRestriction = %v", iv.ClassRestriction)
	}
	if !iv.IsEquipped {
		t.Error("should be equipped")
	}
}

func TestBuildItemView_ZeroValues(t *testing.T) {
	item := &Item{ID: "i1", Name: "Key", Type: ItemKey, Rarity: "common"}
	iv := buildItemView(item)
	if iv.Damage != 0 || iv.Armor != 0 || iv.Healing != 0 {
		t.Error("zero values should stay zero")
	}
	if iv.Range != "" {
		t.Errorf("Range = %q, want empty", iv.Range)
	}
	if iv.ClassRestriction != nil {
		t.Error("no restriction should be nil")
	}
}

func TestBuildTrapView(t *testing.T) {
	trap := &Trap{ID: "t1", Description: "Spike trap", IsDisarmed: true, Difficulty: 15}
	tv := buildTrapView(trap)
	if tv.Description != "Spike trap" {
		t.Errorf("Description = %q", tv.Description)
	}
	if !tv.IsDisarmed {
		t.Error("should be disarmed")
	}
	if tv.Difficulty != 15 {
		t.Errorf("Difficulty = %d, want 15", tv.Difficulty)
	}
}

func TestBuildCombatView(t *testing.T) {
	cs := newTestCombatState()
	cs.RoundNumber = 3
	cs.CurrentTurnIdx = 1

	c1 := &Combatant{ID: "p1", Name: "Hero", IsPlayerChar: true, IsAlive: true, HP: 20, MaxHP: 30, GridX: 0, GridY: 0}
	c2 := &Combatant{ID: "m1", Name: "Goblin", IsPlayerChar: false, IsAlive: true, HP: 8, MaxHP: 10, GridX: 5, GridY: 5}
	cs.Combatants = append(cs.Combatants, c1, c2)
	cs.PlaceOnGrid(c1, 0, 0)
	cs.PlaceOnGrid(c2, 5, 5)

	cv := buildCombatView(cs, NewGameState("test"))
	if cv.RoundNumber != 3 {
		t.Errorf("RoundNumber = %d, want 3", cv.RoundNumber)
	}
	if cv.CurrentTurnIdx != 1 {
		t.Errorf("CurrentTurnIdx = %d, want 1", cv.CurrentTurnIdx)
	}
	if !cv.IsActive {
		t.Error("should be active")
	}
	if len(cv.Combatants) != 2 {
		t.Errorf("combatant count = %d, want 2", len(cv.Combatants))
	}
	if cv.Grid[0][0] != "p1" {
		t.Errorf("grid[0][0] = %q, want p1", cv.Grid[0][0])
	}
	if cv.Grid[5][5] != "m1" {
		t.Errorf("grid[5][5] = %q, want m1", cv.Grid[5][5])
	}
	if cv.Grid[2][2] != "" {
		t.Errorf("empty cell = %q, want empty", cv.Grid[2][2])
	}
}

func TestBuildCombatView_BlockedCell(t *testing.T) {
	cs := newTestCombatState()
	cs.Grid[3][3].IsBlocked = true

	cv := buildCombatView(cs, NewGameState("test"))
	if cv.Grid[3][3] != "blocked" {
		t.Errorf("blocked cell = %q, want 'blocked'", cv.Grid[3][3])
	}
}

func TestBuildCombatView_MovementAndAttackRange(t *testing.T) {
	gs := NewGameState("test")
	party, err := NewParty([]PartyRequest{
		{Name: "Tank", Class: ClassFighter},
		{Name: "Scout", Class: ClassThief},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gs.Party = party

	fighter := party.GetCharacterByName("Tank")
	thief := party.GetCharacterByName("Scout")

	// Give thief a ranged weapon
	bow := &Item{
		ID:       "bow1",
		Name:     "Longbow",
		Type:     ItemWeapon,
		Damage:   4,
		Range:    RangeRanged,
		MaxRange: 5,
		Rarity:   "common",
	}
	gs.AddItem(bow)
	thief.EquippedWeaponID = &bow.ID

	// Give fighter a melee weapon
	sword := &Item{
		ID:     "sword1",
		Name:   "Sword",
		Type:   ItemWeapon,
		Damage: 6,
		Range:  RangeMelee,
		Rarity: "common",
	}
	gs.AddItem(sword)
	fighter.EquippedWeaponID = &sword.ID

	// Add monsters
	gs.AddMonster(&Monster{ID: "m1", Name: "Goblin", HP: 10, MaxHP: 10, IsAlive: true, RoomID: "r1"})
	gs.AddMonster(&Monster{ID: "m2", Name: "Archer", HP: 8, MaxHP: 8, IsAlive: true, RoomID: "r1", IsRanged: true, AttackRange: 3})

	cs := newTestCombatState()
	cs.Combatants = []*Combatant{
		{ID: fighter.ID, Name: "Tank", IsPlayerChar: true, CharacterID: fighter.ID, IsAlive: true, HP: 30, MaxHP: 30},
		{ID: thief.ID, Name: "Scout", IsPlayerChar: true, CharacterID: thief.ID, IsAlive: true, HP: 22, MaxHP: 22},
		{ID: "m1", Name: "Goblin", IsPlayerChar: false, CharacterID: "m1", IsAlive: true, HP: 10, MaxHP: 10},
		{ID: "m2", Name: "Archer", IsPlayerChar: false, CharacterID: "m2", IsAlive: true, HP: 8, MaxHP: 8},
	}
	cs.PlaceOnGrid(cs.Combatants[0], 0, 0)
	cs.PlaceOnGrid(cs.Combatants[1], 1, 0)
	cs.PlaceOnGrid(cs.Combatants[2], 5, 5)
	cs.PlaceOnGrid(cs.Combatants[3], 4, 5)
	gs.Combat = cs
	gs.Mode = ModeCombat

	cv := buildCombatView(cs, gs)

	// Find combatant views by name
	views := make(map[string]*CombatantView)
	for _, v := range cv.Combatants {
		views[v.Name] = v
	}

	// Fighter: movementRange=2, attackRange=1 (melee weapon)
	if views["Tank"].MovementRange != 2 {
		t.Errorf("Fighter movementRange = %d, want 2", views["Tank"].MovementRange)
	}
	if views["Tank"].AttackRange != 1 {
		t.Errorf("Fighter attackRange = %d, want 1", views["Tank"].AttackRange)
	}

	// Thief: movementRange=4, attackRange=5 (ranged weapon MaxRange)
	if views["Scout"].MovementRange != 4 {
		t.Errorf("Thief movementRange = %d, want 4", views["Scout"].MovementRange)
	}
	if views["Scout"].AttackRange != 5 {
		t.Errorf("Thief attackRange = %d, want 5", views["Scout"].AttackRange)
	}

	// Melee monster: movementRange=2, attackRange=1
	if views["Goblin"].MovementRange != 2 {
		t.Errorf("Goblin movementRange = %d, want 2", views["Goblin"].MovementRange)
	}
	if views["Goblin"].AttackRange != 1 {
		t.Errorf("Goblin attackRange = %d, want 1", views["Goblin"].AttackRange)
	}

	// Ranged monster: movementRange=2, attackRange=3
	if views["Archer"].MovementRange != 2 {
		t.Errorf("Archer movementRange = %d, want 2", views["Archer"].MovementRange)
	}
	if views["Archer"].AttackRange != 3 {
		t.Errorf("Archer attackRange = %d, want 3", views["Archer"].AttackRange)
	}
}

func TestBuildCombatView_KnownSpells(t *testing.T) {
	gs := NewGameState("test")
	party, err := NewParty([]PartyRequest{
		{Name: "Gandalf", Class: ClassMagicUser},
		{Name: "Conan", Class: ClassFighter},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gs.Party = party

	mage := party.GetCharacterByName("Gandalf")
	fighter := party.GetCharacterByName("Conan")

	cs := newTestCombatState()
	cs.Combatants = []*Combatant{
		{ID: mage.ID, Name: "Gandalf", IsPlayerChar: true, CharacterID: mage.ID, IsAlive: true, HP: 16, MaxHP: 16},
		{ID: fighter.ID, Name: "Conan", IsPlayerChar: true, CharacterID: fighter.ID, IsAlive: true, HP: 30, MaxHP: 30},
	}
	cs.PlaceOnGrid(cs.Combatants[0], 0, 0)
	cs.PlaceOnGrid(cs.Combatants[1], 1, 0)
	gs.Combat = cs
	gs.Mode = ModeCombat

	cv := buildCombatView(cs, gs)

	views := make(map[string]*CombatantView)
	for _, v := range cv.Combatants {
		views[v.Name] = v
	}

	// MagicUser should have knownSpells
	if len(views["Gandalf"].KnownSpells) != 2 {
		t.Errorf("Gandalf knownSpells length = %d, want 2", len(views["Gandalf"].KnownSpells))
	}
	spellSet := make(map[string]bool)
	for _, s := range views["Gandalf"].KnownSpells {
		spellSet[s] = true
	}
	if !spellSet["heal"] || !spellSet["fireball"] {
		t.Errorf("Gandalf knownSpells = %v, want [heal, fireball]", views["Gandalf"].KnownSpells)
	}

	// Fighter should have no knownSpells
	if len(views["Conan"].KnownSpells) != 0 {
		t.Errorf("Conan knownSpells should be empty, got %v", views["Conan"].KnownSpells)
	}
}

func TestBuildCombatView_ScoutPhaseFields(t *testing.T) {
	cs := newTestCombatState()
	cs.IsScoutPhase = true
	cs.AwaitingScoutDecision = true
	cs.ScoutID = "p1"
	cs.RoundNumber = 1

	cv := buildCombatView(cs, NewGameState("test"))
	if !cv.IsScoutPhase {
		t.Error("IsScoutPhase should be true")
	}
	if !cv.AwaitingScoutDecision {
		t.Error("AwaitingScoutDecision should be true")
	}
}

func TestBuildCombatView_ScoutDoubleMovement(t *testing.T) {
	gs := NewGameState("test")
	party, err := NewParty([]PartyRequest{
		{Name: "Shadow", Class: ClassThief},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gs.Party = party

	thief := party.GetCharacterByName("Shadow")

	cs := newTestCombatState()
	cs.IsScoutPhase = true
	cs.ScoutID = thief.ID
	cs.Combatants = []*Combatant{
		{ID: thief.ID, Name: "Shadow", IsPlayerChar: true, CharacterID: thief.ID, IsAlive: true, HP: 22, MaxHP: 22},
	}
	cs.PlaceOnGrid(cs.Combatants[0], 3, 0)
	gs.Combat = cs
	gs.Mode = ModeCombat

	cv := buildCombatView(cs, gs)

	if len(cv.Combatants) != 1 {
		t.Fatalf("expected 1 combatant, got %d", len(cv.Combatants))
	}

	// Thief base movement is 4, should be doubled to 8 during scout phase
	if cv.Combatants[0].MovementRange != 8 {
		t.Errorf("scout movementRange = %d, want 8 (4 * 2)", cv.Combatants[0].MovementRange)
	}
}

func TestBuildCombatView_NonScoutNoDoubleMovement(t *testing.T) {
	gs := NewGameState("test")
	party, err := NewParty([]PartyRequest{
		{Name: "Shadow", Class: ClassThief},
		{Name: "Conan", Class: ClassFighter},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gs.Party = party

	thief := party.GetCharacterByName("Shadow")
	fighter := party.GetCharacterByName("Conan")

	// Non-scout-phase combat: movement should not be doubled
	cs := newTestCombatState()
	cs.IsScoutPhase = false
	cs.Combatants = []*Combatant{
		{ID: thief.ID, Name: "Shadow", IsPlayerChar: true, CharacterID: thief.ID, IsAlive: true, HP: 22, MaxHP: 22},
		{ID: fighter.ID, Name: "Conan", IsPlayerChar: true, CharacterID: fighter.ID, IsAlive: true, HP: 30, MaxHP: 30},
	}
	cs.PlaceOnGrid(cs.Combatants[0], 0, 0)
	cs.PlaceOnGrid(cs.Combatants[1], 1, 0)
	gs.Combat = cs
	gs.Mode = ModeCombat

	cv := buildCombatView(cs, gs)

	views := make(map[string]*CombatantView)
	for _, v := range cv.Combatants {
		views[v.Name] = v
	}

	// Normal movement ranges (no doubling)
	if views["Shadow"].MovementRange != 4 {
		t.Errorf("thief movementRange = %d, want 4 (no doubling)", views["Shadow"].MovementRange)
	}
	if views["Conan"].MovementRange != 2 {
		t.Errorf("fighter movementRange = %d, want 2", views["Conan"].MovementRange)
	}
}

func TestBuildSnapshot_CharacterInventory(t *testing.T) {
	gs := NewGameState("test-session")
	party, err := NewParty([]PartyRequest{
		{Name: "Archer", Class: ClassFighter},
		{Name: "Mage", Class: ClassMagicUser},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gs.Party = party
	gs.Dungeon = &Dungeon{ID: "d1", Depth: 1}

	room := &Room{ID: "room1", Name: "Test Room", X: 0, Y: 0}
	gs.AddRoom(room)
	party.MoveAllToRoom(room.ID)

	// Add items to Archer's inventory
	archerID := party.Characters[0].ID
	gs.AddItem(&Item{
		ID:          "sword1",
		Name:        "Iron Sword",
		Type:        ItemWeapon,
		Damage:      5,
		Range:       RangeMelee,
		Rarity:      "common",
		CharacterID: &archerID,
		IsEquipped:  true,
	})
	gs.AddItem(&Item{
		ID:          "shield1",
		Name:        "Wooden Shield",
		Type:        ItemArmor,
		Armor:       2,
		Rarity:      "common",
		CharacterID: &archerID,
	})

	snap := gs.BuildSnapshot()
	if snap.Party == nil {
		t.Fatal("snapshot party is nil")
	}

	var archerView, mageView *CharacterView
	for _, cv := range snap.Party.Characters {
		switch cv.Name {
		case "Archer":
			archerView = cv
		case "Mage":
			mageView = cv
		}
	}

	if archerView == nil {
		t.Fatal("archer view not found")
	}
	if mageView == nil {
		t.Fatal("mage view not found")
	}

	// Archer should have 2 items
	if len(archerView.Inventory) != 2 {
		t.Fatalf("archer inventory count = %d, want 2", len(archerView.Inventory))
	}

	// Check that at least one item is equipped
	foundEquipped := false
	for _, iv := range archerView.Inventory {
		if iv.Name == "Iron Sword" {
			if !iv.IsEquipped {
				t.Error("Iron Sword should be equipped")
			}
			if iv.Type != "weapon" {
				t.Errorf("Iron Sword type = %q, want weapon", iv.Type)
			}
			foundEquipped = true
		}
	}
	if !foundEquipped {
		t.Error("did not find Iron Sword in archer inventory")
	}

	// Mage should have an empty (non-nil) inventory
	if mageView.Inventory == nil {
		t.Fatal("mage inventory should be non-nil")
	}
	if len(mageView.Inventory) != 0 {
		t.Errorf("mage inventory count = %d, want 0", len(mageView.Inventory))
	}
}

func TestCharacterView_AC_BaseOnly(t *testing.T) {
	gs := NewGameState("s1")
	party, _ := NewParty([]PartyRequest{{Name: "Fighter", Class: ClassFighter}})
	gs.Party = party
	room := &Room{ID: "r1", X: 0, Y: 0}
	gs.AddRoom(room)
	party.MoveAllToRoom("r1")

	snap := gs.BuildSnapshot()
	cv := snap.Party.Characters[0]
	if cv.AC != 10 {
		t.Errorf("base AC = %d, want 10", cv.AC)
	}
}

func TestCharacterView_AC_WithArmor(t *testing.T) {
	gs := NewGameState("s1")
	party, _ := NewParty([]PartyRequest{{Name: "Fighter", Class: ClassFighter}})
	gs.Party = party
	room := &Room{ID: "r1", X: 0, Y: 0}
	gs.AddRoom(room)
	party.MoveAllToRoom("r1")

	armorID := "armor1"
	gs.AddItem(&Item{ID: armorID, Name: "Chain Mail", Type: ItemArmor, Armor: 3, Rarity: "common"})
	party.Characters[0].EquippedArmorID = &armorID

	snap := gs.BuildSnapshot()
	cv := snap.Party.Characters[0]
	if cv.AC != 13 {
		t.Errorf("AC with armor = %d, want 13", cv.AC)
	}
}

func TestCharacterView_AC_WithArmorAndBuffs(t *testing.T) {
	gs := NewGameState("s1")
	party, _ := NewParty([]PartyRequest{{Name: "Fighter", Class: ClassFighter}})
	gs.Party = party
	room := &Room{ID: "r1", X: 0, Y: 0}
	gs.AddRoom(room)
	party.MoveAllToRoom("r1")

	armorID := "armor1"
	gs.AddItem(&Item{ID: armorID, Name: "Chain Mail", Type: ItemArmor, Armor: 3, Rarity: "common"})
	party.Characters[0].EquippedArmorID = &armorID

	// Set up combat with a shield buff on the fighter
	gs.Mode = ModeCombat
	cs := newTestCombatState()
	combatant := &Combatant{
		ID:           party.Characters[0].ID,
		CharacterID:  party.Characters[0].ID,
		Name:         "Fighter",
		IsPlayerChar: true,
		IsAlive:      true,
		HP:           30,
		MaxHP:        30,
		Buffs: []Buff{
			{Type: BuffACBonus, Value: 4, RoundsLeft: 3}, // shield spell
		},
	}
	cs.Combatants = []*Combatant{combatant}
	cs.PlaceOnGrid(combatant, 0, 0)
	gs.Combat = cs

	snap := gs.BuildSnapshot()
	cv := snap.Party.Characters[0]
	// 10 base + 3 armor + 4 shield buff = 17
	if cv.AC != 17 {
		t.Errorf("AC with armor + buff = %d, want 17", cv.AC)
	}
}

func TestBuildSnapshot_HidesChestItems(t *testing.T) {
	gs := NewGameState("s1")
	party, _ := NewParty([]PartyRequest{{Name: "Fighter", Class: ClassFighter}})
	gs.Party = party
	gs.Dungeon = &Dungeon{ID: "d1", Depth: 1}
	room := &Room{ID: "r1", X: 0, Y: 0}
	gs.AddRoom(room)
	party.MoveAllToRoom("r1")

	roomID := "r1"
	chestID := "chest1"

	// Normal floor item — should appear
	gs.AddItem(&Item{ID: "i1", Name: "Sword", Type: ItemWeapon, Rarity: "common", RoomID: &roomID})
	// Chest item — should be hidden
	gs.AddItem(&Item{ID: "i2", Name: "Hidden Gem", Type: ItemTreasure, Rarity: "rare", RoomID: &roomID, ChestTrapID: &chestID})

	snap := gs.BuildSnapshot()
	if len(snap.RoomItems) != 1 {
		t.Errorf("RoomItems count = %d, want 1 (chest item hidden)", len(snap.RoomItems))
	}
	if snap.RoomItems[0].Name != "Sword" {
		t.Errorf("visible item = %q, want Sword", snap.RoomItems[0].Name)
	}
}

func TestBuildSnapshot_UnopenedChestInTraps(t *testing.T) {
	gs := NewGameState("s1")
	party, _ := NewParty([]PartyRequest{{Name: "Fighter", Class: ClassFighter}})
	gs.Party = party
	gs.Dungeon = &Dungeon{ID: "d1", Depth: 1}
	room := &Room{ID: "r1", X: 0, Y: 0}
	gs.AddRoom(room)
	party.MoveAllToRoom("r1")

	// Unopened chest (not discovered, not opened) — should appear in RoomTraps
	gs.AddTrap(&Trap{ID: "chest1", RoomID: "r1", Location: TrapChest, Description: "A chest.", IsOpened: false})
	// Already-opened chest — should NOT appear
	gs.AddTrap(&Trap{ID: "chest2", RoomID: "r1", Location: TrapChest, Description: "Empty chest.", IsOpened: true})

	snap := gs.BuildSnapshot()
	if len(snap.RoomTraps) != 1 {
		t.Fatalf("RoomTraps count = %d, want 1", len(snap.RoomTraps))
	}
	if snap.RoomTraps[0].ID != "chest1" {
		t.Errorf("trap ID = %q, want chest1", snap.RoomTraps[0].ID)
	}
}

func TestBuildTrapView_LocationAndIsOpened(t *testing.T) {
	trap := &Trap{ID: "t1", Description: "Chest trap", Location: TrapChest, IsDisarmed: true, IsOpened: true, Difficulty: 12}
	tv := buildTrapView(trap)
	if tv.Location != "chest" {
		t.Errorf("Location = %q, want chest", tv.Location)
	}
	if !tv.IsOpened {
		t.Error("IsOpened should be true")
	}
	if !tv.IsDisarmed {
		t.Error("IsDisarmed should be true")
	}
}
