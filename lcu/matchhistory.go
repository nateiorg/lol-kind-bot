package lcu

import (
	"encoding/json"
	"time"
)

type MatchHistoryGame struct {
	GameID            int64  `json:"gameId"`
	GameCreation      int64  `json:"gameCreation"`
	GameDuration      int    `json:"gameDuration"`
	GameEndTimestamp  int64  `json:"gameEndTimestamp"`
	GameMode          string `json:"gameMode"`
	GameType          string `json:"gameType"`
	GameVersion       string `json:"gameVersion"`
	MapID             int    `json:"mapId"`
	PlatformID        string `json:"platformId"`
	QueueID           int    `json:"queueId"`
	SeasonID          int    `json:"seasonId"`
}

type MatchHistory struct {
	Games []MatchHistoryGame `json:"games"`
}

// GetRecentMatchHistory gets the most recent match from match history
// Returns nil, nil if no recent match is found (not an error)
func (c *Client) GetRecentMatchHistory() (*MatchHistoryGame, error) {
	// Try multiple possible match history endpoints
	endpoints := []string{
		"/lol-match-history/v1/matchlist",
		"/lol-match-history/v1/products/lol/current-summoner/matches",
	}

	for _, endpoint := range endpoints {
		data, err := c.Get(endpoint)
		if err != nil {
			continue // Try next endpoint
		}

		// Try parsing as MatchHistory struct
		var history MatchHistory
		if err := json.Unmarshal(data, &history); err == nil {
			if len(history.Games) > 0 {
				return &history.Games[0], nil
			}
		}

		// Try parsing as direct array of games
		var games []MatchHistoryGame
		if err := json.Unmarshal(data, &games); err == nil {
			if len(games) > 0 {
				return &games[0], nil
			}
		}

		// Try parsing as object with "games" field (different structure)
		var obj map[string]interface{}
		if err := json.Unmarshal(data, &obj); err == nil {
			if gamesData, ok := obj["games"].([]interface{}); ok && len(gamesData) > 0 {
				// Try to parse first game
				gameBytes, _ := json.Marshal(gamesData[0])
				var game MatchHistoryGame
				if err := json.Unmarshal(gameBytes, &game); err == nil {
					return &game, nil
				}
			}
		}
	}

	return nil, nil // No matches found, but not an error
}

// IsRecentMatch checks if a match ended recently (within the specified duration)
func IsRecentMatch(gameEndTimestamp int64, maxAge time.Duration) bool {
	if gameEndTimestamp == 0 {
		return false
	}
	
	gameEndTime := time.Unix(gameEndTimestamp/1000, 0)
	age := time.Since(gameEndTime)
	return age >= 0 && age <= maxAge
}

