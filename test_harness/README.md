# Yrpg AI Test Harness

Uses a local LLM (via Ollama) to play the dungeon crawler and verify game mechanics.

## Setup

1. Install [Ollama](https://ollama.com)
2. Pull a model with tool-calling support:
   ```bash
   ollama pull qwen2.5:32b      # good balance of speed and quality
   # or
   ollama pull llama3.1:70b     # stronger but slower
   # or
   ollama pull mistral-small    # lighter option
   ```
3. Install Python dependencies:
   ```bash
   pip install -r test_harness/requirements.txt
   ```

## Usage

```bash
# Start the game server
go run ./cmd/server &

# Run the test agent
python test_harness/agent.py

# Options
python test_harness/agent.py --model qwen2.5:32b --max-turns 100
python test_harness/agent.py --base-url http://192.168.1.50:8080  # remote server
```

## Output

- Console shows real-time play-by-play
- Detailed logs saved to `test_harness/logs/`
- Bugs are collected and summarized at the end

## How It Works

1. The agent sends the system prompt (game rules + verification checklist) to the LLM
2. The LLM decides what game action to take and calls the `game_action` tool
3. The harness executes the action against the game server via HTTP
4. The game state is summarized and fed back to the LLM
5. The LLM verifies the result, reports any bugs, and picks the next action
6. Repeat until game over or max turns reached
