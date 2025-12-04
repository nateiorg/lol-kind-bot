package analyzer

import (
	"fmt"
	"lol-kind-bot/config"
	"lol-kind-bot/eog"
	"lol-kind-bot/monitor"
	"math"
)

type PlayerMetrics struct {
	// Basic stats
	Kills   int
	Deaths  int
	Assists int
	
	// Derived metrics from golden rules
	KDA              float64
	TeamKills        int
	TeamDamageChamps int
	TeamGold         int
	GameMinutes      float64
	CSTotal          int
	CSPM             float64
	KP               float64 // Kill participation
	DamageShare      float64
	DPM              float64 // Damage per minute
	GoldShare        float64
	VSPM             float64 // Vision score per minute
	DTAM             float64 // Damage taken per minute
	TeamDamageTaken  int
	DamageTakenShare float64
	CCPerMinute      float64
	
	// Additional stats
	TotalHealsOnTeammates       int
	TotalDamageShieldedOnTeammates int
	TimeCCingOthers             int
	TotalDamageTaken            int
	TotalDamageSelfMitigated    int
	Role                        string
	
	// Advanced insights
	EfficiencyScore     float64 // Combined efficiency metric (damage + vision + utility)
	TeamplayScore       float64 // How much they enabled teammates
	SurvivabilityScore  float64 // Damage taken vs deaths ratio
	ImpactScore         float64 // Overall impact on game outcome
	GoldEfficiency      float64 // Damage per gold spent
	UtilityScore        float64 // Healing + shielding + CC contribution
	WellRoundedScore    float64 // Balance across multiple categories
}

type PlayerSummary struct {
	SummonerName string   `json:"summonerName"`
	Team         string   `json:"team"`
	Champion     string   `json:"champion"`
	K            int      `json:"k"`
	D            int      `json:"d"`
	A            int      `json:"a"`
	CsPerMin     float64  `json:"csPerMin"`
	DamageShare  float64  `json:"damageShare"`
	VisionScore  int      `json:"visionScore"`
	Tags         []string `json:"tags"`
	Afk          bool     `json:"afk"`
	Metrics      PlayerMetrics `json:"metrics"` // Detailed metrics for LLM analysis
	
	// Standout performance indicators
	HighestDamageInGame    bool    `json:"highestDamageInGame,omitempty"`
	HighestDamageOnTeam    bool    `json:"highestDamageOnTeam,omitempty"`
	MostHealingInGame      bool    `json:"mostHealingInGame,omitempty"`
	MostShieldingInGame    bool    `json:"mostShieldingInGame,omitempty"`
	MostHealingShielding   bool    `json:"mostHealingShielding,omitempty"`
	HighestVisionInGame    bool    `json:"highestVisionInGame,omitempty"`
	HighestVisionOnTeam    bool    `json:"highestVisionOnTeam,omitempty"`
	MostCCInGame           bool    `json:"mostCCInGame,omitempty"`
	MostCCOnTeam           bool    `json:"mostCCOnTeam,omitempty"`
	MostTankingInGame      bool    `json:"mostTankingInGame,omitempty"` // High damage taken with low deaths
	TotalDamage            int     `json:"totalDamage,omitempty"`
	TotalHealing           int     `json:"totalHealing,omitempty"`
	TotalShielding        int     `json:"totalShielding,omitempty"`
	TotalCC                int     `json:"totalCC,omitempty"`
	TotalDamageMitigated   int     `json:"totalDamageMitigated,omitempty"`
	
	// Human-readable formatted versions (for LLM prompts)
	TotalDamageFormatted   string  `json:"totalDamageFormatted,omitempty"`
	TotalHealingFormatted  string  `json:"totalHealingFormatted,omitempty"`
	TotalShieldingFormatted string `json:"totalShieldingFormatted,omitempty"`
	TotalCCFormatted       string  `json:"totalCCFormatted,omitempty"`
	TotalDamageMitigatedFormatted string `json:"totalDamageMitigatedFormatted,omitempty"`
	
	// Clutch moments (from live monitoring)
	LivesSaved            int     `json:"livesSaved,omitempty"`    // Times they saved others
	TimesSaved            int     `json:"timesSaved,omitempty"`     // Times they were saved
	CriticalSaves         int     `json:"criticalSaves,omitempty"`   // Critical saves (< 20% HP)
}

type TeamInsights struct {
	AverageKP            float64 `json:"averageKP,omitempty"`            // Average kill participation
	HighKPCount          int     `json:"highKPCount,omitempty"`          // Number of players with KP > 0.60
	TotalUtility         int     `json:"totalUtility,omitempty"`         // Total healing + shielding
	TotalVision          int     `json:"totalVision,omitempty"`          // Total vision score
	WellRoundedPlayers   int     `json:"wellRoundedPlayers,omitempty"`  // Players excelling in multiple areas
	SynergyScore         float64 `json:"synergyScore,omitempty"`         // Team coordination indicator
	CarryPerformance     string  `json:"carryPerformance,omitempty"`     // "distributed" or "focused"
	TeamComposition      string  `json:"teamComposition,omitempty"`     // "balanced", "damage-heavy", "utility-heavy"
	
	// Human-readable formatted versions
	TotalUtilityFormatted string `json:"totalUtilityFormatted,omitempty"`
	TotalVisionFormatted  string `json:"totalVisionFormatted,omitempty"`
}

type GameAchievements struct {
	HighestDamageInGame    string   `json:"highestDamageInGame,omitempty"`    // Champion name with highest damage
	HighestDamageOnMyTeam  string   `json:"highestDamageOnMyTeam,omitempty"`  // Champion name with highest damage on my team
	MostHealingShielding   string   `json:"mostHealingShielding,omitempty"`   // Champion name with most healing+shielding
	HighestVisionInGame    string   `json:"highestVisionInGame,omitempty"`    // Champion name with highest vision
	HighestVisionOnMyTeam  string   `json:"highestVisionOnMyTeam,omitempty"`  // Champion name with highest vision on my team
	MostCCInGame           string   `json:"mostCCInGame,omitempty"`           // Champion name with most CC
	MostCCOnMyTeam         string   `json:"mostCCOnMyTeam,omitempty"`         // Champion name with most CC on my team
	MostTankingInGame      string   `json:"mostTankingInGame,omitempty"`      // Champion name with most tanking
}

type GameSummary struct {
	GameDurationMinutes float64         `json:"gameDurationMinutes"`
	WinningTeam         string          `json:"winningTeam"`
	MySummonerName      string          `json:"mySummonerName"`
	MyTeam              string          `json:"myTeam"`
	Players             []PlayerSummary `json:"players"`
	AfkOnMyTeam         bool            `json:"afkOnMyTeam"`
	AfkOnEnemyTeam      bool            `json:"afkOnEnemyTeam"`
	
	// Game mode information
	GameMode            string          `json:"gameMode,omitempty"` // e.g., "ARAM", "CLASSIC"
	QueueType           string          `json:"queueType,omitempty"` // e.g., "ARAM", "RANKED_SOLO_5x5"
	GameType            string          `json:"gameType,omitempty"` // e.g., "MATCHED_GAME"
	
	// Match intensity indicators
	IsIntenseMatch      bool            `json:"isIntenseMatch,omitempty"`
	IsComeback          bool            `json:"isComeback,omitempty"`
	TotalKills          int             `json:"totalKills,omitempty"`
	KillDifference       int             `json:"killDifference,omitempty"`
	GoldDifference       int             `json:"goldDifference,omitempty"`
	
	// Team insights
	MyTeamInsights      TeamInsights    `json:"myTeamInsights,omitempty"`
	EnemyTeamInsights   TeamInsights    `json:"enemyTeamInsights,omitempty"`
	
	// Game story indicators
	WasStomp             bool            `json:"wasStomp,omitempty"`             // One-sided game
	WasClose             bool            `json:"wasClose,omitempty"`              // Very close game
	HadClutchMoments     bool            `json:"hadClutchMoments,omitempty"`      // Indicators of clutch plays
	TeamworkHighlight    string          `json:"teamworkHighlight,omitempty"`     // Key teamwork moment
	
	// Explicit achievements - LLM should use these directly, not calculate
	Achievements        GameAchievements `json:"achievements,omitempty"`
}

func AnalyzeGame(stats *eog.EoGStatsBlock, cfg *config.Config) (*GameSummary, error) {
	if len(stats.Participants) == 0 {
		return nil, nil
	}

	gameMinutes := float64(stats.GameDurationSeconds) / 60.0
	players := make([]PlayerSummary, 0, len(stats.Participants))

	// Helper functions defined at top for use throughout
	getPlayerName := func(p eog.EoGParticipant) string {
		if p.SummonerName != "" {
			return p.SummonerName
		}
		if p.RiotIdGameName != "" {
			// Check both tagline formats
			tagline := p.RiotIdTagline
			if tagline == "" {
				tagline = p.RiotIdTagLine
			}
			if tagline != "" {
				return p.RiotIdGameName + "#" + tagline
			}
			return p.RiotIdGameName
		}
		if p.DisplayName != "" {
			return p.DisplayName
		}
		return "Unknown"
	}
	
	getChampionName := func(p eog.EoGParticipant) string {
		if p.ChampionName != "" {
			return p.ChampionName
		}
		if p.ChampionID > 0 {
			return fmt.Sprintf("Champion%d", p.ChampionID)
		}
		return "Unknown"
	}
	
	// Helper function to get stat value (handle nested stats)
	getStatValue := func(p eog.EoGParticipant, statName string) int {
		if p.Stats != nil {
			switch statName {
			case "kills": return p.Stats.Kills
			case "totalDamageDealtToChampions": return p.Stats.TotalDamageDealtToChampions
			case "goldEarned": return p.Stats.GoldEarned
			case "totalDamageTaken": return p.Stats.TotalDamageTaken
			}
		}
		switch statName {
		case "kills": return p.Kills
		case "totalDamageDealtToChampions": return p.TotalDamageDealtToChampions
		case "goldEarned": return p.GoldEarned
		case "totalDamageTaken": return p.TotalDamageTaken
		}
		return 0
	}
	
	// Calculate team totals (first pass)
	teamKills := make(map[int]int)
	teamDamageChamps := make(map[int]int)
	teamGold := make(map[int]int)
	teamDamageTaken := make(map[int]int)
	
	for _, p := range stats.Participants {
		teamKills[p.TeamID] += getStatValue(p, "kills")
		teamDamageChamps[p.TeamID] += getStatValue(p, "totalDamageDealtToChampions")
		teamGold[p.TeamID] += getStatValue(p, "goldEarned")
		teamDamageTaken[p.TeamID] += getStatValue(p, "totalDamageTaken")
	}

	// Helper function to get player stats (handle nested stats object)
	getPlayerStat := func(p eog.EoGParticipant, statName string) int {
		if p.Stats != nil {
			switch statName {
			case "kills": return p.Stats.Kills
			case "deaths": return p.Stats.Deaths
			case "assists": return p.Stats.Assists
			case "totalMinionsKilled": return p.Stats.TotalMinionsKilled
			case "neutralMinionsKilled": return p.Stats.NeutralMinionsKilled
			case "goldEarned": return p.Stats.GoldEarned
			case "totalDamageDealtToChampions": return p.Stats.TotalDamageDealtToChampions
			case "visionScore": return p.Stats.VisionScore
			case "totalDamageTaken": return p.Stats.TotalDamageTaken
			case "timeCCingOthers": return p.Stats.TimeCCingOthers
			case "totalHealsOnTeammates": return p.Stats.TotalHealsOnTeammates
			case "totalDamageShieldedOnTeammates": return p.Stats.TotalDamageShieldedOnTeammates
			}
		}
		// Fallback to top-level fields
		switch statName {
		case "kills": return p.Kills
		case "deaths": return p.Deaths
		case "assists": return p.Assists
		case "totalMinionsKilled": return p.TotalMinionsKilled
		case "neutralMinionsKilled": return p.NeutralMinionsKilled
		case "goldEarned": return p.GoldEarned
		case "totalDamageDealtToChampions": return p.TotalDamageDealtToChampions
		case "visionScore": return p.VisionScore
		case "totalDamageTaken": return p.TotalDamageTaken
		case "timeCCingOthers": return p.TimeCCingOthers
		case "totalHealsOnTeammates": return p.TotalHealsOnTeammates
		case "totalDamageShieldedOnTeammates": return p.TotalDamageShieldedOnTeammates
		case "totalDamageSelfMitigated": return p.TotalDamageSelfMitigated
		}
		return 0
	}
	
	// Find my team
	myTeamID := 0
	for _, p := range stats.Participants {
		playerName := getPlayerName(p)
		if playerName == cfg.MySummonerName {
			myTeamID = p.TeamID
			break
		}
	}

	// Second pass: create player summaries with all derived metrics
	for _, p := range stats.Participants {
		// Get stats (from nested stats object or top-level)
		kills := getPlayerStat(p, "kills")
		deaths := getPlayerStat(p, "deaths")
		assists := getPlayerStat(p, "assists")
		totalMinionsKilled := getPlayerStat(p, "totalMinionsKilled")
		neutralMinionsKilled := getPlayerStat(p, "neutralMinionsKilled")
		goldEarned := getPlayerStat(p, "goldEarned")
		totalDamageDealtToChampions := getPlayerStat(p, "totalDamageDealtToChampions")
		visionScore := getPlayerStat(p, "visionScore")
		totalDamageTaken := getPlayerStat(p, "totalDamageTaken")
		timeCCingOthers := getPlayerStat(p, "timeCCingOthers")
		totalHealsOnTeammates := getPlayerStat(p, "totalHealsOnTeammates")
		totalDamageShieldedOnTeammates := getPlayerStat(p, "totalDamageShieldedOnTeammates")
		totalDamageSelfMitigated := getPlayerStat(p, "totalDamageSelfMitigated")
		
		csTotal := totalMinionsKilled + neutralMinionsKilled
		cspm := float64(csTotal) / math.Max(1.0, gameMinutes)
		
		teamK := teamKills[p.TeamID]
		teamDmg := teamDamageChamps[p.TeamID]
		teamG := teamGold[p.TeamID]
		teamDT := teamDamageTaken[p.TeamID]
		
		kda := float64(kills+assists) / math.Max(1.0, float64(deaths))
		kp := float64(kills+assists) / math.Max(1.0, float64(teamK))
		damageShare := float64(totalDamageDealtToChampions) / math.Max(1.0, float64(teamDmg))
		dpm := float64(totalDamageDealtToChampions) / math.Max(1.0, gameMinutes)
		goldShare := float64(goldEarned) / math.Max(1.0, float64(teamG))
		vspm := float64(visionScore) / math.Max(1.0, gameMinutes)
		dtam := float64(totalDamageTaken) / math.Max(1.0, gameMinutes)
		damageTakenShare := float64(totalDamageTaken) / math.Max(1.0, float64(teamDT))
		ccPerMinute := float64(timeCCingOthers) / math.Max(1.0, gameMinutes)

		// Detect role if not provided by API (heuristic fallback)
		detectedRole := p.Role
		if detectedRole == "" {
			detectedRole = detectRoleFromStats(
				neutralMinionsKilled,
				visionScore,
				totalHealsOnTeammates+totalDamageShieldedOnTeammates,
				cspm,
				damageShare,
				gameMinutes,
			)
		}

		metrics := PlayerMetrics{
			Kills:                        kills,
			Deaths:                       deaths,
			Assists:                      assists,
			KDA:                          kda,
			TeamKills:                    teamK,
			TeamDamageChamps:             teamDmg,
			TeamGold:                     teamG,
			GameMinutes:                  gameMinutes,
			CSTotal:                       csTotal,
			CSPM:                          cspm,
			KP:                            kp,
			DamageShare:                  damageShare,
			DPM:                           dpm,
			GoldShare:                     goldShare,
			VSPM:                          vspm,
			DTAM:                          dtam,
			TeamDamageTaken:               teamDT,
			DamageTakenShare:              damageTakenShare,
			CCPerMinute:                   ccPerMinute,
			TotalHealsOnTeammates:         totalHealsOnTeammates,
			TotalDamageShieldedOnTeammates: totalDamageShieldedOnTeammates,
			TimeCCingOthers:              timeCCingOthers,
			TotalDamageTaken:              totalDamageTaken,
			TotalDamageSelfMitigated:      totalDamageSelfMitigated,
			Role:                          detectedRole,
		}

		players = append(players, PlayerSummary{
			SummonerName: getPlayerName(p),
			Team:         eog.TeamIDToSide(p.TeamID),
			Champion:     getChampionName(p),
			K:            kills,
			D:            deaths,
			A:            assists,
			CsPerMin:     cspm,
			DamageShare:  damageShare,
			VisionScore:  visionScore,
			Tags:         []string{},
			Afk:          false,
			Metrics:      metrics,
			TotalDamage:  totalDamageDealtToChampions,
			TotalHealing: totalHealsOnTeammates,
			TotalShielding: totalDamageShieldedOnTeammates,
			TotalCC: timeCCingOthers,
			TotalDamageMitigated: totalDamageSelfMitigated,
			// Add human-readable formatted versions
			TotalDamageFormatted: FormatNumber(totalDamageDealtToChampions),
			TotalHealingFormatted: FormatNumber(totalHealsOnTeammates),
			TotalShieldingFormatted: FormatNumber(totalDamageShieldedOnTeammates),
			TotalCCFormatted: FormatNumber(timeCCingOthers),
			TotalDamageMitigatedFormatted: FormatNumber(totalDamageSelfMitigated),
		})
	}

	// Detect AFKs using Riot's official leaver flag FIRST (most reliable)
	// Fall back to heuristic detection if flag is not available
	// This MUST happen before standout performance detection to exclude AFKs
	for i := range players {
		p := stats.Participants[i]
		
		// Use Riot's official leaver flag (primary method - most reliable)
		if p.Leaver {
			players[i].Afk = true
			continue
		}
		// Also check nested stats if leaver flag is there
		if p.Stats != nil && p.Stats.Leaver {
			players[i].Afk = true
			continue
		}
		
		// Heuristic fallback: detect AFK using configurable thresholds
		// Only apply if game duration is sufficient and all conditions are met
		if gameMinutes >= cfg.AFKThresholds.MinGameMinutes {
			csTotal := getPlayerStat(p, "totalMinionsKilled") + getPlayerStat(p, "neutralMinionsKilled")
			cspm := float64(csTotal) / math.Max(1.0, gameMinutes)
			totalDamage := getPlayerStat(p, "totalDamageDealtToChampions")
			goldEarned := getPlayerStat(p, "goldEarned")
			kills := getPlayerStat(p, "kills")
			assists := getPlayerStat(p, "assists")
			
			// All heuristic conditions must be met for AFK detection
			if cspm < cfg.AFKThresholds.MaxCsPerMin &&
				totalDamage < cfg.AFKThresholds.MaxDamageToChamp &&
				goldEarned < cfg.AFKThresholds.MaxGoldEarned &&
				kills == 0 && assists == 0 {
				players[i].Afk = true
			}
		}
	}

	// Identify standout performances AFTER AFK detection
	// This ensures AFK players are excluded from standout calculations
	identifyStandoutPerformances(&players, stats, myTeamID)
	
	// Assign tags to non-AFK players using golden rules
	assignGoldenRulesTags(&players, stats, gameMinutes, myTeamID)

	// Determine winning team
	winningTeam := "UNKNOWN"
	for _, p := range stats.Participants {
		if p.Win {
			winningTeam = eog.TeamIDToSide(p.TeamID)
			break
		}
	}

	// Check for AFKs on each team (context flags)
	afkOnMyTeam := false
	afkOnEnemyTeam := false
	for i, player := range players {
		if player.Afk {
			participantTeamID := stats.Participants[i].TeamID
			if myTeamID > 0 && participantTeamID == myTeamID {
				afkOnMyTeam = true
			} else if myTeamID > 0 && participantTeamID != myTeamID {
				afkOnEnemyTeam = true
			}
		}
	}

	// Calculate match intensity indicators with enhanced analysis
	totalKills := 0
	myTeamKills := 0
	enemyTeamKills := 0
	myTeamGold := 0
	enemyTeamGold := 0
	myTeamDamage := 0
	enemyTeamDamage := 0
	myTeamDeaths := 0
	enemyTeamDeaths := 0
	
	for i, player := range players {
		if !player.Afk {
			participantTeamID := stats.Participants[i].TeamID
			kills := getStatValue(stats.Participants[i], "kills")
			gold := getStatValue(stats.Participants[i], "goldEarned")
			damage := getStatValue(stats.Participants[i], "totalDamageDealtToChampions")
			deaths := getStatValue(stats.Participants[i], "deaths")
			
			totalKills += kills
			if participantTeamID == myTeamID {
				myTeamKills += kills
				myTeamGold += gold
				myTeamDamage += damage
				myTeamDeaths += deaths
			} else {
				enemyTeamKills += kills
				enemyTeamGold += gold
				enemyTeamDamage += damage
				enemyTeamDeaths += deaths
			}
		}
	}
	
	killDiff := myTeamKills - enemyTeamKills
	if killDiff < 0 {
		killDiff = -killDiff
	}
	
	goldDiff := myTeamGold - enemyTeamGold
	if goldDiff < 0 {
		goldDiff = -goldDiff
	}
	
	damageDiff := myTeamDamage - enemyTeamDamage
	if damageDiff < 0 {
		damageDiff = -damageDiff
	}
	
	// Enhanced comeback detection: analyze multiple factors
	// A comeback is more likely if:
	// 1. We won despite being behind in kills/gold/damage early (inferred from final stats)
	// 2. The final difference is small (suggests it was close throughout)
	// 3. High total kills (suggests back-and-forth teamfights)
	// 4. Long game duration (more time for comebacks)
	isIntenseMatch := false
	isComeback := false
	
	// Intensity indicators
	if gameMinutes >= 30 {
		// Long games are often intense
		isIntenseMatch = true
	}
	
	if killDiff <= 10 && totalKills >= 40 {
		// Close game with many kills
		isIntenseMatch = true
	}
	
	if totalKills >= 60 {
		// Very high kill count indicates intense teamfights
		isIntenseMatch = true
	}
	
	// Enhanced comeback detection
	didWin := winningTeam == eog.TeamIDToSide(myTeamID)
	if didWin {
		// Multiple indicators of a comeback victory:
		// 1. Close final score (suggests it was competitive)
		// 2. High total kills (back-and-forth action)
		// 3. Long game (time for momentum shifts)
		// 4. Relatively low kill difference despite winning (suggests we were behind)
		comebackScore := 0
		if killDiff <= 8 && totalKills >= 50 {
			comebackScore += 2 // Close score with high action
		}
		if gameMinutes >= 35 {
			comebackScore += 1 // Long game allows for comebacks
		}
		if killDiff <= 5 && totalKills >= 60 {
			comebackScore += 2 // Very close with extreme action
		}
		// If we won but had more deaths, it suggests we were behind early
		if myTeamDeaths > enemyTeamDeaths && totalKills >= 50 {
			comebackScore += 1
		}
		
		if comebackScore >= 3 {
			isComeback = true
			isIntenseMatch = true
		} else if comebackScore >= 2 && gameMinutes >= 30 {
			// Moderate comeback indicators with long game
			isComeback = true
			isIntenseMatch = true
		}
	}
	
	// Additional intensity indicators
	if damageDiff <= 10000 && (myTeamDamage+enemyTeamDamage) >= 200000 {
		// Very close damage totals with high overall damage = intense match
		isIntenseMatch = true
	}
	
	// Calculate team insights
	myTeamInsights := calculateTeamInsights(players, stats, myTeamID, true)
	enemyTeamInsights := calculateTeamInsights(players, stats, myTeamID, false)
	
	// Determine game story indicators
	wasStomp := killDiff >= 20 || goldDiff >= 15000
	wasClose := killDiff <= 5 && totalKills >= 40
	hadClutchMoments := isComeback || (killDiff <= 3 && totalKills >= 50)
	
	// Determine teamwork highlight
	teamworkHighlight := determineTeamworkHighlight(myTeamInsights, players, stats, myTeamID)
	
	// Calculate explicit achievements - LLM should use these directly, not infer from flags
	achievements := calculateAchievements(players, stats, myTeamID)

	return &GameSummary{
		GameDurationMinutes: gameMinutes,
		WinningTeam:         winningTeam,
		MySummonerName:      cfg.MySummonerName,
		MyTeam:              eog.TeamIDToSide(myTeamID),
		Players:             players,
		AfkOnMyTeam:         afkOnMyTeam,
		AfkOnEnemyTeam:      afkOnEnemyTeam,
		IsIntenseMatch:      isIntenseMatch,
		IsComeback:          isComeback,
		TotalKills:          totalKills,
		KillDifference:      killDiff,
		GoldDifference:      goldDiff,
		GameMode:            stats.GameMode,
		QueueType:           stats.QueueType,
		GameType:            stats.GameType,
		MyTeamInsights:      myTeamInsights,
		EnemyTeamInsights:   enemyTeamInsights,
		WasStomp:            wasStomp,
		WasClose:            wasClose,
		HadClutchMoments:    hadClutchMoments,
		TeamworkHighlight:  teamworkHighlight,
		Achievements:        achievements,
	}, nil
}

// calculateAchievements computes explicit achievement data for LLM - no inference needed
func calculateAchievements(players []PlayerSummary, stats *eog.EoGStatsBlock, myTeamID int) GameAchievements {
	achievements := GameAchievements{}
	
	// Find highest damage in game
	maxDamage := 0
	maxDamageChamp := ""
	for _, player := range players {
		if player.Afk {
			continue
		}
		if player.HighestDamageInGame && player.TotalDamage > maxDamage {
			maxDamage = player.TotalDamage
			maxDamageChamp = player.Champion
		}
	}
	if maxDamageChamp != "" {
		achievements.HighestDamageInGame = maxDamageChamp
	}
	
	// Find highest damage on my team
	maxDamageMyTeam := 0
	maxDamageChampMyTeam := ""
	for i, player := range players {
		if player.Afk {
			continue
		}
		participantTeamID := stats.Participants[i].TeamID
		if participantTeamID == myTeamID && player.HighestDamageOnTeam && player.TotalDamage > maxDamageMyTeam {
			maxDamageMyTeam = player.TotalDamage
			maxDamageChampMyTeam = player.Champion
		}
	}
	if maxDamageChampMyTeam != "" {
		achievements.HighestDamageOnMyTeam = maxDamageChampMyTeam
	}
	
	// Find most healing/shielding
	maxHealShield := 0
	maxHealShieldChamp := ""
	for _, player := range players {
		if player.Afk {
			continue
		}
		totalHealShield := player.TotalHealing + player.TotalShielding
		if player.MostHealingShielding && totalHealShield > maxHealShield {
			maxHealShield = totalHealShield
			maxHealShieldChamp = player.Champion
		}
	}
	if maxHealShieldChamp != "" {
		achievements.MostHealingShielding = maxHealShieldChamp
	}
	
	// Find highest vision in game
	maxVision := 0
	maxVisionChamp := ""
	for _, player := range players {
		if player.Afk {
			continue
		}
		if player.HighestVisionInGame && player.VisionScore > maxVision {
			maxVision = player.VisionScore
			maxVisionChamp = player.Champion
		}
	}
	if maxVisionChamp != "" {
		achievements.HighestVisionInGame = maxVisionChamp
	}
	
	// Find highest vision on my team
	maxVisionMyTeam := 0
	maxVisionChampMyTeam := ""
	for i, player := range players {
		if player.Afk {
			continue
		}
		participantTeamID := stats.Participants[i].TeamID
		if participantTeamID == myTeamID && player.HighestVisionOnTeam && player.VisionScore > maxVisionMyTeam {
			maxVisionMyTeam = player.VisionScore
			maxVisionChampMyTeam = player.Champion
		}
	}
	if maxVisionChampMyTeam != "" {
		achievements.HighestVisionOnMyTeam = maxVisionChampMyTeam
	}
	
	// Find most CC in game
	maxCC := 0
	maxCCChamp := ""
	for _, player := range players {
		if player.Afk {
			continue
		}
		if player.MostCCInGame && player.TotalCC > maxCC {
			maxCC = player.TotalCC
			maxCCChamp = player.Champion
		}
	}
	if maxCCChamp != "" {
		achievements.MostCCInGame = maxCCChamp
	}
	
	// Find most CC on my team
	maxCCMyTeam := 0
	maxCCChampMyTeam := ""
	for i, player := range players {
		if player.Afk {
			continue
		}
		participantTeamID := stats.Participants[i].TeamID
		if participantTeamID == myTeamID && player.MostCCOnTeam && player.TotalCC > maxCCMyTeam {
			maxCCMyTeam = player.TotalCC
			maxCCChampMyTeam = player.Champion
		}
	}
	if maxCCChampMyTeam != "" {
		achievements.MostCCOnMyTeam = maxCCChampMyTeam
	}
	
	// Find most tanking in game
	maxTankingScore := 0.0
	maxTankingChamp := ""
	for _, player := range players {
		if player.Afk {
			continue
		}
		if player.MostTankingInGame {
			// Tanking score: damage taken / (deaths + 1)
			deaths := player.D
			if deaths == 0 {
				deaths = 1
			}
			tankingScore := float64(player.Metrics.TotalDamageTaken) / float64(deaths)
			if tankingScore > maxTankingScore {
				maxTankingScore = tankingScore
				maxTankingChamp = player.Champion
			}
		}
	}
	if maxTankingChamp != "" {
		achievements.MostTankingInGame = maxTankingChamp
	}
	
	return achievements
}

// calculateTeamInsights computes sophisticated team-level insights
func calculateTeamInsights(players []PlayerSummary, stats *eog.EoGStatsBlock, myTeamID int, isMyTeam bool) TeamInsights {
	insights := TeamInsights{}
	teamPlayers := []PlayerSummary{}
	
	// Filter players by team
	for i, player := range players {
		if player.Afk {
			continue
		}
		participantTeamID := stats.Participants[i].TeamID
		if isMyTeam && participantTeamID == myTeamID {
			teamPlayers = append(teamPlayers, player)
		} else if !isMyTeam && participantTeamID != myTeamID {
			teamPlayers = append(teamPlayers, player)
		}
	}
	
	if len(teamPlayers) == 0 {
		return insights
	}
	
	// Calculate averages and totals
	totalKP := 0.0
	highKPCount := 0
	totalUtility := 0
	totalVision := 0
	wellRoundedCount := 0
	totalSynergy := 0.0
	
	for _, player := range teamPlayers {
		totalKP += player.Metrics.KP
		if player.Metrics.KP >= 0.60 {
			highKPCount++
		}
		totalUtility += player.TotalHealing + player.TotalShielding
		totalVision += player.VisionScore
		
		// Well-rounded: excelling in 2+ areas
		excellenceCount := 0
		if player.Metrics.DamageShare >= 0.25 { excellenceCount++ }
		if player.Metrics.VSPM >= 1.5 { excellenceCount++ }
		if player.Metrics.KP >= 0.60 { excellenceCount++ }
		if player.Metrics.UtilityScore >= 5.0 { excellenceCount++ }
		if player.Metrics.CSPM >= 6.0 { excellenceCount++ }
		if excellenceCount >= 2 {
			wellRoundedCount++
		}
		
		// Synergy: high KP + utility contribution
		synergy := player.Metrics.KP * 0.6
		if player.Metrics.UtilityScore > 0 {
			synergy += (player.Metrics.UtilityScore / 10.0) * 0.4
		}
		totalSynergy += synergy
	}
	
	insights.AverageKP = totalKP / float64(len(teamPlayers))
	insights.HighKPCount = highKPCount
	insights.TotalUtility = totalUtility
	insights.TotalVision = totalVision
	insights.TotalUtilityFormatted = FormatNumber(totalUtility)
	insights.TotalVisionFormatted = FormatNumber(totalVision)
	insights.WellRoundedPlayers = wellRoundedCount
	insights.SynergyScore = totalSynergy / float64(len(teamPlayers))
	
	// Determine carry performance style
	highDamageCount := 0
	for _, player := range teamPlayers {
		if player.Metrics.DamageShare >= 0.25 {
			highDamageCount++
		}
	}
	if highDamageCount >= 3 {
		insights.CarryPerformance = "distributed"
	} else if highDamageCount >= 1 {
		insights.CarryPerformance = "focused"
	} else {
		insights.CarryPerformance = "balanced"
	}
	
	// Determine team composition
	totalDamage := 0
	for _, player := range teamPlayers {
		totalDamage += player.TotalDamage
	}
	avgDamage := float64(totalDamage) / float64(len(teamPlayers))
	
	if avgDamage >= 50000 && totalUtility < 20000 {
		insights.TeamComposition = "damage-heavy"
	} else if totalUtility >= 30000 {
		insights.TeamComposition = "utility-heavy"
	} else {
		insights.TeamComposition = "balanced"
	}
	
	return insights
}

// determineTeamworkHighlight identifies key teamwork moments to highlight
func determineTeamworkHighlight(insights TeamInsights, players []PlayerSummary, stats *eog.EoGStatsBlock, myTeamID int) string {
	if insights.HighKPCount >= 4 {
		return "exceptional_team_coordination"
	}
	if insights.SynergyScore >= 0.7 {
		return "strong_team_synergy"
	}
	if insights.WellRoundedPlayers >= 3 {
		return "well_rounded_team"
	}
	if insights.TotalUtility >= 30000 {
		return "outstanding_support_play"
	}
	if insights.AverageKP >= 0.65 {
		return "high_team_participation"
	}
	return ""
}

func identifyStandoutPerformances(players *[]PlayerSummary, stats *eog.EoGStatsBlock, myTeamID int) {
	// Find highest values across all players (non-AFK only)
	maxDamage := 0
	maxHealing := 0
	maxShielding := 0
	maxHealingShielding := 0
	maxVision := 0
	maxCC := 0
	maxTankingScore := 0.0 // damage taken / (deaths + 1) - higher is better tanking
	
	for i := range *players {
		if (*players)[i].Afk {
			continue
		}
		if (*players)[i].TotalDamage > maxDamage {
			maxDamage = (*players)[i].TotalDamage
		}
		if (*players)[i].TotalHealing > maxHealing {
			maxHealing = (*players)[i].TotalHealing
		}
		if (*players)[i].TotalShielding > maxShielding {
			maxShielding = (*players)[i].TotalShielding
		}
		totalHealShield := (*players)[i].TotalHealing + (*players)[i].TotalShielding
		if totalHealShield > maxHealingShielding {
			maxHealingShielding = totalHealShield
		}
		if (*players)[i].VisionScore > maxVision {
			maxVision = (*players)[i].VisionScore
		}
		if (*players)[i].TotalCC > maxCC {
			maxCC = (*players)[i].TotalCC
		}
		// Tanking score: high damage taken with low deaths = good tanking
		deaths := (*players)[i].D
		if deaths == 0 {
			deaths = 1 // Avoid division by zero
		}
		tankingScore := float64((*players)[i].Metrics.TotalDamageTaken) / float64(deaths)
		if tankingScore > maxTankingScore {
			maxTankingScore = tankingScore
		}
	}
	
	// Find highest values per team
	teamMaxDamage := make(map[int]int)
	teamMaxVision := make(map[int]int)
	teamMaxCC := make(map[int]int)
	
	for i := range *players {
		if (*players)[i].Afk {
			continue
		}
		teamID := stats.Participants[i].TeamID
		if (*players)[i].TotalDamage > teamMaxDamage[teamID] {
			teamMaxDamage[teamID] = (*players)[i].TotalDamage
		}
		if (*players)[i].VisionScore > teamMaxVision[teamID] {
			teamMaxVision[teamID] = (*players)[i].VisionScore
		}
		if (*players)[i].TotalCC > teamMaxCC[teamID] {
			teamMaxCC[teamID] = (*players)[i].TotalCC
		}
	}
	
	// Mark standout performances
	for i := range *players {
		if (*players)[i].Afk {
			continue
		}
		
		// Highest damage in game
		// Only mark ONE player as highest (the first one found with max damage)
		// This prevents ties from causing confusion
		if (*players)[i].TotalDamage == maxDamage && maxDamage > 0 {
			// Check if we've already marked someone as highest
			alreadyMarked := false
			for j := range *players {
				if (*players)[j].HighestDamageInGame {
					alreadyMarked = true
					break
				}
			}
			if !alreadyMarked {
				(*players)[i].HighestDamageInGame = true
			}
		}
		
		// Highest damage on team
		// Only mark ONE player per team as highest
		teamID := stats.Participants[i].TeamID
		if (*players)[i].TotalDamage == teamMaxDamage[teamID] && teamMaxDamage[teamID] > 0 {
			// Check if we've already marked someone on this team as highest
			alreadyMarkedOnTeam := false
			for j := range *players {
				if stats.Participants[j].TeamID == teamID && (*players)[j].HighestDamageOnTeam {
					alreadyMarkedOnTeam = true
					break
				}
			}
			if !alreadyMarkedOnTeam {
				(*players)[i].HighestDamageOnTeam = true
			}
		}
		
		// Most healing in game
		if (*players)[i].TotalHealing == maxHealing && maxHealing > 0 {
			(*players)[i].MostHealingInGame = true
		}
		
		// Most shielding in game
		if (*players)[i].TotalShielding == maxShielding && maxShielding > 0 {
			(*players)[i].MostShieldingInGame = true
		}
		
		// Most healing + shielding combined
		totalHealShield := (*players)[i].TotalHealing + (*players)[i].TotalShielding
		if totalHealShield == maxHealingShielding && maxHealingShielding > 0 {
			(*players)[i].MostHealingShielding = true
		}
		
		// Highest vision in game
		if (*players)[i].VisionScore == maxVision && maxVision > 0 {
			(*players)[i].HighestVisionInGame = true
		}
		
		// Highest vision on team
		if (*players)[i].VisionScore == teamMaxVision[teamID] && teamMaxVision[teamID] > 0 {
			(*players)[i].HighestVisionOnTeam = true
		}
		
		// Most CC in game
		if (*players)[i].TotalCC == maxCC && maxCC > 0 {
			(*players)[i].MostCCInGame = true
		}
		
		// Most CC on team
		if (*players)[i].TotalCC == teamMaxCC[teamID] && teamMaxCC[teamID] > 0 {
			(*players)[i].MostCCOnTeam = true
		}
		
		// Most tanking in game (high damage taken with low deaths)
		deaths := (*players)[i].D
		if deaths == 0 {
			deaths = 1
		}
		tankingScore := float64((*players)[i].Metrics.TotalDamageTaken) / float64(deaths)
		if tankingScore == maxTankingScore && maxTankingScore > 1000 && deaths <= 5 {
			// Only mark if they took significant damage and didn't die too much
			(*players)[i].MostTankingInGame = true
		}
	}
}

func assignGoldenRulesTags(players *[]PlayerSummary, stats *eog.EoGStatsBlock, gameMinutes float64, myTeamID int) {
	// Group players by team for team-based comparisons
	teamPlayers := make(map[int][]int) // teamID -> []player indices
	for i, p := range *players {
		if !p.Afk {
			teamID := stats.Participants[i].TeamID
			teamPlayers[teamID] = append(teamPlayers[teamID], i)
		}
	}

	// Calculate team objective stats (if not provided)
	teamDragons := make(map[int]int)
	teamBarons := make(map[int]int)
	if stats.TeamDragons != nil {
		teamDragons = stats.TeamDragons
	}
	if stats.TeamBarons != nil {
		teamBarons = stats.TeamBarons
	}

		for i := range *players {
		if (*players)[i].Afk {
			continue
		}

		p := (*players)[i]
		m := p.Metrics
		participant := stats.Participants[i]
		teamID := participant.TeamID
		teamIndices := teamPlayers[teamID]

		// hard_carry: High damage + high kill participation + decent KDA
		if m.DamageShare >= 0.30 && m.KP >= 0.60 && (m.KDA >= 2.0 || (m.Kills+m.Assists) >= 10) {
			(*players)[i].Tags = append((*players)[i].Tags, "hard_carry")
		}

		// frontline_rock: Soaks lots of damage and applies CC without inting
		if m.DamageTakenShare >= 0.30 && m.Deaths <= 8 {
			// Check if CC per minute is top 2 on team
			ccValues := make([]float64, 0)
			for _, idx := range teamIndices {
				ccValues = append(ccValues, (*players)[idx].Metrics.CCPerMinute)
			}
			if isTopN(m.CCPerMinute, ccValues, 2) {
				(*players)[i].Tags = append((*players)[i].Tags, "frontline_rock")
			}
		}

		// vision_mvp: Best vision control on the team
		vspmValues := make([]float64, 0)
		for _, idx := range teamIndices {
			vspmValues = append(vspmValues, (*players)[idx].Metrics.VSPM)
		}
		if isHighest(m.VSPM, vspmValues) && m.VSPM >= 1.5 {
			(*players)[i].Tags = append((*players)[i].Tags, "vision_mvp")
		}

		// utility_mvp: Heals/shields and CC for the team
		if m.Assists >= 10 {
			healShieldValues := make([]int, 0)
			ccValues := make([]float64, 0)
			for _, idx := range teamIndices {
				healShield := (*players)[idx].Metrics.TotalHealsOnTeammates + (*players)[idx].Metrics.TotalDamageShieldedOnTeammates
				healShieldValues = append(healShieldValues, healShield)
				ccValues = append(ccValues, (*players)[idx].Metrics.CCPerMinute)
			}
			healShieldTotal := m.TotalHealsOnTeammates + m.TotalDamageShieldedOnTeammates
			if isHighest(float64(healShieldTotal), floatSlice(healShieldValues)) || isHighest(m.CCPerMinute, ccValues) {
				(*players)[i].Tags = append((*players)[i].Tags, "utility_mvp")
			}
		}

		// objective_brain: High KP on a team with strong objective control (jungler/support)
		if (m.Role == "JUNGLE" || m.Role == "SUPPORT") && m.KP >= 0.60 {
			enemyTeamID := eog.TeamIDBlue
			if teamID == eog.TeamIDBlue {
				enemyTeamID = eog.TeamIDRed
			}
			myDragons := teamDragons[teamID]
			enemyDragons := teamDragons[enemyTeamID]
			myBarons := teamBarons[teamID]
			enemyBarons := teamBarons[enemyTeamID]
			
			if myDragons >= enemyDragons && myBarons >= enemyBarons {
				(*players)[i].Tags = append((*players)[i].Tags, "objective_brain")
			}
		}

		// weakside_warrior: Low-resources but low deaths and decent contribution
		if m.GoldShare <= 0.18 && m.Deaths <= 5 && (m.KP >= 0.40 || m.Assists >= 8) {
			(*players)[i].Tags = append((*players)[i].Tags, "weakside_warrior")
		}

		// heroic_in_loss: Standout positive performance in a loss
		participantWin := false
		if participant.Stats != nil {
			participantWin = participant.Stats.Win
		} else {
			participantWin = participant.Win
		}
		if !participantWin && gameMinutes >= 25 {
			hasOtherTag := false
			for _, tag := range p.Tags {
				if tag == "hard_carry" || tag == "frontline_rock" || tag == "vision_mvp" || tag == "utility_mvp" {
					hasOtherTag = true
					break
				}
			}
			if hasOtherTag {
				(*players)[i].Tags = append((*players)[i].Tags, "heroic_in_loss")
			}
		}
	}
}

// Helper functions for comparisons
func isHighest(value float64, values []float64) bool {
	if len(values) == 0 {
		return false
	}
	maxVal := values[0]
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	return value >= maxVal && value > 0
}

func isTopN(value float64, values []float64, n int) bool {
	if len(values) == 0 || n <= 0 {
		return false
	}
	
	// Sort values in descending order
	sorted := make([]float64, len(values))
	copy(sorted, values)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] < sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	
	// Check if value is in top N
	if n > len(sorted) {
		n = len(sorted)
	}
	for i := 0; i < n; i++ {
		if value >= sorted[i] && value > 0 {
			return true
		}
	}
	return false
}

func floatSlice(ints []int) []float64 {
	result := make([]float64, len(ints))
	for i, v := range ints {
		result[i] = float64(v)
	}
	return result
}

// detectRoleFromStats attempts to infer player role from statistics
// This is a fallback when the API doesn't provide role information
func detectRoleFromStats(
	neutralMinionsKilled int,
	visionScore int,
	healShieldTotal int,
	cspm float64,
	damageShare float64,
	gameMinutes float64,
) string {
	// Jungle: High neutral minions (jungle camps)
	if neutralMinionsKilled >= 50 || (neutralMinionsKilled >= 30 && gameMinutes >= 20) {
		return "JUNGLE"
	}
	
	// Support: High healing/shielding OR very high vision score with low CS
	if healShieldTotal >= 5000 || (visionScore >= 50 && cspm < 2.0) {
		return "SUPPORT"
	}
	
	// ADC/Bottom: High CS per minute (typically highest CS)
	if cspm >= 7.0 {
		return "BOTTOM"
	}
	
	// Mid: Moderate CS, often high damage share
	if cspm >= 5.0 && damageShare >= 0.25 {
		return "MIDDLE"
	}
	
	// Top: Lower CS than mid/adc, often tankier
	if cspm >= 4.0 {
		return "TOP"
	}
	
	// Default fallback
	return ""
}

// IntegrateClutchStats integrates clutch monitoring stats into game summary
func IntegrateClutchStats(summary *GameSummary, clutchStats map[string]*monitor.ClutchStats) {
	if summary == nil || clutchStats == nil {
		return
	}

	// Match clutch stats to players by champion name
	for i := range summary.Players {
		player := &summary.Players[i]
		if stats, found := clutchStats[player.Champion]; found {
			player.LivesSaved = stats.LivesSaved
			player.TimesSaved = stats.TimesSaved
			
			// Count critical saves
			criticalCount := 0
			for _, event := range stats.Events {
				if event.WasCritical {
					criticalCount++
				}
			}
			player.CriticalSaves = criticalCount
			
			// Add tags based on clutch performance
			if stats.LivesSaved >= 5 {
				player.Tags = append(player.Tags, "clutch_savior")
			}
			if criticalCount >= 3 {
				player.Tags = append(player.Tags, "critical_savior")
			}
		}
	}
}
