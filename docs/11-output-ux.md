# Output & UX – Interactions

## Console Logging (Developer Visibility)

The app should log:

- Startup info:
  - Lockfile path.
  - LCU base URL.
  - Current configuration values (at least summary).
- Gameflow phase changes:
  - `Gameflow phase: <phase>`.
- EoG processing:
  - "Detected EndOfGame; fetching stats…"
- Errors:
  - Connection to LCU failures.
  - LLM API failures.
- Quip suggestions:
  - Print suggested messages in a block for debugging.

## User-Facing Messages

After each EoG event:

- The app should:
  - Generate 2–3 messages via LLM.
  - Display them in logs and/or in a simple UI (e.g., tray notification or a pop-up window is optional).
  - If auto-copy is enabled:
    - Copy the **first** message to clipboard automatically.

