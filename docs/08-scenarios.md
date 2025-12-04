# Scenario Intelligence

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
       - e.g., "Sorry for the AFK, you played that well. ggwp."
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
       - e.g., "Sorry for the scuffed game, gl next everyone."
     - No gloating or "easy"/"diff" remarks.
     - Optionally thank them for playing it out.

5. **AFK on Enemy Team, They Still Win**
   - `AfkOnEnemyTeam = true`, `WinningTeam != MyTeam`.
   - Behavior:
     - Praise enemy team for playing well despite AFK.
     - Optional light self/team praise, but keep tone humble.
     - Emphasize respect and "gl next" vibe.

6. **Multiple AFKs (Both Sides)**
   - `AfkOnMyTeam = true` and `AfkOnEnemyTeam = true`.
   - Behavior:
     - Focus on empathy all around.
     - "Scuffed game, gl next" style messages.
     - May highlight standout non-AFK players on both sides without taunting.

7. **Stomp Wins / Stomp Losses (No AFKs)**
   - Lopsided stats within normal play.
   - Behavior:
     - Praise carries and strong supports.
     - Avoid taunting; keep messages upbeat and respectful.

