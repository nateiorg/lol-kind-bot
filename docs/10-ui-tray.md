# UI & System Tray Requirements

## System Tray Icon

The application must:

1. Run as a **background application** with a **system tray icon** (Windows notification area).
2. Provide a context menu when the tray icon is right-clicked, with at least:
   - **Open Settings…**
   - **Toggle Listener On/Off** (or "Pause/Resume Listening").
   - **Exit** (clean shutdown of background listener).
3. Reflect status visually when possible:
   - Example:
     - Normal icon when actively listening.
     - Muted/greyed icon when listener is paused.

## Settings UI

The application must include a basic settings window (launched from tray menu) that allows users to change configuration without editing files manually.

Required settings:

1. **General**
   - My Summoner Name (string).
   - Enable/Disable auto-copy of first quip to clipboard (bool).

2. **LLM Settings**
   - LLM model name (string, e.g., `llama3.1`, `qwen:7b`).
   - LLM server URL (string, default `http://localhost:11434/api/generate`).

3. **Polling & Behavior**
   - Poll interval for gameflow (seconds).
   - End-of-game cooldown (seconds, minimum gap between EoG triggers).
   - Toggle for logging detailed summaries/EoG data (bool).

4. **AFK Thresholds** (v1 can be read-only or advanced section)
   - Minimum game duration (minutes) for AFK checks.
   - Minimum CS per minute.
   - Maximum damage to champs for AFK detection.
   - Maximum gold earned for AFK detection.

Settings behavior:

- Changes should be **persisted** (e.g., in a JSON or YAML config file).
- The listener should pick up new settings without requiring full app restart (e.g., reloading on save or applying in memory for next game).
- UI should validate fields (e.g., non-empty summoner name, valid URL format).

