# Yrpg-backend Progress

Reference plan: `../Yrpg-backend-plan.md`

## Project Status: Code Complete — Tested and Working

Last updated: 2026-02-24

---

## Phase 1: Foundation — COMPLETE

All files created, compiles clean.

| File | Description | Status |
|------|-------------|--------|
| `go.mod` | Module def, deps (gorilla/mux, go-sqlite3) | Done |
| `internal/game/types.go` | All domain types (Character, Room, Monster, Item, Trap, CombatState, etc.) | Done |
| `internal/game/character.go` | ClassStatsTable, NewCharacter, TakeDamage, Heal, Resurrect, class abilities | Done |
| `internal/game/party.go` | NewParty (1-3 chars), formation, GetByClass, IsWiped, MoveAllToRoom | Done |
| `internal/game/state.go` | GameState with RWMutex, all indexes (MonstersByRoom, ItemsByRoom, etc.), RenderMap | Done |
| `internal/game/views.go` | View types + BuildSnapshot() for AI-readable game state | Done |
| `internal/db/schema.sql` | Tables: sessions, session_characters, events, saved_games + indexes | Done |
| `internal/db/sqlite.go` | DB wrapper, embedded schema, WAL mode, foreign keys | Done |

---

## Phase 2: World Generation — COMPLETE

| File | Description | Status |
|------|-------------|--------|
| `internal/generator/procgen.go` | Prim's spanning tree dungeon gen, 14 monster templates, 22 item templates (weapons/armor/consumables/scrolls), 9 trap templates, difficulty scaling, weighted item selection | Done |

---

## Phase 3: Exploration & MCP Server — COMPLETE

| File | Description | Status |
|------|-------------|--------|
| `internal/mcp/server.go` | MCP server core, tool registry, ListTools (22 tools), CallTool dispatch, logEvent + incrementStat helpers | Done |
| `internal/mcp/tools.go` | All exploration handlers: new_game, look, move, take, equip, use_item, open_chest, disarm_trap, sneak, inventory, stats, map, save_game, load_game, finalizeSession | Done |
| `internal/game/traps.go` | CheckTrapDetection, TriggerTrap, AttemptDisarm, format helpers | Done |
| `internal/game/sneak.go` | CheckRoomSneak, AttemptCombatHide, format helpers | Done |
| `cmd/server/main.go` | HTTP server, CORS middleware, /health, /mcp/tools, /mcp/call | Done |

---

## Phase 4: Combat System — COMPLETE

| File | Description | Status |
|------|-------------|--------|
| `internal/game/combat_state.go` | InitCombat (6x6 grid), initiative, turn management, surprise round, buff system (AddBuff, TickBuffs, GetACBonus, IsSleeping, RemoveSleep) | Done |
| `internal/game/combat.go` | CombatAttack (melee/ranged, flanking, crits, buff AC), CombatMove, AttemptRetreat, engagement system, sleep-on-damage wake | Done |
| `internal/game/monster_ai.go` | ExecuteMonsterTurn, greedy pathfinding, ranged/melee targeting, sleep skip | Done |
| `internal/mcp/combat_tools.go` | All combat handlers + sleeping player auto-skip, stat accumulation | Done |

All 6 scroll effects now fully functional in combat:
- Heal: 2d6+4 HP to target
- Resurrect: revive dead at half HP
- Fireball: 3d6 to target + half splash to adjacent enemies
- Lightning: 4d6 single target
- Shield: +4 AC buff for 3 rounds (buff system)
- Sleep: target skips 2 turns, damage wakes (buff system)

---

## Phase 5: Persistence & Analytics — COMPLETE

| File | Description | Status |
|------|-------------|--------|
| `internal/db/sessions.go` | CreateSession, EndSession, UpdateSessionStats, GetSession, ListSessions | Done |
| `internal/db/events.go` | LogEvent (JSON details), CreateSessionCharacter, FinalizeSessionCharacter | Done |
| `internal/db/saves.go` | SaveGame (upsert), LoadGame, LoadGameBySession, ListSaves, DeleteSave | Done |
| `internal/game/save.go` | SerializeState (JSON + RLock), DeserializeState (index rebuilding) | Done |
| `internal/analytics/export.go` | SessionSummaries, ClassPerformance, OutcomesByPartyComposition, ScrollStats, TrapStats, FlankingStats | Done |

Event logging instrumented across: game_start, room_enter, victory, trap_detected, trap_triggered, sneak checks, scroll success/fail/backfire, consumable use, combat attacks (hit/miss/flanking/critical), enemy_defeated, character_killed, party_wipe, combat_victory, combat hide, retreat.

---

## Phase 6: Polish & API Docs — NOT STARTED

- Error handling is present throughout (graceful failures, input validation)
- No formal API documentation
- No balance tuning config

---

## Manual Test Results (2026-02-24)

All 22 MCP tools verified working via curl against running server:

| Feature | Result |
|---------|--------|
| `/health` | `{"status": "healthy"}` |
| `new_game` (3 chars) | Party created, placed at entrance, session persisted to DB |
| `look` | Room description, exits, items, traps displayed correctly |
| `take` / `equip` | Item pickup, equip with class restrictions enforced |
| `sneak` (scout) | Thief scouts ahead — reveals enemies, items, trap count |
| `move` (room entry) | Auto sneak check, trap detection, combat entry sequence |
| `disarm_trap` | Thief disarms with +6 bonus (Roll: 32 vs DC 13) |
| `map` | ASCII dungeon map with explored/unexplored/adjacent cells |
| `save_game` / `load_game` | Full state roundtrip, party restored correctly |
| Combat: move, attack | Grid movement, melee attacks, hidden advantage |
| Combat: defend | +4 AC stance applied |
| Combat: flanking | Advantage triggered when attacking engaged target |
| Monster AI | Monsters attack players, damage synced to characters |
| Combat end | Auto-transition back to exploration after all enemies dead |
| Event logging | 18 events across 12 types recorded in SQLite |
| Stat accumulation | Per-character kills, damage, sneaks tracked correctly |

---

## Remaining Items (nice-to-have)

### 1. Potion combat effects — LOW PRIORITY
- Strength/Dexterity/Defense potions exist as items but don't apply combat buffs
- The buff system is in place — just needs potion-specific handlers in `useConsumable`

### 2. Phase 6: Polish & API Docs
- Formal API documentation (OpenAPI spec or markdown)
- Balance tuning config (currently hardcoded but reasonable)
- Docker compose for development

### 3. Unit tests
- No tests exist yet for any package
- Priority areas: combat mechanics, trap detection, scroll effects, save/load roundtrip

---

## File Inventory

```
Yrpg-backend/
├── cmd/server/main.go                    # HTTP server entry point
├── go.mod / go.sum                       # Go module + deps
├── internal/
│   ├── analytics/export.go               # RAG export queries
│   ├── db/
│   │   ├── schema.sql                    # Embedded DB schema
│   │   ├── sqlite.go                     # DB wrapper
│   │   ├── sessions.go                   # Session CRUD
│   │   ├── events.go                     # Event logging
│   │   └── saves.go                      # Save/load persistence
│   ├── game/
│   │   ├── types.go                      # Domain types
│   │   ├── character.go                  # Character creation + methods
│   │   ├── party.go                      # Party management
│   │   ├── state.go                      # GameState + indexes
│   │   ├── views.go                      # Snapshot builder
│   │   ├── traps.go                      # Trap mechanics
│   │   ├── sneak.go                      # Sneak mechanics
│   │   ├── combat_state.go              # Combat grid + turns
│   │   ├── combat.go                     # Attack/damage/flanking
│   │   ├── monster_ai.go                # Monster AI
│   │   └── save.go                       # State serialization
│   ├── generator/procgen.go             # Dungeon generation
│   └── mcp/
│       ├── server.go                     # MCP dispatch + tool registry
│       ├── tools.go                      # Exploration + utility handlers
│       └── combat_tools.go              # Combat handlers
```

## Architecture Notes

- **MCP pattern:** Tools defined with JSON schemas in `ListTools()`, dispatched via `CallTool()` switch
- **State protection:** GameState uses `sync.RWMutex` for concurrent safety
- **O(1) lookups:** MonstersByRoom, ItemsByRoom, ItemsByChar, TrapsByRoom, RoomsByCoord indexes
- **Buff system:** Buff type on Combatant with rounds tracking, ticked in AdvanceTurn, applied in AC calculations, sleep skips turns
- **Combat grid:** 6x6, party starts row 0, enemies row 5, Manhattan distance for movement, Chebyshev for adjacency
- **d20 mechanics:** Attacks, trap detection, sneak, scroll casting all use d20 + modifiers vs DC
- **Event logging:** Fire-and-forget via `logEvent()`, structured JSON details, doesn't block gameplay
- **Seeded RNG:** Dungeon generation uses seeded rand for reproducibility; gameplay uses time-seeded rand
