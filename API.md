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

Move the party in a direction. Triggers the room entry sequence on first visit. Also triggers combat on re-entry if surviving monsters are present.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `direction` | string | Yes | One of: `north`, `south`, `east`, `west` |

**Example:**
```json
{ "name": "move", "arguments": { "direction": "north" } }
```

**Room Entry Sequence (first visit only):**
1. **Sneak check** - If monsters are present and the party has an alive thief, a stealth roll is made.
   - **Success:** Only the thief enters the room. Solo trap detection runs. Scout combat begins (thief hidden, initiative 100, double movement). This is equivalent to a `scout_ahead` surprise round.
   - **Failure:** Falls through to normal party entry below.
2. **Party enters** - All characters move into the room.
3. **Trap detection** - Party attempts to spot room traps. Thieves get a detection bonus. Detected traps still trigger on the first non-thief character. Undetected traps trigger on the point character (first alive in formation).
4. **Combat begins** if enemies are present.

**Re-entry:** Moving into a previously visited room with surviving monsters (e.g. after a scout retreat) triggers combat again.

**Errors:**
- No exit in that direction
- Monsters in current room (must defeat them before leaving)
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

#### `drop_item`

Drop an item from a character's inventory onto the current room floor. Auto-unequips if the item is equipped.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the character dropping the item |
| `item_id` | string | Yes | ID of the item to drop |

**Notes:**
- If the item is equipped, it is automatically unequipped before dropping
- Item appears on the current room's floor and can be picked up by any character

---

#### `give_item`

Transfer an item from one character to another party member. Auto-unequips if the item is equipped.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the character giving the item |
| `item_id` | string | Yes | ID of the item to give |
| `target_character_id` | string | Yes | ID of the character receiving the item |

**Notes:**
- If the item is equipped, it is automatically unequipped before transferring
- Cannot give an item to the same character

**Errors:**
- Item not in the giver's inventory
- Same character as giver and receiver

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

Open a chest in the current room. Reveals chest loot. May trigger a chest trap.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the character opening the chest |

**Notes:**
- Every chest contains 1-2 items (generated at dungeon creation, hidden until chest is opened)
- Chest loot uses `difficulty + 1` with a 35% second item chance (vs 15% for floor drops)
- If the chest trap was already discovered, warns the player to disarm first
- If undetected, rolls trap detection; failure triggers the trap on the opener
- After a trap triggers, the chest still opens and reveals its loot
- Untrapped chests open immediately and reveal loot
- The `look` command mentions any unopened chests in the room

---

#### `disarm_trap`

Have a character attempt to disarm a discovered trap. Also usable by the scouting thief during the scout decision phase.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the character attempting |
| `trap_id` | string | Yes | ID of the discovered trap |

**Mechanics:** Roll d20 + DEX/2 + class bonus vs trap DC.
- Thieves get +6 bonus
- Success: trap disarmed permanently
- Failure: trap triggers on the disarmer
- For chest traps: loot is revealed on both success and failure (the chest opens either way)

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

#### `set_formation`

Reorder the party's marching order. The first character is the point (triggers traps first), and the last is the rear.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `formation` | array of strings | Yes | Character IDs in desired marching order (front to back). Must include all party members. |

**Example:**
```json
{ "name": "set_formation", "arguments": { "formation": ["char_003", "char_001", "char_002"] } }
```

**Notes:**
- Must include every party member exactly once
- Affects who triggers traps (point character = first alive in formation)
- `FirstNonThief()` also uses formation order (for detected-trap targeting)
- Cannot be used during combat

**Errors:**
- Wrong number of IDs (must match party size)
- Invalid character ID
- Duplicate character ID
- Currently in combat

---

#### `scout_ahead`

Thief physically enters an adjacent room solo for a surprise round with double movement. If the sneak check succeeds, the thief enters combat alone with the enemies unaware. If it fails, the whole party rushes in for normal combat.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the thief character |
| `direction` | string | Yes | Direction to scout: `north`, `south`, `east`, `west` |

**On sneak success:**
1. Only the thief moves to the target room (party stays behind)
2. Traps are checked using solo detection (thief only)
3. Scout combat begins: thief is hidden with initiative 100, monsters are frozen
4. Thief gets a surprise round with **double movement** (8 cells instead of 4)
5. After acting, the thief must choose `signal_party` or `combat_retreat`

**On sneak failure:**
- Entire party enters the room and normal combat begins (no surprise round)

**Errors:**
- Character is not a thief
- No exit in that direction
- No enemies in target room (use `move` instead)
- Currently in combat
- Monsters in current room

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

#### `combat_cantrip`

Magic user fires a free ranged arcane bolt at a target. No spell slot cost.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the magic_user character |
| `target_id` | string | Yes | ID of the enemy combatant to target |

**Mechanics:**
- Only magic users can use this
- Range: 3 (Chebyshev distance)
- Attack roll: d20 + INT/2 vs target AC
- Damage: 1d4 + INT/2 (minimum 1)
- Critical hit (natural 20): 2d4 + INT/2
- Free action — does not cost a spell slot
- Consumes the character's action for the turn

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
- Movement cost uses Chebyshev distance (diagonal moves cost 1)
- Fighters: 3 cells. Magic users: 2 cells. Thieves: 4 cells.
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

**Scout phase retreat:** During the scout decision phase, `combat_retreat` works differently:
- **Not engaged:** Automatic success. Thief returns to the previous room, combat ends.
- **Engaged:** Standard retreat roll. On success: thief returns, combat ends. On failure: party is automatically signaled and joins combat.

---

#### `signal_party`

After a successful `scout_ahead` surprise round, signal the rest of the party to join combat. Only usable during the scout decision phase.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the scouting thief |

**Mechanics:**
1. All party members move to the scout's room
2. Remaining party members are placed on the combat grid (rows 0-1)
3. Initiative is re-rolled for all combatants
4. Scout phase ends; normal combat continues

**Errors:**
- Not in scout decision phase
- Character is not the scout

---

#### `combat_charge`

Fighter charges toward an enemy with double movement range and attacks with +2 bonus damage. Combines move + attack into one action.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the fighter character |
| `target_id` | string | Yes | ID of the enemy to charge |

**Mechanics:**
1. Must be a fresh turn — cannot have already moved or acted
2. Can only be used once per combat
3. Fighter moves up to double their movement range (6 cells) to a cell adjacent to the target
3. Attack is executed with +2 bonus damage on hit
4. If the charge kill triggers cleave (see below), the cleave attack fires automatically
5. Consumes both move and action

**Errors:**
- Character is not a fighter
- Already moved or acted this turn
- Target is too far (no adjacent cell within 6 cells of fighter)
- Target is dead or friendly

---

#### `combat_protect`

Fighter guards an adjacent ally. Monster attacks targeting that ally are redirected to the fighter for 1 round.

**Arguments:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `character_id` | string | Yes | ID of the fighter character |
| `target_id` | string | Yes | ID of the ally to protect |

**Mechanics:**
- Sets `protectedBy` on the target combatant to the fighter's ID
- Protection lasts until the start of the next round
- When a monster attacks the protected ally, damage is redirected to the fighter (if the fighter is still alive and adjacent)
- Consumes the fighter's action

**Errors:**
- Character is not a fighter
- Already acted this turn
- Target is self, dead, or an enemy
- Fighter is not adjacent to the target ally

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
  party?: PartyView;               // omitted when null
  currentRoom?: RoomView;          // omitted when null
  monsters?: MonsterView[];        // omitted when empty
  roomItems?: ItemView[];          // omitted when empty
  roomTraps?: TrapView[];          // discovered traps + unopened chests; omitted when empty
  combat?: CombatView;             // omitted when not in combat
  gameOver: boolean;
  victory: boolean;
  turnNumber: number;
  message?: string;                // omitted when empty
  lastEvent?: GameEvent;           // omitted when null
}
```

**Note:** Fields marked `?` use `omitempty` in JSON serialization — they are absent from the response when null/empty, not present as `null`.

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
  knownSpells?: string[];          // omitted for non-magic_user; present for casters
  ac: number;                      // effective AC: 10 + armor bonus + active buff bonuses
  isAlive: boolean;
  status: "Healthy" | "Wounded" | "Critical" | "Dead";
  inventory: ItemView[];           // all items carried by this character
}
```

**Status Thresholds:**
- `Healthy`: HP > 75% of max
- `Wounded`: HP > 40% and <= 75% of max
- `Critical`: HP <= 40% of max (and alive)
- `Dead`: HP <= 0

---

### Class Stats

| Class | HP | STR | DEX | INT | Move | Init Bonus |
|-------|-----|-----|-----|-----|------|------------|
| fighter | 30 | 14 | 10 | 8 | 3 | +0 |
| magic_user | 16 | 8 | 10 | 16 | 2 | +0 |
| thief | 22 | 10 | 14 | 10 | 4 | +2 |

**HP Progression:** All alive characters gain +2 MaxHP (and +2 current HP) on each combat victory. Base HP values above reflect starting stats only.

**Special abilities:**
- **Fighter:** Highest HP and STR. Best melee damage. **Charge** (double move + attack with +2 damage). **Cleave** (free melee attack on adjacent enemy when a kill is scored). **Protect** (redirect attacks on an adjacent ally to the fighter for 1 round).
- **Magic User:** Spell slots (INT / 4). Starts with heal + fireball. Can learn spells from scrolls. +6 scroll bonus. **Cantrip** (free ranged attack, no spell slot cost). Recovers 1 spell slot on combat victory.
- **Thief:** +4 movement. +2 initiative. +6 trap disarm. +4 retreat. Can sneak/scout/scout_ahead. Can hide in combat.

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

Discovered traps and unopened chests appear in the snapshot.

```typescript
interface TrapView {
  id: string;
  description: string;
  location: "room" | "chest";
  isTriggered: boolean;            // true if the trap has already been triggered
  isDisarmed: boolean;
  isOpened: boolean;               // true once chest has been opened (loot revealed)
  difficulty: number;              // DC for disarm attempt (0 for untrapped chests)
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
  awaitingScoutDecision: boolean;  // true when scout must choose signal_party or combat_retreat
  isScoutPhase: boolean;           // true during thief's surprise round (monsters frozen)
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
  movementRange: number;           // max cells per turn (Chebyshev distance)
  attackRange: number;             // max attack distance (Chebyshev distance)
  knownSpells?: string[];          // spell IDs for magic users, omitted for others
  protectedBy?: string;            // fighter ID protecting this combatant (empty if none)
  hasCharged: boolean;             // true if fighter has used charge this combat
}
```

**`movementRange` values:**
| Source | Value |
|--------|-------|
| Fighter | 3 |
| Magic User | 2 |
| Thief | 4 (8 during scout phase) |
| All monsters | 2 |

**`attackRange` values:**
| Source | Value |
|--------|-------|
| Melee character (no ranged weapon) | 1 |
| Character with ranged weapon | weapon's `maxRange` |
| Melee monster | 1 |
| Ranged monster | monster's `attackRange` |

**`knownSpells`:** Present only for magic users. Contains spell IDs (e.g. `"heal"`, `"fireball"`). Omitted from JSON for non-casters.

**`protectedBy`:** When set, contains the fighter combatant ID who is protecting this combatant. Monster attacks on a protected combatant are redirected to the protector. Cleared at the start of each round. Omitted from JSON when empty.

---

### GameEvent

**Note:** GameEvent and EventDetails use **snake_case** JSON keys (unlike the rest of the snapshot which uses camelCase).

```typescript
interface GameEvent {
  session_id: string;
  turn_number: number;
  event_type: string;
  event_subtype: string;
  actor_id: string;
  actor_class: string;
  target_id: string;
  room_id: string;
  details: EventDetails;
}

interface EventDetails {
  roll?: number;
  dc?: number;
  damage?: number;
  was_critical?: boolean;
  was_flanking?: boolean;
  weapon_used?: string;
  party_hp?: Record<string, number>;
  party_alive?: number;
  actor_pos?: { x: number; y: number };
  target_pos?: { x: number; y: number };
  item_name?: string;
  scroll_effect?: string;
  success?: boolean;
  trap_difficulty?: number;
  was_detected?: boolean;
  result_text?: string;
}
```

**Event Types:**

| Type | Subtypes |
|------|----------|
| `combat` | `attack_hit`, `attack_miss`, `critical_hit`, `flanking_attack`, `monster_attack_hit`, `monster_attack_miss`, `combat_victory`, `retreat_success`, `retreat_fail`, `ambush`, `charge_hit`, `charge_miss`, `cleave`, `protect` |
| `spell` | `spell_cast`, `combat_spell_cast` |
| `item` | `consumable_used`, `scroll_success`, `scroll_fail`, `scroll_backfire` |
| `trap` | `trap_detected`, `trap_triggered`, `trap_disarmed`, `disarm_failed` |
| `stealth` | `sneak_success`, `sneak_fail`, `scout_success`, `scout_fail`, `scout_ahead_success`, `scout_ahead_fail`, `hide_success`, `hide_fail` |
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

**Spell Slots:** Magic users get INT / 4 spell slots. Recharged by resting. Recover 1 spell slot on combat victory.

**Combat Victory Rewards:** All alive characters gain +2 MaxHP and +2 HP. Magic users recover 1 spell slot.

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
- Movement: Chebyshev distance (diagonal moves cost 1).
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

### Fighter Abilities

**Charge (`combat_charge`):**
- Once per combat
- Requires fresh turn (no prior move or action)
- Moves up to double movement range (6 cells) to a cell adjacent to the target
- Attacks with +2 bonus damage
- Consumes both move and action

**Cleave (passive):**
- Triggers when a fighter kills an enemy with a melee attack (via `combat_attack` or `combat_charge`)
- Automatically attacks one alive enemy adjacent to the killed target's position
- No chain-cleaving — only one cleave per kill

**Protect (`combat_protect`):**
- Fighter must be adjacent to the ally
- Sets `protectedBy` on the ally for 1 round
- Monster attacks on the protected ally are redirected to the fighter
- Redirect requires the fighter to still be alive and adjacent when the attack lands
- Cleared at the start of each new round

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
          "ac": 10,
          "isAlive": true,
          "status": "Healthy",
          "inventory": []
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
          "maxSpellSlots": 4,
          "knownSpells": ["heal", "fireball"],
          "ac": 10,
          "isAlive": true,
          "status": "Healthy",
          "inventory": []
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
      "isActive": true,
      "awaitingScoutDecision": false,
      "isScoutPhase": false
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
