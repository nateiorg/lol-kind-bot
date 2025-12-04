# lol-kind-bot – Requirements Specification

## 1. Project Overview

**Working name:** `lol-kind-bot`  
**Goal:**  
A background application that runs on the user’s Windows PC, connects to the League of Legends client, listens for the end-of-game event, analyzes post-game stats (including AFKs/leavers), and generates short, positive, context-aware chat messages the player can copy into the post-game chat.

**Key points:**

- Runs in the background with a **system tray icon**.
- Written in **Go**.
- Uses a **local LLM** (via a local HTTP API such as Ollama) leveraging the user’s **RTX 5090**.
- Provides a **UI for changing settings** (e.g., configuration window launched from tray menu).
- Does **not** automate in-game UI interactions; it only reads local APIs and presents messages to the user.

---

## 2. Technical Stack

### 2.1 Language & Platform

- Primary implementation language: **Go**
- Minimum Go version: **1.22+** (or latest stable).
- Target OS: **Windows 10/11**.
- Distribution format: single executable (`lol-kind-bot.exe`), plus optional config file(s).

### 2.2 External Components

1. **League of Legends Client (LCU) API**
   - Uses the local League client’s **lockfile** to obtain:
     - Protocol (HTTP/HTTPS)
     - Port
     - Auth password
   - Uses authenticated HTTPS against the LCU:
     - Base URL: `{protocol}://127.0.0.1:{port}`.
     - Basic auth: `"riot:<password>"` (Base64 encoded).

2. **LLM Runtime (local)**
   - Example: **Ollama** or similar local LLM server.
   - Provides an HTTP endpoint (e.g., `http://localhost:11434/api/generate`).
   - Configurable model name (e.g., `llama3.1`, `qwen-7b`, etc.).
   - Non-streaming responses are acceptable for v1.

---

## 3. Core Functional Requirements

### 3.1 LCU Connection & Game State Monitoring

The application must:

1. **Locate the lockfile**
   - Default path: `C:\Riot Games\League of Legends\lockfile`.
   - Allow override through environment variable: `LOL_LOCKFILE_PATH`.
   - Poll periodically (e.g., every 3 seconds) until the lockfile exists.
   - Handle League client being opened/closed at any time.

2. **Parse the lockfile**
   - Lockfile format: `processName:PID:port:password:protocol`.
   - Extract:
     - `port` (int)
     - `password` (string)
     - `protocol` (string; e.g., `https`)

3. **Construct LCU Client**
   - Base URL: `{protocol}://127.0.0.1:{port}`.
   - Use HTTP client:
     - TLS: trust self-signed certs (local only).
     - Authorization: `Basic` header with `riot:<password>` (Base64 encoded).
   - Provide a helper `GET` function for endpoints.

4. **Poll gameflow phase**
   - Endpoint: `/lol-gameflow/v1/gameflow-phase`.
   - Poll frequency: ~3 seconds (configurable).
   - Track current and last phase.
   - Detect transition into `EndOfGame` phase.
   - Debounce: Only trigger EoG processing once within a configurable cooldown (e.g., 30 seconds).

5. **Connection Error Handling**
   - On request failure (client down, network issue, etc.):
     - Log error.
     - Attempt to re-read lockfile and recreate client.
     - Continue polling until League is available again.

---

### 3.2 End-of-Game Stats Retrieval

Upon detecting `EndOfGame`:

1. **Fetch End-of-Game stats**
   - Endpoint: `/lol-end-of-game/v1/eog-stats-block`.
   - Parse the JSON response into Go structs.

2. **Required data fields** (per participant):
   - Summoner name
   - Team ID (100 = BLUE, 200 = RED)
   - Champion name
   - Kills
   - Deaths
   - Assists
   - Total minions killed (lane)
   - Neutral minions killed (jungle)
   - Gold earned
   - Total damage dealt to champions
   - Vision score
   - Win flag (bool)

3. **Required data fields** (game-level):
   - Game duration in seconds.

4. **Internal structures**
   - `eogParticipant`:
     - SummonerName
     - TeamID
     - ChampionName
     - Kills, Deaths, Assists
     - TotalMinionsKilled
     - NeutralMinionsKilled
     - GoldEarned
     - TotalDamageDealtToChampions
     - VisionScore
     - Win
   - `eogStatsBlock`:
     - GameDurationSeconds
     - Participants ([]eogParticipant)

---

### 3.3 Derived Summary Structures

The app must convert raw EoG data into usable summary structures:

1. **Player summary**
   - `playerSummary`:
     - `SummonerName` (string)
     - `Team` (string: `"BLUE"` or `"RED"`)
     - `Champion` (string)
     - `K`, `D`, `A` (int)
     - `CsPerMin` (float)
     - `DamageShare` (float; fraction of team damage)
     - `VisionScore` (int)
     - `Tags` ([]string; positive indicators like `"topDamage"`, `"visionHero"`)
     - `Afk` (bool)

2. **Game summary**
   - `gameSummary`:
     - `GameDurationMinutes` (float)
     - `WinningTeam` (string: `"BLUE"` or `"RED"`)
     - `MySummonerName` (string, from config)
     - `MyTeam` (string: `"BLUE"` or `"RED"`, deduced from participants)
     - `Players` ([]playerSummary)
     - `AfkOnMyTeam` (bool)
     - `AfkOnEnemyTeam` (bool)

3. **Team side mapping**
   - Helper function:
     - `teamIdToSide(100) -> "BLUE"`
     - `teamIdToSide(200) -> "RED"`

---

### 3.4 AFK/Leaver Detection

Implement a heuristic to determine AFK/leaver status:

1. **AFK conditions** (initial defaults, configurable):
   - Game duration ≥ 10 minutes.
   - CS per minute < 0.5.
   - Total damage to champions < 1500.
   - Gold earned < 4000.
   - Kills == 0 and Assists == 0.

2. **Behavior:**
   - If all conditions are met, mark `playerSummary.Afk = true`.
   - Do **not** assign any positive tags to AFK players.
   - Set game-level flags:
     - If AFK is on my team: `AfkOnMyTeam = true`.
     - If AFK is on enemy team: `AfkOnEnemyTeam = true`.

3. **Configurability:**
   - Thresholds for AFK detection should be adjustable via config (v2+).

---

### 3.5 Outlier & Tagging Logic

For each **non-AFK** player:

1. **Compute derived stats:**
   - Total CS = TotalMinionsKilled + NeutralMinionsKilled.
   - `CsPerMin = totalCS / gameMinutes`.
   - `DamageShare = playerDamage / totalTeamDamage`.

2. **Assign positive tags (non-exclusive):**
   - `topDamage`:
     - If `DamageShare >= 0.30`.
   - `visionHero`:
     - If vision score meets a threshold (e.g., ≥ 20) or is highest on team.
   - `kdaBeast`:
     - If `Kills + Assists >= 10` (or similar KDA-based condition).
   - `laneFarmer`:
     - If `CsPerMin >= 7`.

3. **No negative tags:**
   - Do not mark players as “bad” or similar.
   - Tags are intended only for highlighting strengths.

---

## 4. Scenario Intelligence

The system must behave intelligently in various match outcomes. The LLM prompt and `gameSummary` must support the following scenarios:

1. **No AFKs, Standard Game**
   - `AfkOnMyTeam = false`, `AfkOnEnemyTeam = false`.
   - Behavior:
     - Praise high performers (especially on own team).
     - Use friendly `ggwp` style.
     - Highlight standout stats via tags.

2. **AFK on My Team, We Lose**
   - `AfkOnMyTeam = true`, `WinningTeam != MyTeam`.
   - Behavior:
     - Never mention AFK player.
     - Praise remaining 4 teammates for effort.
     - At least one message must apologize to enemy team:
       - e.g., “Sorry for the AFK, you played that well. ggwp.”
     - Emphasize empathy and sportsmanship.

3. **AFK on My Team, We Win**
   - `AfkOnMyTeam = true`, `WinningTeam == MyTeam`.
   - Behavior:
     - Ignore AFK player.
     - Praise remaining 4 for clutch effort.
     - Avoid arrogance; optional acknowledgment that it was tough for both teams.

4. **AFK on Enemy Team, We Win**
   - `AfkOnEnemyTeam = true`, `WinningTeam == MyTeam`.
   - Behavior:
     - Show empathy:
       - e.g., “Sorry for the scuffed game, gl next everyone.”
     - No gloating or “easy”/“diff” remarks.
     - Optionally thank them for playing it out.

5. **AFK on Enemy Team, They Still Win**
   - `AfkOnEnemyTeam = true`, `WinningTeam != MyTeam`.
   - Behavior:
     - Praise enemy team for playing well despite AFK.
     - Optional light self/team praise, but keep tone humble.
     - Emphasize respect and “gl next” vibe.

6. **Multiple AFKs (Both Sides)**
   - `AfkOnMyTeam = true` and `AfkOnEnemyTeam = true`.
   - Behavior:
     - Focus on empathy all around.
     - “Scuffed game, gl next” style messages.
     - May highlight standout non-AFK players on both sides without taunting.

7. **Stomp Wins / Stomp Losses (No AFKs)**
   - Lopsided stats within normal play.
   - Behavior:
     - Praise carries and strong supports.
     - Avoid taunting; keep messages upbeat and respectful.

---

## 5. LLM Integration Requirements

### 5.1 Prompt Construction

The app must send a structured prompt to the LLM including:

1. **System-style instruction text** (embedded in prompt):

   - The LLM acts as a **League of Legends post-game sportsmanship coach**.
   - Rules:
     - Never insult, blame, or criticize any player.
     - No sarcasm, no passive-aggressive comments.
     - Highlight what players did well (damage, vision, KDA, CS, etc.).
     - AFK rules:
       - If `AfkOnMyTeam == true`, do not mention AFK player(s). Instead:
         - Praise the remaining four teammates.
         - Include at least one apology to the enemy team.
       - If `AfkOnEnemyTeam == true`, avoid gloating and show empathy.
     - Messages:
       - 1–2 sentences each.
       - Under 150 characters each.
       - Casual gamer language allowed (“ggwp”, “nice damage”, etc.) but wholesome.
       - No profanity, slurs, or toxicity.
       - Each message must appear on its **own line**, no bullet points or numbering.

2. **Game summary JSON**
   - Append serialized `gameSummary` as a JSON block after the instructions.
   - Prompt example shape:

     - Instructions text.
     - `Game summary JSON:\n<json>`

3. **Response format expectation**
   - The LLM should output:
     - 2–3 chat messages.
     - Each message as one line of text (no bullets, no numbering).
   - Post-processing:
     - Split response on newline.
     - Trim whitespace and remove leading `- `, `* `, etc.
     - Filter out empty lines.

### 5.2 LLM HTTP API Call

1. **Endpoint (example with Ollama):**
   - `POST http://localhost:11434/api/generate`

2. **Request body:**
   - JSON with fields:
     - `model` (string): configured model name.
     - `prompt` (string): the full prompt text (instructions + JSON).
     - `stream` (bool): `false` for v1.

3. **Response handling:**
   - Parse the JSON response body.
   - Extract the generated content (e.g., `response` field).
   - Derive messages as described.

4. **Error handling:**
   - If HTTP call fails or returns non-2xx:
     - Log error with reason.
     - Fall back to one or more basic template messages such as:
       - “ggwp everyone, thanks for the game!”
       - “Nice effort team, gl in your next games!”

---

## 6. UI & System Tray Requirements

### 6.1 System Tray Icon

The application must:

1. Run as a **background application** with a **system tray icon** (Windows notification area).
2. Provide a context menu when the tray icon is right-clicked, with at least:
   - **Open Settings…**
   - **Toggle Listener On/Off** (or “Pause/Resume Listening”).
   - **Exit** (clean shutdown of background listener).
3. Reflect status visually when possible:
   - Example:
     - Normal icon when actively listening.
     - Muted/greyed icon when listener is paused.

### 6.2 Settings UI

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

---

## 7. Output & UX – Interactions

### 7.1 Console Logging (Developer Visibility)

The app should log:

- Startup info:
  - Lockfile path.
  - LCU base URL.
  - Current configuration values (at least summary).
- Gameflow phase changes:
  - `Gameflow phase: <phase>`.
- EoG processing:
  - “Detected EndOfGame; fetching stats…”
- Errors:
  - Connection to LCU failures.
  - LLM API failures.
- Quip suggestions:
  - Print suggested messages in a block for debugging.

### 7.2 User-Facing Messages

After each EoG event:

- The app should:
  - Generate 2–3 messages via LLM.
  - Display them in logs and/or in a simple UI (e.g., tray notification or a pop-up window is optional).
  - If auto-copy is enabled:
    - Copy the **first** message to clipboard automatically.

---

## 8. Configuration Management

### 8.1 Config File

- Provide an optional `config.json` (or `.yaml`) file with fields:

  - `mySummonerName` (string)
  - `ollamaModel` (string)
  - `ollamaUrl` (string)
  - `pollIntervalSeconds` (int)
  - `endOfGameCooldownSeconds` (int)
  - `autoCopyToClipboard` (bool)
  - `enableDetailedLogging` (bool)
  - `afkThresholds` (object: `minGameMinutes`, `maxCsPerMin`, etc.)

- On startup:
  - Try to load the config file.
  - If missing, use built-in defaults and emit a log warning.
  - Settings UI must read from and write to this file (or an equivalent persistent storage).

---

## 9. Non-Functional Requirements

1. **Performance**
   - Time from `EndOfGame` detection to quip availability: ideally **≤ 1–2 seconds**.
   - Polling overhead must be minimal and not impact system performance.

2. **Stability**
   - Recover gracefully from:
     - League client restarts.
     - LCU unavailability.
     - LLM server unavailability.

3. **Safety & Tone**
   - Generated output must be:
     - Non-toxic.
     - Non-discriminatory.
     - Focused on sportsmanship and positivity.

4. **Extensibility**
   - Designed to allow future enhancements:
     - Additional tagging types (objective control, role-specific nuances).
     - Additional LLM providers or remote hosted options.
     - Localization (multi-language support).

---

## 10. Summary Prompt for AI Code Generation Tools

> Implement a Go application named `lol-kind-bot` that:
> - Runs on Windows with a system tray icon and a settings UI.
> - Connects to the local League of Legends client using the lockfile and LCU API.
> - Polls `/lol-gameflow/v1/gameflow-phase` to detect the `EndOfGame` phase.
> - On `EndOfGame`, fetches `/lol-end-of-game/v1/eog-stats-block`, normalizes stats, detects AFKs based on configurable thresholds, and assigns positive tags (e.g., topDamage, visionHero, kdaBeast, laneFarmer) to non-AFK players.
> - Constructs a `gameSummary` structure containing game duration, teams, my summoner name/team, AFK flags for each side, and per-player summaries.
> - Sends an instruction-rich prompt plus the `gameSummary` JSON to a local LLM server (e.g., Ollama) and receives 2–3 short, wholesome, copy-pastable post-game chat messages, each on its own line.
> - Prints these messages to the console and, if enabled in settings, automatically copies the first one to the clipboard.
> - Provides a system tray menu with options to open settings, toggle the listener on/off, and exit the app.
> - Allows users to configure summoner name, LLM model name, LLM URL, polling intervals, AFK thresholds, and auto-copy behavior via a graphical settings UI backed by a persistent config file.
> - Always ensures positivity and avoids toxic or sarcastic outputs, handling scenarios like AFKs on my team, AFKs on the enemy team, stomp wins/losses, and multiple AFKs with intelligent and empathetic messaging.
