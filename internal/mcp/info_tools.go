package mcp

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/lehmann314159/yrpg-backend/internal/db"
	"github.com/lehmann314159/yrpg-backend/internal/game"
)

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
	if errors.Is(err, db.ErrNotFound) {
		return s.textResult("No saved game found for that session."), nil
	}
	if err != nil {
		return s.errorResult(fmt.Sprintf("Failed to load game: %v", err)), nil
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
