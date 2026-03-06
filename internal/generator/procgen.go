package generator

import (
	"crypto/rand"
	"fmt"
	mrand "math/rand"

	"github.com/lehmann314159/yrpg-backend/internal/game"
)

// Grid and generation constants
const (
	GridSize = 5

	// Door distribution probabilities
	OneDoorExtraChance = 0.75
	TwoDoorExtraChance = 0.25
	MaxDoorsPerRoom    = 3

	// Monster spawn constants
	MonsterSpawnChance    = 0.70
	MultiMonsterMinDiff   = 3
	MultiMonsterChance    = 0.40
	TripleMonsterMinDiff  = 6
	TripleMonsterChance   = 0.20
	DifficultyScaleFactor = 0.15

	// Item spawn constants
	BaseItemChance    = 0.30
	ItemChancePerDiff = 0.05
	MaxItemChance     = 0.55
	SecondItemChance  = 0.15

	// Trap spawn constants
	BaseTrapChance             = 0.15
	TrapChancePerDiff          = 0.05
	MaxTrapChance              = 0.40
	TrapDifficultyScaleFactor  = 0.10
	ChestSpawnChance           = 0.20
	ChestTrapChance            = 0.50

	// Chest loot constants
	ChestSecondItemChance = 0.35

	// Description weighting
	ScaryDescriptionDist   = 4
	ScaryDescriptionChance = 0.5
)

// DungeonGenerator handles procedural dungeon generation
type DungeonGenerator struct {
	seed   int64
	random *mrand.Rand
}

func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand.Read failed: " + err.Error())
	}
	return fmt.Sprintf("%x", b)
}

// NewDungeonGenerator creates a new dungeon generator with the given seed
func NewDungeonGenerator(seed int64) *DungeonGenerator {
	return &DungeonGenerator{
		seed:   seed,
		random: mrand.New(mrand.NewSource(seed)),
	}
}

// --- Monster Templates ---

type MonsterTemplate struct {
	Name        string
	Description string
	BaseHP      int
	BaseDamage  int
	BaseDex     int
	MinDiff     int
	IsRanged    bool
	AttackRange int
	LootTable   []game.LootEntry
}

var monsterTemplates = []MonsterTemplate{
	// Difficulty 0 - easy
	{Name: "Rat", Description: "A large, mangy rat with beady red eyes.", BaseHP: 5, BaseDamage: 2, BaseDex: 12, MinDiff: 0},
	{Name: "Giant Spider", Description: "A hairy spider the size of a dog, fangs dripping venom.", BaseHP: 7, BaseDamage: 3, BaseDex: 14, MinDiff: 0},

	// Difficulty 1
	{Name: "Goblin", Description: "A small, green-skinned creature with a wicked grin.", BaseHP: 10, BaseDamage: 4, BaseDex: 12, MinDiff: 1},
	{Name: "Goblin Archer", Description: "A goblin perched behind cover, shortbow drawn.", BaseHP: 8, BaseDamage: 3, BaseDex: 14, MinDiff: 1, IsRanged: true, AttackRange: 4},

	// Difficulty 2
	{Name: "Skeleton", Description: "The animated bones of a long-dead warrior.", BaseHP: 15, BaseDamage: 5, BaseDex: 10, MinDiff: 2},
	{Name: "Skeleton Archer", Description: "A skeleton clutching a rotting bow, hollow eyes tracking you.", BaseHP: 12, BaseDamage: 4, BaseDex: 12, MinDiff: 2, IsRanged: true, AttackRange: 5},
	{Name: "Zombie", Description: "A shambling corpse that reeks of death.", BaseHP: 20, BaseDamage: 4, BaseDex: 6, MinDiff: 2},

	// Difficulty 3
	{Name: "Orc", Description: "A hulking brute with tusks and a massive club.", BaseHP: 25, BaseDamage: 8, BaseDex: 10, MinDiff: 3},
	{Name: "Dark Acolyte", Description: "A robed figure chanting in an unknown tongue, dark energy crackling.", BaseHP: 14, BaseDamage: 6, BaseDex: 12, MinDiff: 3, IsRanged: true, AttackRange: 4},

	// Difficulty 4+
	{Name: "Wraith", Description: "A shadowy figure that chills you to the bone.", BaseHP: 20, BaseDamage: 7, BaseDex: 16, MinDiff: 4},
	{Name: "Troll", Description: "A massive, regenerating horror with long claws.", BaseHP: 35, BaseDamage: 10, BaseDex: 8, MinDiff: 5},
	{Name: "Orc Warchief", Description: "A scarred orc in heavy armor, barking orders.", BaseHP: 30, BaseDamage: 9, BaseDex: 10, MinDiff: 5},

	// Difficulty 6+ (deep dungeon)
	{Name: "Shadow Drake", Description: "A wingless drake wreathed in darkness, eyes glowing crimson.", BaseHP: 40, BaseDamage: 12, BaseDex: 14, MinDiff: 6},
	{Name: "Lich", Description: "An undead sorcerer radiating terrible power.", BaseHP: 28, BaseDamage: 10, BaseDex: 12, MinDiff: 7, IsRanged: true, AttackRange: 5},
}

// --- Item Templates ---

type ItemTemplate struct {
	Name             string
	Description      string
	Type             game.ItemType
	Damage           int
	Armor            int
	Healing          int
	Range            game.WeaponRange
	MaxRange         int
	Rarity           string
	ClassRestriction []game.CharacterClass
	ScrollEffect     string
	ScrollDifficulty int
	MinDiff          int     // minimum room difficulty to spawn
	SpawnWeight      float64 // relative spawn weight (higher = more common)
}

var weaponTemplates = []ItemTemplate{
	{Name: "Dagger", Description: "A simple blade, light and quick.", Type: game.ItemWeapon, Damage: 1, Range: game.RangeMelee, Rarity: "common", MinDiff: 0, SpawnWeight: 3.0},
	{Name: "Short Sword", Description: "A well-balanced blade.", Type: game.ItemWeapon, Damage: 2, Range: game.RangeMelee, Rarity: "common", ClassRestriction: []game.CharacterClass{game.ClassFighter, game.ClassThief}, MinDiff: 0, SpawnWeight: 2.5},
	{Name: "Staff", Description: "A gnarled wooden staff humming with faint energy.", Type: game.ItemWeapon, Damage: 1, Range: game.RangeMelee, Rarity: "common", ClassRestriction: []game.CharacterClass{game.ClassMagicUser}, MinDiff: 0, SpawnWeight: 2.0},
	{Name: "Mace", Description: "A heavy flanged mace.", Type: game.ItemWeapon, Damage: 3, Range: game.RangeMelee, Rarity: "uncommon", ClassRestriction: []game.CharacterClass{game.ClassFighter}, MinDiff: 1, SpawnWeight: 1.5},
	{Name: "Long Sword", Description: "A fine steel longsword with a leather-wrapped hilt.", Type: game.ItemWeapon, Damage: 4, Range: game.RangeMelee, Rarity: "uncommon", ClassRestriction: []game.CharacterClass{game.ClassFighter}, MinDiff: 2, SpawnWeight: 1.0},
	{Name: "Throwing Knife", Description: "A balanced blade designed to be thrown.", Type: game.ItemWeapon, Damage: 1, Range: game.RangeRanged, MaxRange: 3, Rarity: "common", ClassRestriction: []game.CharacterClass{game.ClassThief}, MinDiff: 0, SpawnWeight: 2.0},
	{Name: "Short Bow", Description: "A compact bow suited for tight spaces.", Type: game.ItemWeapon, Damage: 2, Range: game.RangeRanged, MaxRange: 4, Rarity: "uncommon", ClassRestriction: []game.CharacterClass{game.ClassFighter, game.ClassThief}, MinDiff: 1, SpawnWeight: 1.5},
	{Name: "Long Bow", Description: "A tall yew bow with impressive range.", Type: game.ItemWeapon, Damage: 4, Range: game.RangeRanged, MaxRange: 5, Rarity: "rare", ClassRestriction: []game.CharacterClass{game.ClassFighter}, MinDiff: 3, SpawnWeight: 0.5},
	{Name: "Great Axe", Description: "A massive double-headed axe that requires both hands.", Type: game.ItemWeapon, Damage: 6, Range: game.RangeMelee, Rarity: "rare", ClassRestriction: []game.CharacterClass{game.ClassFighter}, MinDiff: 4, SpawnWeight: 0.3},
}

var armorTemplates = []ItemTemplate{
	{Name: "Robes", Description: "Simple cloth robes offering minimal protection.", Type: game.ItemArmor, Armor: 1, Rarity: "common", MinDiff: 0, SpawnWeight: 2.0},
	{Name: "Leather Armor", Description: "Hardened leather that turns aside glancing blows.", Type: game.ItemArmor, Armor: 2, Rarity: "common", ClassRestriction: []game.CharacterClass{game.ClassFighter, game.ClassThief}, MinDiff: 0, SpawnWeight: 2.5},
	{Name: "Studded Leather", Description: "Leather reinforced with metal studs.", Type: game.ItemArmor, Armor: 3, Rarity: "uncommon", ClassRestriction: []game.CharacterClass{game.ClassFighter, game.ClassThief}, MinDiff: 2, SpawnWeight: 1.5},
	{Name: "Chain Mail", Description: "Interlocking metal rings that clink softly.", Type: game.ItemArmor, Armor: 4, Rarity: "uncommon", ClassRestriction: []game.CharacterClass{game.ClassFighter}, MinDiff: 3, SpawnWeight: 1.0},
	{Name: "Plate Armor", Description: "Heavy steel plates forged by a master smith.", Type: game.ItemArmor, Armor: 6, Rarity: "rare", ClassRestriction: []game.CharacterClass{game.ClassFighter}, MinDiff: 5, SpawnWeight: 0.3},
}

var consumableTemplates = []ItemTemplate{
	{Name: "Minor Healing Potion", Description: "A small red vial that mends minor wounds.", Type: game.ItemConsumable, Healing: 5, Rarity: "common", MinDiff: 0, SpawnWeight: 3.0},
	{Name: "Healing Potion", Description: "A red vial that restores health.", Type: game.ItemConsumable, Healing: 11, Rarity: "uncommon", MinDiff: 1, SpawnWeight: 2.0},
	{Name: "Greater Healing Potion", Description: "A large red vial brimming with restorative energy.", Type: game.ItemConsumable, Healing: 19, Rarity: "rare", MinDiff: 3, SpawnWeight: 0.8},
}

var scrollTemplates = []ItemTemplate{
	{Name: "Scroll of Heal", Description: "A parchment inscribed with healing runes.", Type: game.ItemScroll, ScrollEffect: "heal", ScrollDifficulty: 10, Rarity: "uncommon", MinDiff: 1, SpawnWeight: 1.5},
	{Name: "Scroll of Shield", Description: "A parchment that conjures a protective barrier.", Type: game.ItemScroll, ScrollEffect: "shield", ScrollDifficulty: 11, Rarity: "uncommon", MinDiff: 2, SpawnWeight: 1.2},
	{Name: "Scroll of Fireball", Description: "A scorched parchment radiating heat.", Type: game.ItemScroll, ScrollEffect: "fireball", ScrollDifficulty: 12, Rarity: "rare", MinDiff: 2, SpawnWeight: 1.0},
	{Name: "Scroll of Sleep", Description: "A parchment that lulls enemies into slumber.", Type: game.ItemScroll, ScrollEffect: "sleep", ScrollDifficulty: 13, Rarity: "rare", MinDiff: 3, SpawnWeight: 0.8},
	{Name: "Scroll of Lightning", Description: "A crackling parchment that unleashes a bolt of energy.", Type: game.ItemScroll, ScrollEffect: "lightning", ScrollDifficulty: 14, Rarity: "rare", MinDiff: 4, SpawnWeight: 0.6},
	{Name: "Scroll of Resurrection", Description: "An ancient parchment glowing with divine power.", Type: game.ItemScroll, ScrollEffect: "resurrect", ScrollDifficulty: 16, Rarity: "legendary", MinDiff: 5, SpawnWeight: 0.2},
}

// --- Trap Templates ---

type TrapTemplate struct {
	Description string
	BaseDamage  int
	BaseDC      int
	Location    game.TrapLocation
	MinDiff     int
}

var roomTrapTemplates = []TrapTemplate{
	{Description: "A tripwire stretches across the passage.", BaseDamage: 4, BaseDC: 10, Location: game.TrapRoom, MinDiff: 1},
	{Description: "Pressure plates are hidden beneath loose flagstones.", BaseDamage: 6, BaseDC: 12, Location: game.TrapRoom, MinDiff: 2},
	{Description: "Tiny holes line the walls — dart launchers.", BaseDamage: 8, BaseDC: 14, Location: game.TrapRoom, MinDiff: 3},
	{Description: "A pit yawns beneath a thin layer of rotted wood.", BaseDamage: 10, BaseDC: 13, Location: game.TrapRoom, MinDiff: 3},
	{Description: "Runes on the floor pulse with malevolent energy.", BaseDamage: 12, BaseDC: 16, Location: game.TrapRoom, MinDiff: 5},
}

var chestTrapTemplates = []TrapTemplate{
	{Description: "A needle is hidden in the chest latch.", BaseDamage: 3, BaseDC: 10, Location: game.TrapChest, MinDiff: 1},
	{Description: "A spring-loaded blade is rigged inside the lid.", BaseDamage: 6, BaseDC: 12, Location: game.TrapChest, MinDiff: 2},
	{Description: "A cloud of poisonous gas is sealed within.", BaseDamage: 8, BaseDC: 14, Location: game.TrapChest, MinDiff: 3},
	{Description: "An explosive rune is etched on the underside of the lid.", BaseDamage: 12, BaseDC: 16, Location: game.TrapChest, MinDiff: 5},
}

// --- Grid Helpers ---

type coord struct {
	x, y int
}

type edge struct {
	from, to  coord
	direction string
}

func getNeighbor(c coord, dir string) coord {
	switch dir {
	case "north":
		return coord{c.x, c.y + 1}
	case "south":
		return coord{c.x, c.y - 1}
	case "east":
		return coord{c.x + 1, c.y}
	case "west":
		return coord{c.x - 1, c.y}
	}
	return c
}

func oppositeDir(dir string) string {
	switch dir {
	case "north":
		return "south"
	case "south":
		return "north"
	case "east":
		return "west"
	case "west":
		return "east"
	}
	return ""
}

// --- Dungeon Generation ---

// GenerateDungeon creates a new procedural dungeon with rooms, connections, and population
func (dg *DungeonGenerator) GenerateDungeon(depth int) (*game.Dungeon, []*game.Room, []*game.RoomConnection, []*game.Monster, []*game.Item, []*game.Trap) {
	dungeon := &game.Dungeon{
		ID:    generateID(),
		Seed:  dg.seed,
		Depth: depth,
	}

	rooms, roomGrid := dg.generateGrid(dungeon.ID)
	connections := dg.generateConnections(roomGrid)

	// Populate all rooms
	allMonsters := make([]*game.Monster, 0)
	allItems := make([]*game.Item, 0)
	allTraps := make([]*game.Trap, 0)

	for _, room := range rooms {
		difficulty := GetRoomDifficulty(room)
		monsters, items, traps := dg.PopulateRoom(room, difficulty)
		allMonsters = append(allMonsters, monsters...)
		allItems = append(allItems, items...)
		allTraps = append(allTraps, traps...)
	}

	return dungeon, rooms, connections, allMonsters, allItems, allTraps
}

// generateGrid creates a 5x5 grid of rooms
func (dg *DungeonGenerator) generateGrid(dungeonID string) ([]*game.Room, map[coord]*game.Room) {
	rooms := make([]*game.Room, 0, GridSize*GridSize)
	roomGrid := make(map[coord]*game.Room)

	for y := 0; y < GridSize; y++ {
		for x := 0; x < GridSize; x++ {
			room := &game.Room{
				ID:          generateID(),
				DungeonID:   dungeonID,
				Name:        dg.generateRoomName(),
				Description: dg.generateRoomDescription(x, y),
				IsEntrance:  x == 0 && y == 0,
				IsExit:      x == GridSize-1 && y == GridSize-1,
				X:           x,
				Y:           y,
			}
			rooms = append(rooms, room)
			roomGrid[coord{x, y}] = room
		}
	}

	return rooms, roomGrid
}

// generateConnections creates room connections using spanning tree + extra doors
func (dg *DungeonGenerator) generateConnections(roomGrid map[coord]*game.Room) []*game.RoomConnection {
	edges := make(map[coord]map[string]bool)
	for c := range roomGrid {
		edges[c] = make(map[string]bool)
	}

	// Step 1: Spanning tree via randomized Prim's
	visited := make(map[coord]bool)
	frontier := make([]edge, 0)

	start := coord{0, 0}
	visited[start] = true
	dg.addFrontierEdges(&frontier, start, visited)

	for len(frontier) > 0 {
		idx := dg.random.Intn(len(frontier))
		e := frontier[idx]
		frontier = append(frontier[:idx], frontier[idx+1:]...)

		if visited[e.to] {
			continue
		}

		visited[e.to] = true
		edges[e.from][e.direction] = true
		edges[e.to][oppositeDir(e.direction)] = true
		dg.addFrontierEdges(&frontier, e.to, visited)
	}

	// Step 2: Add extra doors for variety
	doorCounts := make(map[coord]int)
	for c := range roomGrid {
		doorCounts[c] = len(edges[c])
	}

	possibleEdges := dg.getPossibleEdges(roomGrid, edges)
	dg.shuffleEdges(possibleEdges)

	for _, e := range possibleEdges {
		fromDoors := doorCounts[e.from]
		toDoors := doorCounts[e.to]

		if fromDoors >= MaxDoorsPerRoom || toDoors >= MaxDoorsPerRoom {
			continue
		}

		shouldAdd := false
		if fromDoors == 1 && dg.random.Float32() < OneDoorExtraChance {
			shouldAdd = true
		} else if fromDoors == 2 && dg.random.Float32() < TwoDoorExtraChance {
			shouldAdd = true
		}

		if shouldAdd {
			edges[e.from][e.direction] = true
			edges[e.to][oppositeDir(e.direction)] = true
			doorCounts[e.from]++
			doorCounts[e.to]++
		}
	}

	// Step 3: Convert to RoomConnection slice
	connections := make([]*game.RoomConnection, 0)
	for c, dirs := range edges {
		room := roomGrid[c]
		for dir := range dirs {
			neighbor := getNeighbor(c, dir)
			neighborRoom := roomGrid[neighbor]
			if neighborRoom != nil {
				connections = append(connections, &game.RoomConnection{
					ID:              generateID(),
					RoomID:          room.ID,
					Direction:       dir,
					ConnectedRoomID: neighborRoom.ID,
				})
			}
		}
	}

	return connections
}

func (dg *DungeonGenerator) addFrontierEdges(frontier *[]edge, c coord, visited map[coord]bool) {
	directions := []string{"north", "south", "east", "west"}
	for _, dir := range directions {
		neighbor := getNeighbor(c, dir)
		if neighbor.x >= 0 && neighbor.x < GridSize && neighbor.y >= 0 && neighbor.y < GridSize {
			if !visited[neighbor] {
				*frontier = append(*frontier, edge{from: c, to: neighbor, direction: dir})
			}
		}
	}
}

func (dg *DungeonGenerator) getPossibleEdges(roomGrid map[coord]*game.Room, edges map[coord]map[string]bool) []edge {
	possible := make([]edge, 0)
	directions := []string{"north", "east"}
	for c := range roomGrid {
		for _, dir := range directions {
			neighbor := getNeighbor(c, dir)
			if roomGrid[neighbor] != nil && !edges[c][dir] {
				possible = append(possible, edge{from: c, to: neighbor, direction: dir})
			}
		}
	}
	return possible
}

func (dg *DungeonGenerator) shuffleEdges(edges []edge) {
	for i := len(edges) - 1; i > 0; i-- {
		j := dg.random.Intn(i + 1)
		edges[i], edges[j] = edges[j], edges[i]
	}
}

// --- Room Population ---

// PopulateRoom adds monsters, items, and traps to a room based on difficulty
func (dg *DungeonGenerator) PopulateRoom(room *game.Room, difficulty int) ([]*game.Monster, []*game.Item, []*game.Trap) {
	monsters := make([]*game.Monster, 0)
	items := make([]*game.Item, 0)
	traps := make([]*game.Trap, 0)

	if room.IsEntrance {
		// Safe room: give the party a starting potion
		roomID := room.ID
		startPotion := &game.Item{
			ID:          generateID(),
			Name:        "Minor Healing Potion",
			Description: "A small red vial that mends minor wounds.",
			Type:        game.ItemConsumable,
			Healing:     5,
			Rarity:      "common",
			RoomID:      &roomID,
		}
		items = append(items, startPotion)
		return monsters, items, traps
	}

	if room.IsExit {
		return monsters, items, traps
	}

	// --- Monsters ---
	monsters = dg.spawnMonsters(room, difficulty)

	// --- Items ---
	items = dg.spawnItems(room, difficulty)

	// --- Traps (may include chest loot items) ---
	traps, chestItems := dg.spawnTraps(room, difficulty)
	items = append(items, chestItems...)

	return monsters, items, traps
}

func (dg *DungeonGenerator) spawnMonsters(room *game.Room, difficulty int) []*game.Monster {
	monsters := make([]*game.Monster, 0)

	eligible := make([]MonsterTemplate, 0)
	for _, mt := range monsterTemplates {
		if mt.MinDiff <= difficulty {
			eligible = append(eligible, mt)
		}
	}

	if len(eligible) == 0 || dg.random.Float32() >= MonsterSpawnChance {
		return monsters
	}

	numMonsters := 1
	if difficulty >= TripleMonsterMinDiff && dg.random.Float32() < TripleMonsterChance {
		numMonsters = 3
	} else if difficulty >= MultiMonsterMinDiff && dg.random.Float32() < MultiMonsterChance {
		numMonsters = 2
	}

	for i := 0; i < numMonsters; i++ {
		template := eligible[dg.random.Intn(len(eligible))]
		scaleFactor := 1.0 + float64(difficulty)*DifficultyScaleFactor

		monster := &game.Monster{
			ID:          generateID(),
			Name:        template.Name,
			Description: template.Description,
			HP:          int(float64(template.BaseHP) * scaleFactor),
			MaxHP:       int(float64(template.BaseHP) * scaleFactor),
			Damage:      int(float64(template.BaseDamage) * scaleFactor),
			Dexterity:   template.BaseDex,
			RoomID:      room.ID,
			IsAlive:     true,
			IsRanged:    template.IsRanged,
			AttackRange: template.AttackRange,
			LootTable:   template.LootTable,
		}
		monsters = append(monsters, monster)
	}

	return monsters
}

func (dg *DungeonGenerator) spawnItems(room *game.Room, difficulty int) []*game.Item {
	items := make([]*game.Item, 0)

	itemChance := BaseItemChance + float64(difficulty)*ItemChancePerDiff
	if itemChance > MaxItemChance {
		itemChance = MaxItemChance
	}

	if dg.random.Float64() < itemChance {
		item := dg.pickItem(room, difficulty)
		if item != nil {
			items = append(items, item)
		}
	}

	// Small chance for a second item in harder rooms
	if difficulty >= 3 && dg.random.Float64() < SecondItemChance {
		item := dg.pickItem(room, difficulty)
		if item != nil {
			items = append(items, item)
		}
	}

	return items
}

// pickItem selects a random item from all template pools weighted by spawn weight and difficulty
func (dg *DungeonGenerator) pickItem(room *game.Room, difficulty int) *game.Item {
	// Combine all templates into one pool filtered by difficulty
	type weightedTemplate struct {
		template ItemTemplate
		weight   float64
	}

	pool := make([]weightedTemplate, 0)
	allTemplates := [][]ItemTemplate{weaponTemplates, armorTemplates, consumableTemplates, scrollTemplates}

	for _, templates := range allTemplates {
		for _, t := range templates {
			if t.MinDiff <= difficulty {
				pool = append(pool, weightedTemplate{template: t, weight: t.SpawnWeight})
			}
		}
	}

	if len(pool) == 0 {
		return nil
	}

	// Weighted random selection
	totalWeight := 0.0
	for _, wt := range pool {
		totalWeight += wt.weight
	}

	roll := dg.random.Float64() * totalWeight
	cumulative := 0.0
	selected := pool[len(pool)-1].template // fallback for floating-point edge case
	for _, wt := range pool {
		cumulative += wt.weight
		if roll <= cumulative {
			selected = wt.template
			break
		}
	}

	roomID := room.ID
	item := &game.Item{
		ID:               generateID(),
		Name:             selected.Name,
		Description:      selected.Description,
		Type:             selected.Type,
		Damage:           selected.Damage,
		Armor:            selected.Armor,
		Healing:          selected.Healing,
		Range:            selected.Range,
		MaxRange:         selected.MaxRange,
		Rarity:           selected.Rarity,
		ClassRestriction: selected.ClassRestriction,
		ScrollEffect:     selected.ScrollEffect,
		ScrollDifficulty: selected.ScrollDifficulty,
		RoomID:           &roomID,
	}

	return item
}

func (dg *DungeonGenerator) spawnTraps(room *game.Room, difficulty int) ([]*game.Trap, []*game.Item) {
	traps := make([]*game.Trap, 0)
	chestItems := make([]*game.Item, 0)

	// Room traps
	trapChance := BaseTrapChance + float64(difficulty)*TrapChancePerDiff
	if trapChance > MaxTrapChance {
		trapChance = MaxTrapChance
	}

	if dg.random.Float64() < trapChance {
		trap := dg.pickTrap(room, difficulty, roomTrapTemplates)
		if trap != nil {
			traps = append(traps, trap)
		}
	}

	// Chest with possible trap — every chest gets a Trap entity and loot
	if dg.random.Float64() < ChestSpawnChance {
		var chest *game.Trap
		if dg.random.Float64() < ChestTrapChance {
			chest = dg.pickTrap(room, difficulty, chestTrapTemplates)
		}
		if chest == nil {
			// Untrapped chest: benign trap entity so items have an ID to link to
			chest = &game.Trap{
				ID:          generateID(),
				RoomID:      room.ID,
				Location:    game.TrapChest,
				Description: "A sturdy wooden chest.",
				Damage:      0,
				Difficulty:  0,
				IsDisarmed:  true,
			}
		}
		traps = append(traps, chest)
		chestItems = append(chestItems, dg.spawnChestLoot(room, difficulty, chest.ID)...)
	}

	return traps, chestItems
}

// spawnChestLoot generates 1-2 items linked to a chest trap ID.
// Uses difficulty+1 and a 35% second item chance to reward chest interaction.
func (dg *DungeonGenerator) spawnChestLoot(room *game.Room, difficulty int, chestTrapID string) []*game.Item {
	items := make([]*game.Item, 0, 2)

	// Always one item at difficulty+1
	item := dg.pickItem(room, difficulty+1)
	if item != nil {
		item.ChestTrapID = &chestTrapID
		items = append(items, item)
	}

	// 35% chance of a second item
	if dg.random.Float64() < ChestSecondItemChance {
		item2 := dg.pickItem(room, difficulty+1)
		if item2 != nil {
			item2.ChestTrapID = &chestTrapID
			items = append(items, item2)
		}
	}

	return items
}

func (dg *DungeonGenerator) pickTrap(room *game.Room, difficulty int, templates []TrapTemplate) *game.Trap {
	eligible := make([]TrapTemplate, 0)
	for _, t := range templates {
		if t.MinDiff <= difficulty {
			eligible = append(eligible, t)
		}
	}

	if len(eligible) == 0 {
		return nil
	}

	template := eligible[dg.random.Intn(len(eligible))]
	scaleFactor := 1.0 + float64(difficulty)*TrapDifficultyScaleFactor

	return &game.Trap{
		ID:          generateID(),
		RoomID:      room.ID,
		Location:    template.Location,
		Description: template.Description,
		Damage:      int(float64(template.BaseDamage) * scaleFactor),
		Difficulty:  template.BaseDC + difficulty/2,
	}
}

// GetRoomDifficulty calculates difficulty based on Manhattan distance from entrance
func GetRoomDifficulty(room *game.Room) int {
	return room.X + room.Y
}

// --- Room Name/Description Generation ---

func (dg *DungeonGenerator) generateRoomName() string {
	adjectives := []string{
		"Dark", "Dusty", "Ancient", "Forgotten", "Cursed",
		"Silent", "Echoing", "Gloomy", "Damp", "Musty",
		"Crumbling", "Shadowy", "Foul", "Narrow", "Vast",
	}
	nouns := []string{
		"Chamber", "Hall", "Corridor", "Vault", "Crypt",
		"Passage", "Alcove", "Sanctum", "Den", "Lair",
		"Gallery", "Barracks", "Cellar", "Tomb", "Antechamber",
	}

	adj := adjectives[dg.random.Intn(len(adjectives))]
	noun := nouns[dg.random.Intn(len(nouns))]
	return fmt.Sprintf("%s %s", adj, noun)
}

func (dg *DungeonGenerator) generateRoomDescription(x, y int) string {
	distance := x + y

	if x == 0 && y == 0 {
		return "The entrance to the dungeon. Faint light filters in from behind you."
	}
	if x == GridSize-1 && y == GridSize-1 {
		return "A grand chamber with an ornate door leading to freedom!"
	}

	descriptions := []string{
		"Cold stone walls surround you. Water drips somewhere in the darkness.",
		"Cobwebs hang from the ceiling. The air smells of decay.",
		"Torches flicker weakly on the walls, casting dancing shadows.",
		"Bones are scattered across the floor. Something died here.",
		"Strange runes are carved into the walls, pulsing with faint light.",
		"The ceiling is low here, forcing you to crouch slightly.",
		"Claw marks score the stone walls. Something large passed through.",
		"A cold draft blows through, carrying whispers from deeper within.",
		"Rusted chains hang from the ceiling. Dark stains mark the floor.",
		"Fungal growths cling to the damp walls, giving off a sickly glow.",
	}

	idx := dg.random.Intn(len(descriptions))
	if distance > ScaryDescriptionDist && dg.random.Float32() < ScaryDescriptionChance {
		idx = dg.random.Intn(4) + 6 // Prefer scarier descriptions
	}

	return descriptions[idx]
}