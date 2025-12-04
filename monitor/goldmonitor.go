package monitor

import (
	"lol-kind-bot/config"
	"lol-kind-bot/lcu"
	"log"
	"sync"
	"time"
)

type GoldMonitor struct {
	client          *lcu.Client
	cfg             *config.GoldAnnouncementSettings
	announcedGold   map[int]bool // Track which thresholds we've already announced
	mu              sync.RWMutex
	stopChan        chan struct{}
	running         bool
	onGoldMilestone func(gold int) // Callback for gold announcements
}

func NewGoldMonitor(client *lcu.Client, cfg *config.GoldAnnouncementSettings, onGoldMilestone func(gold int)) *GoldMonitor {
	return &GoldMonitor{
		client:          client,
		cfg:             cfg,
		announcedGold:   make(map[int]bool),
		stopChan:        make(chan struct{}),
		onGoldMilestone: onGoldMilestone,
	}
}

func (m *GoldMonitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	// Reset announced thresholds when starting
	m.announcedGold = make(map[int]bool)
	m.mu.Unlock()

	go m.monitorLoop()
}

func (m *GoldMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopChan)
}

func (m *GoldMonitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

func (m *GoldMonitor) monitorLoop() {
	if !m.cfg.Enabled {
		log.Println("Gold announcements disabled, stopping monitor")
		m.Stop()
		return
	}

	if len(m.cfg.Thresholds) == 0 {
		log.Println("Gold monitor: No thresholds configured, stopping")
		m.Stop()
		return
	}

	if m.onGoldMilestone == nil {
		log.Println("Gold monitor: WARNING - callback is nil! Announcements will not work.")
	}

	pollInterval := time.Duration(m.cfg.PollIntervalSec) * time.Second
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	log.Printf("Gold monitor started (thresholds: %v, poll interval: %v, callback nil: %v)", 
		m.cfg.Thresholds, pollInterval, m.onGoldMilestone == nil)

	// Do an immediate check when starting
	log.Println("Gold monitor: Performing initial gold check...")
	m.checkGold()

	for {
		select {
		case <-m.stopChan:
			log.Println("Gold monitor stopped")
			return
		case <-ticker.C:
			m.checkGold()
		}
	}
}

func (m *GoldMonitor) checkGold() {
	playerData, err := m.client.GetActivePlayerData()
	if err != nil {
		// Game might have ended or not be in progress
		// Log occasionally to help debug (every 10th failure)
		m.mu.Lock()
		checkCount := len(m.announcedGold) // Use announced count as a simple counter
		m.mu.Unlock()
		if checkCount%10 == 0 {
			log.Printf("Gold monitor: Failed to get active player data (game may have ended): %v", err)
		}
		return
	}

	currentGold := int(playerData.CurrentGold)
	
	// Log current gold periodically for debugging (every 30 seconds worth of checks)
	m.mu.RLock()
	thresholds := m.cfg.Thresholds
	announced := m.announcedGold
	checkCount := len(announced)
	m.mu.RUnlock()
	
	// Log current gold more frequently for debugging (every 5 checks = 10 seconds)
	if checkCount%5 == 0 && currentGold > 0 {
		log.Printf("Gold monitor: Current gold: %d (check #%d)", currentGold, checkCount)
	}

	// Check each threshold
	for _, threshold := range thresholds {
		// Only announce if we've reached or exceeded threshold and haven't announced it yet
		if currentGold >= threshold && !announced[threshold] {
			m.mu.Lock()
			m.announcedGold[threshold] = true
			m.mu.Unlock()

			log.Printf("Gold milestone reached: %d gold (threshold: %d)", currentGold, threshold)
			
			if m.onGoldMilestone != nil {
				log.Printf("Calling gold milestone callback for %d gold", threshold)
				m.onGoldMilestone(threshold)
			} else {
				log.Printf("WARNING: Gold milestone callback is nil!")
			}
		}
	}
}

// Reset resets the announced thresholds (call when starting a new game)
func (m *GoldMonitor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.announcedGold = make(map[int]bool)
	log.Println("Gold monitor thresholds reset")
}

