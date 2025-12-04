package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultLockfilePath     = `C:\Riot Games\League of Legends\lockfile`
	DefaultOllamaURL        = "http://localhost:11434/api/generate"
	DefaultOllamaModel      = "llama3.1"
	DefaultPollInterval     = 3
	DefaultEoGCooldown      = 30
	DefaultMinGameMinutes   = 10
	DefaultMaxCsPerMin      = 0.5
	DefaultMaxDamageToChamp = 1500
	DefaultMaxGoldEarned    = 4000
)

type AFKThresholds struct {
	MinGameMinutes   float64 `json:"minGameMinutes"`
	MaxCsPerMin      float64 `json:"maxCsPerMin"`
	MaxDamageToChamp int     `json:"maxDamageToChamp"`
	MaxGoldEarned    int     `json:"maxGoldEarned"`
}

// LLMSettings contains all customization options for LLM message generation
type LLMSettings struct {
	// Message Tone/Demeanor
	Tone string `json:"tone"` // "friendly", "professional", "enthusiastic", "humble", "supportive"
	
	// Message Count
	MinMessages int `json:"minMessages"` // Minimum number of messages to generate
	MaxMessages int `json:"maxMessages"` // Maximum number of messages to generate
	
	// Message Length
	MaxMessageLength int `json:"maxMessageLength"` // Maximum characters per message
	
	// Language Style
	LanguageStyle string `json:"languageStyle"` // "casual", "formal", "enthusiastic", "gamer"
	
	// Focus Areas - what to highlight in messages
	FocusAreas []string `json:"focusAreas"` // ["all", "positive", "kda", "teamplay", "vision"]
	
	// AFK Handling Style
	AFKHandling string `json:"afkHandling"` // "default", "empathetic", "neutral"
	
	// LLM API Parameters
	Temperature float64 `json:"temperature"` // 0.0 to 1.0, controls creativity
	MaxTokens   int     `json:"maxTokens"`   // Maximum tokens in response (0 = use default)
	
	// Custom Instructions - additional instructions appended to the prompt
	CustomInstructions string `json:"customInstructions"` // Optional custom prompt additions
}

type GoldAnnouncementSettings struct {
	Enabled           bool     `json:"enabled"`           // Enable/disable gold announcements
	Thresholds        []int    `json:"thresholds"`        // Gold thresholds to announce (e.g., [1500, 2000, 3000])
	PollIntervalSec   int      `json:"pollIntervalSec"`   // How often to check gold (seconds)
}

type Config struct {
	MySummonerName        string                  `json:"mySummonerName"`
	OllamaModel           string                  `json:"ollamaModel"`
	OllamaURL             string                  `json:"ollamaUrl"`
	PollIntervalSeconds   int                     `json:"pollIntervalSeconds"`
	EndOfGameCooldownSec  int                     `json:"endOfGameCooldownSeconds"`
	AutoCopyToClipboard   bool                    `json:"autoCopyToClipboard"`
	EnableDetailedLogging bool                    `json:"enableDetailedLogging"`
	EnableDebugLogging    bool                    `json:"enableDebugLogging"` // Detailed debug output for LLM and validation (can be overridden by -debug flag)
	AFKThresholds        AFKThresholds            `json:"afkThresholds"`
	LLMSettings           LLMSettings             `json:"llmSettings"`
	GoldAnnouncements     GoldAnnouncementSettings `json:"goldAnnouncements"`
}

func DefaultConfig() *Config {
	return &Config{
		MySummonerName:        "",
		OllamaModel:           DefaultOllamaModel,
		OllamaURL:             DefaultOllamaURL,
		PollIntervalSeconds:   DefaultPollInterval,
		EndOfGameCooldownSec:  DefaultEoGCooldown,
		AutoCopyToClipboard:   true,
		EnableDetailedLogging: false,
		EnableDebugLogging:    false,
		AFKThresholds: AFKThresholds{
			MinGameMinutes:   DefaultMinGameMinutes,
			MaxCsPerMin:      DefaultMaxCsPerMin,
			MaxDamageToChamp: DefaultMaxDamageToChamp,
			MaxGoldEarned:    DefaultMaxGoldEarned,
		},
		LLMSettings: LLMSettings{
			Tone:              "friendly",
			MinMessages:       2,
			MaxMessages:       3,
			MaxMessageLength:  150,
			LanguageStyle:     "casual",
			FocusAreas:        []string{"all"},
			AFKHandling:       "default",
			Temperature:       0.7,
			MaxTokens:         0, // 0 means use default
			CustomInstructions: "",
		},
		GoldAnnouncements: GoldAnnouncementSettings{
			Enabled:         true, // Default enabled as requested
			Thresholds:      []int{1500, 2000, 3000, 4000, 5000}, // Common item breakpoints
			PollIntervalSec: 2, // Check every 2 seconds during active game
		},
	}
}

func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			if err := SaveConfig(configPath, cfg); err != nil {
				return cfg, fmt.Errorf("failed to create default config: %w", err)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults for missing fields
	if cfg.OllamaModel == "" {
		cfg.OllamaModel = DefaultOllamaModel
	}
	if cfg.OllamaURL == "" {
		cfg.OllamaURL = DefaultOllamaURL
	}
	if cfg.PollIntervalSeconds == 0 {
		cfg.PollIntervalSeconds = DefaultPollInterval
	}
	if cfg.EndOfGameCooldownSec == 0 {
		cfg.EndOfGameCooldownSec = DefaultEoGCooldown
	}
	if cfg.AFKThresholds.MinGameMinutes == 0 {
		cfg.AFKThresholds = AFKThresholds{
			MinGameMinutes:   DefaultMinGameMinutes,
			MaxCsPerMin:      DefaultMaxCsPerMin,
			MaxDamageToChamp: DefaultMaxDamageToChamp,
			MaxGoldEarned:    DefaultMaxGoldEarned,
		}
	}
	// Apply defaults for LLM settings
	defaultLLM := DefaultConfig().LLMSettings
	if cfg.LLMSettings.Tone == "" {
		cfg.LLMSettings.Tone = defaultLLM.Tone
	}
	if cfg.LLMSettings.MinMessages == 0 {
		cfg.LLMSettings.MinMessages = defaultLLM.MinMessages
	}
	if cfg.LLMSettings.MaxMessages == 0 {
		cfg.LLMSettings.MaxMessages = defaultLLM.MaxMessages
	}
	if cfg.LLMSettings.MaxMessageLength == 0 {
		cfg.LLMSettings.MaxMessageLength = defaultLLM.MaxMessageLength
	}
	if cfg.LLMSettings.LanguageStyle == "" {
		cfg.LLMSettings.LanguageStyle = defaultLLM.LanguageStyle
	}
	if len(cfg.LLMSettings.FocusAreas) == 0 {
		cfg.LLMSettings.FocusAreas = defaultLLM.FocusAreas
	}
	if cfg.LLMSettings.AFKHandling == "" {
		cfg.LLMSettings.AFKHandling = defaultLLM.AFKHandling
	}
	if cfg.LLMSettings.Temperature == 0 {
		cfg.LLMSettings.Temperature = defaultLLM.Temperature
	}
	
	// Apply defaults for gold announcements
	if len(cfg.GoldAnnouncements.Thresholds) == 0 {
		cfg.GoldAnnouncements = DefaultConfig().GoldAnnouncements
	}
	if cfg.GoldAnnouncements.PollIntervalSec == 0 {
		cfg.GoldAnnouncements.PollIntervalSec = 2
	}

	return &cfg, nil
}

func SaveConfig(configPath string, cfg *Config) error {
	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func GetConfigPath() string {
	exePath, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	exeDir := filepath.Dir(exePath)
	return filepath.Join(exeDir, "config.json")
}

