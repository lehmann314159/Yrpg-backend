package mcp

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/lehmann314159/yrpg-backend/internal/game"
)

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
		return s.errorResult("Current room not found."), nil
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

// handleDropItem drops an item from a character's inventory onto the room floor
func (s *Server) handleDropItem(charID, itemID string) (*ToolResult, error) {
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

	// Check item belongs to this character
	if item.CharacterID == nil || *item.CharacterID != char.ID {
		return s.textResult("That item is not in this character's inventory."), nil
	}

	currentRoom := s.state.GetCurrentRoom()
	if currentRoom == nil {
		return s.errorResult("Current room not found."), nil
	}

	// Auto-unequip if equipped
	var sb strings.Builder
	if item.IsEquipped {
		item.IsEquipped = false
		switch item.Type {
		case game.ItemWeapon:
			if char.EquippedWeaponID != nil && *char.EquippedWeaponID == item.ID {
				char.EquippedWeaponID = nil
			}
		case game.ItemArmor:
			if char.EquippedArmorID != nil && *char.EquippedArmorID == item.ID {
				char.EquippedArmorID = nil
			}
		}
		sb.WriteString(fmt.Sprintf("%s unequips the %s.\n", char.Name, item.Name))
	}

	// Move item: remove from character index, add to room index
	if s.state.ItemsByChar[char.ID] != nil {
		delete(s.state.ItemsByChar[char.ID], itemID)
	}
	if s.state.ItemsByRoom[currentRoom.ID] == nil {
		s.state.ItemsByRoom[currentRoom.ID] = make(map[string]bool)
	}
	s.state.ItemsByRoom[currentRoom.ID][itemID] = true

	item.CharacterID = nil
	item.RoomID = &currentRoom.ID

	sb.WriteString(fmt.Sprintf("%s drops the %s.", char.Name, item.Name))
	return s.textResult(sb.String()), nil
}

// handleGiveItem transfers an item from one character to another
func (s *Server) handleGiveItem(charID, itemID, targetCharID string) (*ToolResult, error) {
	if err := s.requireExploration(); err != nil {
		return err, nil
	}

	char, errResult := s.resolveCharacter(charID)
	if errResult != nil {
		return errResult, nil
	}

	target, errResult := s.resolveCharacter(targetCharID)
	if errResult != nil {
		return errResult, nil
	}

	item, ok := s.state.Items[itemID]
	if !ok {
		return s.textResult("Item not found. Use 'inventory' to see your items."), nil
	}

	// Check item belongs to the giver
	if item.CharacterID == nil || *item.CharacterID != char.ID {
		return s.textResult("That item is not in this character's inventory."), nil
	}

	if char.ID == target.ID {
		return s.textResult("Cannot give an item to the same character."), nil
	}

	// Auto-unequip if equipped
	var sb strings.Builder
	if item.IsEquipped {
		item.IsEquipped = false
		switch item.Type {
		case game.ItemWeapon:
			if char.EquippedWeaponID != nil && *char.EquippedWeaponID == item.ID {
				char.EquippedWeaponID = nil
			}
		case game.ItemArmor:
			if char.EquippedArmorID != nil && *char.EquippedArmorID == item.ID {
				char.EquippedArmorID = nil
			}
		}
		sb.WriteString(fmt.Sprintf("%s unequips the %s.\n", char.Name, item.Name))
	}

	// Transfer: remove from giver's index, add to target's index
	if s.state.ItemsByChar[char.ID] != nil {
		delete(s.state.ItemsByChar[char.ID], itemID)
	}
	if s.state.ItemsByChar[target.ID] == nil {
		s.state.ItemsByChar[target.ID] = make(map[string]bool)
	}
	s.state.ItemsByChar[target.ID][itemID] = true

	item.CharacterID = &target.ID

	sb.WriteString(fmt.Sprintf("%s gives the %s to %s.", char.Name, item.Name, target.Name))
	return s.textResult(sb.String()), nil
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
	// Shield scrolls only work in combat
	if item.ScrollEffect == "shield" && !s.state.InCombat() {
		return s.textResult("The shield scroll can only be used in combat."), nil
	}

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
		return "The shield scroll can only be used in combat." // unreachable; guarded in useScroll

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
		return s.errorResult("Current room not found."), nil
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
			sb.WriteString(fmt.Sprintf("%s\n", FormatTrapDetection(detection)))
			sb.WriteString("The chest is trapped! Consider having a thief disarm it.\n")
			s.logEvent("trap", "trap_detected", detection.DetectorID, "", chest.ID, map[string]interface{}{
				"trap_type": "chest", "trap_desc": chest.Description,
			})
			return s.textResult(sb.String()), nil
		}

		// Trap triggers on the opener, then fall through to reveal loot
		trigger := game.TriggerTrap(chest, char, s.rng)
		sb.WriteString(fmt.Sprintf("%s\n", FormatTrapTrigger(trigger)))
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
	// Allow during exploration OR during scout phase/decision (thief only)
	if s.state.InCombat() {
		cs := s.state.Combat
		isScout := charID == cs.ScoutID
		if !(isScout && (cs.IsScoutPhase || cs.AwaitingScoutDecision)) {
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
	sb.WriteString(FormatDisarmResult(result))

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

		// Sync trap damage to combatant so endCombat doesn't overwrite it
		if s.state.InCombat() {
			if combatant := s.state.Combat.GetCombatant(charID); combatant != nil {
				combatant.HP = char.HP
				if !char.IsAlive {
					combatant.IsAlive = false
					s.state.Combat.RemoveFromGrid(combatant)
				}
			}
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

// rollDice rolls NdS (e.g. 2d6)
func rollDice(rng *rand.Rand, n, sides int) int {
	total := 0
	for i := 0; i < n; i++ {
		total += rng.Intn(sides) + 1
	}
	return total
}
