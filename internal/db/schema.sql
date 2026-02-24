-- Session tracking for analytics
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    seed INTEGER NOT NULL,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP,
    outcome TEXT,  -- 'victory', 'party_wipe', 'retreat', 'abandoned'
    turns_total INTEGER DEFAULT 0,
    rooms_explored INTEGER DEFAULT 0,
    monsters_defeated INTEGER DEFAULT 0,
    traps_encountered INTEGER DEFAULT 0,
    characters_lost INTEGER DEFAULT 0,
    dungeon_depth INTEGER DEFAULT 1
);

-- Party composition per session
CREATE TABLE IF NOT EXISTS session_characters (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    character_name TEXT NOT NULL,
    class TEXT NOT NULL,
    final_hp INTEGER,
    final_status TEXT,  -- 'alive', 'dead'
    kills INTEGER DEFAULT 0,
    damage_dealt INTEGER DEFAULT 0,
    damage_taken INTEGER DEFAULT 0,
    items_used INTEGER DEFAULT 0,
    traps_disarmed INTEGER DEFAULT 0,
    successful_sneaks INTEGER DEFAULT 0
);

-- Structured event log (the core analytics table)
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    turn_number INTEGER NOT NULL,
    event_type TEXT NOT NULL,       -- 'combat', 'trap', 'sneak', 'item', 'movement', 'death', 'victory'
    event_subtype TEXT NOT NULL,    -- 'attack_hit', 'attack_miss', 'flanking_attack', 'trap_triggered', etc.
    actor_id TEXT,                  -- character or monster who acted
    actor_class TEXT,               -- for quick analytics without joins
    target_id TEXT,
    room_id TEXT,
    details TEXT NOT NULL,          -- JSON blob with full context
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Active game state (for save/load)
CREATE TABLE IF NOT EXISTS saved_games (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    state_json TEXT NOT NULL,       -- serialized GameState
    saved_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for common analytics queries
CREATE INDEX IF NOT EXISTS idx_events_session ON events(session_id);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type, event_subtype);
CREATE INDEX IF NOT EXISTS idx_events_actor_class ON events(actor_class, event_type);
CREATE INDEX IF NOT EXISTS idx_session_chars_class ON session_characters(class);
CREATE INDEX IF NOT EXISTS idx_session_chars_session ON session_characters(session_id);
CREATE INDEX IF NOT EXISTS idx_sessions_outcome ON sessions(outcome);