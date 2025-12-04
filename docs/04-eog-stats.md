# End-of-Game Stats Retrieval

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

