package lcu

import (
	"encoding/json"
	"fmt"
)

// ActivePlayerData represents the current player's live game data
type ActivePlayerData struct {
	CurrentGold float64 `json:"currentGold"`
	Level       int     `json:"level"`
	ChampionName string `json:"championName"`
}

// PlayerData represents a player's live game data
type PlayerData struct {
	ChampionName string  `json:"championName"`
	SummonerName string  `json:"summonerName"`
	Team         string  `json:"team"`
	CurrentHealth float64 `json:"currentHealth"`
	MaxHealth     float64 `json:"maxHealth"`
	Level         int     `json:"level"`
	Gold          float64 `json:"gold"`
}

// AllGameData represents all live game data
type AllGameData struct {
	ActivePlayer ActivePlayerData `json:"activePlayer"`
	AllPlayers   []PlayerData    `json:"allPlayers"`
	GameData     struct {
		GameTime float64 `json:"gameTime"`
	} `json:"gameData"`
}

// GetActivePlayerData retrieves the current player's live game data
func (c *Client) GetActivePlayerData() (*ActivePlayerData, error) {
	data, err := c.Get("/liveclientdata/activeplayer")
	if err != nil {
		return nil, fmt.Errorf("failed to get active player data: %w", err)
	}

	// Try parsing with multiple possible field names
	var playerData ActivePlayerData
	
	// First try standard field name
	if err := json.Unmarshal(data, &playerData); err != nil {
		// Try alternative field names that the API might use
		var altData map[string]interface{}
		if json.Unmarshal(data, &altData) == nil {
			// Try different possible gold field names
			if gold, ok := altData["currentGold"].(float64); ok {
				playerData.CurrentGold = gold
			} else if gold, ok := altData["gold"].(float64); ok {
				playerData.CurrentGold = gold
			} else if gold, ok := altData["currentGold"].(int); ok {
				playerData.CurrentGold = float64(gold)
			} else if gold, ok := altData["gold"].(int); ok {
				playerData.CurrentGold = float64(gold)
			} else {
				return nil, fmt.Errorf("failed to find gold field in active player data: %s", string(data))
			}
			
			// Get other fields if available
			if level, ok := altData["level"].(float64); ok {
				playerData.Level = int(level)
			}
			if champ, ok := altData["championName"].(string); ok {
				playerData.ChampionName = champ
			}
		} else {
			return nil, fmt.Errorf("failed to parse active player data: %w (raw: %s)", err, string(data))
		}
	}

	return &playerData, nil
}

// GetAllGameData retrieves all live game data
func (c *Client) GetAllGameData() (*AllGameData, error) {
	data, err := c.Get("/liveclientdata/allgamedata")
	if err != nil {
		return nil, fmt.Errorf("failed to get all game data: %w", err)
	}

	var gameData AllGameData
	if err := json.Unmarshal(data, &gameData); err != nil {
		return nil, fmt.Errorf("failed to parse all game data: %w", err)
	}

	return &gameData, nil
}

// IsGameInProgress checks if a game is currently in progress
func (c *Client) IsGameInProgress() (bool, error) {
	// Try to get live game data - if it exists, game is in progress
	_, err := c.GetAllGameData()
	if err != nil {
		// If endpoint returns error, game is not in progress
		return false, nil
	}
	return true, nil
}

