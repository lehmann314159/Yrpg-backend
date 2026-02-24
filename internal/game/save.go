package game

import (
	"encoding/json"
	"fmt"
)

// SaveState is the JSON-serializable representation of GameState
type SaveState struct {
	SessionID    string                      `json:"session_id"`
	Mode         CombatMode                  `json:"mode"`
	Party        *Party                      `json:"party"`
	Dungeon      *Dungeon                    `json:"dungeon"`
	Rooms        map[string]*Room            `json:"rooms"`
	Connections  map[string][]*RoomConnection `json:"connections"`
	Monsters     map[string]*Monster         `json:"monsters"`
	Items        map[string]*Item            `json:"items"`
	Traps        map[string]*Trap            `json:"traps"`
	VisitedRooms map[string]bool             `json:"visited_rooms"`
	Combat       *CombatState                `json:"combat,omitempty"`
	GameOver     bool                        `json:"game_over"`
	Victory      bool                        `json:"victory"`
	TurnNumber   int                         `json:"turn_number"`
}

// SerializeState converts GameState to JSON
func (gs *GameState) SerializeState() (string, error) {
	gs.RLock()
	defer gs.RUnlock()

	save := &SaveState{
		SessionID:    gs.SessionID,
		Mode:         gs.Mode,
		Party:        gs.Party,
		Dungeon:      gs.Dungeon,
		Rooms:        gs.Rooms,
		Connections:  gs.Connections,
		Monsters:     gs.Monsters,
		Items:        gs.Items,
		Traps:        gs.Traps,
		VisitedRooms: gs.VisitedRooms,
		Combat:       gs.Combat,
		GameOver:     gs.GameOver,
		Victory:      gs.Victory,
	}

	if gs.TurnContext != nil {
		save.TurnNumber = gs.TurnContext.TurnNumber
	}

	data, err := json.Marshal(save)
	if err != nil {
		return "", fmt.Errorf("failed to serialize game state: %w", err)
	}
	return string(data), nil
}

// DeserializeState restores GameState from JSON
func DeserializeState(data string) (*GameState, error) {
	var save SaveState
	if err := json.Unmarshal([]byte(data), &save); err != nil {
		return nil, fmt.Errorf("failed to deserialize game state: %w", err)
	}

	gs := NewGameState(save.SessionID)
	gs.Mode = save.Mode
	gs.Party = save.Party
	gs.Dungeon = save.Dungeon
	gs.GameOver = save.GameOver
	gs.Victory = save.Victory
	gs.Combat = save.Combat
	gs.TurnContext = &TurnContext{TurnNumber: save.TurnNumber}

	// Rebuild rooms and coordinate index
	for _, room := range save.Rooms {
		gs.AddRoom(room)
	}

	// Rebuild connections
	gs.Connections = save.Connections

	// Rebuild monsters and room index
	for _, monster := range save.Monsters {
		gs.Monsters[monster.ID] = monster
		if gs.MonstersByRoom[monster.RoomID] == nil {
			gs.MonstersByRoom[monster.RoomID] = make(map[string]bool)
		}
		gs.MonstersByRoom[monster.RoomID][monster.ID] = true
	}

	// Rebuild items and room/character indexes
	for _, item := range save.Items {
		gs.Items[item.ID] = item
		if item.RoomID != nil {
			if gs.ItemsByRoom[*item.RoomID] == nil {
				gs.ItemsByRoom[*item.RoomID] = make(map[string]bool)
			}
			gs.ItemsByRoom[*item.RoomID][item.ID] = true
		}
		if item.CharacterID != nil {
			if gs.ItemsByChar[*item.CharacterID] == nil {
				gs.ItemsByChar[*item.CharacterID] = make(map[string]bool)
			}
			gs.ItemsByChar[*item.CharacterID][item.ID] = true
		}
	}

	// Rebuild traps and room index
	for _, trap := range save.Traps {
		gs.Traps[trap.ID] = trap
		gs.TrapsByRoom[trap.RoomID] = append(gs.TrapsByRoom[trap.RoomID], trap.ID)
	}

	// Restore visited rooms
	gs.VisitedRooms = save.VisitedRooms

	return gs, nil
}
