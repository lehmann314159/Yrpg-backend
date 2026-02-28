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
		return s.errorResult("Error: current room not found."), nil
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
		return s.errorResult("Error: current room not found."), nil
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

	// Move party
	s.state.Party.MoveAllToRoom(newRoomID)
	isFirstVisit := !s.state.IsRoomVisited(newRoomID)
	s.state.MarkRoomVisited(newRoomID)

	s.logEvent("movement", "room_enter", "", "", "", map[string]interface{}{
		"direction":    direction,
		"room_id":      newRoomID,
		"first_visit":  isFirstVisit,
	})

	newRoom := s.state.Rooms[newRoomID]
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("You move %s...\n\n", direction))
	sb.WriteString(fmt.Sprintf("=== %s ===\n\n", newRoom.Name))
	sb.WriteString(fmt.Sprintf("%s\n", newRoom.Description))

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
		roomTraps := s.state.GetRoomTraps(newRoomID)
		for _, trap := range roomTraps {
			if trap.Location == game.TrapRoom && !trap.IsTriggered && !trap.IsDisarmed {
				detection := game.CheckTrapDetection(s.state.Party, trap, s.rng)
				if detection.Detected {
					sb.WriteString(fmt.Sprintf("\n%s\n", game.FormatTrapDetection(detection)))
					detectorClass := ""
					if dc := s.state.Party.GetCharacter(detection.DetectorID); dc != nil {
						detectorClass = string(dc.Class)
					}
					s.logEvent("trap", "trap_detected", detection.DetectorID, detectorClass, trap.ID, map[string]interface{}{
						"trap_type": trap.Description,
					})

					// Detected trap still triggers on first non-thief
					victim := s.state.Party.FirstNonThief()
					if victim != nil {
						trigger := game.TriggerTrap(trap, victim, s.rng)
						sb.WriteString(fmt.Sprintf("\n%s\n", game.FormatTrapTrigger(trigger)))
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
							s.state.GameOver = true
							s.finalizeSession("party_wipe")
							sb.WriteString("\nYour entire party has fallen. Game over.\nUse 'new_game' to play again.")
							return s.textResult(sb.String()), nil
						}
					}
				} else {
					// Trap triggers on point character
					point := s.state.Party.PointCharacter()
					if point != nil {
						trigger := game.TriggerTrap(trap, point, s.rng)
						sb.WriteString(fmt.Sprintf("\n%s\n", game.FormatTrapTrigger(trigger)))
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
							s.state.GameOver = true
							s.finalizeSession("party_wipe")
							sb.WriteString("\nYour entire party has fallen. Game over.\nUse 'new_game' to play again.")
							return s.textResult(sb.String()), nil
						}
					}
				}
			}
		}

		// 2. Check for enemies
		monsters := s.state.GetRoomMonsters(newRoomID)
		if len(monsters) > 0 {
			// 3. Sneak check (if party has a thief)
			thief := s.state.Party.GetByClass(game.ClassThief)
			sneakResult := (*game.SneakResult)(nil)
			if thief != nil {
				difficulty := generator.GetRoomDifficulty(newRoom)
				sneakResult = game.CheckRoomSneak(thief, difficulty, s.rng)
				sb.WriteString(fmt.Sprintf("\n%s\n", game.FormatSneakResult(sneakResult)))
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
			}

			// Transition to combat
			combatMsg := s.enterCombat(previousRoomID, sneakResult)
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

// handleTake picks up an item for a character
func (s *Server) handleTake(charID, itemID string) (*ToolResult, error) {
	if err := s.requireExploration(); err != nil {
		return err, nil
	}

	char, errResult := s.resolveCharacter(charID)
	if errResult != nil {
		return errResult, nil
	}

	item, ok := s.state.Items[itemID]
	if !ok {
		return s.textResult("Item not found. Use 'look' to see available items."), nil
	}

	currentRoom := s.state.GetCurrentRoom()
	if currentRoom == nil {
		return s.errorResult("Error: current room not found."), nil
	}

	// Check item is in current room
	if item.RoomID == nil || *item.RoomID != currentRoom.ID {
		return s.textResult("That item is not in this room."), nil
	}
	if item.CharacterID != nil {
		return s.textResult("That item is already being carried."), nil
	}

	// Move item: remove from room index, add to character index
	if s.state.ItemsByRoom[currentRoom.ID] != nil {
		delete(s.state.ItemsByRoom[currentRoom.ID], itemID)
	}
	if s.state.ItemsByChar[char.ID] == nil {
		s.state.ItemsByChar[char.ID] = make(map[string]bool)
	}
	s.state.ItemsByChar[char.ID][itemID] = true

	item.RoomID = nil
	item.CharacterID = &char.ID

	return s.textResult(fmt.Sprintf("%s picks up the %s.", char.Name, item.Name)), nil
}

// handleEquip equips a weapon or armor for a character
func (s *Server) handleEquip(charID, itemID string) (*ToolResult, error) {
	if err := s.requireExploration(); err != nil {
		return err, nil
	}

	char, errResult := s.resolveCharacter(charID)
	if errResult != nil {
		return errResult, nil
	}

	item, ok := s.state.Items[itemID]
	if !ok {
		return s.textResult("Item not found. Use 'inventory' to see your items."), nil
	}

	// Check item is in this character's inventory
	if item.CharacterID == nil || *item.CharacterID != char.ID {
		return s.textResult("That item is not in this character's inventory."), nil
	}

	// Check class restriction
	if !char.CanEquip(item) {
		restrictions := make([]string, len(item.ClassRestriction))
		for i, c := range item.ClassRestriction {
			restrictions[i] = string(c)
		}
		return s.textResult(fmt.Sprintf("%s cannot equip %s. Allowed classes: %s.",
			char.Name, item.Name, strings.Join(restrictions, ", "))), nil
	}

	var sb strings.Builder

	switch item.Type {
	case game.ItemWeapon:
		if char.EquippedWeaponID != nil {
			old := s.state.Items[*char.EquippedWeaponID]
			if old != nil {
				old.IsEquipped = false
				sb.WriteString(fmt.Sprintf("%s unequips the %s.\n", char.Name, old.Name))
			}
		}
		char.EquippedWeaponID = &item.ID
		item.IsEquipped = true
		sb.WriteString(fmt.Sprintf("%s equips the %s. (Damage +%d", char.Name, item.Name, item.Damage))
		if item.Range == game.RangeRanged {
			sb.WriteString(fmt.Sprintf(", Range: %d cells", item.MaxRange))
		}
		sb.WriteString(")")

	case game.ItemArmor:
		if char.EquippedArmorID != nil {
			old := s.state.Items[*char.EquippedArmorID]
			if old != nil {
				old.IsEquipped = false
				sb.WriteString(fmt.Sprintf("%s unequips the %s.\n", char.Name, old.Name))
			}
		}
		char.EquippedArmorID = &item.ID
		item.IsEquipped = true
		sb.WriteString(fmt.Sprintf("%s equips the %s. (Armor +%d)", char.Name, item.Name, item.Armor))

	default:
		return s.textResult(fmt.Sprintf("Cannot equip %s — it's not a weapon or armor.", item.Name)), nil
	}

	return s.textResult(sb.String()), nil
}

// handleUseItem uses a consumable or scroll
func (s *Server) handleUseItem(charID, itemID, targetID string) (*ToolResult, error) {
	if err := s.requireActiveGame(); err != nil {
		return err, nil
	}

	char, errResult := s.resolveCharacter(charID)
	if errResult != nil {
		return errResult, nil
	}

	item, ok := s.state.Items[itemID]
	if !ok {
		return s.textResult("Item not found."), nil
	}

	if item.CharacterID == nil || *item.CharacterID != char.ID {
		return s.textResult("That item is not in this character's inventory."), nil
	}

	switch item.Type {
	case game.ItemConsumable:
		return s.useConsumable(char, item)
	case game.ItemScroll:
		return s.useScroll(char, item, targetID)
	default:
		return s.textResult(fmt.Sprintf("Cannot use %s — it's not a consumable or scroll.", item.Name)), nil
	}
}

// useConsumable applies a consumable item's effects
func (s *Server) useConsumable(char *game.Character, item *game.Item) (*ToolResult, error) {
	var msg string

	if item.Healing > 0 {
		oldHP := char.HP
		char.Heal(item.Healing)
		healed := char.HP - oldHP
		msg = fmt.Sprintf("%s uses the %s and recovers %d HP! (HP: %d/%d)",
			char.Name, item.Name, healed, char.HP, char.MaxHP)
	} else {
		msg = fmt.Sprintf("%s uses the %s.", char.Name, item.Name)
	}

	// Remove from inventory
	s.removeItemFromInventory(char.ID, item.ID)

	s.logEvent("item", "consumable_used", char.ID, string(char.Class), "", map[string]interface{}{
		"item": item.Name, "healing": item.Healing,
	})
	s.incrementStat(char.ID, "items_used", 1)

	return s.textResult(msg), nil
}

// useScroll attempts to cast a scroll
func (s *Server) useScroll(caster *game.Character, item *game.Item, targetID string) (*ToolResult, error) {
	// Roll: d20 + intelligence/2 vs scroll difficulty
	roll := s.rng.Intn(20) + 1
	bonus := caster.Intelligence / 2

	// Class modifiers
	switch caster.Class {
	case game.ClassMagicUser:
		bonus += 6
	case game.ClassFighter:
		bonus -= 2
	}

	total := roll + bonus
	dc := item.ScrollDifficulty

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s attempts to read the %s... (Roll: %d+%d=%d vs DC %d)\n",
		caster.Name, item.Name, roll, bonus, total, dc))

	if total >= dc {
		// Success — apply effect
		sb.WriteString(s.applyScrollEffect(caster, item, targetID))

		// Magic users permanently learn spells from successfully cast scrolls
		if caster.Class == game.ClassMagicUser && item.ScrollEffect != "" {
			if game.LearnSpell(caster, item.ScrollEffect) {
				spellDef := game.SpellRegistry[item.ScrollEffect]
				spellName := item.ScrollEffect
				if spellDef != nil {
					spellName = spellDef.Name
				}
				sb.WriteString(fmt.Sprintf("\n%s learned %s!", caster.Name, spellName))
			}
		}

		s.logEvent("item", "scroll_success", caster.ID, string(caster.Class), "", map[string]interface{}{
			"scroll": item.ScrollEffect, "roll": roll, "dc": dc,
		})
	} else if dc-total < 5 {
		// Failure (miss by < 5): consumed, no effect
		sb.WriteString("The scroll's magic fizzles harmlessly. Scroll consumed.")
		s.logEvent("item", "scroll_fail", caster.ID, string(caster.Class), "", map[string]interface{}{
			"scroll": item.ScrollEffect, "roll": roll, "dc": dc,
		})
	} else {
		// Critical failure (miss by >= 5): backfire
		sb.WriteString(s.scrollBackfire(caster, item))
		s.logEvent("item", "scroll_backfire", caster.ID, string(caster.Class), "", map[string]interface{}{
			"scroll": item.ScrollEffect, "roll": roll, "dc": dc,
		})
	}

	// Remove scroll from inventory
	s.removeItemFromInventory(caster.ID, item.ID)
	s.incrementStat(caster.ID, "items_used", 1)

	return s.textResult(sb.String()), nil
}

// applyScrollEffect applies a successful scroll cast
func (s *Server) applyScrollEffect(caster *game.Character, item *game.Item, targetID string) string {
	switch item.ScrollEffect {
	case "heal":
		target := s.resolveScrollTarget(caster, targetID)
		if target == nil {
			return "No valid target for healing."
		}
		healing := rollDice(s.rng, 2, 6) + 4
		oldHP := target.HP
		target.Heal(healing)
		return fmt.Sprintf("Success! %s is bathed in healing light. Restores %d HP. (HP: %d/%d)",
			target.Name, target.HP-oldHP, target.HP, target.MaxHP)

	case "shield":
		if s.state.InCombat() {
			// Apply +4 AC buff to all party combatants for 3 rounds
			for _, c := range s.state.Combat.GetPlayerCombatants() {
				if c.IsAlive {
					c.AddBuff(game.BuffACBonus, 4, 3, "Shield Scroll")
				}
			}
			return "Success! A shimmering barrier surrounds the party. (+4 AC for 3 rounds!)"
		}
		return "Success! A shimmering barrier surrounds the party. (Use in combat for +4 AC for 3 rounds)"

	case "fireball":
		if s.state.InCombat() {
			// Find a target — prefer targetID, else nearest enemy
			targetCombatant := s.findScrollCombatTarget(targetID)
			if targetCombatant == nil {
				return "Success! But no valid target in combat."
			}
			// 3d6 damage to target
			dmg := rollDice(s.rng, 3, 6)
			var result strings.Builder
			result.WriteString(fmt.Sprintf("Success! A ball of fire erupts at %s!", targetCombatant.Name))
			// Damage primary target
			targetCombatant.RemoveSleep()
			targetCombatant.HP -= dmg
			if targetCombatant.HP <= 0 {
				targetCombatant.HP = 0
				targetCombatant.IsAlive = false
				s.state.Combat.RemoveFromGrid(targetCombatant)
				game.RemoveEngagement(s.state.Combat, targetCombatant.ID)
				result.WriteString(fmt.Sprintf(" %d damage — %s falls!", dmg, targetCombatant.Name))
			} else {
				result.WriteString(fmt.Sprintf(" %d damage! (%s HP: %d/%d)", dmg, targetCombatant.Name, targetCombatant.HP, targetCombatant.MaxHP))
			}
			// Splash: half damage to adjacent enemies
			splashDmg := dmg / 2
			if splashDmg > 0 {
				for _, c := range s.state.Combat.GetEnemyCombatants() {
					if c.ID == targetCombatant.ID || !c.IsAlive {
						continue
					}
					if game.IsAdjacent(targetCombatant.GridX, targetCombatant.GridY, c.GridX, c.GridY) {
						c.RemoveSleep()
						c.HP -= splashDmg
						if c.HP <= 0 {
							c.HP = 0
							c.IsAlive = false
							s.state.Combat.RemoveFromGrid(c)
							game.RemoveEngagement(s.state.Combat, c.ID)
							result.WriteString(fmt.Sprintf("\n  Splash: %s takes %d damage and falls!", c.Name, splashDmg))
						} else {
							result.WriteString(fmt.Sprintf("\n  Splash: %s takes %d damage! (HP: %d/%d)", c.Name, splashDmg, c.HP, c.MaxHP))
						}
					}
				}
			}
			return result.String()
		}
		return "Success! A ball of fire erupts! (Use in combat to deal 3d6 damage + splash)"

	case "lightning":
		if s.state.InCombat() {
			targetCombatant := s.findScrollCombatTarget(targetID)
			if targetCombatant == nil {
				return "Success! But no valid target in combat."
			}
			dmg := rollDice(s.rng, 4, 6)
			targetCombatant.RemoveSleep()
			targetCombatant.HP -= dmg
			if targetCombatant.HP <= 0 {
				targetCombatant.HP = 0
				targetCombatant.IsAlive = false
				s.state.Combat.RemoveFromGrid(targetCombatant)
				game.RemoveEngagement(s.state.Combat, targetCombatant.ID)
				return fmt.Sprintf("Success! Lightning strikes %s for %d damage! %s falls!", targetCombatant.Name, dmg, targetCombatant.Name)
			}
			return fmt.Sprintf("Success! Lightning strikes %s for %d damage! (%s HP: %d/%d)",
				targetCombatant.Name, dmg, targetCombatant.Name, targetCombatant.HP, targetCombatant.MaxHP)
		}
		return "Success! A bolt of lightning crackles forth! (Use in combat to deal 4d6 damage)"

	case "sleep":
		if s.state.InCombat() {
			targetCombatant := s.findScrollCombatTarget(targetID)
			if targetCombatant == nil {
				return "Success! But no valid target in combat."
			}
			targetCombatant.AddBuff(game.BuffSleep, 0, 2, "Sleep Scroll")
			return fmt.Sprintf("Success! %s falls into a magical slumber! (Skips next 2 turns — damage wakes them)", targetCombatant.Name)
		}
		return "Success! A wave of drowsiness emanates! (Use in combat — target skips 2 turns)"

	case "resurrect":
		// Find a dead party member
		if s.state.Party != nil {
			for _, c := range s.state.Party.Characters {
				if !c.IsAlive && (targetID == "" || targetID == c.ID) {
					c.Resurrect(c.MaxHP / 2)
					return fmt.Sprintf("Success! Divine light surges into %s! Resurrected at %d/%d HP!",
						c.Name, c.HP, c.MaxHP)
				}
			}
		}
		return "No dead party member to resurrect."

	default:
		return "The scroll's magic takes effect."
	}
}

// scrollBackfire applies a critical failure effect
func (s *Server) scrollBackfire(caster *game.Character, item *game.Item) string {
	switch item.ScrollEffect {
	case "fireball":
		dmg := rollDice(s.rng, 1, 6) + 1 // half effect
		caster.TakeDamage(dmg)
		msg := fmt.Sprintf("BACKFIRE! The fireball explodes in %s's hands! Takes %d damage. (HP: %d/%d)",
			caster.Name, dmg, caster.HP, caster.MaxHP)
		if !caster.IsAlive {
			msg += fmt.Sprintf(" %s has fallen!", caster.Name)
		}
		return msg

	case "lightning":
		dmg := rollDice(s.rng, 2, 6)
		caster.TakeDamage(dmg)
		msg := fmt.Sprintf("BACKFIRE! Lightning arcs back into %s! Takes %d damage. (HP: %d/%d)",
			caster.Name, dmg, caster.HP, caster.MaxHP)
		if !caster.IsAlive {
			msg += fmt.Sprintf(" %s has fallen!", caster.Name)
		}
		return msg

	default:
		return "BACKFIRE! The scroll's magic goes awry, but causes no lasting harm."
	}
}

// handleOpenChest opens a chest in the room (checks for chest traps, reveals loot)
func (s *Server) handleOpenChest(charID string) (*ToolResult, error) {
	if err := s.requireExploration(); err != nil {
		return err, nil
	}

	char, errResult := s.resolveCharacter(charID)
	if errResult != nil {
		return errResult, nil
	}

	currentRoom := s.state.GetCurrentRoom()
	if currentRoom == nil {
		return s.errorResult("Error: current room not found."), nil
	}

	// Find unopened chest traps in this room
	var chest *game.Trap
	for _, trap := range s.state.GetRoomTraps(currentRoom.ID) {
		if trap.Location == game.TrapChest && !trap.IsOpened {
			chest = trap
			break
		}
	}

	if chest == nil {
		return s.textResult("There is no chest to open here."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s opens the chest...\n", char.Name))

	// Handle trapped chests that haven't been dealt with yet
	if !chest.IsDisarmed && !chest.IsTriggered && chest.Damage > 0 {
		if chest.IsDiscovered {
			// Player already knows it's trapped — warn them
			sb.WriteString("Warning: you know this chest is trapped! Use a thief to disarm it first.\n")
			return s.textResult(sb.String()), nil
		}

		// Detection check
		detection := game.CheckTrapDetection(s.state.Party, chest, s.rng)
		if detection.Detected {
			sb.WriteString(fmt.Sprintf("%s\n", game.FormatTrapDetection(detection)))
			sb.WriteString("The chest is trapped! Consider having a thief disarm it.\n")
			s.logEvent("trap", "trap_detected", detection.DetectorID, "", chest.ID, map[string]interface{}{
				"trap_type": "chest", "trap_desc": chest.Description,
			})
			return s.textResult(sb.String()), nil
		}

		// Trap triggers on the opener, then fall through to reveal loot
		trigger := game.TriggerTrap(chest, char, s.rng)
		sb.WriteString(fmt.Sprintf("%s\n", game.FormatTrapTrigger(trigger)))
		s.logEvent("trap", "trap_triggered", char.ID, string(char.Class), chest.ID, map[string]interface{}{
			"trap_type": "chest", "damage": trigger.Damage,
		})
		if !char.IsAlive {
			s.logEvent("death", "character_killed", char.ID, string(char.Class), chest.ID, map[string]interface{}{
				"cause": "chest_trap",
			})
		}
		if s.state.Party.IsWiped() {
			s.state.GameOver = true
			s.finalizeSession("party_wipe")
			sb.WriteString("\nYour entire party has fallen. Game over.\nUse 'new_game' to play again.")
			return s.textResult(sb.String()), nil
		}
	}

	// Open the chest and reveal loot
	chest.IsOpened = true
	items := s.state.RevealChestItems(chest.ID)
	if len(items) > 0 {
		sb.WriteString("\nThe chest contains:\n")
		for _, item := range items {
			sb.WriteString(fmt.Sprintf("  - %s (%s, %s) [ID: %s]\n", item.Name, item.Type, item.Rarity, item.ID))
		}
	} else {
		sb.WriteString("\nThe chest is empty.\n")
	}

	return s.textResult(sb.String()), nil
}

// handleDisarmTrap has a character attempt to disarm a discovered trap
func (s *Server) handleDisarmTrap(charID, trapID string) (*ToolResult, error) {
	// Allow during exploration OR during scout phase (thief only)
	if s.state.InCombat() {
		cs := s.state.Combat
		if !cs.AwaitingScoutDecision || charID != cs.ScoutID {
			return &ToolResult{Content: []ContentBlock{{Type: "text", Text: errInCombat}}}, nil
		}
	} else if err := s.requireExploration(); err != nil {
		return err, nil
	}

	char, errResult := s.resolveCharacter(charID)
	if errResult != nil {
		return errResult, nil
	}

	trap, ok := s.state.Traps[trapID]
	if !ok {
		return s.textResult("Trap not found."), nil
	}

	if !trap.IsDiscovered {
		return s.textResult("You haven't discovered any trap to disarm here."), nil
	}
	if trap.IsTriggered {
		return s.textResult("That trap has already been triggered."), nil
	}
	if trap.IsDisarmed {
		return s.textResult("That trap is already disarmed."), nil
	}

	result := game.AttemptDisarm(char, trap, s.rng)

	var sb strings.Builder
	sb.WriteString(game.FormatDisarmResult(result))

	if result.Success {
		s.logEvent("trap", "trap_disarmed", charID, string(char.Class), trapID, map[string]interface{}{
			"roll": result.Roll, "dc": result.DC,
		})
		s.incrementStat(charID, "traps_disarmed", 1)

		// Chest trap: open and reveal loot on successful disarm
		if trap.Location == game.TrapChest && !trap.IsOpened {
			trap.IsOpened = true
			items := s.state.RevealChestItems(trap.ID)
			if len(items) > 0 {
				sb.WriteString("\nThe chest contains:\n")
				for _, item := range items {
					sb.WriteString(fmt.Sprintf("  - %s (%s, %s) [ID: %s]\n", item.Name, item.Type, item.Rarity, item.ID))
				}
			}
		}
	} else {
		s.logEvent("trap", "disarm_failed", charID, string(char.Class), trapID, map[string]interface{}{
			"roll": result.Roll, "dc": result.DC,
		})
		if result.TriggerResult != nil && result.TriggerResult.VictimDied {
			s.logEvent("death", "character_killed", charID, string(char.Class), trapID, map[string]interface{}{
				"cause": "failed_disarm",
			})
		}
		if s.state.Party.IsWiped() {
			s.state.GameOver = true
			s.finalizeSession("party_wipe")
			sb.WriteString("\nYour entire party has fallen. Game over.\nUse 'new_game' to play again.")
			return s.textResult(sb.String()), nil
		}

		// Chest trap: the chest still opens on failed disarm (trap triggered)
		if trap.Location == game.TrapChest && !trap.IsOpened {
			trap.IsOpened = true
			items := s.state.RevealChestItems(trap.ID)
			if len(items) > 0 {
				sb.WriteString("\nDespite the trap, the chest contains:\n")
				for _, item := range items {
					sb.WriteString(fmt.Sprintf("  - %s (%s, %s) [ID: %s]\n", item.Name, item.Type, item.Rarity, item.ID))
				}
			}
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
		return s.errorResult("Error: current room not found."), nil
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
	sb.WriteString(fmt.Sprintf("%s\n\n", game.FormatSneakResult(sneakResult)))

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

// handleInventory shows all party inventories
func (s *Server) handleInventory() (*ToolResult, error) {
	if err := s.requireInitialized(); err != nil {
		return err, nil
	}

	var sb strings.Builder
	sb.WriteString("=== PARTY INVENTORY ===\n\n")

	for _, c := range s.state.Party.Characters {
		status := "DEAD"
		if c.IsAlive {
			status = fmt.Sprintf("HP: %d/%d", c.HP, c.MaxHP)
		}
		sb.WriteString(fmt.Sprintf("--- %s (%s, %s) ---\n", c.Name, c.Class, status))

		items := s.state.GetCharacterInventory(c.ID)
		if len(items) == 0 {
			sb.WriteString("  (empty)\n")
		}
		for _, item := range items {
			equip := ""
			if item.IsEquipped {
				equip = " [EQUIPPED]"
			}
			sb.WriteString(fmt.Sprintf("  - %s (%s)%s [ID: %s]\n", item.Name, item.Type, equip, item.ID))

			// Show stats
			details := make([]string, 0)
			if item.Damage > 0 {
				details = append(details, fmt.Sprintf("Dmg +%d", item.Damage))
			}
			if item.Armor > 0 {
				details = append(details, fmt.Sprintf("AC +%d", item.Armor))
			}
			if item.Healing > 0 {
				details = append(details, fmt.Sprintf("Heal %d", item.Healing))
			}
			if item.Range == game.RangeRanged {
				details = append(details, fmt.Sprintf("Range %d", item.MaxRange))
			}
			if item.ScrollEffect != "" {
				details = append(details, fmt.Sprintf("Scroll: %s (DC %d)", item.ScrollEffect, item.ScrollDifficulty))
			}
			if len(item.ClassRestriction) > 0 {
				classes := make([]string, len(item.ClassRestriction))
				for i, cr := range item.ClassRestriction {
					classes[i] = string(cr)
				}
				details = append(details, fmt.Sprintf("Classes: %s", strings.Join(classes, "/")))
			}
			if len(details) > 0 {
				sb.WriteString(fmt.Sprintf("    %s\n", strings.Join(details, ", ")))
			}
		}
		sb.WriteString("\n")
	}

	return s.textResult(sb.String()), nil
}

// handleStats shows all party member stats
func (s *Server) handleStats() (*ToolResult, error) {
	if err := s.requireInitialized(); err != nil {
		return err, nil
	}

	var sb strings.Builder
	sb.WriteString("=== PARTY STATS ===\n\n")

	for _, c := range s.state.Party.Characters {
		sb.WriteString(fmt.Sprintf("--- %s the %s ---\n", c.Name, c.Class))
		sb.WriteString(fmt.Sprintf("  Status: %s\n", c.HealthStatus()))
		sb.WriteString(fmt.Sprintf("  HP: %d/%d\n", c.HP, c.MaxHP))
		sb.WriteString(fmt.Sprintf("  STR: %d  DEX: %d  INT: %d\n", c.Strength, c.Dexterity, c.Intelligence))
		sb.WriteString(fmt.Sprintf("  Movement: %d cells  Initiative bonus: %+d\n",
			c.GetMovementRange(), c.GetInitiativeBonus()))

		// Spell slots (magic_user only)
		if c.Class == game.ClassMagicUser && c.MaxSpellSlots > 0 {
			sb.WriteString(fmt.Sprintf("  Spell Slots: %d/%d\n", c.SpellSlots, c.MaxSpellSlots))
			if len(c.KnownSpells) > 0 {
				sb.WriteString(fmt.Sprintf("  Known Spells: %s\n", strings.Join(c.KnownSpells, ", ")))
			}
		}

		// Equipment
		if c.EquippedWeaponID != nil {
			weapon := s.state.Items[*c.EquippedWeaponID]
			if weapon != nil {
				sb.WriteString(fmt.Sprintf("  Weapon: %s (+%d dmg)\n", weapon.Name, weapon.Damage))
			}
		} else {
			sb.WriteString("  Weapon: None (bare hands)\n")
		}
		if c.EquippedArmorID != nil {
			armor := s.state.Items[*c.EquippedArmorID]
			if armor != nil {
				sb.WriteString(fmt.Sprintf("  Armor: %s (+%d AC)\n", armor.Name, armor.Armor))
			}
		} else {
			sb.WriteString("  Armor: None\n")
		}

		sb.WriteString("\n")
	}

	// Formation
	sb.WriteString(fmt.Sprintf("Formation: %s\n", formatFormation(s.state.Party)))

	// Location
	room := s.state.GetCurrentRoom()
	if room != nil {
		sb.WriteString(fmt.Sprintf("Location: %s (%d,%d)\n", room.Name, room.X, room.Y))
	}
	sb.WriteString(fmt.Sprintf("Explored: %.0f%%\n", s.getExplorationPct()))

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

// --- Spell Casting (Exploration) ---

// handleCastSpell casts a known spell outside of combat
func (s *Server) handleCastSpell(charID, spellID, targetID string) (*ToolResult, error) {
	if err := s.requireExploration(); err != nil {
		return err, nil
	}

	char, errResult := s.resolveCharacter(charID)
	if errResult != nil {
		return errResult, nil
	}

	ok, reason := game.CanCast(char, spellID)
	if !ok {
		return s.textResult(reason), nil
	}

	spell := game.SpellRegistry[spellID]
	if spell.CombatOnly {
		return s.textResult(fmt.Sprintf("%s can only be cast during combat.", spell.Name)), nil
	}

	// Spend the slot
	game.SpendSlot(char)

	// Create a synthetic scroll item to reuse applyScrollEffect
	syntheticItem := &game.Item{
		Name:         spell.Name,
		ScrollEffect: spellID,
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s casts %s! (Slots: %d/%d)\n",
		char.Name, spell.Name, char.SpellSlots, char.MaxSpellSlots))
	sb.WriteString(s.applyScrollEffect(char, syntheticItem, targetID))

	s.logEvent("spell", "spell_cast", charID, string(char.Class), targetID, map[string]interface{}{
		"spell": spellID, "slots_remaining": char.SpellSlots,
	})
	s.incrementStat(charID, "spells_cast", 1)

	return s.textResult(sb.String()), nil
}

// --- Rest Mechanic ---

// handleRest lets the party rest in a cleared room to recover HP and spell slots
func (s *Server) handleRest() (*ToolResult, error) {
	if err := s.requireActiveGame(); err != nil {
		return err, nil
	}

	ok, reason := game.CanRest(s.state)
	if !ok {
		return s.textResult(reason), nil
	}

	s.state.ResetTurnContext()

	var sb strings.Builder
	sb.WriteString("=== REST ===\n\n")

	// Heal all alive characters
	sb.WriteString("Recovery:\n")
	for _, c := range s.state.Party.Characters {
		if !c.IsAlive {
			continue
		}
		healing := game.CalcRestHealing(c)
		oldHP := c.HP
		c.Heal(healing)
		healed := c.HP - oldHP
		sb.WriteString(fmt.Sprintf("  %s recovers %d HP. (HP: %d/%d)\n", c.Name, healed, c.HP, c.MaxHP))

		// Recharge spell slots for magic users
		if c.Class == game.ClassMagicUser && c.MaxSpellSlots > 0 {
			game.RechargeSlots(c)
			sb.WriteString(fmt.Sprintf("  %s's spell slots recharged. (Slots: %d/%d)\n",
				c.Name, c.SpellSlots, c.MaxSpellSlots))
		}
	}

	// Calculate ambush chance
	room := s.state.GetCurrentRoom()
	roomDepth := room.X + room.Y // Manhattan distance from entrance
	hasThief := s.state.Party.HasClass(game.ClassThief)
	// Thief must be alive to stand watch
	hasThiefWatch := false
	if hasThief {
		thief := s.state.Party.GetByClass(game.ClassThief)
		if thief != nil && thief.IsAlive {
			hasThiefWatch = true
		}
	}

	ambushChance := game.CalcAmbushChance(roomDepth, hasThiefWatch)

	if hasThiefWatch {
		thief := s.state.Party.GetByClass(game.ClassThief)
		sb.WriteString(fmt.Sprintf("\n%s stands watch. (Ambush chance: %d%%)\n", thief.Name, ambushChance))
	} else {
		sb.WriteString(fmt.Sprintf("\nNo thief on watch. (Ambush chance: %d%%)\n", ambushChance))
	}

	// Roll for ambush
	if game.RollAmbush(s.rng, ambushChance) {
		sb.WriteString("\nYou are ambushed by wandering monsters!\n")

		// Generate ambush monsters
		monsters := s.generateAmbushMonsters(roomDepth, room.ID)
		for _, m := range monsters {
			s.state.AddMonster(m)
		}

		s.logEvent("combat", "ambush", "", "", "", map[string]interface{}{
			"room_depth":    roomDepth,
			"monster_count": len(monsters),
			"thief_watch":   hasThiefWatch,
		})

		// Determine surprise round: monsters get surprise if no thief on watch
		surpriseRound := !hasThiefWatch

		// Enter combat
		combatMsg := s.enterAmbushCombat(room.ID, monsters, surpriseRound)
		sb.WriteString(combatMsg)
	} else {
		sb.WriteString("\nRest complete. No disturbances.\n")
		s.logEvent("interaction", "rest", "", "", "", map[string]interface{}{
			"room_depth":  roomDepth,
			"thief_watch": hasThiefWatch,
		})
	}

	return s.textResult(sb.String()), nil
}

// --- Helpers ---

// removeItemFromInventory removes an item from inventory and the game
func (s *Server) removeItemFromInventory(charID, itemID string) {
	if s.state.ItemsByChar[charID] != nil {
		delete(s.state.ItemsByChar[charID], itemID)
	}
	delete(s.state.Items, itemID)
}

// resolveScrollTarget picks a target for a scroll effect
func (s *Server) resolveScrollTarget(caster *game.Character, targetID string) *game.Character {
	if targetID != "" && s.state.Party != nil {
		c := s.state.Party.GetCharacter(targetID)
		if c != nil && c.IsAlive {
			return c
		}
	}
	// Default to caster
	return caster
}

// findScrollCombatTarget finds an enemy combatant for scroll targeting
func (s *Server) findScrollCombatTarget(targetID string) *game.Combatant {
	if !s.state.InCombat() {
		return nil
	}
	// If specific target given, use it
	if targetID != "" {
		c := s.state.Combat.GetCombatant(targetID)
		if c != nil && !c.IsPlayerChar && c.IsAlive {
			return c
		}
	}
	// Default: nearest alive enemy
	for _, c := range s.state.Combat.GetEnemyCombatants() {
		if c.IsAlive {
			return c
		}
	}
	return nil
}

// formatFormation returns a readable formation string
func formatFormation(party *game.Party) string {
	names := make([]string, 0, len(party.Formation))
	for _, id := range party.Formation {
		c := party.GetCharacter(id)
		if c != nil {
			marker := ""
			if !c.IsAlive {
				marker = " (dead)"
			}
			names = append(names, c.Name+marker)
		}
	}
	return strings.Join(names, " > ")
}

// rollDice rolls NdS (e.g. 2d6)
func rollDice(rng *rand.Rand, n, sides int) int {
	total := 0
	for i := 0; i < n; i++ {
		total += rng.Intn(sides) + 1
	}
	return total
}

// --- Save/Load Handlers ---

// handleSaveGame saves the current game state to the database
func (s *Server) handleSaveGame() (*ToolResult, error) {
	if err := s.requireInitialized(); err != nil {
		return err, nil
	}

	if s.db == nil {
		return s.errorResult("Database not available."), nil
	}

	stateJSON, err := s.state.SerializeState()
	if err != nil {
		return s.errorResult(fmt.Sprintf("Failed to serialize game state: %v", err)), nil
	}

	saveID := s.state.SessionID + "_save"
	if err := s.db.SaveGame(saveID, s.state.SessionID, stateJSON); err != nil {
		return s.errorResult(fmt.Sprintf("Failed to save game: %v", err)), nil
	}

	return s.textResult(fmt.Sprintf("Game saved! Session: %s", s.state.SessionID)), nil
}

// handleLoadGame loads a saved game from the database
func (s *Server) handleLoadGame(sessionID string) (*ToolResult, error) {
	if s.db == nil {
		return s.errorResult("Database not available."), nil
	}

	saved, err := s.db.LoadGameBySession(sessionID)
	if err != nil {
		return s.errorResult(fmt.Sprintf("Failed to load game: %v", err)), nil
	}
	if saved == nil {
		return s.textResult("No saved game found for that session."), nil
	}

	state, err := game.DeserializeState(saved.StateJSON)
	if err != nil {
		return s.errorResult(fmt.Sprintf("Failed to restore game state: %v", err)), nil
	}

	s.state = state
	s.rng = rand.New(rand.NewSource(time.Now().UnixNano()))

	var sb strings.Builder
	sb.WriteString("=== GAME LOADED ===\n\n")
	sb.WriteString(fmt.Sprintf("Session: %s\n", sessionID))
	sb.WriteString(fmt.Sprintf("Mode: %s\n", s.state.Mode))
	if s.state.Party != nil {
		sb.WriteString(fmt.Sprintf("Party: %s\n", formatFormation(s.state.Party)))
		for _, c := range s.state.Party.Characters {
			sb.WriteString(fmt.Sprintf("  %s (%s) — HP: %d/%d %s\n",
				c.Name, c.Class, c.HP, c.MaxHP, c.HealthStatus()))
		}
	}
	sb.WriteString("\nUse 'look' to see your surroundings.")

	return s.textResult(sb.String()), nil
}

// --- Session Finalization ---

// finalizeSession ends the session in the database with outcome and stats
func (s *Server) finalizeSession(outcome string) {
	if s.db == nil || s.state == nil {
		return
	}

	turns := 0
	if s.state.TurnContext != nil {
		turns = s.state.TurnContext.TurnNumber
	}

	rooms := len(s.state.VisitedRooms)

	monstersDefeated := 0
	for _, m := range s.state.Monsters {
		if !m.IsAlive {
			monstersDefeated++
		}
	}

	trapsEncountered := 0
	for _, t := range s.state.Traps {
		if t.IsTriggered || t.IsDiscovered {
			trapsEncountered++
		}
	}

	charsLost := 0
	if s.state.Party != nil {
		for _, c := range s.state.Party.Characters {
			if !c.IsAlive {
				charsLost++
			}
		}
	}

	s.db.EndSession(s.state.SessionID, outcome, turns, rooms, monstersDefeated, trapsEncountered, charsLost)

	// Finalize character stats
	if s.state.Party != nil {
		for _, c := range s.state.Party.Characters {
			status := "alive"
			if !c.IsAlive {
				status = "dead"
			}
			s.db.FinalizeSessionCharacter(c.ID, c.HP, status)
		}
	}
}
