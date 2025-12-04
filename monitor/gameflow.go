package monitor

import (
	"encoding/json"
	"lol-kind-bot/lcu"
	"log"
	"sync"
	"time"
)

type GameflowMonitor struct {
	client          *lcu.Client
	pollInterval    time.Duration
	cooldown        time.Duration
	lastEoGTime     time.Time
	currentPhase    string
	lastPhase       string
	onEndOfGame     func() error
	onPhaseChange   func(newPhase, oldPhase string) // Callback for phase changes
	mu              sync.RWMutex
	stopChan        chan struct{}
	running         bool
}

func NewGameflowMonitor(client *lcu.Client, pollInterval, cooldown time.Duration, onEndOfGame func() error) *GameflowMonitor {
	return &GameflowMonitor{
		client:        client,
		pollInterval:  pollInterval,
		cooldown:      cooldown,
		stopChan:      make(chan struct{}),
		onEndOfGame:   onEndOfGame,
		onPhaseChange: nil,
	}
}

// SetPhaseChangeCallback sets a callback for phase changes
func (m *GameflowMonitor) SetPhaseChangeCallback(callback func(newPhase, oldPhase string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onPhaseChange = callback
}

func (m *GameflowMonitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	go m.pollLoop()
}

func (m *GameflowMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopChan)
}

func (m *GameflowMonitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

func (m *GameflowMonitor) pollLoop() {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkPhase()
		}
	}
}

func (m *GameflowMonitor) checkPhase() {
	data, err := m.client.Get("/lol-gameflow/v1/gameflow-phase")
	if err != nil {
		log.Printf("Failed to get gameflow phase: %v", err)
		return
	}

	var phase string
	if err := json.Unmarshal(data, &phase); err != nil {
		log.Printf("Failed to parse gameflow phase: %v", err)
		return
	}

	m.mu.Lock()
	m.lastPhase = m.currentPhase
	m.currentPhase = phase
	currentPhase := m.currentPhase
	lastPhase := m.lastPhase
	lastEoGTimeValue := m.lastEoGTime
	m.mu.Unlock()

	if currentPhase != lastPhase {
		log.Printf("Gameflow phase: %s", currentPhase)
		
		// Notify phase change (for gold monitor management)
		if m.onPhaseChange != nil {
			m.onPhaseChange(currentPhase, lastPhase)
		}
	}

	if currentPhase == "EndOfGame" && lastPhase != "EndOfGame" {
		now := time.Now()
		if now.Sub(lastEoGTimeValue) >= m.cooldown {
			m.mu.Lock()
			// Double-check after acquiring lock (prevent race conditions)
			if now.Sub(m.lastEoGTime) >= m.cooldown {
				m.lastEoGTime = now
				m.mu.Unlock()

				log.Println("Detected EndOfGame; fetching stats...")
				if m.onEndOfGame != nil {
					// Run in goroutine to prevent blocking the monitor
					go func() {
						if err := m.onEndOfGame(); err != nil {
							log.Printf("Error processing EndOfGame: %v", err)
						}
					}()
				}
			} else {
				m.mu.Unlock()
				log.Printf("EndOfGame detected but cooldown active (last: %v ago)", now.Sub(m.lastEoGTime))
			}
		} else {
			log.Printf("EndOfGame detected but cooldown active (last: %v ago)", now.Sub(lastEoGTimeValue))
		}
	}
}

