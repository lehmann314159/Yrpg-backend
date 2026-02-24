package game

import "time"

// --- Character & Class ---

type CharacterClass string

const (
	ClassFighter   CharacterClass = "fighter"
	ClassMagicUser CharacterClass = "magic_user"
	ClassThief     CharacterClass = "thief"
)

type ClassStats struct {
	BaseHP          int
	BaseStrength    int
	BaseDexterity   int
	BaseIntelligence int
	MovementRange   int // combat grid cells per turn
	InitiativeBonus int
}

type Character struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Class            CharacterClass `json:"class"`
	HP               int            `json:"hp"`
	MaxHP            int            `json:"max_hp"`
	Strength         int            `json:"strength"`
	Dexterity        int            `json:"dexterity"`
	Intelligence     int            `json:"intelligence"`
	SpellSlots       int            `json:"spell_slots"`
	MaxSpellSlots    int            `json:"max_spell_slots"`
	KnownSpells      []string       `json:"known_spells"`
	CurrentRoomID    string         `json:"current_room_id"`
	IsAlive          bool           `json:"is_alive"`
	EquippedWeaponID *string        `json:"equipped_weapon_id,omitempty"`
	EquippedArmorID  *string        `json:"equipped_armor_id,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	DiedAt           *time.Time     `json:"died_at,omitempty"`
}

// --- Party ---

type Party struct {
	ID         string       `json:"id"`
	Characters []*Character `json:"characters"`
	Formation  []string     `json:"formation"` // character IDs in marching order (first = point)
}

// --- Combat ---

type CombatMode string

const (
	ModeExploration CombatMode = "exploration"
	ModeCombat      CombatMode = "combat"
)

type CombatState struct {
	Grid           [6][6]*GridCell    `json:"grid"`
	Combatants     []*Combatant       `json:"combatants"`
	CurrentTurnIdx int                `json:"current_turn_idx"`
	RoundNumber    int                `json:"round_number"`
	Engagements    map[string]string  `json:"engagements"` // combatantID -> engagedWithID
	IsActive       bool               `json:"is_active"`
	PreviousRoomID string             `json:"previous_room_id"`
}

type GridCell struct {
	X          int     `json:"x"`
	Y          int     `json:"y"`
	OccupantID *string `json:"occupant_id,omitempty"`
	IsBlocked  bool    `json:"is_blocked"`
}

// BuffType identifies what a buff does
type BuffType string

const (
	BuffACBonus  BuffType = "ac_bonus"  // adds to effective AC
	BuffSleep    BuffType = "sleep"     // target skips turns
)

// Buff is a temporary effect on a combatant
type Buff struct {
	Type           BuffType `json:"type"`
	Value          int      `json:"value"`           // magnitude (e.g. +4 AC)
	RoundsLeft     int      `json:"rounds_left"`     // decremented each round
	SourceName     string   `json:"source_name"`     // for display (e.g. "Shield Scroll")
}

type Combatant struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	IsPlayerChar bool    `json:"is_player_char"`
	CharacterID  string  `json:"character_id"` // ref to Character or Monster
	Initiative   int     `json:"initiative"`
	HP           int     `json:"hp"`
	MaxHP        int     `json:"max_hp"`
	GridX        int     `json:"grid_x"`
	GridY        int     `json:"grid_y"`
	HasMoved     bool    `json:"has_moved"`
	HasActed     bool    `json:"has_acted"`
	IsAlive      bool    `json:"is_alive"`
	IsHidden     bool    `json:"is_hidden"`
	EngagedWith  *string `json:"engaged_with,omitempty"`
	Buffs        []Buff  `json:"buffs,omitempty"`
}

// --- Items ---

type ItemType string

const (
	ItemWeapon     ItemType = "weapon"
	ItemArmor      ItemType = "armor"
	ItemConsumable ItemType = "consumable"
	ItemScroll     ItemType = "scroll"
	ItemKey        ItemType = "key"
	ItemTreasure   ItemType = "treasure"
)

type WeaponRange string

const (
	RangeMelee  WeaponRange = "melee"
	RangeRanged WeaponRange = "ranged"
)

type Item struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Description      string           `json:"description"`
	Type             ItemType         `json:"type"`
	Damage           int              `json:"damage"`
	Armor            int              `json:"armor"`
	Healing          int              `json:"healing"`
	Range            WeaponRange      `json:"range"`
	MaxRange         int              `json:"max_range"`
	Rarity           string           `json:"rarity"`
	ClassRestriction []CharacterClass `json:"class_restriction,omitempty"`
	ScrollEffect     string           `json:"scroll_effect,omitempty"`
	ScrollDifficulty int              `json:"scroll_difficulty,omitempty"`
	RoomID           *string          `json:"room_id,omitempty"`
	CharacterID      *string          `json:"character_id,omitempty"`
	IsEquipped       bool             `json:"is_equipped"`
}

// --- Traps ---

type TrapLocation string

const (
	TrapRoom  TrapLocation = "room"
	TrapChest TrapLocation = "chest"
)

type Trap struct {
	ID           string       `json:"id"`
	RoomID       string       `json:"room_id"`
	Location     TrapLocation `json:"location"`
	Description  string       `json:"description"`
	Damage       int          `json:"damage"`
	Difficulty   int          `json:"difficulty"`
	IsTriggered  bool         `json:"is_triggered"`
	IsDiscovered bool         `json:"is_discovered"`
	IsDisarmed   bool         `json:"is_disarmed"`
}

// --- Dungeon & Rooms ---

type Dungeon struct {
	ID        string    `json:"id"`
	Seed      int64     `json:"seed"`
	Depth     int       `json:"depth"`
	CreatedAt time.Time `json:"created_at"`
}

type Room struct {
	ID          string   `json:"id"`
	DungeonID   string   `json:"dungeon_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	IsEntrance  bool     `json:"is_entrance"`
	IsExit      bool     `json:"is_exit"`
	X           int      `json:"x"`
	Y           int      `json:"y"`
	Exits       []string `json:"exits"`
}

type RoomConnection struct {
	ID              string `json:"id"`
	RoomID          string `json:"room_id"`
	Direction       string `json:"direction"`
	ConnectedRoomID string `json:"connected_room_id"`
}

// --- Monsters ---

type Monster struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	HP          int         `json:"hp"`
	MaxHP       int         `json:"max_hp"`
	Damage      int         `json:"damage"`
	Dexterity   int         `json:"dexterity"`
	RoomID      string      `json:"room_id"`
	IsAlive     bool        `json:"is_alive"`
	LootTable   []LootEntry `json:"loot_table,omitempty"`
	IsRanged    bool        `json:"is_ranged"`
	AttackRange int         `json:"attack_range"`
}

type LootEntry struct {
	ItemTemplateID string  `json:"item_template_id"`
	DropChance     float64 `json:"drop_chance"`
}

// --- Events & Analytics ---

type GameEvent struct {
	SessionID    string       `json:"session_id"`
	TurnNumber   int          `json:"turn_number"`
	EventType    string       `json:"event_type"`
	EventSubtype string       `json:"event_subtype"`
	ActorID      string       `json:"actor_id"`
	ActorClass   string       `json:"actor_class"`
	TargetID     string       `json:"target_id"`
	RoomID       string       `json:"room_id"`
	Details      EventDetails `json:"details"`
}

type GridPos struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type EventDetails struct {
	// Combat context
	Roll        int  `json:"roll,omitempty"`
	DC          int  `json:"dc,omitempty"`
	Damage      int  `json:"damage,omitempty"`
	WasCritical bool `json:"was_critical,omitempty"`
	WasFlanking bool `json:"was_flanking,omitempty"`
	WeaponUsed  string `json:"weapon_used,omitempty"`

	// Party context (snapshot at time of event)
	PartyHP    map[string]int `json:"party_hp,omitempty"`
	PartyAlive int            `json:"party_alive,omitempty"`

	// Grid context
	ActorPos  *GridPos `json:"actor_pos,omitempty"`
	TargetPos *GridPos `json:"target_pos,omitempty"`

	// Item/scroll context
	ItemName     string `json:"item_name,omitempty"`
	ScrollEffect string `json:"scroll_effect,omitempty"`
	Success      bool   `json:"success,omitempty"`

	// Trap context
	TrapDifficulty int  `json:"trap_difficulty,omitempty"`
	WasDetected    bool `json:"was_detected,omitempty"`

	// Outcome
	ResultText string `json:"result_text,omitempty"`
}

// --- Turn Context ---

type TurnContext struct {
	TurnNumber        int
	LastEvent         *GameEvent
	DefeatedMonsters  []string
	NewItems          []string
	TurnsInRoom       int
	ConsecutiveCombat int
}