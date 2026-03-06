#!/usr/bin/env python3
"""
Yrpg Game Test Agent

An AI-powered test harness that plays the dungeon crawler RPG and verifies
game mechanics using a local LLM via Ollama.

Requirements:
    pip install ollama requests

Usage:
    # Start the game server first:
    #   cd /path/to/Yrpg-backend && go run ./cmd/server
    #
    # Then run the agent:
    python agent.py
    python agent.py --model qwen2.5:32b
    python agent.py --model llama3.1:70b --max-turns 100
    python agent.py --base-url http://192.168.1.50:8080  # remote game server
"""

import argparse
import json
import logging
import sys
import time
from datetime import datetime
from pathlib import Path

import ollama
import requests

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------

DEFAULT_MODEL = "qwen2.5:32b"
DEFAULT_GAME_URL = "http://localhost:8080"
DEFAULT_MAX_TURNS = 200
OLLAMA_OPTIONS = {
    "temperature": 0.7,
    "num_ctx": 16384,
}

# ---------------------------------------------------------------------------
# Game client
# ---------------------------------------------------------------------------


class GameClient:
    """Thin wrapper around the Yrpg HTTP API."""

    def __init__(self, base_url: str):
        self.base_url = base_url.rstrip("/")
        self.session = requests.Session()

    def health(self) -> bool:
        try:
            r = self.session.get(f"{self.base_url}/health", timeout=5)
            return r.ok
        except requests.ConnectionError:
            return False

    def call_tool(self, name: str, arguments: dict) -> dict:
        r = self.session.post(
            f"{self.base_url}/mcp/call",
            json={"name": name, "arguments": arguments},
            timeout=30,
        )
        r.raise_for_status()
        return r.json()


# ---------------------------------------------------------------------------
# Ollama tool definitions
# ---------------------------------------------------------------------------

# We define a single "game_action" tool that the LLM calls. This keeps the
# tool schema simple and lets the LLM specify the MCP tool name + args freely.

TOOLS = [
    {
        "type": "function",
        "function": {
            "name": "game_action",
            "description": (
                "Execute a game action by calling an MCP tool on the server. "
                "Provide the tool name and its arguments."
            ),
            "parameters": {
                "type": "object",
                "properties": {
                    "tool_name": {
                        "type": "string",
                        "description": "The MCP tool name, e.g. 'new_game', 'move', 'combat_attack'.",
                    },
                    "tool_arguments": {
                        "type": "object",
                        "description": "Arguments for the MCP tool as a JSON object.",
                    },
                },
                "required": ["tool_name", "tool_arguments"],
            },
        },
    },
]


# ---------------------------------------------------------------------------
# Agent loop
# ---------------------------------------------------------------------------


class Agent:
    def __init__(
        self,
        model: str,
        game: GameClient,
        max_turns: int,
        log_dir: Path,
    ):
        self.model = model
        self.game = game
        self.max_turns = max_turns
        self.log_dir = log_dir
        self.log_dir.mkdir(parents=True, exist_ok=True)

        self.system_prompt = (Path(__file__).parent / "system_prompt.md").read_text()
        self.messages: list[dict] = []
        self.turn = 0
        self.bugs: list[str] = []

        # Set up file logger
        log_file = self.log_dir / f"run_{datetime.now():%Y%m%d_%H%M%S}.log"
        self.file_logger = logging.getLogger("agent_file")
        self.file_logger.setLevel(logging.DEBUG)
        fh = logging.FileHandler(log_file)
        fh.setFormatter(logging.Formatter("%(asctime)s  %(message)s"))
        self.file_logger.addHandler(fh)

    # ------------------------------------------------------------------
    # Helpers
    # ------------------------------------------------------------------

    def log(self, msg: str):
        print(msg)
        self.file_logger.info(msg)

    def log_json(self, label: str, data):
        text = json.dumps(data, indent=2)
        self.file_logger.debug(f"{label}:\n{text}")

    def truncate_game_state(self, response: dict) -> str:
        """Build a concise text representation of the game response for the LLM."""
        parts = []

        # Always include the text message
        if "content" in response:
            for c in response["content"]:
                if c.get("type") == "text":
                    parts.append(c["text"])

        if response.get("isError"):
            parts.append("[ERROR]")
            return "\n".join(parts)

        gs = response.get("gameState")
        if not gs:
            return "\n".join(parts)

        # Include key state info without overwhelming the context
        parts.append(f"\n--- Game State (mode={gs.get('mode')}, turn={gs.get('turnNumber')}) ---")

        if gs.get("gameOver"):
            parts.append(f"GAME OVER. Victory: {gs.get('victory')}")

        # Party summary
        party = gs.get("party")
        if party:
            for ch in party.get("characters", []):
                equip_info = ""
                for item in ch.get("inventory", []):
                    if item.get("isEquipped"):
                        equip_info += f" [{item['name']}]"
                slots = ""
                if ch.get("maxSpellSlots", 0) > 0:
                    slots = f" slots={ch['spellSlots']}/{ch['maxSpellSlots']}"
                    spells = ch.get("knownSpells", [])
                    if spells:
                        slots += f" spells={spells}"
                parts.append(
                    f"  {ch['name']} ({ch['class']}) HP:{ch['hp']}/{ch['maxHp']} "
                    f"STR:{ch['strength']} DEX:{ch['dexterity']} INT:{ch['intelligence']} "
                    f"AC:{ch['ac']} {ch['status']}{equip_info}{slots} id={ch['id']}"
                )
            parts.append(f"  Formation: {party.get('formation')}")

        # Room
        room = gs.get("currentRoom")
        if room:
            parts.append(f"Room: {room['name']} ({room['id']}) exits={room['exits']} firstVisit={room['isFirstVisit']}")
            if room.get("isEntrance"):
                parts.append("  [ENTRANCE]")
            if room.get("isExit"):
                parts.append("  [EXIT]")

        # Monsters
        monsters = gs.get("monsters", [])
        if monsters:
            for m in monsters:
                parts.append(
                    f"  Monster: {m['name']} HP:{m['hp']}/{m['maxHp']} dmg={m['damage']} "
                    f"threat={m['threat']} defeated={m['isDefeated']} ranged={m['isRanged']} id={m['id']}"
                )

        # Room items
        items = gs.get("roomItems", [])
        if items:
            for it in items:
                parts.append(f"  Floor item: {it['name']} ({it['type']}) id={it['id']}")

        # Traps
        traps = gs.get("roomTraps", [])
        if traps:
            for t in traps:
                state = "disarmed" if t["isDisarmed"] else ("triggered" if t["isTriggered"] else "active")
                parts.append(f"  Trap: {t['description']} loc={t['location']} state={state} DC={t['difficulty']} id={t['id']}")

        # Combat
        combat = gs.get("combat")
        if combat:
            parts.append(f"Combat: round={combat['roundNumber']} turnIdx={combat['currentTurnIdx']} "
                         f"active={combat['isActive']} scout={combat['isScoutPhase']} "
                         f"awaitingScout={combat['awaitingScoutDecision']}")
            for c in combat.get("combatants", []):
                side = "PLAYER" if c["isPlayerChar"] else "ENEMY"
                extra = ""
                if c.get("isHidden"):
                    extra += " HIDDEN"
                if c.get("hasCharged"):
                    extra += " CHARGED"
                if c.get("protectedBy"):
                    extra += f" protectedBy={c['protectedBy']}"
                spells = c.get("knownSpells", [])
                spell_str = f" spells={spells}" if spells else ""
                parts.append(
                    f"  [{side}] {c['name']} HP:{c['hp']}/{c['maxHp']} pos=({c['gridX']},{c['gridY']}) "
                    f"moved={c['hasMoved']} acted={c['hasActed']} alive={c['isAlive']} "
                    f"move={c['movementRange']} atk={c['attackRange']}{spell_str}{extra} id={c['id']}"
                )
            # Grid visualization
            grid = combat.get("grid", [])
            if grid:
                parts.append("  Grid:")
                for y, row in enumerate(grid):
                    cells = []
                    for cell in row:
                        if cell == "":
                            cells.append("  .  ")
                        elif cell == "blocked":
                            cells.append("  X  ")
                        else:
                            # Shorten IDs for display
                            cells.append(f"{cell[:5]}")
                    parts.append(f"    {y}: {' '.join(cells)}")

        return "\n".join(parts)

    # ------------------------------------------------------------------
    # Main loop
    # ------------------------------------------------------------------

    def run(self):
        self.log(f"Starting agent with model={self.model}, max_turns={self.max_turns}")
        self.log(f"Game server: {self.game.base_url}")

        if not self.game.health():
            self.log("ERROR: Game server is not reachable. Start it first.")
            sys.exit(1)
        self.log("Game server is healthy.\n")

        # Initial message to kick things off
        self.messages = [
            {"role": "system", "content": self.system_prompt},
            {
                "role": "user",
                "content": (
                    "Start a new game with a fighter named Kael, a magic_user named Lyra, "
                    "and a thief named Vex. Then explore the dungeon, fight monsters, and "
                    "test as many game mechanics as you can. After each action, verify the "
                    "game state and report any bugs. Try some edge cases too (e.g. charge "
                    "twice, cast without slots, move out of range)."
                ),
            },
        ]

        while self.turn < self.max_turns:
            self.turn += 1
            self.log(f"\n{'='*60}")
            self.log(f"AGENT TURN {self.turn}")
            self.log(f"{'='*60}")

            try:
                response = ollama.chat(
                    model=self.model,
                    messages=self.messages,
                    tools=TOOLS,
                    options=OLLAMA_OPTIONS,
                )
            except Exception as e:
                self.log(f"Ollama error: {e}")
                time.sleep(2)
                continue

            assistant_msg = response["message"]
            self.messages.append(assistant_msg)

            # Print the LLM's reasoning
            if assistant_msg.get("content"):
                self.log(f"\nLLM: {assistant_msg['content']}")

            # Check for bugs reported in text
            if assistant_msg.get("content") and "BUG:" in assistant_msg["content"]:
                for line in assistant_msg["content"].split("\n"):
                    if "BUG:" in line:
                        self.bugs.append(line.strip())

            # Process tool calls
            tool_calls = assistant_msg.get("tool_calls", [])
            if not tool_calls:
                self.log("(No tool calls — LLM is done or thinking)")
                # Give it a nudge
                self.messages.append({
                    "role": "user",
                    "content": "Continue playing. Call game_action to take your next action.",
                })
                continue

            for tc in tool_calls:
                fn = tc["function"]
                fn_name = fn["name"]
                fn_args = fn["arguments"]

                if fn_name != "game_action":
                    self.log(f"Unknown tool call: {fn_name}")
                    self.messages.append({
                        "role": "tool",
                        "content": f"Unknown tool: {fn_name}. Use game_action.",
                    })
                    continue

                tool_name = fn_args.get("tool_name", "")
                tool_args = fn_args.get("tool_arguments", {})

                self.log(f"\n>> game_action: {tool_name}({json.dumps(tool_args)})")

                try:
                    result = self.game.call_tool(tool_name, tool_args)
                    self.log_json(f"Raw response for {tool_name}", result)
                    summary = self.truncate_game_state(result)
                    self.log(f"<< {summary[:500]}{'...' if len(summary) > 500 else ''}")
                except requests.HTTPError as e:
                    summary = f"HTTP error: {e}"
                    self.log(f"<< {summary}")
                except Exception as e:
                    summary = f"Error calling {tool_name}: {e}"
                    self.log(f"<< {summary}")

                self.messages.append({
                    "role": "tool",
                    "content": summary,
                })

                # Check for game over
                if "GAME OVER" in summary:
                    self.log("\n*** GAME OVER detected ***")

            # Trim message history if it gets too long (keep system + last N)
            if len(self.messages) > 80:
                self.messages = [self.messages[0]] + self.messages[-60:]

        self.log(f"\n{'='*60}")
        self.log(f"Agent finished after {self.turn} turns.")
        if self.bugs:
            self.log(f"\nBUGS FOUND ({len(self.bugs)}):")
            for b in self.bugs:
                self.log(f"  - {b}")
        else:
            self.log("No bugs reported.")


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------


def main():
    parser = argparse.ArgumentParser(description="Yrpg AI Test Agent (Ollama)")
    parser.add_argument("--model", default=DEFAULT_MODEL, help=f"Ollama model (default: {DEFAULT_MODEL})")
    parser.add_argument("--base-url", default=DEFAULT_GAME_URL, help=f"Game server URL (default: {DEFAULT_GAME_URL})")
    parser.add_argument("--max-turns", type=int, default=DEFAULT_MAX_TURNS, help=f"Max agent turns (default: {DEFAULT_MAX_TURNS})")
    parser.add_argument("--log-dir", default="test_harness/logs", help="Log directory (default: test_harness/logs)")
    args = parser.parse_args()

    game = GameClient(args.base_url)
    agent = Agent(
        model=args.model,
        game=game,
        max_turns=args.max_turns,
        log_dir=Path(args.log_dir),
    )
    agent.run()


if __name__ == "__main__":
    main()