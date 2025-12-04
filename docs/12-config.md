# Configuration Management

## Config File

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

