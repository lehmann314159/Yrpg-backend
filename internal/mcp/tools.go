package mcp

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/lehmann314159/yrpg-backend/internal/game"
	"github.com/lehmann314159/yrpg-backend/internal/generator"
)

// handleNewGame starts a new game with a party
func (s *Server) handleNewGame(arguments map[string]interface{}) (*ToolResult, error) {
	// Parse character list
	charsRaw, ok := arguments["characters"].([]interface{})
	if !ok || len(charsRaw) == 0 {
		return s.errorResult("Provide 1-3 characters: [{name, class}]. Classes: fighter, magic_user, thief."), nil
	}

	requests := make([]game.PartyRequest, 0, len(charsRaw))
	for _, raw := range charsRaw {
		charMap, ok := raw.(map[string]interface{})
		if !ok {
			return s.errorResult("Each character must be an object with 'name' and 'class' fields."), nil
		}
		name, _ := charMap["name"].(string)
		class, _ := charMap["class"].(string)
		if name == "" || class == "" {
			return s.errorResult("Each character must have a 'name' and 'class'."), nil
		}
		requests = append(requests, game.PartyRequest{
			Name:  name,
			Class: game.CharacterClass(class),
		})
	}

	// Create party
	party, err := game.NewParty(requests)
	if err != nil {
		return s.errorResult(fmt.Sprintf("Failed to create party: %v", err)), nil
	}

	// Generate dungeon
	seed := time.Now().UnixNano()
	s.rng = rand.New(rand.NewSource(seed))
	gen := generator.NewDungeonGenerator(seed)

	sessionID := fmt.Sprintf("%x", seed)
	s.state = game.NewGameState(sessionID)
	s.state.Party = party
	s.state.Dungeon = &game.Dungeon{Seed: seed, Depth: 1}

	dungeon, rooms, connections, monsters, items, traps := gen.GenerateDungeon(1)
	s.state.Dungeon = dungeon

	// Add everything to game state
	for _, room := range rooms {
		s.state.AddRoom(room)
	}
	for _, conn := range connections {
		s.state.AddConnection(conn)
	}
	for _, m := range monsters {
		s.state.AddMonster(m)
	}
	for _, item := range items {
		s.state.AddItem(item)
	}
	for _, trap := range traps {
		s.state.AddTrap(trap)
	}

	// Place party at entrance
	for _, room := range rooms {
		if room.IsEntrance {
			party.MoveAllToRoom(room.ID)
			s.state.MarkRoomVisited(room.ID)
			break
		}
	}

	s.state.ResetTurnContext()

	// Persist session and characters to DB
	if s.db != nil {
		s.db.CreateSession(sessionID, seed, 1)
		for _, c := range party.Characters {
			s.db.CreateSessionCharacter(c.ID, sessionID, c.Name, string(c.Class))
		}
	}
	s.logEvent("interaction", "game_start", "", "", "", map[string]interface{}{
		"party_size": len(party.Characters),
		"seed":       seed,
	})

	// Build response
	var sb strings.Builder
	sb.WriteString("=== NEW GAME STARTED ===\n\n")
	sb.WriteString("Your party:\n")
	for _, c := range party.Characters {
		stats := game.ClassStatsTable[c.Class]
		sb.WriteString(fmt.Sprintf("  %s the %s — HP: %d, STR: %d, DEX: %d, INT: %d, Move: %d\n",
			c.Name, c.Class, c.HP, c.Strength, c.Dexterity, c.Intelligence, stats.MovementRange))
		if c.Class == game.ClassMagicUser && c.MaxSpellSlots > 0 {
			sb.WriteString(fmt.Sprintf("    Spell Slots: %d/%d, Spells: %s\n",
				c.SpellSlots, c.MaxSpellSlots, strings.Join(c.KnownSpells, ", ")))
		}
	}
	sb.WriteString(fmt.Sprintf("\nFormation (front to back): %s\n",
		formatFormation(party)))
	sb.WriteString("\nYou stand at the entrance of a dark dungeon.\n")
	sb.WriteString("Your goal: navigate the dungeon and reach the exit.\n")
	sb.WriteString("Use 'look' to examine your surroundings.")

	return s.textResult(sb.String()), nil
}

// handleLook examines the current room
func (s *Server) handleLook() (*ToolResult, error) {
	if err := s.requireActiveGame(); err != nil {
		return err, nil
	}

	room := s.state.GetCurrentRoom()
	if room == nil {
		return s.errorResult("Current room not found."), nil
	}

	s.state.ResetTurnContext()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== %s ===\n\n", room.Name))
	sb.WriteString(fmt.Sprintf("%s\n\n", room.Description))

	if room.IsEntrance {
		sb.WriteString("[Dungeon Entrance]\n\n")
	}
	if room.IsExit {
		sb.WriteString("[Dungeon Exit — reach here to win!]\n\n")
	}

	// Exits
	exits := s.state.GetRoomExits(room.ID)
	if len(exits) > 0 {
		dirs := make([]string, 0, len(exits))
		for dir := range exits {
			dirs = append(dirs, dir)
		}
		sb.WriteString(fmt.Sprintf("Exits: %s\n\n", strings.Join(dirs, ", ")))
	}

	// Monsters
	monsters := s.state.GetRoomMonsters(room.ID)
	if len(monsters) > 0 {
		sb.WriteString("Enemies:\n")
		for _, m := range monsters {
			rangedTag := ""
			if m.IsRanged {
				rangedTag = " (ranged)"
			}
			sb.WriteString(fmt.Sprintf("  - %s%s — HP: %d/%d [ID: %s]\n    %s\n",
				m.Name, rangedTag, m.HP, m.MaxHP, m.ID, m.Description))
		}
		sb.WriteString("\n")
	}

	// Items on floor
	items := s.state.GetRoomItems(room.ID)
	if len(items) > 0 {
		sb.WriteString("Items:\n")
		for _, item := range items {
			sb.WriteString(fmt.Sprintf("  - %s (%s, %s) [ID: %s]\n    %s\n",
				item.Name, item.Type, item.Rarity, item.ID, item.Description))
		}
		sb.WriteString("\n")
	}

	// Unopened chests
	traps := s.state.GetRoomTraps(room.ID)
	chestCount := 0
	for _, trap := range traps {
		if trap.Location == game.TrapChest && !trap.IsOpened {
			chestCount++
		}
	}
	if chestCount == 1 {
		sb.WriteString("A chest sits in the room. Use 'open_chest' to open it.\n\n")
	} else if chestCount > 1 {
		sb.WriteString(fmt.Sprintf("%d chests sit in the room. Use 'open_chest' to open them.\n\n", chestCount))
	}

	// Discovered traps
	for _, trap := range traps {
		if trap.IsDiscovered && !trap.IsTriggered && !trap.IsDisarmed {
			sb.WriteString(fmt.Sprintf("WARNING — Trap detected: %s (DC %d) [ID: %s]\n",
				trap.Description, trap.Difficulty, trap.ID))
		}
	}

	if len(monsters) > 0 {
		sb.WriteString("\nEnemies block your path! Defeat them to proceed.\n")
	}

	return s.textResult(sb.String()), nil
}

// handleMove moves the party to an adjacent room
func (s *Server) handleMove(direction string) (*ToolResult, error) {
	if err := s.requireExploration(); err != nil {
		return err, nil
	}

	currentRoom := s.state.GetCurrentRoom()
	if currentRoom == nil {
		return s.errorResult("Current room not found."), nil
	}

	// Can't leave if monsters are alive
	if s.state.HasMonstersInRoom(currentRoom.ID) {
		return s.textResult("Cannot leave — enemies are still in the room! Defeat them first."), nil
	}

	// Find exit
	exits := s.state.GetRoomExits(currentRoom.ID)
	newRoomID, ok := exits[direction]
	if !ok {
		return s.textResult(fmt.Sprintf("Cannot move %s — no exit in that direction.", direction)), nil
	}

	previousRoomID := currentRoom.ID
	s.state.ResetTurnContext()

	newRoom := s.state.Rooms[newRoomID]
	isFirstVisit := !s.state.IsRoomVisited(newRoomID)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("You move %s...\n\n", direction))
	sb.WriteString(fmt.Sprintf("=== %s ===\n\n", newRoom.Name))
	sb.WriteString(fmt.Sprintf("%s\n", newRoom.Description))

	// On first visit with enemies, thief may scout ahead before the party enters
	if isFirstVisit {
		monsters := s.state.GetRoomMonsters(newRoomID)
		thief := s.state.Party.GetByClass(game.ClassThief)
		if len(monsters) > 0 && thief != nil && thief.IsAlive {
			difficulty := generator.GetRoomDifficulty(newRoom)
			sneakResult := game.CheckRoomSneak(thief, difficulty, s.rng)
			sb.WriteString(fmt.Sprintf("\n%s\n", FormatSneakResult(sneakResult)))
			subtype := "sneak_fail"
			if sneakResult.Success {
				subtype = "sneak_success"
				s.incrementStat(thief.ID, "successful_sneaks", 1)
			}
			s.logEvent("stealth", subtype, thief.ID, string(thief.Class), "", map[string]interface{}{
				"roll":       sneakResult.Roll,
				"dc":         sneakResult.DC,
				"difficulty": difficulty,
			})

			if sneakResult.Success {
				// Thief enters alone — scout combat
				thief.CurrentRoomID = newRoomID
				s.state.MarkRoomVisited(newRoomID)
				s.logEvent("movement", "room_enter", thief.ID, string(thief.Class), "", map[string]interface{}{
					"direction":   direction,
					"room_id":     newRoomID,
					"first_visit": true,
					"scout_entry": true,
				})

				// Solo trap detection
				trapText, thiefDied := s.detectTrapsSolo(thief, newRoomID)
				sb.WriteString(trapText)
				if thiefDied {
					// Thief died to trap — rest of party charges in
					sb.WriteString("\nThe party hears the trap and rushes in!\n")
					s.state.Party.MoveAllToRoom(newRoomID)
					combatMsg := s.enterCombat(previousRoomID, nil)
					sb.WriteString(combatMsg)
					return s.textResult(sb.String()), nil
				}

				// Init scout combat — only thief on the grid
				cs := game.InitScoutCombat(thief, monsters, previousRoomID, s.state.Items, s.rng)
				s.state.Combat = cs
				s.state.Mode = game.ModeCombat
				s.retreated = make(map[string]bool)

				sb.WriteString(fmt.Sprintf("\n=== SURPRISE ROUND ===\n"))
				sb.WriteString(fmt.Sprintf("%s enters alone! Enemies are unaware.\n\n", thief.Name))
				sb.WriteString(fmt.Sprintf("Enemies (%d):\n", len(monsters)))
				for _, m := range monsters {
					sb.WriteString(fmt.Sprintf("  - %s (HP: %d/%d)\n", m.Name, m.HP, m.MaxHP))
				}
				sb.WriteString(fmt.Sprintf("\n%s has a surprise round with double movement (%d cells)!\n",
					thief.Name, thief.GetMovementRange()*2))
				sb.WriteString("Use combat_move and combat_attack, then choose signal_party or combat_retreat.\n")

				return s.textResult(sb.String()), nil
			}
			// Sneak failed — fall through to normal party entry
		}
	}

	// Normal path: move whole party
	s.state.Party.MoveAllToRoom(newRoomID)
	if isFirstVisit {
		s.state.MarkRoomVisited(newRoomID)
	}

	s.logEvent("movement", "room_enter", "", "", "", map[string]interface{}{
		"direction":   direction,
		"room_id":     newRoomID,
		"first_visit": isFirstVisit,
	})

	// Check for victory
	if newRoom.IsExit {
		s.state.Victory = true
		s.state.GameOver = true
		s.logEvent("victory", "dungeon_escaped", "", "", "", nil)
		s.finalizeSession("victory")
		sb.WriteString("\nYou step through the exit and escape the dungeon!\n\nVICTORY!\n\nUse 'new_game' to play again.")
		return s.textResult(sb.String()), nil
	}

	// Room entry sequence (only on first visit)
	if isFirstVisit {
		// 1. Trap detection
		trapText, wiped := s.detectTrapsParty(newRoomID)
		sb.WriteString(trapText)
		if wiped {
			s.state.GameOver = true
			s.finalizeSession("party_wipe")
			sb.WriteString("\nYour entire party has fallen. Game over.\nUse 'new_game' to play again.")
			return s.textResult(sb.String()), nil
		}

		// 2. Check for enemies (sneak already failed or no thief — normal combat)
		monsters := s.state.GetRoomMonsters(newRoomID)
		if len(monsters) > 0 {
			combatMsg := s.enterCombat(previousRoomID, nil)
			sb.WriteString(combatMsg)
		}
	}

	// Re-entering a room with surviving monsters (e.g. after scout retreat)
	if !isFirstVisit {
		monsters := s.state.GetRoomMonsters(newRoomID)
		if len(monsters) > 0 {
			combatMsg := s.enterCombat(previousRoomID, nil)
			sb.WriteString(combatMsg)
		}
	}

	return s.textResult(sb.String()), nil
}

// handleSneak lets a thief scout an adjacent room without entering
func (s *Server) handleSneak(charID, direction string) (*ToolResult, error) {
	if err := s.requireExploration(); err != nil {
		return err, nil
	}

	char, errResult := s.resolveCharacter(charID)
	if errResult != nil {
		return errResult, nil
	}

	if char.Class != game.ClassThief {
		return s.textResult("Only a thief can scout ahead."), nil
	}

	currentRoom := s.state.GetCurrentRoom()
	if currentRoom == nil {
		return s.errorResult("Current room not found."), nil
	}

	exits := s.state.GetRoomExits(currentRoom.ID)
	targetRoomID, ok := exits[direction]
	if !ok {
		return s.textResult(fmt.Sprintf("Cannot scout %s — no exit in that direction.", direction)), nil
	}

	targetRoom := s.state.Rooms[targetRoomID]
	difficulty := generator.GetRoomDifficulty(targetRoom)
	sneakResult := game.CheckRoomSneak(char, difficulty, s.rng)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s creeps toward the %s exit...\n\n", char.Name, direction))
	sb.WriteString(fmt.Sprintf("%s\n\n", FormatSneakResult(sneakResult)))

	subtype := "scout_fail"
	if sneakResult.Success {
		subtype = "scout_success"
		s.incrementStat(charID, "successful_sneaks", 1)
	}
	s.logEvent("stealth", subtype, charID, string(char.Class), targetRoomID, map[string]interface{}{
		"roll": sneakResult.Roll, "dc": sneakResult.DC,
	})

	if sneakResult.Success {
		sb.WriteString(fmt.Sprintf("=== Scouting: %s ===\n", targetRoom.Name))
		sb.WriteString(fmt.Sprintf("%s\n\n", targetRoom.Description))

		// Report monsters
		monsters := s.state.GetRoomMonsters(targetRoomID)
		if len(monsters) > 0 {
			sb.WriteString(fmt.Sprintf("Enemies spotted (%d):\n", len(monsters)))
			for _, m := range monsters {
				sb.WriteString(fmt.Sprintf("  - %s (HP: %d)\n", m.Name, m.HP))
			}
		} else {
			sb.WriteString("No enemies detected.\n")
		}

		// Report visible traps (thief bonus detection)
		traps := s.state.GetRoomTraps(targetRoomID)
		trapCount := 0
		for _, trap := range traps {
			if !trap.IsTriggered && !trap.IsDisarmed {
				trapCount++
			}
		}
		if trapCount > 0 {
			sb.WriteString(fmt.Sprintf("\nWarning: %s senses %d potential trap(s) ahead.\n", char.Name, trapCount))
		}

		// Report items on floor
		items := s.state.GetRoomItems(targetRoomID)
		if len(items) > 0 {
			sb.WriteString(fmt.Sprintf("\nItems visible (%d):\n", len(items)))
			for _, item := range items {
				sb.WriteString(fmt.Sprintf("  - %s (%s)\n", item.Name, item.Rarity))
			}
		}
	} else {
		sb.WriteString("You couldn't get a clear look without being noticed. The room remains a mystery.")
	}

	return s.textResult(sb.String()), nil
}

// handleSetFormation reorders the party's marching order
func (s *Server) handleSetFormation(formation []string) (*ToolResult, error) {
	if err := s.requireExploration(); err != nil {
		return err, nil
	}

	if err := s.state.Party.SetFormation(formation); err != nil {
		return s.textResult(fmt.Sprintf("Invalid formation: %s", err.Error())), nil
	}

	var sb strings.Builder
	sb.WriteString("Formation updated!\n")
	sb.WriteString(fmt.Sprintf("Marching order (front to back): %s\n", formatFormation(s.state.Party)))
	return s.textResult(sb.String()), nil
}

// handleMap shows the dungeon map
func (s *Server) handleMap() (*ToolResult, error) {
	if err := s.requireInitialized(); err != nil {
		return err, nil
	}

	mapStr := s.state.RenderMap(gridSize())
	return s.textResult(mapStr), nil
}

// --- Trap Helpers ---

// detectTrapsSolo runs solo trap detection + chest detection for a thief entering a room.
// Returns (text, thiefDied).
func (s *Server) detectTrapsSolo(thief *game.Character, roomID string) (string, bool) {
	var sb strings.Builder
	roomTraps := s.state.GetRoomTraps(roomID)

	for _, trap := range roomTraps {
		if trap.Location == game.TrapRoom && !trap.IsTriggered && !trap.IsDisarmed {
			detection := game.CheckTrapDetectionSolo(thief, trap, s.rng)
			if detection.Detected {
				sb.WriteString(fmt.Sprintf("\n%s\n", FormatTrapDetection(detection)))
				s.logEvent("trap", "trap_detected", thief.ID, string(thief.Class), trap.ID, map[string]interface{}{
					"trap_type": trap.Description,
				})
			} else {
				trigger := game.TriggerTrap(trap, thief, s.rng)
				sb.WriteString(fmt.Sprintf("\n%s\n", FormatTrapTrigger(trigger)))
				s.logEvent("trap", "trap_triggered", thief.ID, string(thief.Class), trap.ID, map[string]interface{}{
					"trap_type": trap.Description,
					"damage":    trigger.Damage,
				})
				if !thief.IsAlive {
					s.logEvent("death", "character_killed", thief.ID, string(thief.Class), trap.ID, map[string]interface{}{
						"cause": "trap",
					})
					return sb.String(), true
				}
			}
		}
	}

	// Chest trap detection (passive only — chests don't trigger on entry)
	for _, trap := range roomTraps {
		if trap.Location == game.TrapChest && !trap.IsTriggered && !trap.IsDisarmed && trap.Damage > 0 {
			detection := game.CheckTrapDetectionSolo(thief, trap, s.rng)
			if detection.Detected {
				sb.WriteString(fmt.Sprintf("\n%s\n", FormatTrapDetection(detection)))
			}
		}
	}

	return sb.String(), false
}

// detectTrapsParty runs full party trap detection for room entry.
// Returns (text, partyWiped).
func (s *Server) detectTrapsParty(roomID string) (string, bool) {
	var sb strings.Builder
	roomTraps := s.state.GetRoomTraps(roomID)

	for _, trap := range roomTraps {
		if trap.Location == game.TrapRoom && !trap.IsTriggered && !trap.IsDisarmed {
			detection := game.CheckTrapDetection(s.state.Party, trap, s.rng)
			if detection.Detected {
				sb.WriteString(fmt.Sprintf("\n%s\n", FormatTrapDetection(detection)))
				detectorClass := ""
				if dc := s.state.Party.GetCharacter(detection.DetectorID); dc != nil {
					detectorClass = string(dc.Class)
				}
				s.logEvent("trap", "trap_detected", detection.DetectorID, detectorClass, trap.ID, map[string]interface{}{
					"trap_type": trap.Description,
				})

				victim := s.state.Party.FirstNonThief()
				if victim != nil {
					trigger := game.TriggerTrap(trap, victim, s.rng)
					sb.WriteString(fmt.Sprintf("\n%s\n", FormatTrapTrigger(trigger)))
					s.logEvent("trap", "trap_triggered", victim.ID, string(victim.Class), trap.ID, map[string]interface{}{
						"trap_type": trap.Description,
						"damage":    trigger.Damage,
					})
					if !victim.IsAlive {
						s.logEvent("death", "character_killed", victim.ID, string(victim.Class), trap.ID, map[string]interface{}{
							"cause": "trap",
						})
					}
					if s.state.Party.IsWiped() {
						return sb.String(), true
					}
				}
			} else {
				point := s.state.Party.PointCharacter()
				if point != nil {
					trigger := game.TriggerTrap(trap, point, s.rng)
					sb.WriteString(fmt.Sprintf("\n%s\n", FormatTrapTrigger(trigger)))
					s.logEvent("trap", "trap_triggered", point.ID, string(point.Class), trap.ID, map[string]interface{}{
						"trap_type": trap.Description,
						"damage":    trigger.Damage,
					})
					if !point.IsAlive {
						s.logEvent("death", "character_killed", point.ID, string(point.Class), trap.ID, map[string]interface{}{
							"cause": "trap",
						})
					}
					if s.state.Party.IsWiped() {
						return sb.String(), true
					}
				}
			}
		}
	}

	// Chest trap detection (passive only)
	for _, trap := range roomTraps {
		if trap.Location == game.TrapChest && !trap.IsTriggered && !trap.IsDisarmed && trap.Damage > 0 {
			detection := game.CheckTrapDetection(s.state.Party, trap, s.rng)
			if detection.Detected {
				sb.WriteString(fmt.Sprintf("\n%s\n", FormatTrapDetection(detection)))
			}
		}
	}

	return sb.String(), false
}

// triggerDiscoveredTraps triggers known-but-not-disarmed room traps on the party.
// Returns (text, partyWiped). Used by handleSignalParty.
func (s *Server) triggerDiscoveredTraps(roomID string) (string, bool) {
	var sb strings.Builder
	roomTraps := s.state.GetRoomTraps(roomID)

	for _, trap := range roomTraps {
		if trap.Location == game.TrapRoom && trap.IsDiscovered && !trap.IsTriggered && !trap.IsDisarmed {
			victim := s.state.Party.FirstNonThief()
			if victim != nil {
				trigger := game.TriggerTrap(trap, victim, s.rng)
				sb.WriteString(fmt.Sprintf("%s\n", FormatTrapTrigger(trigger)))
				if !victim.IsAlive {
					s.logEvent("death", "character_killed", victim.ID, string(victim.Class), trap.ID, map[string]interface{}{
						"cause": "trap",
					})
				}
				if s.state.Party.IsWiped() {
					return sb.String(), true
				}
			}
		}
	}

	return sb.String(), false
}
