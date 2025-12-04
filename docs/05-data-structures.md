# Derived Summary Structures

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

