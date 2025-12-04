# Outlier & Tagging Logic

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
   - Do not mark players as "bad" or similar.
   - Tags are intended only for highlighting strengths.

