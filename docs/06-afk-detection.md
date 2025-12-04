# AFK/Leaver Detection

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

