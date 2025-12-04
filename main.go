package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"lol-kind-bot/analyzer"
	"lol-kind-bot/config"
	"lol-kind-bot/eog"
	"lol-kind-bot/lcu"
	"lol-kind-bot/llm"
	"lol-kind-bot/monitor"
	"lol-kind-bot/ui"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/atotto/clipboard"
	"github.com/getlantern/systray"
)

var (
	appConfig     *config.Config
	lcuClient     *lcu.Client
	llmClient     *llm.Client
	gameMonitor   *monitor.GameflowMonitor
	goldMonitor   *monitor.GoldMonitor
	clutchMonitor *monitor.ClutchMonitor
	listening     bool   = true
	lastGameID    string = "" // Track last processed game to prevent duplicates
	dialogMutex   sync.Mutex
	dialogShowing bool       = false
	lastEoGTime   time.Time          // Track last EoG processing time for rate limiting
	eogMutex      sync.Mutex         // Mutex for EoG processing synchronization
	currentPhase  string     = ""    // Track current gameflow phase
	debugMode     bool       = false // Debug mode from command-line flag
)

func main() {
	// Parse command-line flags first
	var showHelp bool
	flag.BoolVar(&debugMode, "debug", false, "Enable debug logging mode")
	flag.BoolVar(&debugMode, "d", false, "Enable debug logging mode (short)")
	flag.BoolVar(&showHelp, "help", false, "Show help message")
	flag.BoolVar(&showHelp, "h", false, "Show help message (short)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s              # Run normally (background)\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -debug       # Run with debug logging enabled\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -d           # Same as -debug\n", os.Args[0])
	}
	flag.Parse()

	if showHelp {
		flag.Usage()
		os.Exit(0)
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Show console if debug mode is enabled
	if !debugMode {
		// Hide console window - run in background
		ui.HideConsole()
	} else {
		log.Println("Debug mode enabled via command-line flag")
	}

	// Initialize COM for Windows UI components (must be done before any UI operations)
	if err := ui.InitializeCOM(); err != nil {
		log.Printf("Warning: Failed to initialize COM: %v (UI dialogs may not work)", err)
	}
	defer ui.UninitializeCOM()

	// Load configuration first (before any UI operations)
	cfgPath := config.GetConfigPath()
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	appConfig = cfg

	// Override config debug setting with command-line flag if provided
	if debugMode {
		appConfig.EnableDebugLogging = true
		log.Println("Debug logging enabled via command-line flag (overriding config)")
	}

	// Show first-run dialog if summoner name is not set
	if cfg.MySummonerName == "" {
		log.Println("Summoner name not configured. Showing first-run dialog...")

		summonerName, ok := ui.ShowFirstRunDialog()
		if !ok || summonerName == "" {
			log.Println("First-run setup cancelled. Exiting...")
			os.Exit(0)
		}

		// Update config with summoner name
		cfg.MySummonerName = summonerName
		if err := config.SaveConfig(cfgPath, cfg); err != nil {
			log.Printf("Failed to save config: %v", err)
		} else {
			log.Printf("Saved summoner name: %s", summonerName)
		}
	}

	log.Printf("Loaded config from: %s", cfgPath)
	log.Printf("My Summoner Name: %s", cfg.MySummonerName)
	log.Printf("LLM Model: %s", cfg.OllamaModel)
	log.Printf("LLM URL: %s", cfg.OllamaURL)

	// Initialize LLM client
	llmClient = llm.NewClient(cfg.OllamaURL, cfg.OllamaModel, &cfg.LLMSettings)

	// Initialize Fyne app (creates app instance, sets theme)
	// This must be called early, before any UI operations
	fyneApp := ui.StartFyneApp()

	// Setup signal handling - will quit Fyne app on signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down...")
		if gameMonitor != nil {
			gameMonitor.Stop()
		}
		// Quit Fyne app - this will cause app.Run() to return
		fyneApp.Quit()
		systray.Quit()
	}()

	// Start system tray in its own goroutine
	// This must run independently of Fyne to avoid blocking
	go func() {
		// systray.Run blocks, so this goroutine will handle all tray events
		systray.Run(onReady, onExit)
	}()
	
	// Give system tray a moment to initialize
	time.Sleep(100 * time.Millisecond)

	// Main loop: try to connect to LCU and monitor
	go func() {
		for {
			if !listening {
				time.Sleep(5 * time.Second)
				continue
			}

			lockfilePath := lcu.GetLockfilePath()
			log.Printf("Checking for lockfile at: %s", lockfilePath)

			lockfileInfo, err := lcu.ParseLockfile(lockfilePath)
			if err != nil {
				log.Printf("Lockfile not found or invalid: %v. Retrying in 3 seconds...", err)
				time.Sleep(3 * time.Second)
				continue
			}

			log.Printf("Found lockfile: port=%s, protocol=%s", lockfileInfo.Port, lockfileInfo.Protocol)

			client, err := lcu.NewClient(lockfileInfo)
			if err != nil {
				log.Printf("Failed to create LCU client: %v. Retrying...", err)
				time.Sleep(3 * time.Second)
				continue
			}

			lcuClient = client
			log.Printf("Connected to LCU at: %s", client.BaseURL)

			// Check if player was recently in a match or is in post-match screen
			checkRecentMatch(client)

			// Check if game is currently in progress on startup
			checkActiveGameOnStartup(client)

			// Create gold monitor (will be started/stopped based on game phase)
			goldMonitor = monitor.NewGoldMonitor(client, &cfg.GoldAnnouncements, func(gold int) {
				log.Printf("Gold milestone callback triggered: %d gold", gold)
				ui.AnnounceGold(gold)
			})
			log.Printf("Gold monitor created (enabled: %v, thresholds: %v)", cfg.GoldAnnouncements.Enabled, cfg.GoldAnnouncements.Thresholds)

			// Create clutch event monitor
			clutchMonitor = monitor.NewClutchMonitor(client, 2*time.Second) // Poll every 2 seconds
			log.Printf("Clutch monitor created")

			// Create and start gameflow monitor
			pollInterval := time.Duration(cfg.PollIntervalSeconds) * time.Second
			cooldown := time.Duration(cfg.EndOfGameCooldownSec) * time.Second
			gameMonitor = monitor.NewGameflowMonitor(client, pollInterval, cooldown, handleEndOfGame)

			// Set phase change callback to manage gold monitor
			gameMonitor.SetPhaseChangeCallback(func(newPhase, oldPhase string) {
				handlePhaseChange(newPhase, oldPhase)
			})

			if !gameMonitor.IsRunning() {
				gameMonitor.Start()
			}

			// Keep monitoring until connection is lost
			for {
				time.Sleep(5 * time.Second)
				// Test connection
				_, err := client.Get("/lol-gameflow/v1/gameflow-phase")
				if err != nil {
					log.Printf("LCU connection lost: %v. Reconnecting...", err)
					gameMonitor.Stop()
					lcuClient = nil
					break
				}
			}
		}
	}()

	// Start Fyne event loop - MUST be called directly from main goroutine
	// This blocks until app.Quit() is called (handled by signal handler above)
	// All other operations run in goroutines, so this is fine
	fyneApp.Run()
	
	// If we get here, app was quit - cleanup already done by signal handler
	log.Println("Application exited")
}

// checkActiveGameOnStartup checks if a game is currently in progress when app starts
func checkActiveGameOnStartup(client *lcu.Client) {
	// Check gameflow phase first
	if phaseData, err := client.Get("/lol-gameflow/v1/gameflow-phase"); err == nil {
		var phase string
		if err := json.Unmarshal(phaseData, &phase); err == nil {
			currentPhase = phase

			// Check if game is in progress
			if phase == "InProgress" || phase == "GameStart" {
				log.Printf("Detected active game on startup (phase: %s), starting gold monitor...", phase)

				// Small delay to ensure game is fully loaded
				time.Sleep(2 * time.Second)

				// Start gold monitoring
				if goldMonitor != nil && !goldMonitor.IsRunning() {
					goldMonitor.Reset() // Reset thresholds for new game
					goldMonitor.Start()
				}
				return
			}
		}
	}

	// Also try to check live game data directly
	if inProgress, err := client.IsGameInProgress(); err == nil && inProgress {
		log.Println("Detected active game via live game data, starting gold monitor...")
		if goldMonitor != nil && !goldMonitor.IsRunning() {
			goldMonitor.Reset()
			goldMonitor.Start()
		}
	}
}

// checkRecentMatch checks if the player is in post-match screen or had a recent match
func checkRecentMatch(client *lcu.Client) {
	// First, check current gameflow phase
	if phaseData, err := client.Get("/lol-gameflow/v1/gameflow-phase"); err == nil {
		var phase string
		if err := json.Unmarshal(phaseData, &phase); err == nil {
			currentPhase = phase

			if phase == "EndOfGame" {
				log.Println("Detected EndOfGame phase on startup, processing...")
				go func() {
					time.Sleep(2 * time.Second) // Small delay to ensure stats are ready
					if err := handleEndOfGame(); err != nil {
						log.Printf("Error processing EndOfGame on startup: %v", err)
					}
				}()
				return // Already handling EndOfGame, no need to check match history
			}
		}
	}

	// Check for recent match history (within last 5 minutes)
	// This catches cases where the game ended but we're no longer in EndOfGame phase
	if recentMatch, err := client.GetRecentMatchHistory(); err == nil && recentMatch != nil {
		maxAge := 5 * time.Minute
		if lcu.IsRecentMatch(recentMatch.GameEndTimestamp, maxAge) {
			gameEndTime := time.Unix(recentMatch.GameEndTimestamp/1000, 0)
			age := time.Since(gameEndTime)
			log.Printf("Detected recent match ended %v ago (within %v window), attempting to process...", age, maxAge)

			// Try to fetch EoG stats - they might still be available
			go func() {
				time.Sleep(1 * time.Second) // Small delay
				if err := handleEndOfGame(); err != nil {
					log.Printf("Could not process recent match (stats may no longer be available): %v", err)
				}
			}()
		} else if recentMatch != nil {
			gameEndTime := time.Unix(recentMatch.GameEndTimestamp/1000, 0)
			age := time.Since(gameEndTime)
			log.Printf("Most recent match ended %v ago (outside %v window)", age, maxAge)
		}
	} else if err != nil {
		log.Printf("Could not check match history: %v", err)
	}
}

func handleEndOfGame() error {
	// Prevent concurrent processing of the same game
	eogMutex.Lock()
	defer eogMutex.Unlock()

	// Rate limiting: prevent processing if we just processed recently (additional safety)
	if !lastEoGTime.IsZero() && time.Since(lastEoGTime) < 5*time.Second {
		log.Printf("Skipping EoG processing - too soon after last processing (%v ago)", time.Since(lastEoGTime))
		return nil
	}

	if lcuClient == nil {
		return fmt.Errorf("LCU client not available")
	}

	log.Println("Processing EndOfGame event...")
	lastEoGTime = time.Now()

	// Fetch EoG stats first
	data, err := lcuClient.Get("/lol-end-of-game/v1/eog-stats-block")
	if err != nil {
		return fmt.Errorf("failed to fetch EoG stats: %w", err)
	}

	// Extract game ID to prevent duplicate processing and verify it's the latest match
	var rawData map[string]interface{}
	var currentGameID string
	var currentGameIDInt int64

	if err := json.Unmarshal(data, &rawData); err == nil {
		if gameIDRaw, ok := rawData["gameId"]; ok {
			// Handle both string and numeric game IDs
			switch v := gameIDRaw.(type) {
			case string:
				currentGameID = v
			case float64:
				currentGameIDInt = int64(v)
				currentGameID = fmt.Sprintf("%d", currentGameIDInt)
			case int64:
				currentGameIDInt = v
				currentGameID = fmt.Sprintf("%d", v)
			default:
				currentGameID = fmt.Sprintf("%v", gameIDRaw)
			}
			log.Printf("EoG stats game ID: %s", currentGameID)

			// Check if this game was already processed
			dialogMutex.Lock()
			if currentGameID == lastGameID {
				dialogMutex.Unlock()
				log.Printf("Game %s already processed, skipping duplicate event", currentGameID)
				return nil
			}
			dialogMutex.Unlock()

			// Verify this is the latest match by checking match history
			// This ensures we don't process old matches
			recentMatch, err := lcuClient.GetRecentMatchHistory()
			if err == nil && recentMatch != nil && recentMatch.GameID != 0 {
				latestMatchID := fmt.Sprintf("%d", recentMatch.GameID)
				gameEndTime := time.Unix(recentMatch.GameEndTimestamp/1000, 0)
				age := time.Since(gameEndTime)
				log.Printf("Latest match from history: GameID=%s, ended %v ago", latestMatchID, age)

				// Compare game IDs (handle both string and int64 comparisons)
				if currentGameID != latestMatchID {
					// Also try numeric comparison if both are numeric
					if currentGameIDInt != 0 && currentGameIDInt != recentMatch.GameID {
						log.Printf("WARNING: Game ID mismatch! EoG stats ID=%s (%d), Latest match ID=%s (%d). Skipping to avoid processing old match.",
							currentGameID, currentGameIDInt, latestMatchID, recentMatch.GameID)
						return fmt.Errorf("game ID mismatch - not processing old match (EoG: %s, Latest: %s)",
							currentGameID, latestMatchID)
					} else if currentGameIDInt == 0 {
						// If we couldn't parse as int64, still compare strings
						log.Printf("WARNING: Game ID mismatch! EoG stats ID=%s, Latest match ID=%s. Skipping to avoid processing old match.",
							currentGameID, latestMatchID)
						return fmt.Errorf("game ID mismatch - not processing old match (EoG: %s, Latest: %s)",
							currentGameID, latestMatchID)
					}
				}
				log.Printf("Verified: Game ID %s matches latest match", currentGameID)
			} else if recentMatch == nil {
				log.Printf("No recent match history available for verification")
			}

			// Mark this game as processed
			dialogMutex.Lock()
			lastGameID = currentGameID
			dialogMutex.Unlock()
			log.Printf("Processing game ID: %s", currentGameID)
		}
	}

	// Log raw response for debugging
	log.Printf("Raw EoG stats response length: %d bytes", len(data))
	if len(data) > 0 {
		// Parse as generic JSON to inspect structure
		var rawData map[string]interface{}
		if err := json.Unmarshal(data, &rawData); err == nil {
			keys := getKeys(rawData)
			log.Printf("Top-level keys in EoG response (%d keys): %v", len(keys), keys)

			// Check for participants in various possible locations
			if statsBlock, ok := rawData["statsBlock"]; ok {
				log.Printf("Found statsBlock field")
				if statsBlockMap, ok := statsBlock.(map[string]interface{}); ok {
					blockKeys := getKeys(statsBlockMap)
					log.Printf("statsBlock keys (%d keys): %v", len(blockKeys), blockKeys)
					if participants, ok := statsBlockMap["participants"]; ok {
						if participantsArr, ok := participants.([]interface{}); ok {
							log.Printf("Found %d participants in statsBlock", len(participantsArr))
						}
					}
				}
			}
			if teams, ok := rawData["teams"]; ok {
				log.Printf("Found teams field")
				if teamsArr, ok := teams.([]interface{}); ok {
					log.Printf("Found %d teams", len(teamsArr))
					for i, team := range teamsArr {
						if teamMap, ok := team.(map[string]interface{}); ok {
							teamKeys := getKeys(teamMap)
							log.Printf("Team %d keys: %v", i, teamKeys)
							if players, ok := teamMap["players"]; ok {
								if playersArr, ok := players.([]interface{}); ok {
									log.Printf("Team %d has %d players", i, len(playersArr))
									if len(playersArr) > 0 {
										// Log first player structure in detail
										if firstPlayer, ok := playersArr[0].(map[string]interface{}); ok {
											playerKeys := getKeys(firstPlayer)
											log.Printf("Team %d first player keys (%d keys): %v", i, len(playerKeys), playerKeys)
											// Log some key values to understand structure
											if summonerName, ok := firstPlayer["summonerName"]; ok {
												log.Printf("  summonerName: %v", summonerName)
											}
											// Check if stats are nested
											if statsRaw, ok := firstPlayer["stats"]; ok {
												if statsMap, ok := statsRaw.(map[string]interface{}); ok {
													log.Printf("  Found nested stats with keys: %v", func() []string {
														keys := make([]string, 0, len(statsMap))
														for k := range statsMap {
															keys = append(keys, k)
														}
														return keys
													}())
													if dmg, ok := statsMap["totalDamageDealtToChampions"].(float64); ok {
														log.Printf("  totalDamageDealtToChampions in stats: %.0f", dmg)
													}
												}
											} else {
												// Check top-level
												if dmg, ok := firstPlayer["totalDamageDealtToChampions"].(float64); ok {
													log.Printf("  totalDamageDealtToChampions at top-level: %.0f", dmg)
												}
											}
											if riotIdGameName, ok := firstPlayer["riotIdGameName"]; ok {
												log.Printf("  riotIdGameName: %v", riotIdGameName)
											}
											if displayName, ok := firstPlayer["displayName"]; ok {
												log.Printf("  displayName: %v", displayName)
											}
											if championName, ok := firstPlayer["championName"]; ok {
												log.Printf("  championName: %v", championName)
											}
											if championId, ok := firstPlayer["championId"]; ok {
												log.Printf("  championId: %v", championId)
											}
										}
									}
								}
							}
							if participants, ok := teamMap["participants"]; ok {
								if participantsArr, ok := participants.([]interface{}); ok {
									log.Printf("Team %d has %d participants", i, len(participantsArr))
								}
							}
						}
					}
				}
			}
			if participants, ok := rawData["participants"]; ok {
				log.Printf("Found participants at top level, type: %T", participants)
				if participantsArr, ok := participants.([]interface{}); ok {
					log.Printf("Found %d participants at top level", len(participantsArr))
				}
			}
		} else {
			log.Printf("Failed to parse as JSON map: %v", err)
		}
	}

	stats, err := eog.ParseEoGStats(data)
	if err != nil {
		log.Printf("Failed to parse EoG stats. Raw data: %s", string(data))
		return fmt.Errorf("failed to parse EoG stats: %w", err)
	}

	if appConfig.EnableDetailedLogging {
		statsJSON, _ := json.MarshalIndent(stats, "", "  ")
		log.Printf("EoG Stats:\n%s", string(statsJSON))
	}

	// Check if we have participants
	if len(stats.Participants) == 0 {
		log.Printf("EoG stats parsed but no participants found. GameDurationSeconds: %d", stats.GameDurationSeconds)
		// Check if the response was empty or just had no participants
		if len(data) == 0 || string(data) == "{}" || string(data) == "null" {
			return fmt.Errorf("end-of-game stats endpoint returned empty data. Try waiting a few seconds after the game ends.")
		}
		return fmt.Errorf("no game data available (empty participants). Make sure you're in the post-game screen or have finished a match recently")
	}

	// Get clutch stats if monitor was running
	var clutchStats map[string]*monitor.ClutchStats
	if clutchMonitor != nil && clutchMonitor.IsRunning() {
		clutchStats = clutchMonitor.GetStats()
		if appConfig.EnableDebugLogging && len(clutchStats) > 0 {
			log.Printf("[CLUTCH] Collected clutch stats for %d champions", len(clutchStats))
			for champ, stats := range clutchStats {
				log.Printf("[CLUTCH] %s: LivesSaved=%d, TimesSaved=%d, CriticalSaves=%d",
					champ, stats.LivesSaved, stats.TimesSaved,
					func() int {
						count := 0
						for _, e := range stats.Events {
							if e.WasCritical {
								count++
							}
						}
						return count
					}())
			}
		}
	}

	// Analyze game
	gameSummary, err := analyzer.AnalyzeGame(stats, appConfig)
	if err != nil {
		return fmt.Errorf("failed to analyze game: %w", err)
	}

	if gameSummary == nil {
		return fmt.Errorf("no game summary generated (empty participants?)")
	}

	// Integrate clutch stats into game summary
	if clutchStats != nil && len(clutchStats) > 0 {
		analyzer.IntegrateClutchStats(gameSummary, clutchStats)
		if appConfig.EnableDebugLogging {
			log.Printf("[CLUTCH] Integrated clutch stats into game summary")
		}
	}

	// Generate messages via LLM
	summaryJSON, _ := json.MarshalIndent(gameSummary, "", "  ")

	// Debug logging for standout flags and damage accuracy verification
	if appConfig.EnableDebugLogging {
		log.Printf("[DEBUG] === DAMAGE ACCURACY CHECK ===")
		log.Printf("[DEBUG] All players' damage values:")
		maxDamageSeen := 0
		maxDamagePlayer := ""
		for i, player := range gameSummary.Players {
			log.Printf("[DEBUG] Player %d (%s): totalDamage=%d, highestDamageInGame=%v, highestDamageOnTeam=%v, afk=%v",
				i, player.Champion, player.TotalDamage, player.HighestDamageInGame, player.HighestDamageOnTeam, player.Afk)
			if !player.Afk && player.TotalDamage > maxDamageSeen {
				maxDamageSeen = player.TotalDamage
				maxDamagePlayer = player.Champion
			}
		}
		log.Printf("[DEBUG] Max damage in game: %d (by %s)", maxDamageSeen, maxDamagePlayer)

		// Verify flags match actual max damage
		highestDamagePlayers := []string{}
		for _, player := range gameSummary.Players {
			if player.HighestDamageInGame && !player.Afk {
				highestDamagePlayers = append(highestDamagePlayers, player.Champion)
				if player.TotalDamage != maxDamageSeen {
					log.Printf("[ERROR] INCONSISTENCY: %s marked as highestDamageInGame but has %d damage, max is %d!",
						player.Champion, player.TotalDamage, maxDamageSeen)
				} else {
					log.Printf("[DEBUG] ✓ %s correctly marked as highestDamageInGame with %d damage", player.Champion, player.TotalDamage)
				}
			}
		}
		if len(highestDamagePlayers) == 0 && maxDamageSeen > 0 {
			log.Printf("[ERROR] INCONSISTENCY: No player marked as highestDamageInGame but max damage is %d!", maxDamageSeen)
		}
		log.Printf("[DEBUG] === END DAMAGE CHECK ===")
	}

	if appConfig.EnableDetailedLogging {
		log.Printf("Game Summary:\n%s", string(summaryJSON))
	}

	// Use agentic system for message generation
	if appConfig.EnableDebugLogging {
		log.Printf("[AGENTIC] Starting agentic message generation system")
		log.Printf("[AGENTIC] Game Summary JSON (first 2000 chars):\n%s", func() string {
			jsonStr := string(summaryJSON)
			if len(jsonStr) > 2000 {
				return jsonStr[:2000] + "..."
			}
			return jsonStr
		}())
	}

	agenticSystem := llm.NewAgenticSystem(llmClient, gameSummary, &appConfig.LLMSettings)
	messages, err := agenticSystem.GenerateMessages(appConfig.EnableDebugLogging)
	if err != nil {
		log.Printf("Agentic message generation failed: %v. Using fallback messages.", err)
		messages = []string{
			"ggwp everyone, thanks for the game!",
			"Nice effort team, gl in your next games!",
		}
	}

	// Display messages
	log.Println("\n=== Suggested Post-Game Messages ===")
	for i, msg := range messages {
		log.Printf("%d. %s", i+1, msg)
	}
	log.Println("==================================")

	// Auto-copy first message to clipboard if enabled
	if appConfig.AutoCopyToClipboard && len(messages) > 0 {
		if err := copyToClipboard(messages[0]); err != nil {
			log.Printf("Failed to copy to clipboard: %v", err)
		} else {
			log.Printf("Copied first message to clipboard: %s", messages[0])
			// Show toast notification
			ui.ShowToast("LoL Kind Bot", "First message copied to clipboard!")
		}
	} else if len(messages) > 0 {
		// Show toast even if auto-copy is disabled
		ui.ShowToast("LoL Kind Bot", "Post-game messages ready!")
	}

	// Show popup window with message suggestions (only one at a time)
	// Ensure Fyne app is initialized before creating windows
	dialogMutex.Lock()
	if !dialogShowing {
		dialogShowing = true
		dialogMutex.Unlock()

		// Initialize Fyne app and ensure it's ready
		_ = ui.GetFyneApp()

		// Use a goroutine to create the window, but ensure app is running first
		go func() {
			// Small delay to ensure app.Run() has started
			time.Sleep(200 * time.Millisecond)

			// Use RunOnMain to ensure UI operations happen on the correct thread
			ui.RunOnMain(func() {
				defer func() {
					dialogMutex.Lock()
					dialogShowing = false
					dialogMutex.Unlock()
				}()

				ui.ShowMessagesDialog(messages)
			})
		}()
	} else {
		dialogMutex.Unlock()
		log.Println("Dialog already showing, skipping")
	}

	return nil
}

// handlePhaseChange manages gold monitor based on game phase changes
func handlePhaseChange(newPhase, oldPhase string) {
	currentPhase = newPhase

	log.Printf("Phase change: %s -> %s", oldPhase, newPhase)

	// Start monitors when game starts
	if (newPhase == "InProgress" || newPhase == "GameStart") &&
		(oldPhase != "InProgress" && oldPhase != "GameStart") {
		log.Printf("Game started (phase: %s), starting monitors...", newPhase)

		// Start gold monitor
		if goldMonitor != nil {
			if !goldMonitor.IsRunning() {
				goldMonitor.Reset()
				goldMonitor.Start()
				log.Printf("Gold monitor started successfully")
			} else {
				log.Printf("Gold monitor already running")
			}
		} else {
			log.Printf("ERROR: Gold monitor is nil!")
		}

		// Start clutch monitor
		if clutchMonitor != nil {
			if !clutchMonitor.IsRunning() {
				clutchMonitor.Reset()
				clutchMonitor.Start()
				log.Printf("Clutch monitor started successfully")
			} else {
				log.Printf("Clutch monitor already running")
			}
		}
	}

	// Stop monitors when game ends
	if (newPhase == "EndOfGame" || newPhase == "WaitingForStats" || newPhase == "PreEndOfGame") &&
		(oldPhase == "InProgress" || oldPhase == "GameStart") {
		log.Printf("Game ended (phase: %s), stopping monitors...", newPhase)

		// Stop gold monitor
		if goldMonitor != nil && goldMonitor.IsRunning() {
			goldMonitor.Stop()
		}

		// Stop clutch monitor
		if clutchMonitor != nil && clutchMonitor.IsRunning() {
			clutchMonitor.Stop()
			log.Printf("Clutch monitor stopped - collected %d clutch events", len(clutchMonitor.GetStats()))
		}
	}

	// Also stop if we're back in lobby/champ select
	if (newPhase == "Lobby" || newPhase == "ChampSelect" || newPhase == "ReadyCheck") &&
		(oldPhase == "InProgress" || oldPhase == "GameStart") {
		log.Printf("Left game (phase: %s), stopping gold monitor...", newPhase)
		if goldMonitor != nil && goldMonitor.IsRunning() {
			goldMonitor.Stop()
		}
	}
}

func copyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func onReady() {
	iconData := getIconData()
	if len(iconData) > 0 {
		systray.SetIcon(iconData)
	}
	// If icon data is empty, systray will use default icon - that's fine
	systray.SetTitle("LoL Kind Bot")
	systray.SetTooltip("LoL Kind Bot - Post-game positivity generator")

	mToggle := systray.AddMenuItem("Toggle Listener", "Pause/Resume listening")
	mSettings := systray.AddMenuItem("Open Settings", "Configure the bot")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Exit", "Quit the application")

	go func() {
		for {
			select {
			case <-mToggle.ClickedCh:
				listening = !listening
				if listening {
					mToggle.SetTitle("Pause Listener")
					mToggle.SetTooltip("Pause listening for games")
					log.Println("Listener resumed")
				} else {
					mToggle.SetTitle("Resume Listener")
					mToggle.SetTooltip("Resume listening for games")
					if gameMonitor != nil {
						gameMonitor.Stop()
					}
					log.Println("Listener paused")
				}
			case <-mSettings.ClickedCh:
				// Keep console hidden - this is a background app
				log.Println("Opening settings...")

				// Run settings dialog in a goroutine to avoid blocking system tray
				go func() {
					// Create callback function to generate messages from last match
					generateCallback := func() {
						// Check if LCU client is available
						if lcuClient == nil {
						// Try to connect to LCU
						lockfilePath := lcu.GetLockfilePath()
						lockfileInfo, err := lcu.ParseLockfile(lockfilePath)
						if err != nil {
							ui.ShowToast("LoL Kind Bot", "League client not found. Please start League of Legends first.")
							log.Printf("Failed to find League client: %v", err)
							return
						}

						client, err := lcu.NewClient(lockfileInfo)
						if err != nil {
							ui.ShowToast("LoL Kind Bot", "Failed to connect to League client.")
							log.Printf("Failed to create LCU client: %v", err)
							return
						}
						lcuClient = client
					}

					// Check if we're in EndOfGame phase or have recent match
					phaseData, err := lcuClient.Get("/lol-gameflow/v1/gameflow-phase")
					if err == nil {
						var phase string
						if err := json.Unmarshal(phaseData, &phase); err == nil {
							if phase != "EndOfGame" {
								ui.ShowToast("LoL Kind Bot", "Not in post-game screen. Please finish a match first or wait for the post-game screen.")
								log.Printf("Current phase: %s (not EndOfGame)", phase)
								return
							}
						}
					}

					// Small delay to ensure stats are ready
					time.Sleep(1 * time.Second)

					// Generate messages
					if err := handleEndOfGame(); err != nil {
						log.Printf("Failed to generate messages from last match: %v", err)
						if !appConfig.EnableDebugLogging {
							errorMsg := err.Error()
							if len(errorMsg) > 80 {
								errorMsg = errorMsg[:77] + "..."
							}
							ui.ShowToast("LoL Kind Bot", errorMsg)
						}
					} else if !appConfig.EnableDebugLogging {
						ui.ShowToast("LoL Kind Bot", "Messages generated successfully!")
					}
				}

					// Show settings dialog - it will handle its own event loop
					newCfg, ok := ui.ShowSettingsDialogWithCallback(appConfig, generateCallback)
					if ok && newCfg != nil {
						cfgPath := config.GetConfigPath()
						if err := config.SaveConfig(cfgPath, newCfg); err != nil {
							log.Printf("Failed to save settings: %v", err)
						} else {
							appConfig = newCfg
							// Reinitialize LLM client with new settings
							llmClient = llm.NewClient(newCfg.OllamaURL, newCfg.OllamaModel, &newCfg.LLMSettings)
							log.Printf("Settings saved successfully")
						}
					}

					// Ensure console is hidden after dialog closes (unless debug mode)
					if !debugMode {
						ui.HideConsole()
					}
				}()
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {
	log.Println("Exiting...")
	if gameMonitor != nil {
		gameMonitor.Stop()
	}
	os.Exit(0)
}

func getIconData() []byte {
	// Placeholder icon data - in production, embed a proper icon
	// For now, return empty data (will use default system icon)
	return []byte{}
}
