package monitor

import (
	"lol-kind-bot/lcu"
	"log"
	"sync"
	"time"
)

// ClutchEvent represents a clutch moment (heal, shield, CC save)
type ClutchEvent struct {
	EventType    string    `json:"eventType"`    // "heal", "shield", "cc_save"
	Timestamp    time.Time `json:"timestamp"`
	GameTime     float64   `json:"gameTime"`     // Game time in seconds
	FromChampion string    `json:"fromChampion"` // Who provided the save
	ToChampion   string    `json:"toChampion"`   // Who was saved
	Amount       float64   `json:"amount"`       // Amount healed/shielded
	HealthBefore float64   `json:"healthBefore"` // Health before event
	HealthAfter  float64   `json:"healthAfter"`  // Health after event
	WasCritical  bool      `json:"wasCritical"`  // Was this a critical save (< 20% HP)
	Context      string    `json:"context"`      // Additional context
}

// ClutchStats tracks clutch moments for a player
type ClutchStats struct {
	Champion    string        `json:"champion"`
	LivesSaved  int           `json:"livesSaved"`  // Times they saved others
	TimesSaved  int           `json:"timesSaved"` // Times they were saved
	HealsGiven  int           `json:"healsGiven"`
	ShieldsGiven int          `json:"shieldsGiven"`
	CCSaves     int           `json:"ccSaves"`
	Events      []ClutchEvent `json:"events"`
	mu          sync.RWMutex
}

// ClutchMonitor monitors live game events for clutch moments
type ClutchMonitor struct {
	client      *lcu.Client
	stats       map[string]*ClutchStats // Champion -> stats
	lastHealth  map[string]float64      // Champion -> last known health
	lastUpdate  time.Time
	pollInterval time.Duration
	stopChan    chan struct{}
	running     bool
	mu          sync.RWMutex
}

// NewClutchMonitor creates a new clutch event monitor
func NewClutchMonitor(client *lcu.Client, pollInterval time.Duration) *ClutchMonitor {
	return &ClutchMonitor{
		client:       client,
		stats:         make(map[string]*ClutchStats),
		lastHealth:    make(map[string]float64),
		pollInterval:  pollInterval,
		stopChan:      make(chan struct{}),
		running:       false,
	}
}

// Start begins monitoring clutch events
func (cm *ClutchMonitor) Start() {
	cm.mu.Lock()
	if cm.running {
		cm.mu.Unlock()
		return
	}
	cm.running = true
	cm.mu.Unlock()

	log.Println("[CLUTCH] Starting clutch event monitor")
	go cm.monitorLoop()
}

// Stop stops monitoring
func (cm *ClutchMonitor) Stop() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if !cm.running {
		return
	}
	cm.running = false
	close(cm.stopChan)
	log.Println("[CLUTCH] Stopped clutch event monitor")
}

// IsRunning returns if monitor is running
func (cm *ClutchMonitor) IsRunning() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.running
}

// GetStats returns all clutch stats
func (cm *ClutchMonitor) GetStats() map[string]*ClutchStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make(map[string]*ClutchStats)
	for champ, stats := range cm.stats {
		stats.mu.RLock()
		result[champ] = &ClutchStats{
			Champion:    stats.Champion,
			LivesSaved:  stats.LivesSaved,
			TimesSaved:  stats.TimesSaved,
			HealsGiven:  stats.HealsGiven,
			ShieldsGiven: stats.ShieldsGiven,
			CCSaves:     stats.CCSaves,
			Events:      append([]ClutchEvent{}, stats.Events...),
		}
		stats.mu.RUnlock()
	}
	return result
}

// Reset clears all stats
func (cm *ClutchMonitor) Reset() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.stats = make(map[string]*ClutchStats)
	cm.lastHealth = make(map[string]float64)
	cm.lastUpdate = time.Time{}
	log.Println("[CLUTCH] Reset clutch stats")
}

// monitorLoop polls for live game data and detects clutch events
func (cm *ClutchMonitor) monitorLoop() {
	ticker := time.NewTicker(cm.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cm.stopChan:
			return
		case <-ticker.C:
			cm.checkForClutchEvents()
		}
	}
}

// checkForClutchEvents checks live game data for clutch moments
func (cm *ClutchMonitor) checkForClutchEvents() {
	// Get all game data
	allData, err := cm.client.GetAllGameData()
	if err != nil {
		// Game not in progress or error
		return
	}

	gameTime := allData.GameData.GameTime
	
	// Track health changes for all players
	for _, player := range allData.AllPlayers {
		if player.ChampionName == "" {
			continue
		}

		currentHealth := player.CurrentHealth
		maxHealth := player.MaxHealth
		if maxHealth <= 0 {
			continue
		}
		
		healthPercent := (currentHealth / maxHealth) * 100.0

		cm.mu.Lock()
		lastHealth, exists := cm.lastHealth[player.ChampionName]
		
		// Initialize stats if needed
		if _, hasStats := cm.stats[player.ChampionName]; !hasStats {
			cm.stats[player.ChampionName] = &ClutchStats{
				Champion: player.ChampionName,
				Events:   []ClutchEvent{},
			}
		}

		// Detect health recovery (potential heal/shield)
		if exists && lastHealth > 0 && lastHealth < currentHealth {
			healthDiff := currentHealth - lastHealth
			
			// Significant health gain (> 50 HP) when low on health (< 30%)
			if healthDiff > 50 && healthPercent < 30 {
				wasCritical := (lastHealth / maxHealth) * 100.0 < 20
				
				event := ClutchEvent{
					EventType:    "potential_heal",
					Timestamp:    time.Now(),
					GameTime:     gameTime,
					ToChampion:   player.ChampionName,
					Amount:       healthDiff,
					HealthBefore: lastHealth,
					HealthAfter:  currentHealth,
					WasCritical:  wasCritical,
					Context:      "Health recovery detected",
				}

				cm.stats[player.ChampionName].mu.Lock()
				cm.stats[player.ChampionName].TimesSaved++
				cm.stats[player.ChampionName].Events = append(cm.stats[player.ChampionName].Events, event)
				cm.stats[player.ChampionName].mu.Unlock()

				if wasCritical {
					log.Printf("[CLUTCH] Critical save detected: %s recovered from %.0f to %.0f HP (%.1f%% -> %.1f%%)", 
						player.ChampionName, lastHealth, currentHealth, 
						(lastHealth/maxHealth)*100, healthPercent)
				}
			}
		}

		cm.lastHealth[player.ChampionName] = currentHealth
		cm.mu.Unlock()
	}

	cm.lastUpdate = time.Now()
}
