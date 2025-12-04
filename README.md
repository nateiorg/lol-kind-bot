# LoL Kind Bot

A background application for Windows that connects to the League of Legends client, analyzes post-game stats, and generates positive, context-aware chat messages using a local LLM.

## Features

- 🔍 Monitors League of Legends client for end-of-game events
- 📊 Analyzes post-game statistics including AFK detection
- 🤖 Generates wholesome post-game messages using local LLM (Ollama)
- 📋 Auto-copies messages to clipboard (optional)
- 🎯 System tray integration with pause/resume functionality
- ⚙️ Configurable settings via JSON config file

## Requirements

- Windows 10/11
- Go 1.22 or later
- League of Legends client installed
- Local LLM server (e.g., [Ollama](https://ollama.ai/)) running

## Installation

1. Clone this repository:
   ```bash
   git clone <repository-url>
   cd lol-kind-bot
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Build the application:
   ```bash
   go build -o lol-kind-bot.exe .
   ```

## Configuration

On first run, the application will create a `config.json` file in the same directory as the executable. You can edit this file or use the settings UI (coming soon) to configure:

- **My Summoner Name**: Your in-game summoner name (required for team detection)
- **LLM Settings**: Model name and server URL (default: Ollama at `http://localhost:11434/api/generate`)
- **Polling Intervals**: How often to check for game state changes
- **AFK Detection Thresholds**: Criteria for detecting AFK players
- **Auto-copy**: Automatically copy the first message to clipboard

Example `config.json`:
```json
{
  "mySummonerName": "YourSummonerName",
  "ollamaModel": "llama3.1",
  "ollamaUrl": "http://localhost:11434/api/generate",
  "pollIntervalSeconds": 3,
  "endOfGameCooldownSeconds": 30,
  "autoCopyToClipboard": true,
  "enableDetailedLogging": false,
  "afkThresholds": {
    "minGameMinutes": 10,
    "maxCsPerMin": 0.5,
    "maxDamageToChamp": 1500,
    "maxGoldEarned": 4000
  }
}
```

## Usage

1. Start your local LLM server (e.g., Ollama):
   ```bash
   ollama serve
   ```

2. Make sure you have a model downloaded:
   ```bash
   ollama pull llama3.1
   ```

3. Run the application:
   ```bash
   ./lol-kind-bot.exe
   ```

4. The application will appear in your system tray. Right-click to:
   - Toggle listener on/off
   - Open settings (coming soon)
   - Exit

5. When a game ends, the bot will:
   - Fetch end-of-game statistics
   - Analyze player performance and detect AFKs
   - Generate 2-3 positive messages via LLM
   - Display messages in console logs
   - Copy the first message to clipboard (if enabled)

## How It Works

1. **LCU Connection**: Reads the League client lockfile to connect to the local LCU API
2. **Game Monitoring**: Polls the gameflow phase endpoint to detect `EndOfGame`
3. **Stats Analysis**: Fetches and processes end-of-game statistics
4. **AFK Detection**: Identifies AFK players based on configurable thresholds
5. **Tagging**: Assigns positive tags to high performers (topDamage, visionHero, kdaBeast, laneFarmer)
6. **Message Generation**: Sends game summary to LLM with instructions for generating positive messages
7. **Output**: Displays messages and optionally copies to clipboard

## Project Structure

```
lol-kind-bot/
├── analyzer/      # Game analysis and summary generation
├── config/        # Configuration management
├── docs/          # Documentation (broken down from specs.md)
├── eog/           # End-of-game stats structures
├── llm/           # LLM client and prompt construction
├── lcu/           # League Client API client
├── monitor/       # Gameflow phase monitoring
├── main.go        # Main application entry point
└── config.json    # Configuration file (created on first run)
```

## Development

The project is organized into modular packages:

- `config`: Handles loading/saving configuration
- `lcu`: LCU API client with lockfile parsing
- `monitor`: Gameflow phase polling and EndOfGame detection
- `eog`: End-of-game stats data structures
- `analyzer`: Game analysis, AFK detection, and tagging
- `llm`: LLM integration and prompt construction

## License

[Add your license here]

## Contributing

[Add contribution guidelines here]

