# API Reference

Complete API documentation for the Yrpg party-based dungeon crawler backend.

## Base URL

```
Development: http://localhost:8080
```

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/mcp/tools` | List available MCP tools |
| POST | `/mcp/call` | Execute an MCP tool |

---

## MCP Tool Call

All game actions go through the MCP call endpoint.

### Request

```http
POST /mcp/call
Content-Type: application/json

{
  "name": "tool_name",
  "arguments": { ... }
}
```

### Response

```json
{
  "content": [
    { "type": "text", "text": "Human-readable result message" }
  ],
  "gameState": { ... }
}
```

### Error Response

```json
{
  "content": [
    { "type": "text", "text": "Error description" }
  ],
  "isError": true
}
```

When `isError` is true, `gameState` is omitted.

---

## MCP Tools

### Party & Game Management

#### `new_game`

Start a new game with a party of 1-3 characters.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `characters` | array | Yes | Array of `{name, class}` objects. Max 3. |

Each character object:
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | Yes | Character name (must be unique within the party) |
| `class` | string | Yes | One of: `fighter`, `magic_user`, `thief` |

**Example:**
```json
{
  "name": "new_game",
  "arguments": {
    "characters": [
      { "name": "Conan", "class": "fighter" },
      { "name": "Gandalf", "class": "magic_user" },
      { "name": "Shadow", "class": "thief" }
    ]
  }
}
```

**Notes:**
- Generates a procedural dungeon with rooms, monsters, items, and traps
- Party starts at the dungeon entrance
- Formation order matches the array order (first = point character)

**Errors:**
- Empty character list
- More than 3 characters
- Duplicate names
- Invalid class

---

### Exploration Tools

#### `look`

Examine the current room. Returns exits, monsters, items, and discovered traps.

**Arguments:** None

**Example:**
```json
{ "name": "look", "arguments": {} }
```

---

#### `move`

Move the party in a direction. Triggers the room entry sequence on first visit: trap detection, sneak check, combat initiation.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `direction` | string | Yes | One of: `north`, `south`, `east`, `west` |

**Example:**
```json
{ "name": "move", "arguments": { "direction": "north" } }
```

**Room Entry Sequence (first visit only):**
1. **Trap detection** - Party attempts to spot room traps. Thieves get a detection bonus.
2. **Undetected traps trigger** on the point character (first alive in formation).
3. **Sneak check** - If monsters are present and the party has a thief, a stealth roll is made. Success gives the thief a hidden advantage in combat.
4. **Combat begins** if enemies are present.

**Errors:**
- No exit in that direction
- Monsters blocking the path (must defeat them first)
- Currently in combat

---

#### `take`

Have a character pick up an item from the current room floor.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the character picking up the item |
| `item_id` | string | Yes | ID of the item to pick up |

**Example:**
```json
{ "name": "take", "arguments": { "character_id": "abc...", "item_id": "def..." } }
```

---

#### `equip`

Equip a weapon or armor from a character's inventory. Enforces class restrictions.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the character equipping |
| `item_id` | string | Yes | ID of the item to equip |

**Notes:**
- Weapons add to damage (melee uses STR, ranged uses DEX)
- Armor adds to defense (AC)
- Equipping replaces the current item of that slot (old item returns to inventory)
- Items with `classRestriction` can only be equipped by listed classes

---

#### `use_item`

Use a consumable or scroll from a character's inventory.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the character using the item |
| `item_id` | string | Yes | ID of the item to use |
| `target_id` | string | No | Target character ID (for healing/resurrection effects) |

**Consumables:** Destroyed after use. Health potions heal the user.

**Scrolls:** Roll d20 + INT/2 + class modifier vs scroll DC.
- **Success:** Effect applied. Magic users permanently learn the spell.
- **Failure (miss by < 5):** Scroll consumed, no effect.
- **Critical failure (miss by >= 5):** Backfire — damage scrolls hurt the caster.

**Class modifiers for scrolls:**
| Class | Modifier |
|-------|----------|
| magic_user | +6 |
| thief | +0 |
| fighter | -2 |

---

#### `open_chest`

Open a chest in the current room. May trigger a chest trap.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the character opening the chest |

**Notes:**
- If the chest trap was already discovered, warns the player to disarm first
- If undetected, rolls trap detection; failure triggers the trap on the opener

---

#### `disarm_trap`

Have a character attempt to disarm a discovered trap.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the character attempting |
| `trap_id` | string | Yes | ID of the discovered trap |

**Mechanics:** Roll d20 + DEX/2 + class bonus vs trap DC.
- Thieves get +6 bonus
- Success: trap disarmed permanently
- Failure: trap triggers on the disarmer

---

#### `sneak`

Thief scouts an adjacent room without entering. Reveals enemies, traps, and items on success.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the thief character |
| `direction` | string | Yes | Direction to scout: `north`, `south`, `east`, `west` |

**Notes:**
- Only thieves can use this
- Roll d20 + DEX/2 vs DC (based on room difficulty)
- Success reveals room contents without triggering traps or combat

---

#### `inventory`

View all party members' inventories and equipment.

**Arguments:** None

---

#### `stats`

View all party member stats, equipment, spell slots, and formation.

**Arguments:** None

---

#### `map`

View the dungeon map showing explored, adjacent, and unknown areas.

**Arguments:** None

---

### Persistence Tools

#### `save_game`

Save the current game state to the database.

**Arguments:** None

**Response:** Includes the session ID needed for loading.

---

#### `load_game`

Load a previously saved game by session ID.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `session_id` | string | Yes | Session ID from a previous save |

---

### Spell & Rest Tools

#### `cast_spell`

Cast a known spell outside of combat. Only non-combat spells (heal, resurrect) can be cast.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the magic_user casting |
| `spell_id` | string | Yes | Spell ID (e.g., `heal`, `resurrect`) |
| `target_id` | string | No | Target character ID (defaults to self) |

---

#### `combat_cast_spell`

Cast a known spell during combat. Any known spell can be cast.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the magic_user casting |
| `spell_id` | string | Yes | Spell ID |
| `target_id` | string | No | Target ID (character for heals, enemy combatant for attacks) |

---

#### `rest`

Rest in a cleared room to recover HP and spell slots. Risk of wandering monster ambush.

**Arguments:** None

**Rest Mechanics:**
- Heals each alive character for ~33% of their MaxHP
- Recharges all spell slots for magic users
- Ambush chance: 30% + 2% per room depth, halved if a thief is alive to stand watch
- Ambush spawns 1-3 monsters scaled to depth; monsters get a surprise round if no thief is on watch

---

### Combat Tools

All combat tools require an active combat encounter. Each character gets one move and one action per turn. Turn order follows initiative.

#### `combat_status`

View combat grid, positions, initiative order, and HP.

**Arguments:** None

---

#### `combat_move`

Move a character on the 6x6 combat grid.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the character to move |
| `x` | number | Yes | Target X position (0-5) |
| `y` | number | Yes | Target Y position (0-5) |

**Notes:**
- Movement cost uses Manhattan distance
- Fighters and magic users: 2 cells. Thieves: 4 cells.
- Cannot move to occupied or blocked cells
- One move per turn

---

#### `combat_attack`

Attack a target. Melee requires adjacency; ranged requires an equipped ranged weapon and target within range.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the attacking character |
| `target_id` | string | Yes | ID of the enemy combatant to attack |

**Notes:**
- Cannot attack your own party members
- Adjacency uses Chebyshev distance (diagonals count as 1)
- Creates mutual engagement on melee hit
- Attacking breaks hidden status (even on miss)

---

#### `combat_use_item`

Use an item or scroll during combat. Consumes the character's action.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the character using the item |
| `item_id` | string | Yes | ID of the item to use |
| `target_id` | string | No | Optional target character ID |

---

#### `combat_defend`

Take a full defense stance. Grants +4 AC for the round.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the defending character |

---

#### `combat_hide`

Thief attempts to hide in combat. On success: breaks engagement and next attack has advantage.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the thief attempting to hide |

**Notes:**
- Only thieves can hide
- Roll d20 + DEX/2 vs highest enemy DEX/2 + 10
- Success: hidden (monsters can't target, next attack has advantage)

---

#### `combat_retreat`

Attempt to flee combat. Triggers opportunity attacks from engaged enemies.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the character retreating |

**Mechanics:**
1. If engaged, the engaged enemy gets a free opportunity attack
2. Roll d20 + DEX/2 vs DC 12. Thieves get +4 bonus.
3. Success: character is removed from the grid
4. If all living party members retreat, the party flees to the previous room

---

#### `end_turn`

End the current character's turn without acting.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the character ending their turn |

---

## Game State Snapshot

Every successful response includes a `gameState` object with the complete game state for UI rendering.

### Full Structure

```typescript
interface GameStateSnapshot {
  mode: "exploration" | "combat";
  party: PartyView | null;
  currentRoom: RoomView | null;
  monsters: MonsterView[];
  roomItems: ItemView[];
  roomTraps: TrapView[];           // only discovered traps
  combat: CombatView | null;
  mapGrid: MapCell[][];
  gameOver: boolean;
  victory: boolean;
  turnNumber: number;
  message: string;
  lastEvent: GameEvent | null;
}
```

---

## Type Definitions

### PartyView

```typescript
interface PartyView {
  characters: CharacterView[];
  formation: string[];             // character IDs in marching order
}
```

---

### CharacterView

```typescript
interface CharacterView {
  id: string;
  name: string;
  class: "fighter" | "magic_user" | "thief";
  hp: number;
  maxHp: number;
  strength: number;
  dexterity: number;
  intelligence: number;
  spellSlots: number;              // 0 for non-magic_user
  maxSpellSlots: number;
  knownSpells: string[];           // empty for non-magic_user
  isAlive: boolean;
  status: "Healthy" | "Wounded" | "Critical" | "Dead";
}
```

**Status Thresholds:**
- `Healthy`: HP > 75% of max
- `Wounded`: HP 40-75% of max
- `Critical`: HP < 40% of max
- `Dead`: HP <= 0

---

### Class Stats

| Class | HP | STR | DEX | INT | Move | Init Bonus |
|-------|-----|-----|-----|-----|------|------------|
| fighter | 30 | 14 | 10 | 8 | 2 | +0 |
| magic_user | 16 | 8 | 10 | 16 | 2 | +0 |
| thief | 22 | 10 | 14 | 10 | 4 | +2 |

**Special abilities:**
- **Fighter:** Highest HP and STR. Best melee damage.
- **Magic User:** Spell slots (INT / 5). Starts with heal + fireball. Can learn spells from scrolls. +6 scroll bonus.
- **Thief:** +4 movement. +2 initiative. +6 trap disarm. +4 retreat. Can sneak/scout. Can hide in combat.

---

### RoomView

```typescript
interface RoomView {
  id: string;
  name: string;
  description: string;
  isEntrance: boolean;
  isExit: boolean;
  x: number;
  y: number;
  exits: string[];                 // e.g. ["north", "east"]
  isFirstVisit: boolean;
}
```

---

### MonsterView

```typescript
interface MonsterView {
  id: string;
  name: string;
  description: string;
  hp: number;
  maxHp: number;
  damage: number;
  threat: "trivial" | "normal" | "dangerous" | "deadly";
  isDefeated: boolean;
  isRanged: boolean;
}
```

**Threat Calculation (based on MaxHP):**
| MaxHP | Threat |
|-------|--------|
| <= 8 | trivial |
| 9-19 | normal |
| 20-29 | dangerous |
| >= 30 | deadly |

---

### ItemView

```typescript
interface ItemView {
  id: string;
  name: string;
  description: string;
  type: "weapon" | "armor" | "consumable" | "scroll" | "key" | "treasure";
  damage?: number;
  armor?: number;
  healing?: number;
  range?: "melee" | "ranged";
  rarity: "common" | "uncommon" | "rare" | "legendary";
  classRestriction?: string[];     // e.g. ["fighter", "thief"]
  isEquipped: boolean;
}
```

---

### TrapView

Only discovered traps appear in the snapshot.

```typescript
interface TrapView {
  id: string;
  description: string;
  isDisarmed: boolean;
  difficulty: number;              // DC for disarm attempt
}
```

---

### CombatView

Present when `mode === "combat"`.

```typescript
interface CombatView {
  grid: string[][];                // 6x6 grid. Cell values: combatant ID, "blocked", or ""
  combatants: CombatantView[];
  currentTurnIdx: number;
  roundNumber: number;
  isActive: boolean;
}
```

---

### CombatantView

```typescript
interface CombatantView {
  id: string;
  name: string;
  isPlayerChar: boolean;
  hp: number;
  maxHp: number;
  gridX: number;                   // 0-5
  gridY: number;                   // 0-5
  hasMoved: boolean;
  hasActed: boolean;
  isAlive: boolean;
  isHidden: boolean;
  movementRange: number;           // max cells per turn (Manhattan distance)
  attackRange: number;             // max attack distance (Chebyshev distance)
  knownSpells?: string[];          // spell IDs for magic users, omitted for others
}
```

**`movementRange` values:**
| Source | Value |
|--------|-------|
| Fighter | 2 |
| Magic User | 2 |
| Thief | 4 |
| All monsters | 2 |

**`attackRange` values:**
| Source | Value |
|--------|-------|
| Melee character (no ranged weapon) | 1 |
| Character with ranged weapon | weapon's `maxRange` |
| Melee monster | 1 |
| Ranged monster | monster's `attackRange` |

**`knownSpells`:** Present only for magic users. Contains spell IDs (e.g. `"heal"`, `"fireball"`). Omitted from JSON for non-casters.

---

### MapCell

```typescript
interface MapCell {
  x: number;
  y: number;
  roomId?: string;
  status: "unknown" | "visited" | "current" | "adjacent" | "exit";
  hasPlayer: boolean;
  exits?: string[];
}
```

---

### GameEvent

```typescript
interface GameEvent {
  sessionId: string;
  turnNumber: number;
  eventType: string;
  eventSubtype: string;
  actorId: string;
  actorClass: string;
  targetId: string;
  roomId: string;
  details: EventDetails;
}

interface EventDetails {
  roll?: number;
  dc?: number;
  damage?: number;
  wasCritical?: boolean;
  wasFlanking?: boolean;
  weaponUsed?: string;
  partyHp?: Record<string, number>;
  partyAlive?: number;
  actorPos?: { x: number; y: number };
  targetPos?: { x: number; y: number };
  itemName?: string;
  scrollEffect?: string;
  success?: boolean;
  trapDifficulty?: number;
  wasDetected?: boolean;
  resultText?: string;
}
```

**Event Types:**

| Type | Subtypes |
|------|----------|
| `combat` | `attack_hit`, `attack_miss`, `critical_hit`, `flanking_attack`, `monster_attack_hit`, `monster_attack_miss`, `combat_victory`, `retreat_success`, `retreat_fail`, `ambush` |
| `spell` | `spell_cast`, `combat_spell_cast` |
| `item` | `consumable_used`, `scroll_success`, `scroll_fail`, `scroll_backfire` |
| `trap` | `trap_detected`, `trap_triggered`, `trap_disarmed`, `disarm_failed` |
| `stealth` | `sneak_success`, `sneak_fail`, `scout_success`, `scout_fail`, `hide_success`, `hide_fail` |
| `movement` | `room_enter` |
| `interaction` | `game_start`, `rest` |
| `death` | `character_killed`, `enemy_defeated`, `party_wipe` |
| `victory` | `dungeon_escaped` |

---

## Spell System

### Available Spells

| ID | Name | Combat Only | Effect |
|----|------|-------------|--------|
| `heal` | Heal | No | 2d6+4 HP to target (default: self) |
| `resurrect` | Resurrect | No | Revive a dead party member at 50% HP |
| `fireball` | Fireball | Yes | 3d6 damage to target + half splash to adjacent enemies |
| `lightning` | Lightning Bolt | Yes | 4d6 damage to single target |
| `shield` | Shield | Yes | +4 AC to all party combatants for 3 rounds |
| `sleep` | Sleep | Yes | Target skips 2 turns (damage wakes them) |

**Spell Slots:** Magic users get INT / 5 spell slots. Recharged by resting.

**Starting Spells:** Magic users begin knowing `heal` and `fireball`.

**Learning Spells:** When a magic_user successfully casts a scroll, they permanently learn that spell.

---

## Combat Mechanics

### Turn Structure
1. Initiative: d20 + DEX/2 + class bonus. Higher goes first.
2. Each turn: one move + one action (attack, defend, cast spell, use item, hide, retreat).
3. After a player acts, turn advances. Monster turns run automatically.

### Grid
- 6x6 grid. Party spawns at row 0, monsters at rows 4-5.
- Movement: Manhattan distance.
- Adjacency/attack range: Chebyshev distance (diagonals = 1).

### Attack Roll
- **Roll:** d20 + modifier vs target AC
- **Melee modifier:** STR / 2
- **Ranged modifier:** DEX / 2
- **Target AC:** 10 (base) + armor bonus + buff AC bonus
- **Critical hit:** Natural 20. Doubles damage dice.
- **Flanking:** Attacking a target engaged with someone else gives advantage (roll twice, take better).
- **Hidden advantage:** Attacking from hidden gives advantage.

### Damage
- **Base:** 1d6 (2d6 on critical)
- **Melee bonus:** STR/2 + weapon damage
- **Ranged bonus:** DEX/2 + weapon damage
- **Minimum:** 1

### Monster AI
Monsters act automatically after player turns:
- **Sleeping:** Skip turn
- **Melee:** Move toward nearest visible player (2 cells/turn), attack if adjacent
- **Ranged:** Attack if in range, otherwise move closer
- **Targeting:** Prefer engaged target, then nearest non-hidden player

### Engagement
- Melee attacks create mutual engagement between attacker and target
- Engaged combatants provoke opportunity attacks when retreating
- Hiding breaks engagement

### Buffs
- **AC Bonus (shield, defend):** Adds to defense for N rounds
- **Sleep:** Target skips turns, damage wakes them
- Buffs tick down each round

---

## Trap Mechanics

### Detection
- On room entry, party rolls to detect each trap
- Thieves get a detection bonus
- Detected traps are revealed in the snapshot

### Triggering
- Undetected room traps trigger on the point character (first alive in formation)
- Chest traps trigger on the character who opens the chest
- Damage is applied directly to the victim

### Disarming
- Only discovered traps can be disarmed
- Roll: d20 + DEX/2 + class bonus vs trap DC
- Thieves get +6 bonus
- Failure triggers the trap on the disarmer

---

## Example: Full Combat Response

```json
{
  "content": [
    { "type": "text", "text": "Conan hits Goblin for 9 damage! (Goblin HP: 1/10)" }
  ],
  "gameState": {
    "mode": "combat",
    "party": {
      "characters": [
        {
          "id": "char_001",
          "name": "Conan",
          "class": "fighter",
          "hp": 28,
          "maxHp": 30,
          "strength": 14,
          "dexterity": 10,
          "intelligence": 8,
          "spellSlots": 0,
          "maxSpellSlots": 0,
          "knownSpells": [],
          "isAlive": true,
          "status": "Healthy"
        },
        {
          "id": "char_002",
          "name": "Gandalf",
          "class": "magic_user",
          "hp": 16,
          "maxHp": 16,
          "strength": 8,
          "dexterity": 10,
          "intelligence": 16,
          "spellSlots": 2,
          "maxSpellSlots": 3,
          "knownSpells": ["heal", "fireball"],
          "isAlive": true,
          "status": "Healthy"
        }
      ],
      "formation": ["char_001", "char_002"]
    },
    "currentRoom": {
      "id": "room_005",
      "name": "Dusty Chamber",
      "description": "Cold stone walls surround you.",
      "isEntrance": false,
      "isExit": false,
      "x": 1,
      "y": 0,
      "exits": ["south", "east"],
      "isFirstVisit": false
    },
    "monsters": [
      {
        "id": "mon_001",
        "name": "Goblin",
        "description": "A small green creature.",
        "hp": 1,
        "maxHp": 10,
        "damage": 4,
        "threat": "normal",
        "isDefeated": false,
        "isRanged": false
      }
    ],
    "roomItems": [],
    "roomTraps": [],
    "combat": {
      "grid": [
        ["char_001", "", "", "", "", ""],
        ["", "char_002", "", "", "", ""],
        ["", "", "", "", "", ""],
        ["", "", "", "", "", ""],
        ["", "", "mon_001", "", "", ""],
        ["", "", "", "", "", ""]
      ],
      "combatants": [
        {
          "id": "char_001",
          "name": "Conan",
          "isPlayerChar": true,
          "hp": 28,
          "maxHp": 30,
          "gridX": 0,
          "gridY": 0,
          "hasMoved": true,
          "hasActed": true,
          "isAlive": true,
          "isHidden": false,
          "movementRange": 2,
          "attackRange": 1
        },
        {
          "id": "char_002",
          "name": "Gandalf",
          "isPlayerChar": true,
          "hp": 16,
          "maxHp": 16,
          "gridX": 1,
          "gridY": 1,
          "hasMoved": false,
          "hasActed": false,
          "isAlive": true,
          "isHidden": false,
          "movementRange": 2,
          "attackRange": 1,
          "knownSpells": ["heal", "fireball"]
        },
        {
          "id": "mon_001",
          "name": "Goblin",
          "isPlayerChar": false,
          "hp": 1,
          "maxHp": 10,
          "gridX": 2,
          "gridY": 4,
          "hasMoved": false,
          "hasActed": false,
          "isAlive": true,
          "isHidden": false,
          "movementRange": 2,
          "attackRange": 1
        }
      ],
      "currentTurnIdx": 1,
      "roundNumber": 2,
      "isActive": true
    },
    "gameOver": false,
    "victory": false,
    "turnNumber": 5,
    "message": "",
    "lastEvent": null
  }
}
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `DB_PATH` | `./yrpg.db` | SQLite database path |
| `CORS_ORIGINS` | `localhost:3000,5173` | Comma-separated allowed CORS origins |

---

## Dungeon Layout

- Grid-based procedural generation
- Entrance at (0, 0), exit at far corner
- Fully connected via spanning tree + extra corridors
- Room difficulty scales with Manhattan distance from entrance
- Monster HP/damage scale: `1.0 + (difficulty * 0.15)`
