package lcu

import (
	"encoding/json"
	"fmt"
)

type CurrentSummoner struct {
	DisplayName string `json:"displayName"`
	SummonerID  int64  `json:"summonerId"`
	Puuid       string `json:"puuid"`
}

func (c *Client) GetCurrentSummoner() (*CurrentSummoner, error) {
	data, err := c.Get("/lol-summoner/v1/current-summoner")
	if err != nil {
		return nil, fmt.Errorf("failed to get current summoner: %w", err)
	}

	var summoner CurrentSummoner
	if err := json.Unmarshal(data, &summoner); err != nil {
		return nil, fmt.Errorf("failed to parse summoner data: %w", err)
	}

	return &summoner, nil
}

