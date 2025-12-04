package eog

import (
	"encoding/json"
	"fmt"
)

const (
	TeamIDBlue = 100
	TeamIDRed  = 200
)

type EoGParticipant struct {
	// Name fields (multiple possible formats)
	SummonerName  string `json:"summonerName,omitempty"`
	RiotIdGameName string `json:"riotIdGameName,omitempty"`
	DisplayName   string `json:"displayName,omitempty"`
	RiotIdTagline string `json:"riotIdTagline,omitempty"`
	RiotIdTagLine string `json:"riotIdTagLine,omitempty"` // API uses capital L
	
	TeamID        int    `json:"teamId"`
	
	// Champion fields
	ChampionName  string `json:"championName,omitempty"`
	ChampionID    int    `json:"championId,omitempty"`
	
	// Stats
	Kills                       int    `json:"kills"`
	Deaths                      int    `json:"deaths"`
	Assists                     int    `json:"assists"`
	TotalMinionsKilled          int    `json:"totalMinionsKilled"`
	NeutralMinionsKilled        int    `json:"neutralMinionsKilled"`
	GoldEarned                  int    `json:"goldEarned"`
	TotalDamageDealtToChampions int    `json:"totalDamageDealtToChampions"`
	VisionScore                 int    `json:"visionScore"`
	Win                         bool   `json:"win"`
	
	// Additional fields for golden rules (may not be in API, default to 0)
	TotalDamageTaken            int `json:"totalDamageTaken,omitempty"`
	TimeCCingOthers             int `json:"timeCCingOthers,omitempty"`
	TotalHealsOnTeammates       int `json:"totalHealsOnTeammates,omitempty"`
	TotalDamageShieldedOnTeammates int `json:"totalDamageShieldedOnTeammates,omitempty"`
	TotalDamageSelfMitigated    int `json:"totalDamageSelfMitigated,omitempty"`
	Role                        string `json:"role,omitempty"` // "TOP", "JUNGLE", "MIDDLE", "BOTTOM", "SUPPORT"
	Leaver                      bool `json:"leaver,omitempty"` // Riot's official AFK/leaver flag
	
	// Stats might be nested in a stats object
	Stats                       *EoGPlayerStats `json:"stats,omitempty"`
}

type EoGPlayerStats struct {
	Kills                       int `json:"kills"`
	Deaths                      int `json:"deaths"`
	Assists                     int `json:"assists"`
	TotalMinionsKilled          int `json:"totalMinionsKilled"`
	NeutralMinionsKilled        int `json:"neutralMinionsKilled"`
	GoldEarned                  int `json:"goldEarned"`
	TotalDamageDealtToChampions int `json:"totalDamageDealtToChampions"`
	VisionScore                 int `json:"visionScore"`
	Win                         bool `json:"win"`
	Leaver                      bool `json:"leaver,omitempty"` // Riot's official AFK/leaver flag
	TotalDamageTaken            int `json:"totalDamageTaken,omitempty"`
	TimeCCingOthers             int `json:"timeCCingOthers,omitempty"`
	TotalHealsOnTeammates       int `json:"totalHealsOnTeammates,omitempty"`
	TotalDamageShieldedOnTeammates int `json:"totalDamageShieldedOnTeammates,omitempty"`
	TotalDamageSelfMitigated    int `json:"totalDamageSelfMitigated,omitempty"`
}

// EoGStatsBlockWrapper represents the top-level wrapper that the API returns
type EoGStatsBlockWrapper struct {
	GameLength   int              `json:"gameLength"`
	GameDuration int              `json:"gameDuration"`
	StatsBlock   *EoGStatsBlock  `json:"statsBlock,omitempty"`
	Teams        []EoGTeam        `json:"teams,omitempty"`
	Participants []EoGParticipant `json:"participants,omitempty"` // Sometimes at top level
	// Allow other fields to be ignored (basePoints, battleBoostIpEarned, etc.)
}

type EoGTeam struct {
	TeamID       int              `json:"teamId"`
	Players      []EoGParticipant `json:"players,omitempty"` // API uses "players" not "participants"
	Participants []EoGParticipant `json:"participants,omitempty"` // Fallback
}

type EoGStatsBlock struct {
	GameDurationSeconds int              `json:"gameDuration"`
	GameLength          int              `json:"gameLength"` // Alternative field name
	Participants        []EoGParticipant `json:"participants"`
	
	// Game mode information
	GameMode            string           `json:"gameMode,omitempty"` // e.g., "ARAM", "CLASSIC", "URF"
	QueueType           string           `json:"queueType,omitempty"` // e.g., "ARAM", "RANKED_SOLO_5x5", "NORMAL_DRAFT_PICK_5X5"
	GameType            string           `json:"gameType,omitempty"` // e.g., "MATCHED_GAME", "CUSTOM_GAME"
	
	// Team objective stats (may not be in API, will be calculated if missing)
	TeamDragons map[int]int `json:"teamDragons,omitempty"` // teamID -> count
	TeamBarons  map[int]int `json:"teamBarons,omitempty"`  // teamID -> count
}

func ParseEoGStats(data []byte) (*EoGStatsBlock, error) {
	// Parse as generic JSON to inspect structure
	var rawData map[string]interface{}
	if err := json.Unmarshal(data, &rawData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw data: %w", err)
	}
	
	// Extract game duration
	gameDuration := 0
	if gameLength, ok := rawData["gameLength"].(float64); ok {
		gameDuration = int(gameLength)
	} else if gameDurationRaw, ok := rawData["gameDuration"].(float64); ok {
		gameDuration = int(gameDurationRaw)
	}
	
	// Extract game mode information
	gameMode := ""
	if gm, ok := rawData["gameMode"].(string); ok {
		gameMode = gm
	}
	queueType := ""
	if qt, ok := rawData["queueType"].(string); ok {
		queueType = qt
	}
	gameType := ""
	if gt, ok := rawData["gameType"].(string); ok {
		gameType = gt
	}
	
	// Try to extract participants from teams array
	var allParticipants []EoGParticipant
	
	if teamsRaw, ok := rawData["teams"].([]interface{}); ok {
		for _, teamRaw := range teamsRaw {
			if teamMap, ok := teamRaw.(map[string]interface{}); ok {
				if playersRaw, ok := teamMap["players"].([]interface{}); ok {
					for _, playerRaw := range playersRaw {
						// Marshal player back to JSON for proper unmarshaling
						playerJSON, err := json.Marshal(playerRaw)
						if err != nil {
							continue
						}
						
						var participant EoGParticipant
						playerMap, isMap := playerRaw.(map[string]interface{})
						
						if err := json.Unmarshal(playerJSON, &participant); err != nil {
							// If unmarshaling fails, try to extract stats manually
							if isMap {
								participant = extractParticipantFromMap(playerMap)
							}
						} else {
							// Normalize riotIdTagLine -> riotIdTagline
							if participant.RiotIdTagLine != "" && participant.RiotIdTagline == "" {
								participant.RiotIdTagline = participant.RiotIdTagLine
							}
							
							// ALWAYS extract from nested stats object if present (API uses uppercase keys)
							// This ensures we get stats even if JSON unmarshaling didn't populate them correctly
							if isMap && playerMap != nil {
								if statsMap, ok := playerMap["stats"].(map[string]interface{}); ok {
									// Extract from uppercase keys in nested stats
									if v, ok := statsMap["TOTAL_DAMAGE_DEALT_TO_CHAMPIONS"].(float64); ok {
										participant.TotalDamageDealtToChampions = int(v)
									}
									if v, ok := statsMap["CHAMPIONS_KILLED"].(float64); ok {
										participant.Kills = int(v)
									}
									if v, ok := statsMap["NUM_DEATHS"].(float64); ok {
										participant.Deaths = int(v)
									}
									if v, ok := statsMap["ASSISTS"].(float64); ok {
										participant.Assists = int(v)
									}
									if v, ok := statsMap["GOLD_EARNED"].(float64); ok {
										participant.GoldEarned = int(v)
									}
									if v, ok := statsMap["MINIONS_KILLED"].(float64); ok {
										participant.TotalMinionsKilled = int(v)
									}
									if v, ok := statsMap["NEUTRAL_MINIONS_KILLED"].(float64); ok {
										participant.NeutralMinionsKilled = int(v)
									}
									if v, ok := statsMap["WIN"].(bool); ok {
										participant.Win = v
									}
									if v, ok := statsMap["TOTAL_DAMAGE_TAKEN"].(float64); ok {
										participant.TotalDamageTaken = int(v)
									}
									if v, ok := statsMap["TIME_CCING_OTHERS"].(float64); ok {
										participant.TimeCCingOthers = int(v)
									}
									if v, ok := statsMap["TOTAL_HEAL_ON_TEAMMATES"].(float64); ok {
										participant.TotalHealsOnTeammates = int(v)
									}
									if v, ok := statsMap["TOTAL_DAMAGE_SHIELDED_ON_TEAMMATES"].(float64); ok {
										participant.TotalDamageShieldedOnTeammates = int(v)
									}
									if v, ok := statsMap["TOTAL_DAMAGE_SELF_MITIGATED"].(float64); ok {
										participant.TotalDamageSelfMitigated = int(v)
									}
									// Vision score - try multiple possible keys
									if v, ok := statsMap["VISION_SCORE"].(float64); ok {
										participant.VisionScore = int(v)
									} else if v, ok := statsMap["visionScore"].(float64); ok {
										participant.VisionScore = int(v)
									}
									if v, ok := statsMap["WAS_AFK"].(bool); ok {
										participant.Leaver = v
									}
									// Create/update Stats object with extracted values
									if participant.Stats == nil {
										participant.Stats = &EoGPlayerStats{}
									}
									participant.Stats.TotalDamageDealtToChampions = participant.TotalDamageDealtToChampions
									participant.Stats.Kills = participant.Kills
									participant.Stats.Deaths = participant.Deaths
									participant.Stats.Assists = participant.Assists
									participant.Stats.GoldEarned = participant.GoldEarned
									participant.Stats.TotalMinionsKilled = participant.TotalMinionsKilled
									participant.Stats.NeutralMinionsKilled = participant.NeutralMinionsKilled
									participant.Stats.Win = participant.Win
									participant.Stats.TotalDamageTaken = participant.TotalDamageTaken
									participant.Stats.TimeCCingOthers = participant.TimeCCingOthers
									participant.Stats.TotalHealsOnTeammates = participant.TotalHealsOnTeammates
									participant.Stats.TotalDamageShieldedOnTeammates = participant.TotalDamageShieldedOnTeammates
									participant.Stats.Leaver = participant.Leaver
								}
							}
							
							// If Stats object exists but top-level fields are empty, copy from Stats (fallback)
							if participant.Stats != nil {
								if participant.TotalDamageDealtToChampions == 0 && participant.Stats.TotalDamageDealtToChampions > 0 {
									participant.TotalDamageDealtToChampions = participant.Stats.TotalDamageDealtToChampions
								}
								if participant.Kills == 0 && participant.Stats.Kills > 0 {
									participant.Kills = participant.Stats.Kills
								}
								if participant.Deaths == 0 && participant.Stats.Deaths > 0 {
									participant.Deaths = participant.Stats.Deaths
								}
								if participant.Assists == 0 && participant.Stats.Assists > 0 {
									participant.Assists = participant.Stats.Assists
								}
								if participant.GoldEarned == 0 && participant.Stats.GoldEarned > 0 {
									participant.GoldEarned = participant.Stats.GoldEarned
								}
								if participant.VisionScore == 0 && participant.Stats.VisionScore > 0 {
									participant.VisionScore = participant.Stats.VisionScore
								}
								if participant.TotalMinionsKilled == 0 && participant.Stats.TotalMinionsKilled > 0 {
									participant.TotalMinionsKilled = participant.Stats.TotalMinionsKilled
								}
								if participant.NeutralMinionsKilled == 0 && participant.Stats.NeutralMinionsKilled > 0 {
									participant.NeutralMinionsKilled = participant.Stats.NeutralMinionsKilled
								}
								if !participant.Win {
									participant.Win = participant.Stats.Win
								}
								if participant.TotalDamageTaken == 0 && participant.Stats.TotalDamageTaken > 0 {
									participant.TotalDamageTaken = participant.Stats.TotalDamageTaken
								}
								if participant.TimeCCingOthers == 0 && participant.Stats.TimeCCingOthers > 0 {
									participant.TimeCCingOthers = participant.Stats.TimeCCingOthers
								}
								if participant.TotalHealsOnTeammates == 0 && participant.Stats.TotalHealsOnTeammates > 0 {
									participant.TotalHealsOnTeammates = participant.Stats.TotalHealsOnTeammates
								}
								if participant.TotalDamageShieldedOnTeammates == 0 && participant.Stats.TotalDamageShieldedOnTeammates > 0 {
									participant.TotalDamageShieldedOnTeammates = participant.Stats.TotalDamageShieldedOnTeammates
								}
								if !participant.Leaver {
									participant.Leaver = participant.Stats.Leaver
								}
							}
							
							// If stats are nested in playerMap but not extracted, extract them
							if isMap {
								// Debug: log first player's stats structure
								if len(allParticipants) == 0 {
									if statsRaw, hasStats := playerMap["stats"]; hasStats {
										fmt.Printf("[DEBUG] Stats found in playerMap, type: %T, value: %v\n", statsRaw, statsRaw)
									} else {
										fmt.Printf("[DEBUG] No 'stats' key in playerMap. Top-level keys: ")
										for k := range playerMap {
											fmt.Printf("%s ", k)
										}
										fmt.Printf("\n")
									}
								}
								
								if statsMap, ok := playerMap["stats"].(map[string]interface{}); ok {
									// Stats are nested - extract them to top-level fields
									// Handle both camelCase and UPPER_CASE key formats
									if v, ok := statsMap["totalDamageDealtToChampions"].(float64); ok && participant.TotalDamageDealtToChampions == 0 {
										participant.TotalDamageDealtToChampions = int(v)
									} else if v, ok := statsMap["TOTAL_DAMAGE_DEALT_TO_CHAMPIONS"].(float64); ok && participant.TotalDamageDealtToChampions == 0 {
										participant.TotalDamageDealtToChampions = int(v)
									}
									// Helper to get value with fallback to uppercase key
									getStatValue := func(camelKey, upperKey string) (float64, bool) {
										if v, ok := statsMap[camelKey].(float64); ok {
											return v, true
										}
										if v, ok := statsMap[upperKey].(float64); ok {
											return v, true
										}
										return 0, false
									}
									
									if v, ok := getStatValue("kills", "CHAMPIONS_KILLED"); ok && participant.Kills == 0 {
										participant.Kills = int(v)
									}
									if v, ok := getStatValue("deaths", "NUM_DEATHS"); ok && participant.Deaths == 0 {
										participant.Deaths = int(v)
									}
									if v, ok := getStatValue("assists", "ASSISTS"); ok && participant.Assists == 0 {
										participant.Assists = int(v)
									}
									if v, ok := getStatValue("goldEarned", "GOLD_EARNED"); ok && participant.GoldEarned == 0 {
										participant.GoldEarned = int(v)
									}
									// Vision score might be calculated or in a different field
									// For now, try common variations
									if v, ok := statsMap["visionScore"].(float64); ok && participant.VisionScore == 0 {
										participant.VisionScore = int(v)
									}
									if v, ok := getStatValue("totalMinionsKilled", "MINIONS_KILLED"); ok && participant.TotalMinionsKilled == 0 {
										participant.TotalMinionsKilled = int(v)
									}
									if v, ok := getStatValue("neutralMinionsKilled", "NEUTRAL_MINIONS_KILLED"); ok && participant.NeutralMinionsKilled == 0 {
										participant.NeutralMinionsKilled = int(v)
									}
									if v, ok := statsMap["win"].(bool); ok {
										participant.Win = v
									} else if v, ok := statsMap["WIN"].(bool); ok {
										participant.Win = v
									}
									// Create Stats object from nested stats
									participant.Stats = &EoGPlayerStats{
										Kills:                       participant.Kills,
										Deaths:                      participant.Deaths,
										Assists:                     participant.Assists,
										TotalMinionsKilled:          participant.TotalMinionsKilled,
										NeutralMinionsKilled:        participant.NeutralMinionsKilled,
										GoldEarned:                  participant.GoldEarned,
										TotalDamageDealtToChampions: participant.TotalDamageDealtToChampions,
										VisionScore:                 participant.VisionScore,
										Win:                         participant.Win,
									}
									// Extract additional stats if present (handle both formats)
									if v, ok := getStatValue("totalDamageTaken", "TOTAL_DAMAGE_TAKEN"); ok {
										participant.TotalDamageTaken = int(v)
										if participant.Stats != nil {
											participant.Stats.TotalDamageTaken = int(v)
										}
									}
									if v, ok := getStatValue("timeCCingOthers", "TIME_CCING_OTHERS"); ok {
										participant.TimeCCingOthers = int(v)
										if participant.Stats != nil {
											participant.Stats.TimeCCingOthers = int(v)
										}
									}
									if v, ok := getStatValue("totalHealsOnTeammates", "TOTAL_HEAL_ON_TEAMMATES"); ok {
										participant.TotalHealsOnTeammates = int(v)
										if participant.Stats != nil {
											participant.Stats.TotalHealsOnTeammates = int(v)
										}
									}
									if v, ok := getStatValue("totalDamageShieldedOnTeammates", "TOTAL_DAMAGE_SHIELDED_ON_TEAMMATES"); ok {
										participant.TotalDamageShieldedOnTeammates = int(v)
										if participant.Stats != nil {
											participant.Stats.TotalDamageShieldedOnTeammates = int(v)
										}
									}
									if v, ok := getStatValue("totalDamageSelfMitigated", "TOTAL_DAMAGE_SELF_MITIGATED"); ok {
										participant.TotalDamageSelfMitigated = int(v)
										if participant.Stats != nil {
											participant.Stats.TotalDamageSelfMitigated = int(v)
										}
									}
									if v, ok := statsMap["leaver"].(bool); ok {
										participant.Leaver = v
										if participant.Stats != nil {
											participant.Stats.Leaver = v
										}
									} else if v, ok := statsMap["WAS_AFK"].(bool); ok {
										participant.Leaver = v
										if participant.Stats != nil {
											participant.Stats.Leaver = v
										}
									}
								}
							}
						}
						
						// Only add if we have at least a name
						if participant.SummonerName != "" || participant.RiotIdGameName != "" || participant.DisplayName != "" {
							allParticipants = append(allParticipants, participant)
						}
					}
				}
			}
		}
	}
	
	// If we found participants, return them
	if len(allParticipants) > 0 {
		return &EoGStatsBlock{
			Participants:        allParticipants,
			GameDurationSeconds: gameDuration,
			GameMode:            gameMode,
			QueueType:           queueType,
			GameType:            gameType,
		}, nil
	}
	
	// Fallback: try parsing as wrapper structure
	var wrapper EoGStatsBlockWrapper
	if err := json.Unmarshal(data, &wrapper); err == nil {
		// Check if we have participants at top level
		if len(wrapper.Participants) > 0 {
			result := &EoGStatsBlock{
				Participants: wrapper.Participants,
			}
			if wrapper.GameLength > 0 {
				result.GameDurationSeconds = wrapper.GameLength
			} else if wrapper.GameDuration > 0 {
				result.GameDurationSeconds = wrapper.GameDuration
			}
			return result, nil
		}
		
		// Check if we have a nested statsBlock
		if wrapper.StatsBlock != nil && len(wrapper.StatsBlock.Participants) > 0 {
			result := wrapper.StatsBlock
			if result.GameDurationSeconds == 0 {
				if wrapper.GameLength > 0 {
					result.GameDurationSeconds = wrapper.GameLength
				} else if wrapper.GameDuration > 0 {
					result.GameDurationSeconds = wrapper.GameDuration
				}
			}
			return result, nil
		}
	}
	
	// Final fallback: try parsing as direct EoGStatsBlock
	var stats EoGStatsBlock
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse EoG stats: %w", err)
	}
	
	if stats.GameDurationSeconds == 0 && stats.GameLength > 0 {
		stats.GameDurationSeconds = stats.GameLength
	}
	
	return &stats, nil
}

// extractParticipantFromMap manually extracts participant data from a map
func extractParticipantFromMap(playerMap map[string]interface{}) EoGParticipant {
	var p EoGParticipant
	
	// Extract name fields
	if v, ok := playerMap["summonerName"].(string); ok {
		p.SummonerName = v
	}
	if v, ok := playerMap["riotIdGameName"].(string); ok {
		p.RiotIdGameName = v
	}
	if v, ok := playerMap["riotIdTagLine"].(string); ok {
		p.RiotIdTagLine = v
		p.RiotIdTagline = v // Also set lowercase version
	}
	if v, ok := playerMap["riotIdTagline"].(string); ok {
		p.RiotIdTagline = v
	}
	if v, ok := playerMap["displayName"].(string); ok {
		p.DisplayName = v
	}
	
	// Extract team ID
	if v, ok := playerMap["teamId"].(float64); ok {
		p.TeamID = int(v)
	}
	
	// Extract champion fields
	if v, ok := playerMap["championName"].(string); ok {
		p.ChampionName = v
	}
	if v, ok := playerMap["championId"].(float64); ok {
		p.ChampionID = int(v)
	}
	
	// Extract leaver flag from top-level (Riot's official AFK indicator)
	if v, ok := playerMap["leaver"].(bool); ok {
		p.Leaver = v
	}
	
	// Extract stats (may be nested)
	if statsMap, ok := playerMap["stats"].(map[string]interface{}); ok {
		// Extract from nested stats object - check BOTH lowercase and uppercase keys
		// API uses uppercase keys like TOTAL_DAMAGE_DEALT_TO_CHAMPIONS
		
		// Kills
		if v, ok := statsMap["kills"].(float64); ok {
			p.Kills = int(v)
		} else if v, ok := statsMap["CHAMPIONS_KILLED"].(float64); ok {
			p.Kills = int(v)
		}
		
		// Deaths
		if v, ok := statsMap["deaths"].(float64); ok {
			p.Deaths = int(v)
		} else if v, ok := statsMap["NUM_DEATHS"].(float64); ok {
			p.Deaths = int(v)
		}
		
		// Assists
		if v, ok := statsMap["assists"].(float64); ok {
			p.Assists = int(v)
		} else if v, ok := statsMap["ASSISTS"].(float64); ok {
			p.Assists = int(v)
		}
		
		// Minions
		if v, ok := statsMap["totalMinionsKilled"].(float64); ok {
			p.TotalMinionsKilled = int(v)
		} else if v, ok := statsMap["MINIONS_KILLED"].(float64); ok {
			p.TotalMinionsKilled = int(v)
		}
		
		// Neutral minions
		if v, ok := statsMap["neutralMinionsKilled"].(float64); ok {
			p.NeutralMinionsKilled = int(v)
		} else if v, ok := statsMap["NEUTRAL_MINIONS_KILLED"].(float64); ok {
			p.NeutralMinionsKilled = int(v)
		}
		
		// Gold
		if v, ok := statsMap["goldEarned"].(float64); ok {
			p.GoldEarned = int(v)
		} else if v, ok := statsMap["GOLD_EARNED"].(float64); ok {
			p.GoldEarned = int(v)
		}
		
		// Damage to champions
		if v, ok := statsMap["totalDamageDealtToChampions"].(float64); ok {
			p.TotalDamageDealtToChampions = int(v)
		} else if v, ok := statsMap["TOTAL_DAMAGE_DEALT_TO_CHAMPIONS"].(float64); ok {
			p.TotalDamageDealtToChampions = int(v)
		}
		
		// Vision score - try multiple possible keys
		if v, ok := statsMap["VISION_SCORE"].(float64); ok {
			p.VisionScore = int(v)
		} else if v, ok := statsMap["visionScore"].(float64); ok {
			p.VisionScore = int(v)
		}
		
		// Win
		if v, ok := statsMap["win"].(bool); ok {
			p.Win = v
		} else if v, ok := statsMap["WIN"].(bool); ok {
			p.Win = v
		}
		
		// Damage taken
		if v, ok := statsMap["totalDamageTaken"].(float64); ok {
			p.TotalDamageTaken = int(v)
		} else if v, ok := statsMap["TOTAL_DAMAGE_TAKEN"].(float64); ok {
			p.TotalDamageTaken = int(v)
		}
		
		// CC
		if v, ok := statsMap["timeCCingOthers"].(float64); ok {
			p.TimeCCingOthers = int(v)
		} else if v, ok := statsMap["TIME_CCING_OTHERS"].(float64); ok {
			p.TimeCCingOthers = int(v)
		}
		
		// Healing
		if v, ok := statsMap["totalHealsOnTeammates"].(float64); ok {
			p.TotalHealsOnTeammates = int(v)
		} else if v, ok := statsMap["TOTAL_HEAL_ON_TEAMMATES"].(float64); ok {
			p.TotalHealsOnTeammates = int(v)
		}
		
		// Shielding
		if v, ok := statsMap["totalDamageShieldedOnTeammates"].(float64); ok {
			p.TotalDamageShieldedOnTeammates = int(v)
		} else if v, ok := statsMap["TOTAL_DAMAGE_SHIELDED_ON_TEAMMATES"].(float64); ok {
			p.TotalDamageShieldedOnTeammates = int(v)
		}
		
		// Damage mitigated
		if v, ok := statsMap["totalDamageSelfMitigated"].(float64); ok {
			p.TotalDamageSelfMitigated = int(v)
		} else if v, ok := statsMap["TOTAL_DAMAGE_SELF_MITIGATED"].(float64); ok {
			p.TotalDamageSelfMitigated = int(v)
		}
		
		// Leaver/AFK
		if v, ok := statsMap["leaver"].(bool); ok {
			p.Leaver = v
		} else if v, ok := statsMap["WAS_AFK"].(bool); ok {
			p.Leaver = v
		}
		
		// Create nested stats object for analyzer
		p.Stats = &EoGPlayerStats{
			Kills:                       p.Kills,
			Deaths:                      p.Deaths,
			Assists:                     p.Assists,
			TotalMinionsKilled:          p.TotalMinionsKilled,
			NeutralMinionsKilled:        p.NeutralMinionsKilled,
			GoldEarned:                  p.GoldEarned,
			TotalDamageDealtToChampions: p.TotalDamageDealtToChampions,
			VisionScore:                 p.VisionScore,
			Win:                         p.Win,
			Leaver:                      p.Leaver,
			TotalDamageTaken:            p.TotalDamageTaken,
			TimeCCingOthers:             p.TimeCCingOthers,
			TotalHealsOnTeammates:        p.TotalHealsOnTeammates,
			TotalDamageShieldedOnTeammates: p.TotalDamageShieldedOnTeammates,
			TotalDamageSelfMitigated:     p.TotalDamageSelfMitigated,
		}
	} else {
		// Extract from top-level (fallback)
		if v, ok := playerMap["kills"].(float64); ok {
			p.Kills = int(v)
		}
		if v, ok := playerMap["deaths"].(float64); ok {
			p.Deaths = int(v)
		}
		if v, ok := playerMap["assists"].(float64); ok {
			p.Assists = int(v)
		}
		if v, ok := playerMap["totalMinionsKilled"].(float64); ok {
			p.TotalMinionsKilled = int(v)
		}
		if v, ok := playerMap["neutralMinionsKilled"].(float64); ok {
			p.NeutralMinionsKilled = int(v)
		}
		if v, ok := playerMap["goldEarned"].(float64); ok {
			p.GoldEarned = int(v)
		}
		if v, ok := playerMap["totalDamageDealtToChampions"].(float64); ok {
			p.TotalDamageDealtToChampions = int(v)
		}
		if v, ok := playerMap["visionScore"].(float64); ok {
			p.VisionScore = int(v)
		}
		if v, ok := playerMap["win"].(bool); ok {
			p.Win = v
		}
		if v, ok := playerMap["totalDamageTaken"].(float64); ok {
			p.TotalDamageTaken = int(v)
		}
		if v, ok := playerMap["timeCCingOthers"].(float64); ok {
			p.TimeCCingOthers = int(v)
		}
		if v, ok := playerMap["totalHealsOnTeammates"].(float64); ok {
			p.TotalHealsOnTeammates = int(v)
		}
		if v, ok := playerMap["totalDamageShieldedOnTeammates"].(float64); ok {
			p.TotalDamageShieldedOnTeammates = int(v)
		}
		if v, ok := playerMap["role"].(string); ok {
			p.Role = v
		}
		// Extract leaver flag from top-level (it's usually at top level, not in stats)
		if v, ok := playerMap["leaver"].(bool); ok {
			p.Leaver = v
		}
	}
	
	return p
}

func TeamIDToSide(teamID int) string {
	switch teamID {
	case TeamIDBlue:
		return "BLUE"
	case TeamIDRed:
		return "RED"
	default:
		return "UNKNOWN"
	}
}

