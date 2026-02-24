package mcp

import (
	"fmt"
	"log"
	"math/rand"

	"github.com/lehmann314159/yrpg-backend/internal/db"
	"github.com/lehmann314159/yrpg-backend/internal/game"
	"github.com/lehmann314159/yrpg-backend/internal/generator"
)

// Server implements the MCP protocol for the dungeon crawler
type Server struct {
	state     *game.GameState
	rng       *rand.Rand
	retreated map[string]bool // tracks which combatants have successfully retreated
	db        *db.DB          // database for persistence and analytics
}

// NewServer creates a new MCP server instance
func NewServer(database *db.DB) *Server {
	return &Server{db: database}
}

// logEvent is a fire-and-forget event logger that doesn't block gameplay
func (s *Server) logEvent(eventType, eventSubtype, actorID, actorClass, targetID string, details interface{}) {
	if s.db == nil || s.state == nil {
		return
	}
	roomID := ""
	if room := s.state.GetCurrentRoom(); room != nil {
		roomID = room.ID
	}
	turnNumber := 0
	if s.state.TurnContext != nil {
		turnNumber = s.state.TurnContext.TurnNumber
	}
	if err := s.db.LogEvent(s.state.SessionID, turnNumber, eventType, eventSubtype,
		actorID, actorClass, targetID, roomID, details); err != nil {
		log.Printf("Failed to log event: %v", err)
	}
}

// incrementStat is a fire-and-forget stat incrementer
func (s *Server) incrementStat(charID, field string, amount int) {
	if s.db == nil {
		return
	}
	if err := s.db.IncrementCharacterStat(charID, field, amount); err != nil {
		log.Printf("Failed to increment stat: %v", err)
	}
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// ToolResult represents the result of a tool call
type ToolResult struct {
	Content   []ContentBlock              `json:"content"`
	IsError   bool                        `json:"isError,omitempty"`
	GameState *game.GameStateSnapshot     `json:"gameState,omitempty"`
}

// ContentBlock represents a content block in the result
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Error messages
const (
	errNoGame          = "No game in progress. Use 'new_game' to start."
	errGameOver        = "Game is over. Use 'new_game' to play again."
	errVictory         = "You have escaped the dungeon! Victory! Use 'new_game' to play again."
	errPartyWiped      = "Your party has been wiped out. Use 'new_game' to play again."
	errInCombat        = "You are in combat! Use combat tools (combat_move, combat_attack, etc.)."
	errNotInCombat     = "You are not in combat."
	errNotYourTurn     = "It's not that character's turn."
	errInvalidCharID   = "Invalid character_id. Use 'stats' to see your party."
	errCharacterDead   = "That character is dead."
)

// requireActiveGame checks if a game is in progress and not over.
func (s *Server) requireActiveGame() *ToolResult {
	if s.state == nil || !s.state.IsInitialized() {
		return &ToolResult{Content: []ContentBlock{{Type: "text", Text: errNoGame}}}
	}
	if s.state.GameOver {
		if s.state.Victory {
			return &ToolResult{Content: []ContentBlock{{Type: "text", Text: errVictory}}}
		}
		return &ToolResult{Content: []ContentBlock{{Type: "text", Text: errPartyWiped}}}
	}
	return nil
}

// requireExploration checks for active game in exploration mode
func (s *Server) requireExploration() *ToolResult {
	if err := s.requireActiveGame(); err != nil {
		return err
	}
	if s.state.InCombat() {
		return &ToolResult{Content: []ContentBlock{{Type: "text", Text: errInCombat}}}
	}
	return nil
}

// requireCombat checks for active game in combat mode
func (s *Server) requireCombat() *ToolResult {
	if err := s.requireActiveGame(); err != nil {
		return err
	}
	if !s.state.InCombat() {
		return &ToolResult{Content: []ContentBlock{{Type: "text", Text: errNotInCombat}}}
	}
	return nil
}

// requirePlayerTurn checks that it's a player character's turn and returns the combatant
func (s *Server) requirePlayerTurn(charID string) (*game.Combatant, *ToolResult) {
	current := s.state.Combat.GetCurrentCombatant()
	if current == nil {
		return nil, &ToolResult{Content: []ContentBlock{{Type: "text", Text: "No current combatant."}}}
	}
	if !current.IsPlayerChar {
		return nil, &ToolResult{Content: []ContentBlock{{Type: "text", Text: "It's the enemy's turn. Wait for monsters to act."}}}
	}
	if current.ID != charID {
		return nil, &ToolResult{Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("It's %s's turn, not that character's.", current.Name)}}}
	}
	return current, nil
}

// requireInitialized checks only if a game is initialized (for read-only operations)
func (s *Server) requireInitialized() *ToolResult {
	if s.state == nil || !s.state.IsInitialized() {
		return &ToolResult{Content: []ContentBlock{{Type: "text", Text: errNoGame}}}
	}
	return nil
}

// resolveCharacter finds a character by ID and validates they're alive
func (s *Server) resolveCharacter(charID string) (*game.Character, *ToolResult) {
	if s.state.Party == nil {
		return nil, &ToolResult{Content: []ContentBlock{{Type: "text", Text: errInvalidCharID}}}
	}
	c := s.state.Party.GetCharacter(charID)
	if c == nil {
		return nil, &ToolResult{Content: []ContentBlock{{Type: "text", Text: errInvalidCharID}}}
	}
	if !c.IsAlive {
		return nil, &ToolResult{Content: []ContentBlock{{Type: "text", Text: errCharacterDead}}}
	}
	return c, nil
}

// textResult creates a simple text ToolResult with game state snapshot
func (s *Server) textResult(msg string) *ToolResult {
	return &ToolResult{
		Content:   []ContentBlock{{Type: "text", Text: msg}},
		GameState: s.state.BuildSnapshot(),
	}
}

// errorResult creates an error ToolResult
func (s *Server) errorResult(msg string) *ToolResult {
	return &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: msg}},
		IsError: true,
	}
}

// ListTools returns all available MCP tools
func (s *Server) ListTools() []Tool {
	return []Tool{
		// Party & Game Management
		{
			Name:        "new_game",
			Description: "Start a new game with a party of 1-3 characters",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"characters": map[string]interface{}{
						"type":        "array",
						"description": "Array of character definitions [{name, class}]. Classes: fighter, magic_user, thief. Max 3.",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"name":  map[string]interface{}{"type": "string", "description": "Character name"},
								"class": map[string]interface{}{"type": "string", "description": "Character class: fighter, magic_user, or thief"},
							},
							"required": []string{"name", "class"},
						},
					},
				},
				"required": []string{"characters"},
			},
		},

		// Exploration Tools
		{
			Name:        "look",
			Description: "Examine the current room — see exits, monsters, items, and discovered traps",
			InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
		{
			Name:        "move",
			Description: "Move the party in a direction (north, south, east, west). Triggers room entry sequence: sneak check, trap check, enemy check.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"direction": map[string]interface{}{
						"type": "string",
						"description": "Direction to move",
						"enum": []string{"north", "south", "east", "west"},
					},
				},
				"required": []string{"direction"},
			},
		},
		{
			Name:        "take",
			Description: "Have a character pick up an item from the current room",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"character_id": map[string]interface{}{"type": "string", "description": "ID of the character picking up the item"},
					"item_id":      map[string]interface{}{"type": "string", "description": "ID of the item to pick up"},
				},
				"required": []string{"character_id", "item_id"},
			},
		},
		{
			Name:        "equip",
			Description: "Equip a weapon or armor from a character's inventory (enforces class restrictions)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"character_id": map[string]interface{}{"type": "string", "description": "ID of the character equipping the item"},
					"item_id":      map[string]interface{}{"type": "string", "description": "ID of the item to equip"},
				},
				"required": []string{"character_id", "item_id"},
			},
		},
		{
			Name:        "use_item",
			Description: "Use a consumable or scroll from a character's inventory",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"character_id": map[string]interface{}{"type": "string", "description": "ID of the character using the item"},
					"item_id":      map[string]interface{}{"type": "string", "description": "ID of the item to use"},
					"target_id":    map[string]interface{}{"type": "string", "description": "ID of the target character (for healing/resurrection scrolls)"},
				},
				"required": []string{"character_id", "item_id"},
			},
		},
		{
			Name:        "open_chest",
			Description: "Open a chest in the current room (may trigger chest trap)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"character_id": map[string]interface{}{"type": "string", "description": "ID of the character opening the chest"},
				},
				"required": []string{"character_id"},
			},
		},
		{
			Name:        "disarm_trap",
			Description: "Have a character attempt to disarm a discovered trap (thieves get +6 bonus)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"character_id": map[string]interface{}{"type": "string", "description": "ID of the character attempting to disarm"},
					"trap_id":      map[string]interface{}{"type": "string", "description": "ID of the discovered trap to disarm"},
				},
				"required": []string{"character_id", "trap_id"},
			},
		},
		{
			Name:        "sneak",
			Description: "Thief scouts an adjacent room without entering — reveals enemies and traps",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"character_id": map[string]interface{}{"type": "string", "description": "ID of the thief character"},
					"direction":    map[string]interface{}{"type": "string", "description": "Direction to scout", "enum": []string{"north", "south", "east", "west"}},
				},
				"required": []string{"character_id", "direction"},
			},
		},
		{
			Name:        "inventory",
			Description: "View all party members' inventories and equipment",
			InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
		{
			Name:        "stats",
			Description: "View all party member stats, equipment, and status",
			InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
		{
			Name:        "map",
			Description: "View the dungeon map showing explored areas",
			InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},

		// Persistence Tools
		{
			Name:        "save_game",
			Description: "Save the current game state",
			InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
		{
			Name:        "load_game",
			Description: "Load a saved game by session ID",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{"type": "string", "description": "Session ID of the game to load"},
				},
				"required": []string{"session_id"},
			},
		},

		// Spell & Rest Tools
		{
			Name:        "cast_spell",
			Description: "Cast a known spell outside combat (non-combat-only spells like heal, resurrect)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"character_id": map[string]interface{}{"type": "string", "description": "ID of the magic_user casting the spell"},
					"spell_id":     map[string]interface{}{"type": "string", "description": "Spell to cast (e.g. heal, resurrect)"},
					"target_id":    map[string]interface{}{"type": "string", "description": "Target character ID (optional, defaults to self)"},
				},
				"required": []string{"character_id", "spell_id"},
			},
		},
		{
			Name:        "combat_cast_spell",
			Description: "Cast a known spell during combat (any spell)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"character_id": map[string]interface{}{"type": "string", "description": "ID of the magic_user casting the spell"},
					"spell_id":     map[string]interface{}{"type": "string", "description": "Spell to cast"},
					"target_id":    map[string]interface{}{"type": "string", "description": "Target ID (character for heals, enemy combatant for attacks)"},
				},
				"required": []string{"character_id", "spell_id"},
			},
		},
		{
			Name:        "rest",
			Description: "Rest in a cleared room to recover HP and spell slots. Risk of ambush by wandering monsters.",
			InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},

		// Combat Tools
		{
			Name:        "combat_status",
			Description: "View combat grid, positions, initiative order, and HP",
			InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		},
		{
			Name:        "combat_move",
			Description: "Move a character on the combat grid (movement phase)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"character_id": map[string]interface{}{"type": "string", "description": "ID of the character to move"},
					"x":            map[string]interface{}{"type": "number", "description": "Target X position (0-5)"},
					"y":            map[string]interface{}{"type": "number", "description": "Target Y position (0-5)"},
				},
				"required": []string{"character_id", "x", "y"},
			},
		},
		{
			Name:        "combat_attack",
			Description: "Attack a target (melee if adjacent, ranged if weapon equipped and in range)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"character_id": map[string]interface{}{"type": "string", "description": "ID of the attacking character"},
					"target_id":    map[string]interface{}{"type": "string", "description": "ID of the target to attack"},
				},
				"required": []string{"character_id", "target_id"},
			},
		},
		{
			Name:        "combat_use_item",
			Description: "Use an item or scroll during combat",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"character_id": map[string]interface{}{"type": "string", "description": "ID of the character using the item"},
					"item_id":      map[string]interface{}{"type": "string", "description": "ID of the item to use"},
					"target_id":    map[string]interface{}{"type": "string", "description": "Optional target character ID"},
				},
				"required": []string{"character_id", "item_id"},
			},
		},
		{
			Name:        "combat_defend",
			Description: "Take a full defense stance (+4 AC this round)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"character_id": map[string]interface{}{"type": "string", "description": "ID of the defending character"},
				},
				"required": []string{"character_id"},
			},
		},
		{
			Name:        "combat_hide",
			Description: "Thief attempts to hide in combat (breaks engagement, next attack has advantage)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"character_id": map[string]interface{}{"type": "string", "description": "ID of the thief attempting to hide"},
				},
				"required": []string{"character_id"},
			},
		},
		{
			Name:        "combat_retreat",
			Description: "Attempt to flee combat (triggers opportunity attacks from engaged enemies)",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"character_id": map[string]interface{}{"type": "string", "description": "ID of the character retreating"},
				},
				"required": []string{"character_id"},
			},
		},
		{
			Name:        "end_turn",
			Description: "End the current character's turn without acting",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"character_id": map[string]interface{}{"type": "string", "description": "ID of the character ending their turn"},
				},
				"required": []string{"character_id"},
			},
		},
	}
}

// CallTool dispatches an MCP tool call
func (s *Server) CallTool(name string, arguments map[string]interface{}) (*ToolResult, error) {
	switch name {
	case "new_game":
		return s.handleNewGame(arguments)
	case "look":
		return s.handleLook()
	case "move":
		dir, _ := arguments["direction"].(string)
		if dir == "" {
			return nil, fmt.Errorf("direction is required")
		}
		return s.handleMove(dir)
	case "take":
		charID, _ := arguments["character_id"].(string)
		itemID, _ := arguments["item_id"].(string)
		if charID == "" || itemID == "" {
			return nil, fmt.Errorf("character_id and item_id are required")
		}
		return s.handleTake(charID, itemID)
	case "equip":
		charID, _ := arguments["character_id"].(string)
		itemID, _ := arguments["item_id"].(string)
		if charID == "" || itemID == "" {
			return nil, fmt.Errorf("character_id and item_id are required")
		}
		return s.handleEquip(charID, itemID)
	case "use_item":
		charID, _ := arguments["character_id"].(string)
		itemID, _ := arguments["item_id"].(string)
		targetID, _ := arguments["target_id"].(string)
		if charID == "" || itemID == "" {
			return nil, fmt.Errorf("character_id and item_id are required")
		}
		return s.handleUseItem(charID, itemID, targetID)
	case "open_chest":
		charID, _ := arguments["character_id"].(string)
		if charID == "" {
			return nil, fmt.Errorf("character_id is required")
		}
		return s.handleOpenChest(charID)
	case "disarm_trap":
		charID, _ := arguments["character_id"].(string)
		trapID, _ := arguments["trap_id"].(string)
		if charID == "" || trapID == "" {
			return nil, fmt.Errorf("character_id and trap_id are required")
		}
		return s.handleDisarmTrap(charID, trapID)
	case "sneak":
		charID, _ := arguments["character_id"].(string)
		dir, _ := arguments["direction"].(string)
		if charID == "" || dir == "" {
			return nil, fmt.Errorf("character_id and direction are required")
		}
		return s.handleSneak(charID, dir)
	case "inventory":
		return s.handleInventory()
	case "stats":
		return s.handleStats()
	case "map":
		return s.handleMap()
	case "save_game":
		return s.handleSaveGame()
	case "load_game":
		sessionID, _ := arguments["session_id"].(string)
		if sessionID == "" {
			return nil, fmt.Errorf("session_id is required")
		}
		return s.handleLoadGame(sessionID)

	// Spell & Rest tools
	case "cast_spell":
		charID, _ := arguments["character_id"].(string)
		spellID, _ := arguments["spell_id"].(string)
		targetID, _ := arguments["target_id"].(string)
		if charID == "" || spellID == "" {
			return nil, fmt.Errorf("character_id and spell_id are required")
		}
		return s.handleCastSpell(charID, spellID, targetID)
	case "combat_cast_spell":
		charID, _ := arguments["character_id"].(string)
		spellID, _ := arguments["spell_id"].(string)
		targetID, _ := arguments["target_id"].(string)
		if charID == "" || spellID == "" {
			return nil, fmt.Errorf("character_id and spell_id are required")
		}
		return s.handleCombatCastSpell(charID, spellID, targetID)
	case "rest":
		return s.handleRest()

	// Combat tools
	case "combat_status":
		return s.handleCombatStatus()
	case "combat_move":
		charID, _ := arguments["character_id"].(string)
		xf, _ := arguments["x"].(float64)
		yf, _ := arguments["y"].(float64)
		if charID == "" {
			return nil, fmt.Errorf("character_id is required")
		}
		return s.handleCombatMove(charID, int(xf), int(yf))
	case "combat_attack":
		charID, _ := arguments["character_id"].(string)
		targetID, _ := arguments["target_id"].(string)
		if charID == "" || targetID == "" {
			return nil, fmt.Errorf("character_id and target_id are required")
		}
		return s.handleCombatAttack(charID, targetID)
	case "combat_use_item":
		charID, _ := arguments["character_id"].(string)
		itemID, _ := arguments["item_id"].(string)
		targetID, _ := arguments["target_id"].(string)
		if charID == "" || itemID == "" {
			return nil, fmt.Errorf("character_id and item_id are required")
		}
		return s.handleCombatUseItem(charID, itemID, targetID)
	case "combat_defend":
		charID, _ := arguments["character_id"].(string)
		if charID == "" {
			return nil, fmt.Errorf("character_id is required")
		}
		return s.handleCombatDefend(charID)
	case "combat_hide":
		charID, _ := arguments["character_id"].(string)
		if charID == "" {
			return nil, fmt.Errorf("character_id is required")
		}
		return s.handleCombatHide(charID)
	case "combat_retreat":
		charID, _ := arguments["character_id"].(string)
		if charID == "" {
			return nil, fmt.Errorf("character_id is required")
		}
		return s.handleCombatRetreat(charID)
	case "end_turn":
		charID, _ := arguments["character_id"].(string)
		if charID == "" {
			return nil, fmt.Errorf("character_id is required")
		}
		return s.handleEndTurn(charID)

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// getExplorationPct calculates percentage of dungeon explored
func (s *Server) getExplorationPct() float64 {
	if len(s.state.Rooms) == 0 {
		return 0
	}
	visited := 0
	for roomID := range s.state.Rooms {
		if s.state.IsRoomVisited(roomID) {
			visited++
		}
	}
	return float64(visited) / float64(len(s.state.Rooms)) * 100
}

// gridSize exposes the generator grid size for map rendering
func gridSize() int {
	return generator.GridSize
}