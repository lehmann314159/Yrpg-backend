package mcp

import (
	"fmt"
	"strings"

	"github.com/lehmann314159/yrpg-backend/internal/game"
	"github.com/lehmann314159/yrpg-backend/internal/generator"
)

const errAwaitingScoutDecision = "The scout must decide: use 'signal_party' to bring the party in, or 'combat_retreat' to pull back."

// handleCombatStatus shows the current combat state
func (s *Server) handleCombatStatus() (*ToolResult, error) {
	if err := s.requireCombat(); err != nil {
		return err, nil
	}

	cs := s.state.Combat
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("=== COMBAT — Round %d ===\n\n", cs.RoundNumber))

	// Current turn
	current := cs.GetCurrentCombatant()
	if current != nil {
		sb.WriteString(fmt.Sprintf("Current turn: %s", current.Name))
		if current.IsPlayerChar {
			sb.WriteString(" (YOUR TURN)")
		} else {
			sb.WriteString(" (ENEMY)")
		}
		sb.WriteString("\n\n")
	}

	// Initiative order
	sb.WriteString("Initiative order:\n")
	for i, c := range cs.Combatants {
		if !c.IsAlive {
			continue
		}
		marker := "  "
		if i == cs.CurrentTurnIdx {
			marker = "> "
		}
		side := "ALLY"
		if !c.IsPlayerChar {
			side = "ENEMY"
		}
		sb.WriteString(fmt.Sprintf("%s%s [%s] — HP: %d/%d, Pos: (%d,%d), Init: %d",
			marker, c.Name, side, c.HP, c.MaxHP, c.GridX, c.GridY, c.Initiative))
		if c.IsHidden {
			sb.WriteString(" [HIDDEN]")
		}
		if c.HasMoved {
			sb.WriteString(" [moved]")
		}
		if c.HasActed {
			sb.WriteString(" [acted]")
		}
		sb.WriteString(fmt.Sprintf(" [ID: %s]", c.ID))
		sb.WriteString("\n")
	}

	// Grid
	sb.WriteString("\nGrid (Y increases upward, party starts at bottom):\n")
	sb.WriteString(renderCombatGrid(cs))

	return s.textResult(sb.String()), nil
}

// handleCombatMove moves a character on the combat grid
func (s *Server) handleCombatMove(charID string, x, y int) (*ToolResult, error) {
	if err := s.requireCombat(); err != nil {
		return err, nil
	}
	if s.state.Combat.AwaitingScoutDecision {
		return s.textResult(errAwaitingScoutDecision), nil
	}

	combatant, errResult := s.requirePlayerTurn(charID)
	if errResult != nil {
		return errResult, nil
	}

	char, errResult := s.resolveCharacter(charID)
	if errResult != nil {
		return errResult, nil
	}

	moveRange := char.GetMovementRange()
	if s.state.Combat.IsScoutPhase && combatant.ID == s.state.Combat.ScoutID {
		moveRange *= 2
	}
	result := game.CombatMove(s.state.Combat, combatant, x, y, moveRange)

	return s.textResult(result.Message), nil
}

// handleCombatAttack executes an attack in combat
func (s *Server) handleCombatAttack(charID, targetID string) (*ToolResult, error) {
	if err := s.requireCombat(); err != nil {
		return err, nil
	}
	if s.state.Combat.AwaitingScoutDecision {
		return s.textResult(errAwaitingScoutDecision), nil
	}

	combatant, errResult := s.requirePlayerTurn(charID)
	if errResult != nil {
		return errResult, nil
	}

	if combatant.HasActed {
		return s.textResult(fmt.Sprintf("%s has already acted this turn.", combatant.Name)), nil
	}

	char, errResult := s.resolveCharacter(charID)
	if errResult != nil {
		return errResult, nil
	}

	target := s.state.Combat.GetCombatant(targetID)
	if target == nil || !target.IsAlive {
		return s.textResult("Target not found or already dead."), nil
	}
	if target.IsPlayerChar {
		return s.textResult("Cannot attack your own party member."), nil
	}

	// Get equipped weapon
	var weapon *game.Item
	if char.EquippedWeaponID != nil {
		weapon = s.state.Items[*char.EquippedWeaponID]
	}

	result := game.CombatAttack(s.state.Combat, combatant, target, char, weapon, s.rng)
	combatant.HasActed = true

	var sb strings.Builder

	// Check range failure
	if !result.Hit && result.Roll == 0 {
		if weapon != nil && weapon.Range == game.RangeRanged {
			sb.WriteString(fmt.Sprintf("Target is out of range! (Max range: %d cells)", weapon.MaxRange))
		} else {
			sb.WriteString("Target is not adjacent! Move closer for melee or equip a ranged weapon.")
		}
		combatant.HasActed = false // don't consume action on range failure
		return s.textResult(sb.String()), nil
	}

	sb.WriteString(game.FormatAttackResult(result))

	// Log combat event
	subtype := "attack_miss"
	if result.Hit {
		subtype = "attack_hit"
		if result.Flanking {
			subtype = "flanking_attack"
		}
		if result.Critical {
			subtype = "critical_hit"
		}
	}
	s.logEvent("combat", subtype, charID, string(char.Class), targetID, map[string]interface{}{
		"roll":         result.Roll,
		"damage":       result.Damage,
		"was_critical": result.Critical,
		"was_flanking": result.Flanking,
	})
	if result.Hit {
		s.incrementStat(charID, "damage_dealt", result.Damage)
	}
	if result.TargetDied {
		s.logEvent("death", "enemy_defeated", charID, string(char.Class), targetID, nil)
		s.incrementStat(charID, "kills", 1)
	}

	// Cleave: when a fighter kills an enemy with melee, free attack on adjacent enemy
	if result.TargetDied && char.Class == game.ClassFighter &&
		(weapon == nil || weapon.Range != game.RangeRanged) {
		// Find an alive enemy adjacent to the killed target's last position
		var cleaveTarget *game.Combatant
		for _, c := range s.state.Combat.GetEnemyCombatants() {
			if c.IsAlive && game.IsAdjacent(target.GridX, target.GridY, c.GridX, c.GridY) {
				cleaveTarget = c
				break
			}
		}
		if cleaveTarget != nil {
			cleaveResult := game.CombatAttack(s.state.Combat, combatant, cleaveTarget, char, weapon, s.rng)
			sb.WriteString("\nCleave! " + game.FormatAttackResult(cleaveResult))
			s.logEvent("combat", "cleave", charID, string(char.Class), cleaveTarget.ID, map[string]interface{}{
				"roll":   cleaveResult.Roll,
				"damage": cleaveResult.Damage,
			})
			if cleaveResult.Hit {
				s.incrementStat(charID, "damage_dealt", cleaveResult.Damage)
			}
			if cleaveResult.TargetDied {
				s.logEvent("death", "enemy_defeated", charID, string(char.Class), cleaveTarget.ID, nil)
				s.incrementStat(charID, "kills", 1)
			}
		}
	}

	// Check combat end
	msg := s.checkCombatEnd(&sb)
	if msg != "" {
		return s.textResult(sb.String()), nil
	}

	// Advance turn after action
	sb.WriteString(s.advanceTurnAndRunMonsters())

	return s.textResult(sb.String()), nil
}

// handleCombatUseItem uses an item during combat (consumes action)
func (s *Server) handleCombatUseItem(charID, itemID, targetID string) (*ToolResult, error) {
	if err := s.requireCombat(); err != nil {
		return err, nil
	}
	if s.state.Combat.AwaitingScoutDecision {
		return s.textResult(errAwaitingScoutDecision), nil
	}

	combatant, errResult := s.requirePlayerTurn(charID)
	if errResult != nil {
		return errResult, nil
	}

	if combatant.HasActed {
		return s.textResult(fmt.Sprintf("%s has already acted this turn.", combatant.Name)), nil
	}

	// Delegate to use_item handler (works in both modes)
	result, err := s.handleUseItem(charID, itemID, targetID)
	if err != nil {
		return result, err
	}

	combatant.HasActed = true

	// Sync HP changes from characters to their combatants (covers healing others, self-healing, etc.)
	for _, pc := range s.state.Combat.GetPlayerCombatants() {
		ch := s.state.Party.GetCharacter(pc.CharacterID)
		if ch != nil {
			pc.HP = ch.HP
		}
	}

	// Advance turn
	var sb strings.Builder
	sb.WriteString(result.Content[0].Text)
	sb.WriteString(s.advanceTurnAndRunMonsters())

	return s.textResult(sb.String()), nil
}

// handleCombatDefend puts a character in full defense stance
func (s *Server) handleCombatDefend(charID string) (*ToolResult, error) {
	if err := s.requireCombat(); err != nil {
		return err, nil
	}
	if s.state.Combat.AwaitingScoutDecision {
		return s.textResult(errAwaitingScoutDecision), nil
	}

	combatant, errResult := s.requirePlayerTurn(charID)
	if errResult != nil {
		return errResult, nil
	}

	if combatant.HasActed {
		return s.textResult(fmt.Sprintf("%s has already acted this turn.", combatant.Name)), nil
	}

	combatant.HasActed = true
	combatant.AddBuff(game.BuffACBonus, game.DefendACBonus, 1, "Defend")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s takes a defensive stance. (+%d AC this round)\n",
		combatant.Name, game.DefendACBonus))

	sb.WriteString(s.advanceTurnAndRunMonsters())

	return s.textResult(sb.String()), nil
}

// handleCombatHide has a thief attempt to hide
func (s *Server) handleCombatHide(charID string) (*ToolResult, error) {
	if err := s.requireCombat(); err != nil {
		return err, nil
	}
	if s.state.Combat.AwaitingScoutDecision {
		return s.textResult(errAwaitingScoutDecision), nil
	}

	combatant, errResult := s.requirePlayerTurn(charID)
	if errResult != nil {
		return errResult, nil
	}

	if combatant.HasActed {
		return s.textResult(fmt.Sprintf("%s has already acted this turn.", combatant.Name)), nil
	}

	char, errResult := s.resolveCharacter(charID)
	if errResult != nil {
		return errResult, nil
	}

	if char.Class != game.ClassThief {
		return s.textResult("Only a thief can hide in combat."), nil
	}

	// Get alive enemy monsters for perception DC
	enemies := make([]*game.Monster, 0)
	for _, c := range s.state.Combat.GetEnemyCombatants() {
		if c.IsAlive {
			if m, ok := s.state.Monsters[c.CharacterID]; ok {
				enemies = append(enemies, m)
			}
		}
	}

	result := game.AttemptCombatHide(char, enemies, s.rng)
	combatant.HasActed = true

	var sb strings.Builder
	sb.WriteString(game.FormatCombatHideResult(result))
	sb.WriteString("\n")

	if result.Success {
		combatant.IsHidden = true
		game.RemoveEngagement(s.state.Combat, combatant.ID)
	}
	hideSubtype := "hide_fail"
	if result.Success {
		hideSubtype = "hide_success"
		s.incrementStat(charID, "successful_sneaks", 1)
	}
	s.logEvent("stealth", hideSubtype, charID, string(char.Class), "", map[string]interface{}{
		"roll": result.Roll, "dc": result.DC,
	})

	sb.WriteString(s.advanceTurnAndRunMonsters())

	return s.textResult(sb.String()), nil
}

// handleCombatRetreat has a character attempt to flee
func (s *Server) handleCombatRetreat(charID string) (*ToolResult, error) {
	if err := s.requireCombat(); err != nil {
		return err, nil
	}

	cs := s.state.Combat

	// Scout-phase retreat: thief retreats alone back to previous room
	if cs.AwaitingScoutDecision && charID == cs.ScoutID {
		combatant := cs.GetCombatant(charID)
		if combatant == nil {
			return s.textResult("Scout not found in combat."), nil
		}
		char, errResult := s.resolveCharacter(charID)
		if errResult != nil {
			return errResult, nil
		}

		var sb strings.Builder

		// Check if scout is engaged with any enemy
		_, engaged := cs.Engagements[combatant.ID]
		if !engaged {
			// No engagement: automatic success
			cs.RemoveFromGrid(combatant)
			sb.WriteString(fmt.Sprintf("%s slips away unnoticed and returns to the party.\n", combatant.Name))
			prevRoom := cs.PreviousRoomID
			s.endCombat()
			char.CurrentRoomID = prevRoom
			return s.textResult(sb.String()), nil
		}

		// Has engagement: standard retreat roll
		result := game.AttemptRetreat(cs, combatant, char, s.rng)
		sb.WriteString(game.FormatRetreatResult(result))
		sb.WriteString("\n")

		// Sync HP from opportunity attack
		if result.OpportunityAttack != nil {
			char.HP = combatant.HP
			if !combatant.IsAlive && char.IsAlive {
				char.HP = 0
				char.IsAlive = false
			}
		}

		if result.Success {
			// Retreat succeeded: end combat, move thief back
			prevRoom := cs.PreviousRoomID
			s.endCombat()
			char.CurrentRoomID = prevRoom
			sb.WriteString(fmt.Sprintf("%s retreats to the previous room.\n", char.Name))
			return s.textResult(sb.String()), nil
		}

		// Retreat failed: auto-signal party
		sb.WriteString(fmt.Sprintf("%s is caught! The party rushes in!\n", char.Name))
		monsters := s.state.GetRoomMonsters(char.CurrentRoomID)
		s.state.Party.MoveAllToRoom(char.CurrentRoomID)
		game.AddPartyToCombat(cs, s.state.Party, monsters, s.state.Items, s.rng)

		// Show initiative order
		sb.WriteString("\nNew initiative order:\n")
		for _, c := range cs.Combatants {
			if !c.IsAlive {
				continue
			}
			side := "ALLY"
			if !c.IsPlayerChar {
				side = "ENEMY"
			}
			sb.WriteString(fmt.Sprintf("  %s [%s] — Init: %d, HP: %d/%d, Pos: (%d,%d)\n",
				c.Name, side, c.Initiative, c.HP, c.MaxHP, c.GridX, c.GridY))
		}

		// Run monster turns if first combatant is a monster
		current := cs.GetCurrentCombatant()
		if current != nil && !current.IsPlayerChar {
			sb.WriteString(s.runMonsterTurns())
		} else if current != nil {
			sb.WriteString(fmt.Sprintf("\n%s's turn!\n", current.Name))
		}

		return s.textResult(sb.String()), nil
	}

	combatant, errResult := s.requirePlayerTurn(charID)
	if errResult != nil {
		return errResult, nil
	}

	if combatant.HasActed {
		return s.textResult(fmt.Sprintf("%s has already acted this turn.", combatant.Name)), nil
	}

	char, errResult := s.resolveCharacter(charID)
	if errResult != nil {
		return errResult, nil
	}

	result := game.AttemptRetreat(s.state.Combat, combatant, char, s.rng)
	combatant.HasActed = true

	retreatSubtype := "retreat_fail"
	if result.Success {
		retreatSubtype = "retreat_success"
	}
	s.logEvent("combat", retreatSubtype, charID, string(char.Class), "", map[string]interface{}{
		"opportunity_attack": result.OpportunityAttack != nil,
	})

	var sb strings.Builder
	sb.WriteString(game.FormatRetreatResult(result))
	sb.WriteString("\n")

	// Sync HP changes from opportunity attacks
	if result.OpportunityAttack != nil {
		char.HP = combatant.HP
		if !combatant.IsAlive {
			char.TakeDamage(char.HP + 1)
		}
	}

	if result.Success {
		if s.retreated == nil {
			s.retreated = make(map[string]bool)
		}
		s.retreated[combatant.ID] = true

		// Check if entire party has retreated
		if game.CheckPartyRetreat(s.state.Combat, s.retreated) {
			sb.WriteString("\nThe party retreats to the previous room!\n")
			prevRoom := s.state.Combat.PreviousRoomID
			s.endCombat()
			// Move party back to previous room
			s.state.Party.MoveAllToRoom(prevRoom)
			return s.textResult(sb.String()), nil
		}
	}

	// Check combat end (in case opportunity attack killed everyone)
	msg := s.checkCombatEnd(&sb)
	if msg != "" {
		return s.textResult(sb.String()), nil
	}

	sb.WriteString(s.advanceTurnAndRunMonsters())

	return s.textResult(sb.String()), nil
}

// handleEndTurn ends the current character's turn
func (s *Server) handleEndTurn(charID string) (*ToolResult, error) {
	if err := s.requireCombat(); err != nil {
		return err, nil
	}
	if s.state.Combat.AwaitingScoutDecision {
		return s.textResult(errAwaitingScoutDecision), nil
	}

	combatant, errResult := s.requirePlayerTurn(charID)
	if errResult != nil {
		return errResult, nil
	}

	combatant.HasActed = true
	combatant.HasMoved = true

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s ends their turn.\n", combatant.Name))
	sb.WriteString(s.advanceTurnAndRunMonsters())

	return s.textResult(sb.String()), nil
}

// --- Spell Casting (Combat) ---

// handleCombatCastSpell casts a known spell during combat
func (s *Server) handleCombatCastSpell(charID, spellID, targetID string) (*ToolResult, error) {
	if err := s.requireCombat(); err != nil {
		return err, nil
	}
	if s.state.Combat.AwaitingScoutDecision {
		return s.textResult(errAwaitingScoutDecision), nil
	}

	combatant, errResult := s.requirePlayerTurn(charID)
	if errResult != nil {
		return errResult, nil
	}

	if combatant.HasActed {
		return s.textResult(fmt.Sprintf("%s has already acted this turn.", combatant.Name)), nil
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

	// Spend the slot
	game.SpendSlot(char)
	combatant.HasActed = true

	// Create a synthetic scroll item to reuse applyScrollEffect
	syntheticItem := &game.Item{
		Name:         spell.Name,
		ScrollEffect: spellID,
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s casts %s! (Slots: %d/%d)\n",
		char.Name, spell.Name, char.SpellSlots, char.MaxSpellSlots))
	sb.WriteString(s.applyScrollEffect(char, syntheticItem, targetID))

	// Sync HP changes from heals to combatant
	combatant.HP = char.HP

	s.logEvent("spell", "combat_spell_cast", charID, string(char.Class), targetID, map[string]interface{}{
		"spell": spellID, "slots_remaining": char.SpellSlots,
	})
	s.incrementStat(charID, "spells_cast", 1)

	// Check combat end
	msg := s.checkCombatEnd(&sb)
	if msg != "" {
		return s.textResult(sb.String()), nil
	}

	// Advance turn after action
	sb.WriteString(s.advanceTurnAndRunMonsters())

	return s.textResult(sb.String()), nil
}

// handleCombatCantrip fires a free ranged arcane bolt (no spell slot cost)
func (s *Server) handleCombatCantrip(charID, targetID string) (*ToolResult, error) {
	if err := s.requireCombat(); err != nil {
		return err, nil
	}
	if s.state.Combat.AwaitingScoutDecision {
		return s.textResult(errAwaitingScoutDecision), nil
	}

	combatant, errResult := s.requirePlayerTurn(charID)
	if errResult != nil {
		return errResult, nil
	}

	if combatant.HasActed {
		return s.textResult(fmt.Sprintf("%s has already acted this turn.", combatant.Name)), nil
	}

	char, errResult := s.resolveCharacter(charID)
	if errResult != nil {
		return errResult, nil
	}

	if char.Class != game.ClassMagicUser {
		return s.textResult("Only a magic_user can use cantrip."), nil
	}

	target := s.state.Combat.GetCombatant(targetID)
	if target == nil || !target.IsAlive {
		return s.textResult("Target not found or already dead."), nil
	}
	if target.IsPlayerChar {
		return s.textResult("Cannot target your own party member."), nil
	}

	// Range check: Chebyshev distance <= 3
	dist := game.ChebyshevDistance(combatant.GridX, combatant.GridY, target.GridX, target.GridY)
	if dist > 3 {
		return s.textResult(fmt.Sprintf("Target is out of range! (Distance: %d, max range: 3)", dist)), nil
	}

	combatant.HasActed = true

	intMod := char.Intelligence / 2
	roll := s.rng.Intn(20) + 1 // d20
	total := roll + intMod

	// Target AC: base defense + any AC buffs
	targetAC := game.BaseDefense + target.GetACBonus()

	var sb strings.Builder
	isCrit := roll == 20
	hit := total >= targetAC || isCrit

	if hit {
		// Damage: 1d4 + INT/2, crit doubles dice to 2d4
		numDice := 1
		if isCrit {
			numDice = 2
		}
		damage := 0
		for i := 0; i < numDice; i++ {
			damage += s.rng.Intn(4) + 1
		}
		damage += intMod
		if damage < 1 {
			damage = 1
		}

		target.HP -= damage
		targetDied := false
		if target.HP <= 0 {
			target.HP = 0
			target.IsAlive = false
			targetDied = true
			s.state.Combat.RemoveFromGrid(target)
			game.RemoveEngagement(s.state.Combat, target.ID)
		}

		if isCrit {
			sb.WriteString(fmt.Sprintf("CRITICAL! %s blasts %s with an arcane bolt! ", combatant.Name, target.Name))
		} else {
			sb.WriteString(fmt.Sprintf("%s hits %s with an arcane bolt! ", combatant.Name, target.Name))
		}
		sb.WriteString(fmt.Sprintf("(d20:%d +%d = %d vs AC %d) %d damage. (%s HP: %d/%d)\n",
			roll, intMod, total, targetAC, damage, target.Name, target.HP, target.MaxHP))

		subtype := "cantrip_hit"
		if isCrit {
			subtype = "cantrip_crit"
		}
		s.logEvent("combat", subtype, charID, string(char.Class), targetID, map[string]interface{}{
			"roll": roll, "damage": damage, "was_critical": isCrit,
		})
		s.incrementStat(charID, "damage_dealt", damage)

		if targetDied {
			sb.WriteString(fmt.Sprintf("%s is destroyed!\n", target.Name))
			s.logEvent("death", "enemy_defeated", charID, string(char.Class), targetID, nil)
			s.incrementStat(charID, "kills", 1)
		}
	} else {
		sb.WriteString(fmt.Sprintf("%s's arcane bolt misses %s. (d20:%d +%d = %d vs AC %d)\n",
			combatant.Name, target.Name, roll, intMod, total, targetAC))
		s.logEvent("combat", "cantrip_miss", charID, string(char.Class), targetID, map[string]interface{}{
			"roll": roll,
		})
	}

	// Check combat end
	msg := s.checkCombatEnd(&sb)
	if msg != "" {
		return s.textResult(sb.String()), nil
	}

	sb.WriteString(s.advanceTurnAndRunMonsters())

	return s.textResult(sb.String()), nil
}

// --- Ambush Combat ---

// generateAmbushMonsters creates 1-3 monsters scaled to room depth for an ambush
func (s *Server) generateAmbushMonsters(depth int, roomID string) []*game.Monster {
	// Monster templates for ambush generation (subset of procgen templates)
	type ambushTemplate struct {
		Name        string
		Description string
		BaseHP      int
		BaseDamage  int
		BaseDex     int
		MinDiff     int
	}

	templates := []ambushTemplate{
		{Name: "Rat", Description: "A large, mangy rat with beady red eyes.", BaseHP: 5, BaseDamage: 2, BaseDex: 12, MinDiff: 0},
		{Name: "Goblin", Description: "A small, green-skinned creature with a wicked grin.", BaseHP: 10, BaseDamage: 4, BaseDex: 12, MinDiff: 1},
		{Name: "Skeleton", Description: "The animated bones of a long-dead warrior.", BaseHP: 15, BaseDamage: 5, BaseDex: 10, MinDiff: 2},
		{Name: "Orc", Description: "A hulking brute with tusks and a massive club.", BaseHP: 25, BaseDamage: 8, BaseDex: 10, MinDiff: 3},
		{Name: "Wraith", Description: "A shadowy figure that chills you to the bone.", BaseHP: 20, BaseDamage: 7, BaseDex: 16, MinDiff: 4},
	}

	// Filter eligible templates
	eligible := make([]ambushTemplate, 0)
	for _, t := range templates {
		if t.MinDiff <= depth {
			eligible = append(eligible, t)
		}
	}
	if len(eligible) == 0 {
		eligible = templates[:1] // fallback to rat
	}

	// 1-3 monsters based on depth
	numMonsters := 1
	if depth >= 4 && s.rng.Float32() < 0.4 {
		numMonsters = 3
	} else if depth >= 2 && s.rng.Float32() < 0.5 {
		numMonsters = 2
	}

	scaleFactor := 1.0 + float64(depth)*0.15

	monsters := make([]*game.Monster, 0, numMonsters)
	for i := 0; i < numMonsters; i++ {
		t := eligible[s.rng.Intn(len(eligible))]
		m := &game.Monster{
			ID:          fmt.Sprintf("ambush_%x", s.rng.Int63()),
			Name:        t.Name,
			Description: t.Description,
			HP:          int(float64(t.BaseHP) * scaleFactor),
			MaxHP:       int(float64(t.BaseHP) * scaleFactor),
			Damage:      int(float64(t.BaseDamage) * scaleFactor),
			Dexterity:   t.BaseDex,
			RoomID:      roomID,
			IsAlive:     true,
		}
		monsters = append(monsters, m)
	}

	return monsters
}

// enterAmbushCombat starts combat from an ambush with optional surprise round for monsters
func (s *Server) enterAmbushCombat(roomID string, monsters []*game.Monster, surpriseRound bool) string {
	// Use the existing InitCombat but with no sneak result (party didn't sneak)
	cs := game.InitCombat(s.state.Party, monsters, roomID, nil, s.state.Items, s.rng)
	s.state.Combat = cs
	s.state.Mode = game.ModeCombat
	s.retreated = make(map[string]bool)

	// If surprise round, reorder so all monsters go first
	if surpriseRound {
		playerCombatants := make([]*game.Combatant, 0)
		enemyCombatants := make([]*game.Combatant, 0)
		for _, c := range cs.Combatants {
			if c.IsPlayerChar {
				playerCombatants = append(playerCombatants, c)
			} else {
				enemyCombatants = append(enemyCombatants, c)
			}
		}
		// Enemies first, then players
		cs.Combatants = append(enemyCombatants, playerCombatants...)
		cs.CurrentTurnIdx = 0
	}

	var sb strings.Builder
	sb.WriteString("\n=== AMBUSH COMBAT ===\n\n")
	if surpriseRound {
		sb.WriteString("The monsters catch you off guard! They get a surprise round!\n\n")
	}

	sb.WriteString("Initiative order:\n")
	for _, c := range cs.Combatants {
		side := "ALLY"
		if !c.IsPlayerChar {
			side = "ENEMY"
		}
		sb.WriteString(fmt.Sprintf("  %s [%s] — Init: %d, HP: %d/%d, Pos: (%d,%d)\n",
			c.Name, side, c.Initiative, c.HP, c.MaxHP, c.GridX, c.GridY))
	}

	// Run monster turns if they go first (surprise round or normal initiative)
	current := cs.GetCurrentCombatant()
	if current != nil && !current.IsPlayerChar {
		sb.WriteString(s.runMonsterTurns())
	} else if current != nil {
		sb.WriteString(fmt.Sprintf("\n%s's turn! Use combat tools to respond.\n", current.Name))
	}

	return sb.String()
}

// --- Combat helpers ---

// enterCombat transitions from exploration to combat mode
func (s *Server) enterCombat(previousRoomID string, sneakResult *game.SneakResult) string {
	currentRoom := s.state.GetCurrentRoom()
	if currentRoom == nil {
		return ""
	}

	monsters := s.state.GetRoomMonsters(currentRoom.ID)
	if len(monsters) == 0 {
		return ""
	}

	cs := game.InitCombat(s.state.Party, monsters, previousRoomID, sneakResult, s.state.Items, s.rng)
	s.state.Combat = cs
	s.state.Mode = game.ModeCombat
	s.retreated = make(map[string]bool)

	var sb strings.Builder
	sb.WriteString("\n=== COMBAT BEGINS ===\n\n")
	sb.WriteString("Initiative order:\n")
	for _, c := range cs.Combatants {
		side := "ALLY"
		if !c.IsPlayerChar {
			side = "ENEMY"
		}
		sb.WriteString(fmt.Sprintf("  %s [%s] — Init: %d, HP: %d/%d, Pos: (%d,%d)",
			c.Name, side, c.Initiative, c.HP, c.MaxHP, c.GridX, c.GridY))
		if c.IsHidden {
			sb.WriteString(" [HIDDEN]")
		}
		sb.WriteString("\n")
	}

	// If first turn is a monster, run monster turns
	current := cs.GetCurrentCombatant()
	if current != nil && !current.IsPlayerChar {
		sb.WriteString(s.runMonsterTurns())
	} else if current != nil {
		sb.WriteString(fmt.Sprintf("\n%s's turn! Use combat_move, combat_attack, or other combat tools.\n", current.Name))
	}

	return sb.String()
}

// advanceTurnAndRunMonsters advances the turn and processes monster turns until a player's turn
func (s *Server) advanceTurnAndRunMonsters() string {
	if !s.state.InCombat() {
		return ""
	}

	cs := s.state.Combat

	// During scout phase, after the thief acts, set awaiting decision instead of advancing
	if cs.IsScoutPhase {
		cs.AwaitingScoutDecision = true
		return "\n\nSurprise round complete. Choose: 'signal_party' to bring the party in, or 'combat_retreat' to pull back."
	}

	cs.AdvanceTurn()

	var sb strings.Builder

	// Check combat end after advancing
	msg := s.checkCombatEnd(&sb)
	if msg != "" {
		return sb.String()
	}

	// Run monster turns until it's a player's turn
	sb.WriteString(s.runMonsterTurns())

	return sb.String()
}

// runMonsterTurns processes all consecutive monster turns
func (s *Server) runMonsterTurns() string {
	if !s.state.InCombat() {
		return ""
	}

	cs := s.state.Combat

	// Monsters are frozen during scout phase
	if cs.IsScoutPhase {
		return ""
	}

	var sb strings.Builder

	for {
		current := cs.GetCurrentCombatant()
		if current == nil || !cs.IsActive {
			break
		}

		// Sleeping player: auto-skip their turn
		if current.IsPlayerChar && current.IsSleeping() {
			sb.WriteString(fmt.Sprintf("\n%s is asleep and cannot act!", current.Name))
			cs.AdvanceTurn()
			continue
		}

		// Stop when we reach an awake player's turn
		if current.IsPlayerChar {
			break
		}

		// Run monster AI
		monster, ok := s.state.Monsters[current.CharacterID]
		if !ok || !current.IsAlive {
			cs.AdvanceTurn()
			continue
		}

		action := game.ExecuteMonsterTurn(cs, current, monster, s.rng)
		sb.WriteString("\n" + action.Message)

		// Log monster attack event
		if action.AttackResult != nil {
			mSubtype := "attack_miss"
			if action.AttackResult.Hit {
				mSubtype = "attack_hit"
			}
			s.logEvent("combat", "monster_"+mSubtype, current.CharacterID, monster.Name, action.AttackResult.TargetID, map[string]interface{}{
				"roll":   action.AttackResult.Roll,
				"damage": action.AttackResult.Damage,
			})
		}

		// Sync damage to party characters
		if action.AttackResult != nil && action.AttackResult.Hit {
			// Track damage taken
			targetCombatant := cs.GetCombatant(action.AttackResult.TargetID)
			if targetCombatant != nil {
				charTarget := s.state.Party.GetCharacter(targetCombatant.CharacterID)
				if charTarget != nil {
					s.incrementStat(charTarget.ID, "damage_taken", action.AttackResult.Damage)
					charTarget.HP = targetCombatant.HP
				}
			}
		}
		if action.AttackResult != nil && action.AttackResult.TargetDied {
			char := s.state.Party.GetCharacter(action.AttackResult.TargetID)
			if char != nil {
				char.HP = 0
				char.TakeDamage(1) // trigger death
				s.logEvent("death", "character_killed", action.AttackResult.TargetID, string(char.Class), current.CharacterID, map[string]interface{}{
					"cause": "monster_attack", "killer": monster.Name,
				})
			}
		}

		// Check combat end
		endMsg := s.checkCombatEnd(&sb)
		if endMsg != "" {
			return sb.String()
		}

		cs.AdvanceTurn()
	}

	// Announce next player's turn
	current := cs.GetCurrentCombatant()
	if current != nil && current.IsPlayerChar && cs.IsActive {
		sb.WriteString(fmt.Sprintf("\n\n%s's turn! [ID: %s] Pos: (%d,%d)",
			current.Name, current.ID, current.GridX, current.GridY))
		if current.HasMoved {
			sb.WriteString(" [already moved]")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// checkCombatEnd checks if combat should end and handles the transition
func (s *Server) checkCombatEnd(sb *strings.Builder) string {
	if !s.state.InCombat() {
		return ""
	}

	result := game.ResolveCombatEnd(s.state.Combat)
	if result == nil {
		return ""
	}

	sb.WriteString("\n" + result.Message + "\n")

	if result.PartyWiped {
		s.state.GameOver = true
		game.SyncCombatToCharacters(s.state.Combat, s.state.Party)
		s.logEvent("death", "party_wipe", "", "", "", nil)
		s.finalizeSession("party_wipe")
		sb.WriteString("Use 'new_game' to play again.\n")
	}

	if result.AllEnemiesDead {
		// During scout phase, move the full party to the scout's room before ending
		if s.state.Combat != nil && s.state.Combat.IsScoutPhase {
			scout := s.state.Party.GetCharacter(s.state.Combat.ScoutID)
			if scout != nil {
				s.state.Party.MoveAllToRoom(scout.CurrentRoomID)
			}
		}
		// Recover 1 spell slot for alive mages on combat victory
		for _, ch := range s.state.Party.Characters {
			if ch.IsAlive && ch.Class == game.ClassMagicUser && ch.SpellSlots < ch.MaxSpellSlots {
				ch.SpellSlots++
				sb.WriteString(fmt.Sprintf("%s recovers a spell slot from the victory. (%d/%d)\n", ch.Name, ch.SpellSlots, ch.MaxSpellSlots))
			}
		}
		s.logEvent("combat", "combat_victory", "", "", "", nil)
		s.endCombat()
		// HP progression: all alive characters gain +2 max HP on combat victory
		for _, ch := range s.state.Party.Characters {
			if ch.IsAlive {
				ch.MaxHP += 2
				ch.HP += 2
				sb.WriteString(fmt.Sprintf("%s grows stronger! MaxHP increased to %d. (HP: %d/%d)\n",
					ch.Name, ch.MaxHP, ch.HP, ch.MaxHP))
			}
		}
		sb.WriteString("You may now explore or move on.\n")
	}

	return result.Message
}

// endCombat transitions back to exploration mode and syncs state
func (s *Server) endCombat() {
	if s.state.Combat == nil {
		return
	}

	// Sync combat HP back to characters and monsters
	game.SyncCombatToCharacters(s.state.Combat, s.state.Party)
	game.SyncCombatToMonsters(s.state.Combat, s.state.Monsters)

	s.state.Combat = nil
	s.state.Mode = game.ModeExploration
	s.retreated = nil
}

// renderCombatGrid produces a text rendering of the combat grid
func renderCombatGrid(cs *game.CombatState) string {
	var sb strings.Builder

	sb.WriteString("   0  1  2  3  4  5\n")
	for y := game.GridHeight - 1; y >= 0; y-- {
		sb.WriteString(fmt.Sprintf("%d ", y))
		for x := 0; x < game.GridWidth; x++ {
			cell := cs.Grid[y][x]
			if cell.OccupantID != nil {
				// Find combatant
				c := cs.GetCombatant(*cell.OccupantID)
				if c != nil {
					if c.IsPlayerChar {
						// Use first letter of name
						sb.WriteString(fmt.Sprintf("[%c]", c.Name[0]))
					} else {
						sb.WriteString(fmt.Sprintf("{%c}", c.Name[0]))
					}
				} else {
					sb.WriteString("[?]")
				}
			} else if cell.IsBlocked {
				sb.WriteString("[X]")
			} else {
				sb.WriteString(" . ")
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n[X] = Party  {X} = Enemy  . = Empty\n")
	return sb.String()
}

// --- Scout Ahead ---

// handleScoutAhead has a thief enter an adjacent room solo for a surprise round
func (s *Server) handleScoutAhead(charID, direction string) (*ToolResult, error) {
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

	// Can't leave if monsters are alive in current room
	if s.state.HasMonstersInRoom(currentRoom.ID) {
		return s.textResult("Cannot scout — enemies are still in this room!"), nil
	}

	exits := s.state.GetRoomExits(currentRoom.ID)
	targetRoomID, ok := exits[direction]
	if !ok {
		return s.textResult(fmt.Sprintf("Cannot scout %s — no exit in that direction.", direction)), nil
	}

	// Check target room has monsters
	monsters := s.state.GetRoomMonsters(targetRoomID)
	if len(monsters) == 0 {
		return s.textResult("No enemies in that room. Use 'move' instead."), nil
	}

	targetRoom := s.state.Rooms[targetRoomID]
	previousRoomID := currentRoom.ID

	s.state.ResetTurnContext()

	// Sneak check (same DC as existing sneak)
	difficulty := generator.GetRoomDifficulty(targetRoom)
	sneakResult := game.CheckRoomSneak(char, difficulty, s.rng)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s creeps into the room to the %s...\n\n", char.Name, direction))
	sb.WriteString(fmt.Sprintf("%s\n", game.FormatSneakResult(sneakResult)))

	subtype := "scout_ahead_fail"
	if sneakResult.Success {
		subtype = "scout_ahead_success"
		s.incrementStat(charID, "successful_sneaks", 1)
	}
	s.logEvent("stealth", subtype, charID, string(char.Class), targetRoomID, map[string]interface{}{
		"roll": sneakResult.Roll, "dc": sneakResult.DC,
	})

	if sneakResult.Success {
		// Move only the thief to target room
		char.CurrentRoomID = targetRoomID
		isFirstVisit := !s.state.IsRoomVisited(targetRoomID)
		s.state.MarkRoomVisited(targetRoomID)

		// Handle traps on first visit using solo detection
		if isFirstVisit {
			roomTraps := s.state.GetRoomTraps(targetRoomID)
			for _, trap := range roomTraps {
				if trap.Location == game.TrapRoom && !trap.IsTriggered && !trap.IsDisarmed {
					detection := game.CheckTrapDetectionSolo(char, trap, s.rng)
					if detection.Detected {
						sb.WriteString(fmt.Sprintf("\n%s\n", game.FormatTrapDetection(detection)))
					} else {
						// Trap triggers on the thief
						trigger := game.TriggerTrap(trap, char, s.rng)
						sb.WriteString(fmt.Sprintf("\n%s\n", game.FormatTrapTrigger(trigger)))
						if !char.IsAlive {
							s.state.GameOver = true
							s.finalizeSession("party_wipe")
							sb.WriteString("\nThe scout has fallen! Game over.\nUse 'new_game' to play again.")
							return s.textResult(sb.String()), nil
						}
					}
				}
			}
			// Chest trap detection (passive only — chests don't trigger on entry)
			for _, trap := range roomTraps {
				if trap.Location == game.TrapChest && !trap.IsTriggered && !trap.IsDisarmed && trap.Damage > 0 {
					detection := game.CheckTrapDetectionSolo(char, trap, s.rng)
					if detection.Detected {
						sb.WriteString(fmt.Sprintf("\n%s\n", game.FormatTrapDetection(detection)))
					}
				}
			}
		}

		// Init scout combat
		cs := game.InitScoutCombat(char, monsters, previousRoomID, s.state.Items, s.rng)
		s.state.Combat = cs
		s.state.Mode = game.ModeCombat
		s.retreated = make(map[string]bool)

		sb.WriteString(fmt.Sprintf("\n=== SURPRISE ROUND ===\n"))
		sb.WriteString(fmt.Sprintf("%s enters alone! Enemies are unaware.\n\n", char.Name))

		sb.WriteString(fmt.Sprintf("Enemies (%d):\n", len(monsters)))
		for _, m := range monsters {
			sb.WriteString(fmt.Sprintf("  - %s (HP: %d/%d)\n", m.Name, m.HP, m.MaxHP))
		}

		sb.WriteString(fmt.Sprintf("\n%s has a surprise round with double movement (%d cells)!\n",
			char.Name, char.GetMovementRange()*2))
		sb.WriteString("Use combat_move and combat_attack, then choose signal_party or combat_retreat.\n")
	} else {
		// Sneak failed: full party enters, normal combat
		sb.WriteString("\nDetected! The whole party rushes in!\n")
		s.state.Party.MoveAllToRoom(targetRoomID)
		isFirstVisit := !s.state.IsRoomVisited(targetRoomID)
		s.state.MarkRoomVisited(targetRoomID)

		// Handle traps on first visit (full party detection)
		if isFirstVisit {
			roomTraps := s.state.GetRoomTraps(targetRoomID)
			for _, trap := range roomTraps {
				if trap.Location == game.TrapRoom && !trap.IsTriggered && !trap.IsDisarmed {
					detection := game.CheckTrapDetection(s.state.Party, trap, s.rng)
					if detection.Detected {
						sb.WriteString(fmt.Sprintf("\n%s\n", game.FormatTrapDetection(detection)))
						// Detected trap still triggers on first non-thief
						victim := s.state.Party.FirstNonThief()
						if victim != nil {
							trigger := game.TriggerTrap(trap, victim, s.rng)
							sb.WriteString(fmt.Sprintf("\n%s\n", game.FormatTrapTrigger(trigger)))
							if s.state.Party.IsWiped() {
								s.state.GameOver = true
								s.finalizeSession("party_wipe")
								sb.WriteString("\nYour entire party has fallen. Game over.\nUse 'new_game' to play again.")
								return s.textResult(sb.String()), nil
							}
						}
					} else {
						point := s.state.Party.PointCharacter()
						if point != nil {
							trigger := game.TriggerTrap(trap, point, s.rng)
							sb.WriteString(fmt.Sprintf("\n%s\n", game.FormatTrapTrigger(trigger)))
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
			// Chest trap detection (passive only)
			for _, trap := range roomTraps {
				if trap.Location == game.TrapChest && !trap.IsTriggered && !trap.IsDisarmed && trap.Damage > 0 {
					detection := game.CheckTrapDetection(s.state.Party, trap, s.rng)
					if detection.Detected {
						sb.WriteString(fmt.Sprintf("\n%s\n", game.FormatTrapDetection(detection)))
					}
				}
			}
		}

		combatMsg := s.enterCombat(previousRoomID, nil)
		sb.WriteString(combatMsg)
	}

	return s.textResult(sb.String()), nil
}

// handleSignalParty brings the rest of the party into scout combat
func (s *Server) handleSignalParty(charID string) (*ToolResult, error) {
	if err := s.requireCombat(); err != nil {
		return err, nil
	}

	cs := s.state.Combat
	if !cs.AwaitingScoutDecision {
		return s.textResult("Not awaiting a scout decision. Use this after a scout_ahead surprise round."), nil
	}
	if charID != cs.ScoutID {
		return s.textResult("Only the scouting thief can signal the party."), nil
	}

	scout := s.state.Party.GetCharacter(charID)
	if scout == nil {
		return s.textResult("Scout not found."), nil
	}

	// Move all party to scout's room
	s.state.Party.MoveAllToRoom(scout.CurrentRoomID)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s signals the party! They rush into the room.\n\n", scout.Name))

	// Discovered-but-not-disarmed room traps trigger on first non-thief
	roomTraps := s.state.GetRoomTraps(scout.CurrentRoomID)
	for _, trap := range roomTraps {
		if trap.Location == game.TrapRoom && trap.IsDiscovered && !trap.IsTriggered && !trap.IsDisarmed {
			victim := s.state.Party.FirstNonThief()
			if victim != nil {
				trigger := game.TriggerTrap(trap, victim, s.rng)
				sb.WriteString(fmt.Sprintf("%s\n", game.FormatTrapTrigger(trigger)))
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
		}
	}

	// Add remaining party members to combat
	monsters := s.state.GetRoomMonsters(scout.CurrentRoomID)
	game.AddPartyToCombat(cs, s.state.Party, monsters, s.state.Items, s.rng)

	sb.WriteString("Initiative order:\n")
	for _, c := range cs.Combatants {
		if !c.IsAlive {
			continue
		}
		side := "ALLY"
		if !c.IsPlayerChar {
			side = "ENEMY"
		}
		sb.WriteString(fmt.Sprintf("  %s [%s] — Init: %d, HP: %d/%d, Pos: (%d,%d)\n",
			c.Name, side, c.Initiative, c.HP, c.MaxHP, c.GridX, c.GridY))
	}

	// Run monster turns if first combatant is a monster
	current := cs.GetCurrentCombatant()
	if current != nil && !current.IsPlayerChar {
		sb.WriteString(s.runMonsterTurns())
	} else if current != nil {
		sb.WriteString(fmt.Sprintf("\n%s's turn! Use combat tools.\n", current.Name))
	}

	return s.textResult(sb.String()), nil
}

// handleCombatCharge has a fighter charge toward an enemy with double movement and bonus damage
func (s *Server) handleCombatCharge(charID, targetID string) (*ToolResult, error) {
	if err := s.requireCombat(); err != nil {
		return err, nil
	}
	if s.state.Combat.AwaitingScoutDecision {
		return s.textResult(errAwaitingScoutDecision), nil
	}

	combatant, errResult := s.requirePlayerTurn(charID)
	if errResult != nil {
		return errResult, nil
	}

	if combatant.HasMoved || combatant.HasActed {
		return s.textResult("Charge requires a fresh turn — cannot have moved or acted yet."), nil
	}

	if combatant.HasCharged {
		return s.textResult("Charge can only be used once per combat."), nil
	}

	char, errResult := s.resolveCharacter(charID)
	if errResult != nil {
		return errResult, nil
	}

	if char.Class != game.ClassFighter {
		return s.textResult("Only a fighter can charge."), nil
	}

	target := s.state.Combat.GetCombatant(targetID)
	if target == nil || !target.IsAlive {
		return s.textResult("Target not found or already dead."), nil
	}
	if target.IsPlayerChar {
		return s.textResult("Cannot charge your own party member."), nil
	}

	moveRange := char.GetMovementRange() * 2
	destX, destY, ok := game.FindChargeDestination(s.state.Combat, combatant, target, moveRange)
	if !ok {
		return s.textResult("Target is too far to charge!"), nil
	}

	var sb strings.Builder

	// Move fighter to charge destination
	if destX != combatant.GridX || destY != combatant.GridY {
		s.state.Combat.PlaceOnGrid(combatant, destX, destY)
		sb.WriteString(fmt.Sprintf("%s charges to (%d,%d)! ", combatant.Name, destX, destY))
	} else {
		sb.WriteString(fmt.Sprintf("%s charges! ", combatant.Name))
	}

	combatant.HasMoved = true
	combatant.HasActed = true
	combatant.HasCharged = true

	// Get equipped weapon
	var weapon *game.Item
	if char.EquippedWeaponID != nil {
		weapon = s.state.Items[*char.EquippedWeaponID]
	}

	result := game.CombatAttack(s.state.Combat, combatant, target, char, weapon, s.rng)

	// Add charge bonus damage (+2)
	if result.Hit {
		result.Damage += 2
		target.HP -= 2
		if target.HP <= 0 && target.IsAlive {
			target.HP = 0
			target.IsAlive = false
			result.TargetDied = true
			s.state.Combat.RemoveFromGrid(target)
			game.RemoveEngagement(s.state.Combat, target.ID)
		}
		result.TargetHP = target.HP
	}

	sb.WriteString(game.FormatAttackResult(result))

	// Log events
	subtype := "charge_miss"
	if result.Hit {
		subtype = "charge_hit"
	}
	s.logEvent("combat", subtype, charID, string(char.Class), targetID, map[string]interface{}{
		"roll":   result.Roll,
		"damage": result.Damage,
	})
	if result.Hit {
		s.incrementStat(charID, "damage_dealt", result.Damage)
	}
	if result.TargetDied {
		s.logEvent("death", "enemy_defeated", charID, string(char.Class), targetID, nil)
		s.incrementStat(charID, "kills", 1)
	}

	// Cleave check after charge kill
	if result.TargetDied && (weapon == nil || weapon.Range != game.RangeRanged) {
		var cleaveTarget *game.Combatant
		for _, c := range s.state.Combat.GetEnemyCombatants() {
			if c.IsAlive && game.IsAdjacent(target.GridX, target.GridY, c.GridX, c.GridY) {
				cleaveTarget = c
				break
			}
		}
		if cleaveTarget != nil {
			cleaveResult := game.CombatAttack(s.state.Combat, combatant, cleaveTarget, char, weapon, s.rng)
			sb.WriteString("\nCleave! " + game.FormatAttackResult(cleaveResult))
			s.logEvent("combat", "cleave", charID, string(char.Class), cleaveTarget.ID, map[string]interface{}{
				"roll":   cleaveResult.Roll,
				"damage": cleaveResult.Damage,
			})
			if cleaveResult.Hit {
				s.incrementStat(charID, "damage_dealt", cleaveResult.Damage)
			}
			if cleaveResult.TargetDied {
				s.logEvent("death", "enemy_defeated", charID, string(char.Class), cleaveTarget.ID, nil)
				s.incrementStat(charID, "kills", 1)
			}
		}
	}

	// Check combat end
	msg := s.checkCombatEnd(&sb)
	if msg != "" {
		return s.textResult(sb.String()), nil
	}

	sb.WriteString(s.advanceTurnAndRunMonsters())

	return s.textResult(sb.String()), nil
}

// handleCombatProtect has a fighter guard an adjacent ally, redirecting attacks to themselves
func (s *Server) handleCombatProtect(charID, targetID string) (*ToolResult, error) {
	if err := s.requireCombat(); err != nil {
		return err, nil
	}
	if s.state.Combat.AwaitingScoutDecision {
		return s.textResult(errAwaitingScoutDecision), nil
	}

	combatant, errResult := s.requirePlayerTurn(charID)
	if errResult != nil {
		return errResult, nil
	}

	if combatant.HasActed {
		return s.textResult(fmt.Sprintf("%s has already acted this turn.", combatant.Name)), nil
	}

	char, errResult := s.resolveCharacter(charID)
	if errResult != nil {
		return errResult, nil
	}

	if char.Class != game.ClassFighter {
		return s.textResult("Only a fighter can protect allies."), nil
	}

	if targetID == charID {
		return s.textResult("Cannot protect yourself."), nil
	}

	targetCombatant := s.state.Combat.GetCombatant(targetID)
	if targetCombatant == nil || !targetCombatant.IsAlive {
		return s.textResult("Target ally not found or dead."), nil
	}
	if !targetCombatant.IsPlayerChar {
		return s.textResult("Can only protect party members."), nil
	}

	if !game.IsAdjacent(combatant.GridX, combatant.GridY, targetCombatant.GridX, targetCombatant.GridY) {
		return s.textResult(fmt.Sprintf("%s must be adjacent to %s to protect them.", combatant.Name, targetCombatant.Name)), nil
	}

	targetCombatant.ProtectedBy = combatant.ID
	combatant.HasActed = true

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s shields %s, ready to intercept attacks!\n", combatant.Name, targetCombatant.Name))

	s.logEvent("combat", "protect", charID, string(char.Class), targetID, nil)

	sb.WriteString(s.advanceTurnAndRunMonsters())

	return s.textResult(sb.String()), nil
}
