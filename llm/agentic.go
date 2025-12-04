package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"lol-kind-bot/analyzer"
	"lol-kind-bot/config"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
)

// AdvocateTestimony represents a worker's advocacy for a player
type AdvocateTestimony struct {
	PlayerIndex    int    `json:"playerIndex"`
	Champion       string `json:"champion"`
	Testimony      string `json:"testimony"`      // The compelling argument
	KeyPoints      []string `json:"keyPoints"`    // Fact-driven points
	Validated      bool   `json:"validated"`      // Whether validation worker approved
	ValidationNote string `json:"validationNote"` // Notes from validation worker
	Score          float64 `json:"score"`         // Score from judging panel
}

// ValidationResult represents validation worker's assessment
type ValidationResult struct {
	PlayerIndex    int    `json:"playerIndex"`
	Approved       bool   `json:"approved"`
	Notes          string `json:"notes"`
	IssuesFound    []string `json:"issuesFound"`
}

// JudgeRanking represents a judge's ranking of all testimonies
type JudgeRanking struct {
	Rankings []struct {
		PlayerIndex int     `json:"playerIndex"`
		Score       float64 `json:"score"`
		Reason      string  `json:"reason"`
	} `json:"rankings"`
}

// AgenticSystem orchestrates the multi-agent message generation
type AgenticSystem struct {
	client      *Client
	gameSummary *analyzer.GameSummary
	llmSettings *config.LLMSettings
}

// NewAgenticSystem creates a new agentic system
func NewAgenticSystem(client *Client, gameSummary *analyzer.GameSummary, llmSettings *config.LLMSettings) *AgenticSystem {
	return &AgenticSystem{
		client:      client,
		gameSummary: gameSummary,
		llmSettings: llmSettings,
	}
}

// GenerateMessages runs the full agentic workflow
func (as *AgenticSystem) GenerateMessages(enableDebug bool) ([]string, error) {
	// Phase 1: Advocate - 10 workers advocate for each player
	testimonies, err := as.runAdvocatePhase(enableDebug)
	if err != nil {
		return nil, fmt.Errorf("advocate phase failed: %w", err)
	}

	// Phase 2: Validate - validation workers verify claims
	err = as.runValidationPhase(testimonies, enableDebug)
	if err != nil {
		return nil, fmt.Errorf("validation phase failed: %w", err)
	}

	// Phase 3: Judge - panel of 2 judges rank all testimonies
	err = as.runJudgingPhase(testimonies, enableDebug)
	if err != nil {
		return nil, fmt.Errorf("judging phase failed: %w", err)
	}

	// Phase 4: Generate messages for top N candidates
	messages, err := as.runMessageGenerationPhase(testimonies, enableDebug)
	if err != nil {
		return nil, fmt.Errorf("message generation phase failed: %w", err)
	}

	return messages, nil
}

// runAdvocatePhase creates 10 advocate workers, one per player
func (as *AgenticSystem) runAdvocatePhase(enableDebug bool) ([]*AdvocateTestimony, error) {
	if enableDebug {
		log.Printf("[AGENTIC] Starting advocate phase - 10 workers advocating for players")
	}

	gameSummaryJSON, err := json.MarshalIndent(as.gameSummary, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal game summary: %w", err)
	}

	testimonies := make([]*AdvocateTestimony, len(as.gameSummary.Players))
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Create one advocate worker per player
	for i, player := range as.gameSummary.Players {
		// Skip AFK players
		if player.Afk {
			testimonies[i] = &AdvocateTestimony{
				PlayerIndex: i,
				Champion:    player.Champion,
				Validated:   false,
			}
			continue
		}

		wg.Add(1)
		go func(idx int, p analyzer.PlayerSummary) {
			defer wg.Done()

			testimony := as.advocateForPlayer(idx, p, string(gameSummaryJSON), enableDebug)
			
			mu.Lock()
			testimonies[idx] = testimony
			mu.Unlock()

			if enableDebug {
				log.Printf("[AGENTIC] Advocate %d (%s) completed testimony", idx, p.Champion)
			}
		}(i, player)
	}

	wg.Wait()

	if enableDebug {
		log.Printf("[AGENTIC] Advocate phase complete - %d testimonies generated", len(testimonies))
	}

	return testimonies, nil
}

// advocateForPlayer creates an advocate worker prompt for a specific player
func (as *AgenticSystem) advocateForPlayer(playerIndex int, player analyzer.PlayerSummary, gameSummaryJSON string, enableDebug bool) *AdvocateTestimony {
	prompt := as.buildAdvocatePrompt(playerIndex, player, gameSummaryJSON)

	response, err := as.client.generateRaw(prompt)
	if err != nil {
		log.Printf("[AGENTIC] Advocate worker failed for %s: %v", player.Champion, err)
		return &AdvocateTestimony{
			PlayerIndex: playerIndex,
			Champion:    player.Champion,
			Validated:   false,
		}
	}

	// Parse the response to extract testimony and key points
	testimony := as.parseAdvocateResponse(response, playerIndex, player.Champion)

	return testimony
}

// buildAdvocatePrompt creates the prompt for an advocate worker
func (as *AgenticSystem) buildAdvocatePrompt(playerIndex int, player analyzer.PlayerSummary, gameSummaryJSON string) string {
	// Extract player-specific data
	playerJSON, _ := json.MarshalIndent(player, "", "  ")

	// Determine if this player won or lost
	didWin := player.Team == as.gameSummary.WinningTeam
	winLossContext := "WON"
	if !didWin {
		winLossContext = "LOST"
	}

	return fmt.Sprintf(`You are an advocate worker for League of Legends post-game analysis. Your SOLE PURPOSE is to advocate for why %s (player index %d) deserves a shout-out message.

CRITICAL WIN/LOSS CONTEXT:
- This player's team %s the game
- Winning team: %s
- This player's team: %s
- You MUST tailor your advocacy to reflect this outcome
- If they LOST: Focus on individual skill, mechanics, kit mastery, and standout plays despite the loss
- If they WON: You can mention their contribution to the victory, but still focus on their individual achievements
- NEVER claim they "helped secure the win" or "contributed to victory" if they LOST
- NEVER use phrases like "great win", "secured the victory", "helped win" if they LOST

YOUR ROLE:
- You are a passionate advocate for %s
- Your job is to create a compelling, fact-driven testimony
- Focus on their achievements, contributions, and standout moments
- Be specific and use actual game data to support your claims
- Remember: This player %s - tailor your language accordingly

PLAYER DATA:
%s

FULL GAME SUMMARY:
%s

CRITICAL RULES:
1. Use ONLY facts from the game data - check achievements, tags, metrics, and stats
2. Reference specific achievements: highestDamageInGame, mostHealingShielding, highestVisionInGame, etc.
3. Highlight their tags: %s
4. Mention their metrics: KP=%.2f, DamageShare=%.2f%%, VisionScore=%d, etc.
5. CRITICAL: If clutch stats are available, highlight them!
   - LivesSaved: %d times they saved teammates
   - TimesSaved: %d times they were saved
   - CriticalSaves: %d critical saves (< 20%% HP)
   - Use phrases like "Soraka saved my life %d TIMES!" or "clutch heals securing the win"
6. Be compelling but truthful - don't exaggerate or make false claims
7. Focus on what makes them stand out
8. CRITICAL: Match your language to the win/loss outcome - if they lost, don't use victory language

OUTPUT FORMAT:
You must output a JSON object with this exact structure:
{
  "testimony": "A compelling 2-3 sentence argument for why this player deserves recognition (tailored to win/loss)",
  "keyPoints": ["fact 1", "fact 2", "fact 3"]
}

The testimony should be compelling and fact-driven. The keyPoints should be specific achievements or metrics that support the testimony.
Remember: If they lost, focus on individual skill and mechanics, not team victory.

Generate your advocacy now:`,
		player.Champion, playerIndex,
		winLossContext,
		as.gameSummary.WinningTeam,
		player.Team,
		player.Champion,
		winLossContext,
		string(playerJSON),
		gameSummaryJSON,
		strings.Join(player.Tags, ", "),
		player.Metrics.KP,
		player.DamageShare*100,
		player.VisionScore,
		player.LivesSaved,
		player.TimesSaved,
		player.CriticalSaves)
}

// parseAdvocateResponse parses the advocate worker's response
func (as *AgenticSystem) parseAdvocateResponse(response string, playerIndex int, champion string) *AdvocateTestimony {
	// Try to extract JSON from response
	response = strings.TrimSpace(response)
	
	// Find JSON object in response
	startIdx := strings.Index(response, "{")
	endIdx := strings.LastIndex(response, "}")
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		// Fallback: create basic testimony
		return &AdvocateTestimony{
			PlayerIndex: playerIndex,
			Champion:    champion,
			Testimony:   fmt.Sprintf("%s had a solid performance this game.", champion),
			KeyPoints:   []string{"Participated in the game"},
			Validated:   false,
		}
	}

	jsonStr := response[startIdx : endIdx+1]
	var result struct {
		Testimony string   `json:"testimony"`
		KeyPoints []string `json:"keyPoints"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		// Fallback
		return &AdvocateTestimony{
			PlayerIndex: playerIndex,
			Champion:    champion,
			Testimony:   fmt.Sprintf("%s contributed to the game.", champion),
			KeyPoints:   []string{"Active participant"},
			Validated:   false,
		}
	}

	return &AdvocateTestimony{
		PlayerIndex: playerIndex,
		Champion:    champion,
		Testimony:   result.Testimony,
		KeyPoints:   result.KeyPoints,
		Validated:   false,
	}
}

// runValidationPhase validates all testimonies
func (as *AgenticSystem) runValidationPhase(testimonies []*AdvocateTestimony, enableDebug bool) error {
	if enableDebug {
		log.Printf("[AGENTIC] Starting validation phase")
	}

	gameSummaryJSON, err := json.MarshalIndent(as.gameSummary, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal game summary: %w", err)
	}

	// Create validation workers (batch processing for efficiency)
	// Use single validator for speed - validation is less critical than scoring
	numValidators := 1

	var wg sync.WaitGroup
	validationResults := make([]*ValidationResult, len(testimonies))
	var mu sync.Mutex

	// Each validator checks all testimonies
	for v := 0; v < numValidators; v++ {
		wg.Add(1)
		go func(validatorID int) {
			defer wg.Done()

			results := as.validateTestimonies(testimonies, string(gameSummaryJSON), validatorID, enableDebug)
			
			mu.Lock()
			// Merge validation results (approve only if all validators approve)
			for _, result := range results {
				// Use PlayerIndex from result to safely index into validationResults
				idx := result.PlayerIndex
				if idx < 0 || idx >= len(validationResults) {
					// Invalid index, skip this result
					continue
				}
				if validationResults[idx] == nil {
					validationResults[idx] = result
				} else {
					// Both must approve
					validationResults[idx].Approved = validationResults[idx].Approved && result.Approved
					if !result.Approved {
						validationResults[idx].Notes += " | " + result.Notes
						validationResults[idx].IssuesFound = append(validationResults[idx].IssuesFound, result.IssuesFound...)
					}
				}
			}
			mu.Unlock()
		}(v)
	}

	wg.Wait()

	// Apply validation results to testimonies
	for i, result := range validationResults {
		if result != nil && i < len(testimonies) && testimonies[i] != nil {
			testimonies[i].Validated = result.Approved
			testimonies[i].ValidationNote = result.Notes
		}
	}

	if enableDebug {
		validatedCount := 0
		for _, t := range testimonies {
			if t != nil && t.Validated {
				validatedCount++
			}
		}
		log.Printf("[AGENTIC] Validation phase complete - %d/%d testimonies validated", validatedCount, len(testimonies))
	}

	return nil
}

// validateTestimonies validates all testimonies
func (as *AgenticSystem) validateTestimonies(testimonies []*AdvocateTestimony, gameSummaryJSON string, validatorID int, enableDebug bool) []*ValidationResult {
	// Build prompt for validation worker
	prompt := as.buildValidationPrompt(testimonies, gameSummaryJSON, validatorID)

	response, err := as.client.generateRaw(prompt)
	if err != nil {
		log.Printf("[AGENTIC] Validation worker %d failed: %v", validatorID, err)
		// Default: approve all if validation fails
		results := make([]*ValidationResult, len(testimonies))
		for i := range results {
			results[i] = &ValidationResult{
				PlayerIndex: i,
				Approved:    true,
				Notes:       "Validation worker error - defaulting to approved",
			}
		}
		return results
	}

	// Parse validation response
	return as.parseValidationResponse(response, len(testimonies))
}

// buildValidationPrompt creates prompt for validation worker
func (as *AgenticSystem) buildValidationPrompt(testimonies []*AdvocateTestimony, gameSummaryJSON string, validatorID int) string {
	testimoniesJSON, _ := json.MarshalIndent(testimonies, "", "  ")

	return fmt.Sprintf(`You are a validation worker for League of Legends post-game analysis. Your SOLE PURPOSE is to verify that advocate workers' claims are factually accurate.

YOUR ROLE:
- Review each advocate testimony and verify all claims against the game data
- Check that achievements, stats, and metrics are correctly cited
- Flag any false claims, exaggerations, or unsupported statements
- Approve testimonies that are factually accurate and compelling

ADVOCATE TESTIMONIES:
%s

FULL GAME SUMMARY (SOURCE OF TRUTH):
%s

VALIDATION RULES:
1. Check achievements object - verify claims about "most damage", "highest vision", etc.
2. Verify tags are correctly referenced
3. Verify metrics match the data
4. Ensure no false claims or exaggerations
5. CRITICAL: Check win/loss language - verify testimonies match the game outcome
   - Check each player's team vs winning team in the game summary
   - If a player LOST, reject testimonies that mention "win", "victory", "secured", "helped win", "great win", etc.
   - If a player WON, testimonies can mention victory but should focus on individual achievements
6. Approve if testimony is factually accurate, compelling, and matches win/loss context

OUTPUT FORMAT:
You must output a JSON array with validation results:
[
  {
    "playerIndex": 0,
    "approved": true,
    "notes": "All claims verified",
    "issuesFound": []
  },
  {
    "playerIndex": 1,
    "approved": false,
    "notes": "Claimed highest damage but achievement shows otherwise",
    "issuesFound": ["Incorrect damage claim"]
  },
  ...
]

Validate all testimonies now:`,
		string(testimoniesJSON),
		gameSummaryJSON)
}

// parseValidationResponse parses validation worker's response
func (as *AgenticSystem) parseValidationResponse(response string, expectedCount int) []*ValidationResult {
	response = strings.TrimSpace(response)
	
	// Find JSON array in response
	startIdx := strings.Index(response, "[")
	endIdx := strings.LastIndex(response, "]")
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		// Default: approve all
		results := make([]*ValidationResult, expectedCount)
		for i := range results {
			results[i] = &ValidationResult{
				PlayerIndex: i,
				Approved:    true,
				Notes:       "Could not parse validation - defaulting to approved",
			}
		}
		return results
	}

	jsonStr := response[startIdx : endIdx+1]
	var results []*ValidationResult
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		// Default: approve all
		results = make([]*ValidationResult, expectedCount)
		for i := range results {
			results[i] = &ValidationResult{
				PlayerIndex: i,
				Approved:    true,
				Notes:       "Failed to parse validation - defaulting to approved",
			}
		}
		return results
	}

	// Ensure we have results for all players
	if len(results) < expectedCount {
		// Pad with approved defaults
		for i := len(results); i < expectedCount; i++ {
			results = append(results, &ValidationResult{
				PlayerIndex: i,
				Approved:    true,
				Notes:       "No validation provided - defaulting to approved",
			})
		}
	}

	return results
}

// runJudgingPhase has a panel of judges rank all testimonies
func (as *AgenticSystem) runJudgingPhase(testimonies []*AdvocateTestimony, enableDebug bool) error {
	if enableDebug {
		log.Printf("[AGENTIC] Starting judging phase - panel of 2 judges")
	}

	gameSummaryJSON, err := json.MarshalIndent(as.gameSummary, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal game summary: %w", err)
	}

	// Create panel of 2 judges (reduced from 5 for performance)
	numJudges := 2
	judgeScores := make([][]float64, numJudges)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for j := 0; j < numJudges; j++ {
		wg.Add(1)
		go func(judgeID int) {
			defer wg.Done()

			scores := as.judgeTestimonies(testimonies, string(gameSummaryJSON), judgeID, enableDebug)
			
			mu.Lock()
			judgeScores[judgeID] = scores
			mu.Unlock()
		}(j)
	}

	wg.Wait()

	// Average scores from all judges
	for i, testimony := range testimonies {
		if testimony == nil {
			continue
		}
		totalScore := 0.0
		scoreCount := 0
		for _, scores := range judgeScores {
			if i < len(scores) {
				totalScore += scores[i]
				scoreCount++
			}
		}
		if scoreCount > 0 {
			testimony.Score = totalScore / float64(scoreCount)
		}
	}

	if enableDebug {
		log.Printf("[AGENTIC] Judging phase complete - scores assigned")
		for i, t := range testimonies {
			if t != nil {
				log.Printf("[AGENTIC]   Player %d (%s): Score %.2f", i, t.Champion, t.Score)
			}
		}
	}

	return nil
}

// judgeTestimonies has a judge rank all testimonies
func (as *AgenticSystem) judgeTestimonies(testimonies []*AdvocateTestimony, gameSummaryJSON string, judgeID int, enableDebug bool) []float64 {
	prompt := as.buildJudgePrompt(testimonies, gameSummaryJSON, judgeID)

	response, err := as.client.generateRaw(prompt)
	if err != nil {
		log.Printf("[AGENTIC] Judge %d failed: %v", judgeID, err)
		// Default: equal scores
		scores := make([]float64, len(testimonies))
		for i := range scores {
			scores[i] = 5.0 // Default middle score
		}
		return scores
	}

	return as.parseJudgeResponse(response, len(testimonies))
}

// buildJudgePrompt creates prompt for a judge
func (as *AgenticSystem) buildJudgePrompt(testimonies []*AdvocateTestimony, gameSummaryJSON string, judgeID int) string {
	testimoniesJSON, _ := json.MarshalIndent(testimonies, "", "  ")

	return fmt.Sprintf(`You are judge #%d in a panel of 2 judges evaluating League of Legends post-game shout-out candidates.

YOUR ROLE:
- Evaluate all advocate testimonies and rank them
- Consider: compelling arguments, factual accuracy, standout achievements, overall impact
- Score each testimony from 0.0 to 10.0
- Higher scores = more deserving of a shout-out message

VALIDATED TESTIMONIES:
%s

FULL GAME SUMMARY:
%s

JUDGING CRITERIA:
1. Compelling testimony (how well did the advocate make their case?)
2. Factual accuracy (are claims supported by data?)
3. Standout achievements (did the player do something exceptional?)
4. Overall impact (how much did this player contribute?)

OUTPUT FORMAT:
You must output a JSON array with scores for each player:
[
  {"playerIndex": 0, "score": 8.5, "reason": "Strong damage and vision control"},
  {"playerIndex": 1, "score": 7.2, "reason": "Good support play"},
  ...
]

Score all testimonies now:`,
		judgeID+1,
		string(testimoniesJSON),
		gameSummaryJSON)
}

// parseJudgeResponse parses judge's scoring response
func (as *AgenticSystem) parseJudgeResponse(response string, expectedCount int) []float64 {
	response = strings.TrimSpace(response)
	
	// Find JSON array
	startIdx := strings.Index(response, "[")
	endIdx := strings.LastIndex(response, "]")
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		// Default scores
		scores := make([]float64, expectedCount)
		for i := range scores {
			scores[i] = 5.0
		}
		return scores
	}

	jsonStr := response[startIdx : endIdx+1]
	var rankings []struct {
		PlayerIndex int     `json:"playerIndex"`
		Score       float64 `json:"score"`
		Reason      string  `json:"reason"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &rankings); err != nil {
		// Default scores
		scores := make([]float64, expectedCount)
		for i := range scores {
			scores[i] = 5.0
		}
		return scores
	}

	// Map rankings to scores array
	scores := make([]float64, expectedCount)
	for _, r := range rankings {
		if r.PlayerIndex >= 0 && r.PlayerIndex < expectedCount {
			// Clamp score to 0-10
			if r.Score < 0 {
				r.Score = 0
			}
			if r.Score > 10 {
				r.Score = 10
			}
			scores[r.PlayerIndex] = r.Score
		}
	}

	// Fill missing scores with defaults
	for i := range scores {
		if scores[i] == 0 && i < len(rankings) {
			scores[i] = 5.0
		}
	}

	return scores
}

// runMessageGenerationPhase generates messages for top N candidates
func (as *AgenticSystem) runMessageGenerationPhase(testimonies []*AdvocateTestimony, enableDebug bool) ([]string, error) {
	if enableDebug {
		log.Printf("[AGENTIC] Starting message generation phase")
	}

	// Determine how many messages to generate based on config
	numMessages := as.llmSettings.MaxMessages
	if numMessages > len(testimonies) {
		numMessages = len(testimonies)
	}

	// Sort testimonies by score (highest first)
	sortedTestimonies := make([]*AdvocateTestimony, 0, len(testimonies))
	for _, t := range testimonies {
		if t != nil && t.Champion != "" {
			sortedTestimonies = append(sortedTestimonies, t)
		}
	}

	sort.Slice(sortedTestimonies, func(i, j int) bool {
		return sortedTestimonies[i].Score > sortedTestimonies[j].Score
	})

	// Take top N
	topN := numMessages
	if topN > len(sortedTestimonies) {
		topN = len(sortedTestimonies)
	}
	topCandidates := sortedTestimonies[:topN]

	if enableDebug {
		log.Printf("[AGENTIC] Generating messages for top %d candidates", topN)
		for i, t := range topCandidates {
			log.Printf("[AGENTIC]   %d. %s (Score: %.2f)", i+1, t.Champion, t.Score)
		}
	}

	// Generate messages for each top candidate (PARALLELIZED for performance)
	gameSummaryJSON, _ := json.MarshalIndent(as.gameSummary, "", "  ")
	messages := make([]string, 0, topN)
	
	var wg sync.WaitGroup
	var mu sync.Mutex
	messageChan := make(chan string, topN)
	
	// Generate messages in parallel
	for _, candidate := range topCandidates {
		wg.Add(1)
		go func(cand *AdvocateTestimony) {
			defer wg.Done()
			message := as.generateMessageForCandidate(cand, string(gameSummaryJSON), enableDebug)
			if message != "" {
				// Validate win/loss context before adding
				if as.validateWinLossContext(message, cand.PlayerIndex, enableDebug) {
					messageChan <- message
				} else if enableDebug {
					log.Printf("[AGENTIC] Filtered message for %s due to win/loss mismatch: %s", cand.Champion, message)
				}
			}
		}(candidate)
	}
	
	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(messageChan)
	}()
	
	// Collect messages
	for msg := range messageChan {
		mu.Lock()
		messages = append(messages, msg)
		mu.Unlock()
	}

	// Fallback if no messages generated
	if len(messages) == 0 {
		if enableDebug {
			log.Printf("[AGENTIC] No messages generated, using fallback")
		}
		messages = generateContextualFallbackMessages(string(gameSummaryJSON))
	}

	// Ensure we have at least MinMessages (pad with fallback if needed)
	if len(messages) < as.llmSettings.MinMessages {
		fallback := generateContextualFallbackMessages(string(gameSummaryJSON))
		for len(messages) < as.llmSettings.MinMessages && len(fallback) > 0 {
			// Check if fallback message already exists
			exists := false
			for _, msg := range messages {
				if msg == fallback[0] {
					exists = true
					break
				}
			}
			if !exists {
				messages = append(messages, fallback[0])
			}
			fallback = fallback[1:]
		}
	}

	// Limit to MaxMessages
	if len(messages) > as.llmSettings.MaxMessages {
		messages = messages[:as.llmSettings.MaxMessages]
	}

	if enableDebug {
		log.Printf("[AGENTIC] Message generation complete - %d messages generated", len(messages))
	}

	return messages, nil
}

// generateMessageForCandidate generates a single message for a candidate
func (as *AgenticSystem) generateMessageForCandidate(candidate *AdvocateTestimony, gameSummaryJSON string, enableDebug bool) string {
	player := as.gameSummary.Players[candidate.PlayerIndex]
	
	prompt := as.buildMessagePrompt(candidate, player, gameSummaryJSON)

	response, err := as.client.generateRaw(prompt)
	if err != nil {
		log.Printf("[AGENTIC] Message generation failed for %s: %v", candidate.Champion, err)
		return ""
	}

	// Parse and clean the message
	message := strings.TrimSpace(response)
	
	// Remove any JSON formatting if present
	message = strings.Trim(message, `"`)
	message = strings.Trim(message, `'`)
	
	// Remove common prefixes
	prefixes := []string{
		"Message:",
		"Shout-out:",
		"Message for " + candidate.Champion + ":",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(message, prefix) {
			message = strings.TrimPrefix(message, prefix)
			message = strings.TrimSpace(message)
		}
	}

	// Limit length
	maxLen := as.llmSettings.MaxMessageLength
	if maxLen > 0 && len(message) > maxLen {
		message = message[:maxLen]
		// Try to end at a sentence
		lastPeriod := strings.LastIndex(message, ".")
		if lastPeriod > maxLen*2/3 {
			message = message[:lastPeriod+1]
		}
	}

	return message
}

// validateWinLossContext checks if a message matches the player's win/loss outcome
func (as *AgenticSystem) validateWinLossContext(message string, playerIndex int, enableDebug bool) bool {
	if playerIndex < 0 || playerIndex >= len(as.gameSummary.Players) {
		return true // Can't validate, allow through
	}

	player := as.gameSummary.Players[playerIndex]
	didWin := player.Team == as.gameSummary.WinningTeam
	lowerMsg := strings.ToLower(message)

	// List of win-related phrases that should NOT appear for losing players
	winPhrases := []string{
		"helped secure the win",
		"secured the win",
		"helped win",
		"great win",
		"secured the victory",
		"contributed to victory",
		"contributed to the win",
		"helped secure victory",
		"victory",
		"won the game",
		"winning",
	}

	// Check if message contains win phrases
	hasWinPhrase := false
	for _, phrase := range winPhrases {
		if strings.Contains(lowerMsg, phrase) {
			hasWinPhrase = true
			break
		}
	}

	// If player lost but message has win phrases, reject it
	if !didWin && hasWinPhrase {
		if enableDebug {
			log.Printf("[AGENTIC] Win/loss validation failed: Player %s lost but message contains win language", player.Champion)
		}
		return false
	}

	return true
}

// buildLanguageStyleForAgentic returns language style instructions for agentic system
func buildLanguageStyleForAgentic(style string) string {
	switch style {
	case "formal":
		return "Use more formal language. Avoid slang and casual expressions."
	case "enthusiastic":
		return "Use energetic and expressive language. Show excitement appropriately."
	case "gamer":
		return `Use authentic gamer chat style! Examples: "njnj", "clutch heals securing the win", "Yuumi kept me alive and I was sure I was dead 4 times omg", "wp", "gg", "ty", "mb", "gj", "pog", "insane", "cracked", "nuts". Use lowercase, abbreviations, and authentic gaming expressions. Be enthusiastic but casual - like you're typing in post-game chat. Use "omg", "lol", "haha" naturally. Keep it real and relatable to actual League chat.`
	default: // "casual"
		return `Use casual gamer language naturally. "ggwp", "nice damage", etc. are appropriate.`
	}
}

// buildMessagePrompt creates prompt for generating a message for a candidate
func (as *AgenticSystem) buildMessagePrompt(candidate *AdvocateTestimony, player analyzer.PlayerSummary, gameSummaryJSON string) string {
	// Determine if this player won or lost
	didWin := player.Team == as.gameSummary.WinningTeam
	winLossContext := "WON"
	if !didWin {
		winLossContext = "LOST"
	}

	return fmt.Sprintf(`You are generating a post-game shout-out message for %s.

CRITICAL WIN/LOSS CONTEXT:
- This player's team %s the game
- Winning team: %s
- This player's team: %s
- You MUST tailor your message to reflect this outcome

This player was selected by a panel of judges as deserving recognition based on:
- Testimony: %s
- Key Points: %s
- Score: %.2f/10.0

FULL GAME SUMMARY:
%s

MESSAGE REQUIREMENTS:
- Under %d characters
- 1-2 sentences
- Positive, wholesome, sportsmanlike
- Reference the champion by name (%s)
- Focus on their standout achievements
- Language style: %s
- No team color references (RED/BLUE)
- No future game references

CRITICAL WIN/LOSS LANGUAGE RULES:
- If they WON: You can mention their contribution to victory, but focus on individual achievements
  Examples: "Great damage from %s!" "Amazing plays from %s!"
- If they LOST: Focus on individual skill, mechanics, kit mastery, and standout plays
  Examples: "Rough game, but %s's mechanics were on point!" "Sucks to lose, but %s's kit execution was clean!"
- NEVER say "helped secure the win", "contributed to victory", "great win", "secured the victory" if they LOST
- NEVER use victory language for losing players
- If they lost, use phrases like: "Rough game, but...", "Sucks to lose, but...", "Tough loss, but...", "Awe, dang, but..."

CLUTCH MOMENTS HIGHLIGHTING:
- LivesSaved: %d - Times this player saved teammates (heals, shields, CC saves)
- TimesSaved: %d - Times this player was saved by teammates  
- CriticalSaves: %d - Critical saves when player was < 20%% HP
- If LivesSaved > 0: Highlight their clutch saves! Examples: "Soraka saved my life %d times!" or "clutch heals securing the win"
- If CriticalSaves >= 3: Emphasize critical saves! Example: "Saved me %d times when I was sure I was dead!"
- If TimesSaved > 0: Acknowledge teamwork! Example: "Thanks for keeping me alive!"
- Use authentic gamer language: "omg", "clutch", "saved my life", etc.

Generate ONE message now (just the message text, no labels or formatting):`,
		candidate.Champion,
		winLossContext,
		as.gameSummary.WinningTeam,
		player.Team,
		candidate.Testimony,
		strings.Join(candidate.KeyPoints, ", "),
		candidate.Score,
		gameSummaryJSON,
		as.llmSettings.MaxMessageLength,
		candidate.Champion,
		buildLanguageStyleForAgentic(as.llmSettings.LanguageStyle),
		candidate.Champion,
		candidate.Champion,
		candidate.Champion,
		candidate.Champion,
		player.LivesSaved,
		player.TimesSaved,
		player.CriticalSaves,
		player.LivesSaved,
		player.CriticalSaves,
		player.TimesSaved)
}

// generateRaw makes a raw LLM call and returns the response
func (c *Client) generateRaw(prompt string) (string, error) {
	reqBody := GenerateRequest{
		Model:       c.Model,
		Prompt:      prompt,
		Stream:      false,
		Temperature: c.Config.Temperature,
	}
	
	if c.Config.MaxTokens > 0 {
		reqBody.NumPredict = c.Config.MaxTokens
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use shared HTTP client for connection pooling and parallel requests
	if c.httpClient == nil {
		c.httpClient = SharedHTTPClient
	}

	resp, err := c.httpClient.Post(c.URL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var genResp GenerateResponse
	if err := json.Unmarshal(body, &genResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return genResp.Response, nil
}

