package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"lol-kind-bot/config"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Client struct {
	URL        string
	Model      string
	Config     *config.LLMSettings
	httpClient *http.Client
}

var (
	// SharedHTTPClient is a shared HTTP client with connection pooling for parallel requests
	SharedHTTPClient *http.Client
	sharedClientOnce sync.Once
)

func init() {
	// Initialize shared HTTP client with high concurrency support
	sharedClientOnce.Do(func() {
		transport := &http.Transport{
			MaxIdleConns:        100,              // Maximum idle connections
			MaxIdleConnsPerHost: 100,              // Maximum idle connections per host (allows many parallel requests)
			MaxConnsPerHost:     0,                // 0 = unlimited connections per host
			IdleConnTimeout:     90 * time.Second, // Keep connections alive
			DisableKeepAlives:   false,           // Enable keep-alive for connection reuse
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		}
		
		SharedHTTPClient = &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second, // Per-request timeout
		}
	})
}

type GenerateRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	Stream      bool    `json:"stream"`
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"` // Max tokens for Ollama
}

type GenerateResponse struct {
	Response string `json:"response"`
}

func NewClient(url, model string, llmSettings *config.LLMSettings) *Client {
	return &Client{
		URL:        url,
		Model:      model,
		Config:     llmSettings,
		httpClient: SharedHTTPClient, // Use shared client for connection pooling
	}
}

func BuildPrompt(gameSummaryJSON string, llmSettings *config.LLMSettings) string {
	// Parse game summary to extract context
	var gameSummary map[string]interface{}
	intenseMatchContext := ""
	teamContext := ""
	gameModeContext := ""
	
	if err := json.Unmarshal([]byte(gameSummaryJSON), &gameSummary); err == nil {
		// Extract team context
		myTeam, _ := gameSummary["myTeam"].(string)
		winningTeam, _ := gameSummary["winningTeam"].(string)
		mySummonerName, _ := gameSummary["mySummonerName"].(string)
		
		// Extract game mode information
		gameMode, _ := gameSummary["gameMode"].(string)
		queueType, _ := gameSummary["queueType"].(string)
		_, _ = gameSummary["gameType"].(string) // gameType available but not currently used
		
		// Build game mode context
		if gameMode != "" || queueType != "" {
			modeName := queueType
			if modeName == "" {
				modeName = gameMode
			}
			
			// Determine mode characteristics
			if strings.Contains(strings.ToUpper(modeName), "ARAM") {
				gameModeContext = `
GAME MODE CONTEXT - ARAM:
- This is an ARAM (All Random All Mid) match - a fast-paced, single-lane game mode focused on constant teamfighting.
- ARAM is more casual and fun-focused than ranked play - keep messages lighthearted and fun.
- High kill counts and damage numbers are normal in ARAM - don't overemphasize them.
- Focus on fun moments, good plays, and the chaotic nature of ARAM.
- If this is ARAM: Mayhem (with augments), acknowledge the unique augments and enhanced chaos!`
			} else if strings.Contains(strings.ToUpper(modeName), "URF") {
				gameModeContext = `
GAME MODE CONTEXT - URF:
- This is URF (Ultra Rapid Fire) - an ultra-fast, high-damage, chaotic game mode.
- URF is all about fun and chaos - keep messages energetic and fun-focused.
- Everything happens faster in URF - acknowledge the pace and intensity.
- High damage and kill counts are expected - focus on fun moments and crazy plays.`
			} else if strings.Contains(strings.ToUpper(modeName), "RANKED") {
				gameModeContext = `
GAME MODE CONTEXT - RANKED:
- This is a ranked match - more competitive and serious than casual modes.
- Keep messages respectful and focused on good plays and teamwork.
- Acknowledge competitive performance and strategic play.
- Be more measured in tone - ranked players appreciate recognition of skill.`
			} else if strings.Contains(strings.ToUpper(modeName), "NORMAL") {
				gameModeContext = `
GAME MODE CONTEXT - NORMAL:
- This is a normal match - casual but still competitive.
- Keep messages friendly and positive.
- Acknowledge good plays and teamwork without being overly serious.`
			}
		}
		
		// Build team context instructions
		if myTeam != "" && winningTeam != "" {
			didWin := myTeam == winningTeam
			enemyTeam := "RED"
			if myTeam == "RED" {
				enemyTeam = "BLUE"
			}
			
			teamContext = fmt.Sprintf(`
CRITICAL CONTEXT - UNDERSTAND THE REALITY:
- This is a SINGLE MATCH with RANDOM PLAYERS. You will likely never see these players again.
- Team colors (RED/BLUE) are meaningless - players don't identify with "team RED" or "team BLUE".
- This is NOT a permanent team. These are strangers matched together for ONE game.
- Do NOT reference team colors (RED/BLUE) - they're irrelevant and no one cares.
- Do NOT imply future games together ("next game", "keep pushing", "keep it up", "see you next time").
- Focus on THIS match only - acknowledge what happened, praise good plays, move on.

WIN/LOSS AWARENESS:
- You %s this game.
- If you WON: Acknowledge good plays from teammates and opponents. Celebrate the victory briefly. NEVER apologize to your own team when you win. ONLY apologize to the ENEMY team if there was an AFK on your team (unfair advantage).
- If you LOST: Use a more casual, relaxed tone. Acknowledge the loss naturally ("Awe, dang", "Sucks to lose", "Rough one", etc.) but keep it light. Focus on highlighting INDIVIDUAL player kits, mechanics, and standout plays rather than teamplay achievements. Examples: "Awe, dang. Sucks to lose, but seeing how insane [champion]'s kit is... Wow, props." or "Rough game, but [champion]'s mechanics were on point!" or "Tough loss, but [champion]'s plays were impressive!" Focus on individual skill and kit mastery rather than team coordination when losing. Still be positive and supportive, but realistic about the outcome.

LOSS-SPECIFIC MESSAGING GUIDANCE:
When you LOST, prioritize these approaches:
1. INDIVIDUAL KIT HIGHLIGHTS: Focus on champion kits, mechanics, and individual skill rather than team coordination. Examples:
   - "Awe, dang. Sucks to lose, but seeing how insane [champion]'s kit is... Wow, props."
   - "Rough one, but [champion]'s mechanics were on point!"
   - "Tough loss, but [champion]'s plays were impressive!"
   - "Sucks to lose, but [champion]'s kit execution was clean!"
2. CASUAL ACKNOWLEDGMENT: Keep it light and natural. Don't force positivity about teamplay when it's a loss.
3. SKILL RECOGNITION: Highlight individual skill, kit mastery, and mechanics that stood out despite the loss.
4. AVOID OVEREMPHASIZING TEAMPLAY: In losses, team coordination is harder to convey positively. Focus on individual achievements instead.
5. TONE VARIETY: Mix casual acknowledgments ("Awe, dang", "Rough one", "Sucks to lose") with skill recognition ("props", "on point", "impressive", "clean execution").

APOLOGY RULES:
- ONLY apologize to the ENEMY team (%s), NEVER to your own team (%s).
- Apologize ONLY if you WON and there was an AFK on your team (unfair advantage).
- If you LOST, NEVER apologize - you have nothing to apologize for!
- If you WON normally (no AFK), do NOT apologize - celebrate instead!
- NEVER say things like "apologize to [your own team]" or "sorry [your own team]" when you won.

PLAYER REFERENCES - BE SMART:
- When praising players, identify them by their CHAMPION NAME, not team color.
- Examples: "Great plays from Gwen!" NOT "Great plays from Gwen on RED team!"
- Examples: "Nice damage from [champion]!" NOT "Nice damage from [champion] on [team]!"
- When addressing opponents, use "enemy team", "opponents", or "enemy [champion]".
- NEVER say "team RED" or "team BLUE" - these are meaningless labels that no one cares about.
- NEVER say "BLUE team" or "RED team" - completely remove team color references.
- NEVER say "our team" or "your team" in a way that implies permanence.
- Focus on individual champions and their performance, not team identity.
- If you catch yourself writing "BLUE team" or "RED team", delete it and rewrite without team colors.

STANDOUT PERFORMANCE HIGHLIGHTING:
- Pay close attention to the "highestDamageInGame", "highestDamageOnTeam", "mostHealingInGame", "mostShieldingInGame", "mostHealingShielding", "highestVisionInGame", and "highestVisionOnTeam" flags in the player data.
- If YOU (%s) have standout flags (especially highestDamageInGame, mostHealingShielding, highestVisionInGame), make sure to highlight YOUR achievements in at least one message!
- When highlighting standout performances, be specific: "Gwen dealt the most damage in the game!" or "Amazing healing and shielding from [champion]!"
- Focus on actual standout stats from the game data, not generic praise.
- If a player has multiple standout flags, mention them together: "Incredible damage AND vision control from [champion]!"
- Remember: Reference champions by name, NOT by team color. "Gwen dealt the most damage!" NOT "Gwen on RED team dealt the most damage!"

DEEP DATA ANALYSIS - LEVERAGE THE FULL DATA:
You have access to rich, detailed game statistics. Analyze beyond surface-level numbers to find meaningful insights:

1. ROLE-SPECIFIC ACHIEVEMENTS:
   - Look at each player's "tags" array (hard_carry, frontline_rock, vision_mvp, utility_mvp, objective_brain, weakside_warrior, heroic_in_loss).
   - Notice role-appropriate excellence: a support with high damage share is impressive! A tank dealing top damage is noteworthy!
   - Consider the "metrics" object: KP (kill participation), DamageShare, DamageTakenShare, CCPerMinute, VSPM (vision score per minute), DPM (damage per minute).

2. CONTEXTUAL PERFORMANCE:
   - High damage in a LOSS? That's a "heroic_in_loss" - acknowledge their kit mastery and individual skill despite the outcome! Focus on how impressive their mechanics were, not just the effort.
   - High vision score? That's "vision_mvp" - recognize the map control!
   - High damage taken with low deaths? That's "frontline_rock" or "mostTankingInGame" - appreciate the tanking!
   - High assists with healing/shielding? That's "utility_mvp" - celebrate the support play!
   - High crowd control (CC)? That's "mostCCInGame" - recognize the control and setup plays!
   - High damage mitigated? That's excellent defensive play - acknowledge the survivability!

3. RELATIVE METRICS (More Meaningful Than Absolute):
   - "damageShare" > 0.30 means they dealt 30%%+ of team damage - that's significant!
   - "KP" (kill participation) > 0.60 means they were involved in 60%%+ of team kills - great teamplay!
   - "DamageTakenShare" > 0.30 with low deaths means they soaked damage effectively - frontline excellence!
   - High "CCPerMinute" means they applied crowd control frequently - control mage/tank excellence!

4. EFFICIENCY METRICS:
   - High "DPM" (damage per minute) = consistent damage output throughout the game
   - High "CSPM" (CS per minute) = excellent farming and gold efficiency
   - High "VSPM" (vision per minute) = consistent map control and warding

5. TEAM DYNAMICS & SYNERGY - USE TEAM INSIGHTS DATA:
   The game summary includes "myTeamInsights" which provides sophisticated team-level analysis:
   
   - "averageKP": Average kill participation across your team (0.65+ = excellent coordination!)
   - "highKPCount": Number of players with KP > 0.60 (4+ = exceptional team coordination!)
   - "totalUtility": Total healing + shielding (30000+ = outstanding support play across the team!)
   - "totalVision": Total vision score (high = excellent map control!)
   - "wellRoundedPlayers": Players excelling in 2+ areas (3+ = well-rounded team!)
   - "synergyScore": Team coordination indicator (0.7+ = strong team synergy!)
   - "carryPerformance": "distributed" = damage spread across team, "focused" = one main carry
   - "teamComposition": "balanced", "damage-heavy", or "utility-heavy"
   
   USE THESE INSIGHTS TO CRAFT INTELLIGENT MESSAGES:
   - If "highKPCount" >= 4: Celebrate exceptional team coordination! "Amazing teamwork everyone!"
   - If "synergyScore" >= 0.7: Highlight strong team synergy! "Great coordination team!"
   - If "wellRoundedPlayers" >= 3: Acknowledge well-rounded team! "Everyone contributed in different ways!"
   - If "totalUtility" >= 30000: Recognize outstanding support play! "Amazing support from the team!"
   - If "carryPerformance" == "distributed": "Great damage distribution across the team!"
   - If "teamComposition" == "utility-heavy": "Outstanding utility and support play!"
   
   Also check "teamworkHighlight" field:
   - "exceptional_team_coordination": Highlight the exceptional coordination
   - "strong_team_synergy": Emphasize the team synergy
   - "well_rounded_team": Acknowledge diverse contributions
   - "outstanding_support_play": Celebrate support contributions
   - "high_team_participation": Recognize high involvement

6. ADVANCED METRICS - DEEPER INSIGHTS:
   Each player has advanced metrics in their "metrics" object:
   
   - "EfficiencyScore": Combined efficiency (damage + vision + utility) - higher = more well-rounded
   - "TeamplayScore": How much they enabled teammates (0.7+ = excellent teamplay!)
   - "SurvivabilityScore": Damage taken per death (higher = better tanking/survivability)
   - "ImpactScore": Overall impact on game (0.5+ = high impact player)
   - "GoldEfficiency": Damage per gold spent (higher = more efficient)
   - "UtilityScore": Healing + shielding + CC contribution (5+ = significant utility)
   - "WellRoundedScore": Balance across multiple categories (0.75+ = well-rounded player)
   
   USE THESE TO FIND HIDDEN GEMS:
   - High EfficiencyScore but not highest damage? They're a well-rounded contributor - mention it!
   - High TeamplayScore? They enabled teammates - celebrate the enabling!
   - High SurvivabilityScore with low deaths? Excellent tanking - recognize it!
   - High ImpactScore? They had significant impact - acknowledge it!
   - High GoldEfficiency? They made the most of their gold - recognize efficiency!
   - High WellRoundedScore? They excelled in multiple areas - celebrate versatility!

7. YOUR PERFORMANCE ANALYSIS:
   - Look at YOUR (%s) metrics specifically: What tags do you have? What's your damage share? Your KP? Your healing/shielding?
   - If you have "hard_carry" tag with high damage share, highlight that you carried!
   - If you have "utility_mvp" with high assists and healing, celebrate your support play!
   - If you have "vision_mvp", acknowledge your map control!
   - If you have "frontline_rock" with high damage taken share, recognize your tanking!

8. GAME STORY & CONTEXT - USE GAME INDICATORS:
   The game summary includes contextual indicators:
   
   - "wasStomp": One-sided game (true = dominant performance)
   - "wasClose": Very close game (true = nail-biter, acknowledge the competitiveness!)
   - "hadClutchMoments": Indicators of clutch plays (true = highlight clutch moments!)
   - "isIntenseMatch": Intense, competitive match (already handled above)
   - "isComeback": Comeback victory (already handled above)
   
   USE THESE TO TAILOR MESSAGES:
   - If "wasClose": "What a close game!" "That was intense!"
   - If "hadClutchMoments": "Some clutch plays there!" "Great clutch moments!"
   - If "wasStomp" and you won: "Dominant performance team!" (but stay humble)
   - If "wasStomp" and you lost: Acknowledge their dominance gracefully, but still find individual highlights - someone's kit execution, mechanics, or plays that stood out despite the loss

9. NUANCED OBSERVATIONS - RECOGNIZE DIVERSE CONTRIBUTIONS (THIS IS CRITICAL):
   League of Legends is a TEAM game with diverse roles and contributions. Not everyone needs to deal damage!
   
   DAMAGE DEALERS:
   - A player with high damage AND high vision = exceptional all-around performance
   - A player with high damage share but low KP = solo carry style (still valuable!)
   - High damage in a loss = impressive kit mastery and mechanics despite outcome - highlight their individual skill!
   
   TANKS/FRONTLINE:
   - A player with low deaths but high damage taken = excellent positioning and tanking (frontline excellence!)
   - High damage taken with low deaths AND high CC = perfect frontline tank (celebrate the tanking!)
   - High damage mitigated = excellent defensive itemization and positioning (acknowledge the survivability!)
   - If "mostTankingInGame" flag is true, recognize their frontline contribution!
   
   SUPPORTS/UTILITY:
   - High healing/shielding numbers = saved teammates' lives multiple times (mention impact!)
   - High assists with low deaths = excellent support/utility play (recognize the enabling!)
   - If "mostHealingShielding" or "mostHealingInGame" or "mostShieldingInGame" flags are true, celebrate their support play!
   
   CONTROL/CC:
   - High crowd control (CC) = enabled teamfights and picks (recognize the setup plays!)
   - If "mostCCInGame" or "mostCCOnTeam" flags are true, acknowledge their control contribution!
   
   VISION:
   - High vision score = map control and information (recognize the warding!)
   - If "highestVisionInGame" or "highestVisionOnTeam" flags are true, celebrate their vision control!
   
   CRITICAL: Look at the "tags" array and standout flags! They tell you exactly what each player excelled at.
   Remember: A support with high healing/shielding is just as valuable as a carry with high damage!
   Recognize ALL types of contributions, not just damage!

10. INTELLIGENT MESSAGE CRAFTING - PUT IT ALL TOGETHER:
    Combine multiple insights to create truly insightful messages:
    
    - Don't just say "good game" - find what made it good!
    - If team has high synergy score + well-rounded players: "Amazing coordination and everyone contributed!"
    - If multiple players have high KP: "Great teamwork everyone!"
    - If someone has high efficiency score + well-rounded score: "Incredible all-around performance from [champion]!"
    - If team has high utility + high vision: "Outstanding support and vision control!"
    - If game was close + had clutch moments: "What a close game with some clutch plays!"
    
    BE SPECIFIC AND INSIGHTFUL:
    - Instead of "good game": "Great coordination team!" or "Amazing synergy everyone!"
    - Instead of "nice damage": "Incredible damage distribution across the team!" or "Outstanding carry performance!"
    - Instead of "good support": "Outstanding utility and healing from [champion]!"
    - Instead of "good vision": "Excellent map control and vision from [champion]!"
    
    FIND THE STORY IN THE DATA:
    - Look at team insights to understand HOW the team won/lost
    - Use advanced metrics to find players who contributed in less obvious ways
    - Combine multiple indicators to paint a picture of team performance
    - Celebrate not just individual achievements, but team dynamics and synergy

Remember: The data tells a story. Look for patterns, relationships, and context. Don't just mention numbers - explain what they MEAN in the context of the game! Use the team insights and advanced metrics to craft messages that show you truly understand what happened in the game!`, 
				map[bool]string{true: "WON", false: "LOST"}[didWin],
				enemyTeam, myTeam,
				mySummonerName,
				mySummonerName)
		}
		
		// Check for intense match
		if isIntense, ok := gameSummary["isIntenseMatch"].(bool); ok && isIntense {
			intenseMatchContext = "\n\nIMPORTANT: This was an INTENSE MATCH! The game was close, competitive, and exciting. "
			
			if isComeback, ok := gameSummary["isComeback"].(bool); ok && isComeback {
				intenseMatchContext += "This was a COMEBACK VICTORY - your team pulled through despite challenges. "
			}
			
			if totalKills, ok := gameSummary["totalKills"].(float64); ok && totalKills >= 60 {
				intenseMatchContext += fmt.Sprintf("The match featured intense teamfights with %d total kills. ", int(totalKills))
			}
			
			if killDiff, ok := gameSummary["killDifference"].(float64); ok && killDiff <= 10 {
				intenseMatchContext += fmt.Sprintf("The final score was very close (only %d kill difference). ", int(killDiff))
			}
			
			if gameMinutes, ok := gameSummary["gameDurationMinutes"].(float64); ok && gameMinutes >= 35 {
				intenseMatchContext += fmt.Sprintf("The game lasted %.1f minutes, showing both teams fought hard until the end. ", gameMinutes)
			}
			
			intenseMatchContext += "Tailor your messages to reflect the intensity, competitiveness, and excitement of this match. Acknowledge the close nature of the game and celebrate the hard-fought victory (or graceful defeat)."
		}
		
		// Add game mode context to summary if available
		if gameMode != "" || queueType != "" {
			// Game mode context is already built above, but ensure it's included in instructions
		}
	}
	
	// Build tone-specific instructions
	toneInstructions := buildToneInstructions(llmSettings.Tone)
	
	// Build language style instructions
	languageInstructions := buildLanguageStyleInstructions(llmSettings.LanguageStyle)
	
	// Build focus area instructions
	focusInstructions := buildFocusAreaInstructions(llmSettings.FocusAreas)
	
	// Build AFK handling instructions
	afkInstructions := buildAFKHandlingInstructions(llmSettings.AFKHandling)
	
	// Build message format instructions
	messageCount := fmt.Sprintf("%d-%d", llmSettings.MinMessages, llmSettings.MaxMessages)
	messageLength := fmt.Sprintf("Under %d characters each", llmSettings.MaxMessageLength)
	
	instructions := `You are a League of Legends post-game sportsmanship coach. Your role is to generate positive, wholesome messages for post-game chat.

Core Rules:
- Never insult, blame, or criticize any player.
- No sarcasm, no passive-aggressive comments.
- Always maintain a positive, sportsmanlike attitude.
- Be context-aware: understand which team you're on, which team won, and tailor messages accordingly.

` + teamContext + `

` + gameModeContext + `

` + toneInstructions + `

` + languageInstructions + `

` + focusInstructions + `

AFK Handling:
` + afkInstructions + `

CRITICAL AFK RULES:
- ONLY mention AFKs if the game data explicitly shows "afk": true for any player OR "afkOnMyTeam": true OR "afkOnEnemyTeam": true.
- If NO players have "afk": true in the JSON, then there were NO AFKs - do NOT mention AFKs at all!
- Do NOT assume or infer AFKs based on low stats - only use what Riot's API explicitly reports.
- Check the "players" array: if no player has "afk": true, then there were NO AFKs in this game.

Message Format:
- Generate ` + messageCount + ` messages.
- ` + messageLength + `.
- 1-2 sentences per message.
- Casual gamer language allowed ("ggwp", "nice damage", etc.) but wholesome.
- No profanity, slurs, or toxicity.
- Each message must appear on its own line, no bullet points or numbering.

CRITICAL MESSAGING RULES:
- NEVER reference team colors (RED/BLUE) - they're meaningless labels.
- NEVER imply future games ("next game", "keep pushing", "keep it up", "see you next time", "gl next").
- NEVER say "team RED" or "team BLUE" - reference champions by name instead.
- Focus on THIS match only - acknowledge what happened, praise good plays, move on.
- These are random players for one match - treat them as such, not as a permanent team.
- Use champion names to identify players, not team colors.
- Keep messages brief and focused on the current game's performance.

` + intenseMatchContext + `

` + llmSettings.CustomInstructions + `

CRITICAL ACCURACY RULES - USE EXPLICIT DATA ONLY:
The game summary includes an "achievements" object with EXPLICIT data. Use this data directly - do NOT infer or calculate anything!

1. DAMAGE CLAIMS - USE EXPLICIT ACHIEVEMENTS DATA:
   - Check the "achievements" object at the root of the game summary JSON.
   - To say a player "dealt the most damage in the game", check "achievements.highestDamageInGame" - if it equals that champion's name, you can say it.
   - To say a player "dealt the most damage on their team", check "achievements.highestDamageOnMyTeam" - if it equals that champion's name, you can say it.
   - If "achievements.highestDamageInGame" is empty or doesn't match the champion name, do NOT say they dealt the most damage.
   - NEVER infer damage rankings from "totalDamage" numbers or player flags - ONLY use the explicit "achievements" object.
   - Example: If "achievements.highestDamageInGame": "Gwen", then ONLY Gwen dealt the most damage. Do NOT say any other champion dealt the most damage.

2. VISION CLAIMS - USE EXPLICIT ACHIEVEMENTS DATA:
   - Check "achievements.highestVisionInGame" - if it equals a champion's name, that champion had the highest vision.
   - Check "achievements.highestVisionOnMyTeam" - if it equals a champion's name, that champion had the highest vision on your team.
   - If these fields are empty or don't match the champion name, do NOT mention vision achievements.

3. HEALING/SHIELDING CLAIMS - USE EXPLICIT ACHIEVEMENTS DATA:
   - Check "achievements.mostHealingShielding" - if it equals a champion's name, that champion had the most healing/shielding.
   - If this field is empty or doesn't match the champion name, do NOT mention healing/shielding achievements.

4. CC (CROWD CONTROL) CLAIMS - USE EXPLICIT ACHIEVEMENTS DATA:
   - Check "achievements.mostCCInGame" - if it equals a champion's name, that champion had the most CC.
   - Check "achievements.mostCCOnMyTeam" - if it equals a champion's name, that champion had the most CC on your team.
   - If these fields are empty or don't match the champion name, do NOT mention CC achievements.

5. TANKING CLAIMS - USE EXPLICIT ACHIEVEMENTS DATA:
   - Check "achievements.mostTankingInGame" - if it equals a champion's name, that champion had the most tanking.
   - If this field is empty or doesn't match the champion name, do NOT mention tanking achievements.

6. STAT NUMBERS - DO NOT USE SPECIFIC NUMBERS OR TECHNICAL TERMS:
   - Do NOT mention specific numbers like "23 assists", "40 assists", "39 assists", etc.
   - Do NOT use technical terms like "VSPM", "DPM", "CSPM", "KP", "damage share", etc.
   - The JSON may contain these numbers, but mentioning them makes messages feel robotic and unnatural.
   - Instead use natural language: "great assists", "lots of assists", "amazing assists", "excellent vision control", "consistent damage".
   - Exception: You can mention K/D/A if it's exceptional (like "perfect KDA" for 0 deaths).

7. TAGS - ONLY USE EXPLICIT TAGS:
   - Only mention tags that are explicitly in the "tags" array for that player.
   - Do NOT infer tags from stats or metrics.

8. VERIFICATION PROCESS - USE ACHIEVEMENTS OBJECT FIRST:
   - ALWAYS check the "achievements" object at the root of the JSON FIRST before making any claims.
   - The "achievements" object contains EXPLICIT data: "highestDamageInGame": "Gwen" means ONLY Gwen dealt the most damage.
   - If "achievements.highestDamageInGame" is empty or doesn't match a champion name, NO ONE dealt the most damage - don't claim it!
   - When in doubt, use generic positive messages: "Great game!", "Nice plays!", "Well played!"
   - It's better to be generic than to make false claims.

9. EXAMPLES OF CORRECT vs INCORRECT USING ACHIEVEMENTS:
   - CORRECT: If "achievements.highestDamageInGame": "Gwen" → "Gwen dealt the most damage in the game!"
   - INCORRECT: If "achievements.highestDamageInGame": "Gwen" but you mention Fizz → Do NOT say "Fizz dealt the most damage"
   - CORRECT: If "achievements.highestDamageInGame": "" (empty) → Do NOT say anyone dealt the most damage
   - CORRECT: If "achievements.mostHealingShielding": "Yuumi" → "Yuumi's healing and shielding saved the day!"
   - INCORRECT: If "achievements.mostHealingShielding": "Yuumi" but you mention Senna → Do NOT say "Senna had the most healing"
   - CRITICAL: The "achievements" object is the SOURCE OF TRUTH. If a champion name is NOT in the achievements object for a category, they did NOT achieve that!

CRITICAL OUTPUT FORMAT - THIS IS THE MOST IMPORTANT PART:
You MUST output ONLY the raw message text, one message per line. 

DO NOT include:
- ANY introductory text (no "Here are...", "Okay, here are...", "Based on...", "Here are 2-3...", etc.)
- ANY meta-commentary or explanations  
- ANY labels like "Message 1:", "Message 2:", etc.
- Phrases like "one message per line", "based on the game data", "positive post-game chat messages"
- Quotation marks around messages (no ", ", ', or smart quotes)
- Numbering or bullet points
- ANY text that isn't the actual message itself

CORRECT FORMAT EXAMPLE (copy this style exactly):
Garen dealt the most damage in the game!
Great healing from Senna!
Well played everyone, ggwp!

INCORRECT FORMAT (DO NOT DO THIS - these will be filtered out):
Okay, here are 2-3 short, positive post-game chat messages based on the game summary above:
Message 1: "Garen dealt the most damage!"
Message 2: "Great healing from Senna!"

CRITICAL: START OUTPUTTING MESSAGES IMMEDIATELY. Do NOT write any introductory text. Do NOT repeat instructions. Do NOT say "Okay, here are..." or "Let's analyze..." Just write the messages, one per line:

Generate ` + messageCount + ` short, positive post-game chat messages based on the game summary below:

Game summary JSON:
` + gameSummaryJSON

	return instructions
}

func buildToneInstructions(tone string) string {
	switch tone {
	case "professional":
		return `Tone: Professional and respectful. Use formal language while remaining warm and encouraging.`
	case "enthusiastic":
		return `Tone: Enthusiastic and energetic. Show excitement and celebrate good plays. Use exclamation marks sparingly but effectively.`
	case "humble":
		return `Tone: Humble and modest. Focus on team effort and avoid highlighting individual achievements excessively.`
	case "supportive":
		return `Tone: Supportive and encouraging. Emphasize growth, effort, and positive reinforcement.`
	default: // "friendly"
		return `Tone: Friendly and casual. Be warm, approachable, and use natural conversational language.`
	}
}

func buildLanguageStyleInstructions(style string) string {
	switch style {
	case "formal":
		return `Language Style: Use more formal language. Avoid slang and casual expressions.`
	case "enthusiastic":
		return `Language Style: Use energetic and expressive language. Show excitement appropriately.`
	case "gamer":
		return `Language Style: Use authentic gamer chat style! Examples:
- "njnj" (nice nice)
- "clutch heals securing the win"
- "Yuumi kept me alive and I was sure I was dead 4 times omg"
- "wp" (well played)
- "gg" (good game)
- "ty" (thank you)
- "mb" (my bad)
- "gj" (good job)
- "pog" (play of the game)
- "insane" / "cracked" / "nuts"
- Use lowercase, abbreviations, and authentic gaming expressions
- Be enthusiastic but casual - like you're typing in post-game chat
- Use "omg", "lol", "haha" naturally
- Keep it real and relatable to actual League chat`
	default: // "casual"
		return `Language Style: Use casual gamer language naturally. "ggwp", "nice damage", etc. are appropriate.`
	}
}

func buildFocusAreaInstructions(focusAreas []string) string {
	if len(focusAreas) == 0 || (len(focusAreas) == 1 && focusAreas[0] == "all") {
		return `What to Highlight: Mention all positive aspects - damage, vision, KDA, CS, teamplay, etc.`
	}
	
	highlights := []string{}
	for _, area := range focusAreas {
		switch area {
		case "positive":
			highlights = append(highlights, "only positive achievements")
		case "kda":
			highlights = append(highlights, "KDA and kill participation")
		case "teamplay":
			highlights = append(highlights, "teamwork and assists")
		case "vision":
			highlights = append(highlights, "vision control and support play")
		}
	}
	
	if len(highlights) == 0 {
		return `What to Highlight: Mention all positive aspects - damage, vision, KDA, CS, teamplay, etc.`
	}
	
	return `What to Highlight: Focus on ` + strings.Join(highlights, ", ") + `.`
}

func buildAFKHandlingInstructions(handling string) string {
	switch handling {
	case "empathetic":
		return `- If AfkOnMyTeam is true AND you WON, show extra empathy. Acknowledge the difficulty of playing 4v5. Praise teammates extensively. Include sincere apologies to the ENEMY team (not your own team) for the unfair advantage.
- If AfkOnMyTeam is true AND you LOST, do NOT apologize - you have nothing to apologize for! Simply acknowledge your teammates' efforts despite the disadvantage.
- If AfkOnEnemyTeam is true, express genuine sympathy. Avoid any hint of gloating. Acknowledge it was an unfair match.`
	case "neutral":
		return `- If AfkOnMyTeam is true AND you WON, do not mention the AFK player. Focus on praising teammates. Optionally apologize to the ENEMY team for the unfair advantage.
- If AfkOnMyTeam is true AND you LOST, do NOT apologize - you have nothing to apologize for!
- If AfkOnEnemyTeam is true, acknowledge it neutrally without excessive sympathy or gloating.`
	default: // "default"
		return `- If AfkOnMyTeam is true AND you WON, do not mention AFK player(s). Instead:
  - Praise the remaining four teammates.
  - Include at least one apology to the ENEMY team (not your own team) for the unfair advantage.
- If AfkOnMyTeam is true AND you LOST, do NOT apologize - you have nothing to apologize for! Simply praise your teammates' efforts.
- If AfkOnEnemyTeam is true, avoid gloating and show empathy.`
	}
}

func (c *Client) Generate(prompt string, gameSummaryJSON string, enableDebug bool) ([]string, error) {
	reqBody := GenerateRequest{
		Model:       c.Model,
		Prompt:      prompt,
		Stream:      false,
		Temperature: c.Config.Temperature,
	}
	
	// Add max tokens if configured
	if c.Config.MaxTokens > 0 {
		reqBody.NumPredict = c.Config.MaxTokens
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use shared HTTP client for connection pooling and parallel requests
	if c.httpClient == nil {
		c.httpClient = SharedHTTPClient
	}

	resp, err := c.httpClient.Post(c.URL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var genResp GenerateResponse
	if err := json.Unmarshal(body, &genResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	messages := parseMessages(genResp.Response, c.Config.MaxMessageLength)
	
	if enableDebug {
		log.Printf("[DEBUG] Raw LLM response length: %d chars", len(genResp.Response))
		log.Printf("[DEBUG] Raw LLM response:\n%s", genResp.Response)
		log.Printf("[DEBUG] Parsed %d messages before validation", len(messages))
		for i, msg := range messages {
			log.Printf("[DEBUG]   Message %d: %s", i+1, msg)
		}
	}
	
	// Validate messages against game data to catch incorrect claims
	messages = validateMessages(messages, gameSummaryJSON, enableDebug)
	
	if enableDebug {
		log.Printf("[DEBUG] After validation: %d messages", len(messages))
		for i, msg := range messages {
			log.Printf("[DEBUG]   Validated message %d: %s", i+1, msg)
		}
	}
	
	if len(messages) == 0 {
		if enableDebug {
			log.Printf("[DEBUG] No valid messages after validation, using fallback")
		}
		// Fallback messages - generate context-aware fallbacks if possible
		return generateContextualFallbackMessages(gameSummaryJSON), nil
	}

	// Limit to configured max messages
	if len(messages) > c.Config.MaxMessages {
		messages = messages[:c.Config.MaxMessages]
	}
	// Ensure we have at least min messages (pad with contextual fallback if needed)
	if len(messages) < c.Config.MinMessages {
		fallback := generateContextualFallbackMessages(gameSummaryJSON)
		for len(messages) < c.Config.MinMessages && len(fallback) > 0 {
			messages = append(messages, fallback[0])
			fallback = fallback[1:]
		}
	}

	return messages, nil
}

func parseMessages(response string, maxLength int) []string {
	if maxLength == 0 {
		maxLength = 200 // Default buffer
	}
	maxLengthWithBuffer := maxLength + 50 // Allow some buffer
	
	lines := strings.Split(response, "\n")
	messages := make([]string, 0, len(lines))
	

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Remove bullet points and numbering
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimPrefix(line, "• ")
		// Remove numbered prefixes (1., 2., etc.)
		for i := 1; i <= 20; i++ {
			prefix := fmt.Sprintf("%d. ", i)
			line = strings.TrimPrefix(line, prefix)
		}
		line = strings.TrimSpace(line)
		
		// Remove quotation marks (both " and ") and smart quotes
		line = strings.Trim(line, `"`)
		line = strings.Trim(line, `"`)
		line = strings.Trim(line, `'`)
		line = strings.Trim(line, `'`)
		// Remove smart quotes (Unicode left/right double quotation marks and apostrophes)
		// U+201C = ", U+201D = ", U+2018 = ', U+2019 = '
		line = strings.ReplaceAll(line, "\u201C", ``)  // Left double quotation mark
		line = strings.ReplaceAll(line, "\u201D", ``)  // Right double quotation mark
		line = strings.ReplaceAll(line, "\u2018", `'`) // Left single quotation mark -> regular apostrophe
		line = strings.ReplaceAll(line, "\u2019", `'`) // Right single quotation mark -> regular apostrophe
		line = strings.ReplaceAll(line, "\u201A", `'`) // Single low-9 quotation mark
		line = strings.ReplaceAll(line, "\u201B", `'`) // Single high-reversed-9 quotation mark
		line = strings.ReplaceAll(line, "\u201E", `"`) // Double low-9 quotation mark
		line = strings.ReplaceAll(line, "\u201F", `"`) // Double high-reversed-9 quotation mark
		// Fix em dashes and en dashes (U+2013 = en dash, U+2014 = em dash)
		line = strings.ReplaceAll(line, "\u2013", `-`) // En dash -> regular dash
		line = strings.ReplaceAll(line, "\u2014", `-`) // Em dash -> regular dash
		line = strings.ReplaceAll(line, "\u2015", `-`) // Horizontal bar -> regular dash
		// Also handle the character sequences that appear as ΓÇÖ (which is actually U+2019)
		line = strings.ReplaceAll(line, "ΓÇÖ", `'`)
		line = strings.ReplaceAll(line, "ΓÇ£", ``)
		line = strings.ReplaceAll(line, "ΓÇ¥", ``)
		line = strings.TrimSpace(line)
		
		// Skip lines that are too long
		if len(line) > maxLengthWithBuffer {
			continue
		}
		
		// Skip lines that look like meta-commentary or explanations
		// Be more specific to avoid filtering valid messages that contain these words
		lowerLine := strings.ToLower(line)
		// Skip lines that look like meta-commentary or explanations
		shouldSkip := false
		
		// Always skip patterns (regardless of length)
		alwaysSkipPatterns := []string{
			"okay, here are",
			"okay, here's",
			"okay, let's",
			"let's recap",
			"let's analyze",
			"here are 2-3",
			"here are a couple",
			"here are two",
			"here are a few",
			"here are some",
			"one message per line",
			"based on the game data",
			"based on the game summary",
			"based on the game summary above",
			"based on this",
			"and a shoutout",
			"you are a league",
			"your role is to",
			"be context-aware",
			"here's a quick",
			"here's a",
			"quick post-game analysis",
			"post-game analysis",
		}
		for _, pattern := range alwaysSkipPatterns {
			if strings.Contains(lowerLine, pattern) {
				shouldSkip = true
				break
			}
		}
		
		// Skip other patterns if line is short (likely meta-commentary)
		if !shouldSkip {
			skipPatterns := []string{
				"here are the messages",
				"based on the json",
				"based on the provided",
				"based on the data",
				"game summary json",
				"json data shows",
				"messages:",
				"message 1:",
				"message 2:",
				"message 3:",
				"generate messages",
				"output messages",
				"following messages",
				"below are the messages",
				"above are the messages",
				"these are the messages",
				"the messages are",
				"post-game chat messages",
				"designed for a",
				"positive post-game",
				"short, positive",
				"seriously, those",
			}
			for _, pattern := range skipPatterns {
				if strings.Contains(lowerLine, pattern) && len(line) < 80 {
					shouldSkip = true
					break
				}
			}
		}
		if shouldSkip {
			continue
		}
		
		// Skip lines that are just punctuation or very short (likely formatting artifacts)
		// But allow short valid messages like "ggwp"
		if len(line) < 4 {
			continue
		}
		
		messages = append(messages, line)
	}

	return messages
}

// validateMessages checks if LLM claims match actual game data flags
func validateMessages(messages []string, gameSummaryJSON string, enableDebug bool) []string {
	var gameSummary map[string]interface{}
	if err := json.Unmarshal([]byte(gameSummaryJSON), &gameSummary); err != nil {
		return messages // If we can't parse, return as-is
	}
	
	playersRaw, ok := gameSummary["players"].([]interface{})
	if !ok {
		return messages
	}
	
	// Build a map of champion -> flags for quick lookup
	championFlags := make(map[string]map[string]bool)
	validChampions := make(map[string]bool) // Track which champions actually exist in the game
	for _, pRaw := range playersRaw {
		if pMap, ok := pRaw.(map[string]interface{}); ok {
			champion, _ := pMap["champion"].(string)
			if champion != "" {
				validChampions[strings.ToLower(champion)] = true
			}
			flags := make(map[string]bool)
			flags["highestDamageInGame"] = false
			flags["highestDamageOnTeam"] = false
			flags["mostHealingShielding"] = false
			flags["highestVisionInGame"] = false
			flags["highestVisionOnTeam"] = false
			
			if v, ok := pMap["highestDamageInGame"].(bool); ok {
				flags["highestDamageInGame"] = v
			}
			if v, ok := pMap["highestDamageOnTeam"].(bool); ok {
				flags["highestDamageOnTeam"] = v
			}
			if v, ok := pMap["mostHealingShielding"].(bool); ok {
				flags["mostHealingShielding"] = v
			}
			if v, ok := pMap["highestVisionInGame"].(bool); ok {
				flags["highestVisionInGame"] = v
			}
			if v, ok := pMap["highestVisionOnTeam"].(bool); ok {
				flags["highestVisionOnTeam"] = v
			}
			
			championFlags[champion] = flags
		}
	}
	
	validated := make([]string, 0, len(messages))
	for _, msg := range messages {
		lowerMsg := strings.ToLower(msg)
		
		// First, check if message mentions any champions that don't exist in the game
		hasInvalidChampion := false
		for champion := range championFlags {
			championLower := strings.ToLower(champion)
			// Check if message mentions this champion
			if strings.Contains(lowerMsg, championLower) {
				// Champion exists, continue to other checks
				continue
			}
		}
		// Check for common champion names that might be hallucinated
		commonChampions := []string{"gwen", "yasuo", "zed", "akali", "katarina", "riven", "fiora", "irelia", "jax", "tryndamere"}
		for _, champ := range commonChampions {
			if strings.Contains(lowerMsg, champ) && !validChampions[champ] {
				if enableDebug {
					log.Printf("[DEBUG] VALIDATION: Filtering message mentioning invalid champion %s (not in game)", champ)
				}
				hasInvalidChampion = true
				break
			}
		}
		
	// Check for incorrect damage and vision claims
	hasIncorrectDamageClaim := false
	hasIncorrectVisionClaim := false
	hasIncorrectHealingClaim := false
	for champion, flags := range championFlags {
		championLower := strings.ToLower(champion)
		// Check if message claims this champion dealt most damage IN THE GAME (strict check)
		if strings.Contains(lowerMsg, championLower) {
			// Check for "most damage in the game" or "most damage" (which implies in-game)
			claimsMostDamageInGame := strings.Contains(lowerMsg, "most damage in the game") || 
			                          strings.Contains(lowerMsg, "most damage") ||
			                          strings.Contains(lowerMsg, "highest damage") ||
			                          strings.Contains(lowerMsg, "dealt the most")
			claimsMostDamageOnTeam := strings.Contains(lowerMsg, "most damage on") || 
			                          strings.Contains(lowerMsg, "most damage on their team")
			
			if claimsMostDamageInGame && !flags["highestDamageInGame"] {
				// Incorrect claim - this champion doesn't have highestDamageInGame flag
				if enableDebug {
					log.Printf("[DEBUG] VALIDATION: Filtering incorrect damage claim - %s claimed 'most damage in the game' but highestDamageInGame=%v", champion, flags["highestDamageInGame"])
				}
				hasIncorrectDamageClaim = true
				break
			}
			if claimsMostDamageOnTeam && !flags["highestDamageOnTeam"] {
				// Incorrect claim - this champion doesn't have highestDamageOnTeam flag
				if enableDebug {
					log.Printf("[DEBUG] VALIDATION: Filtering incorrect damage claim - %s claimed 'most damage on team' but highestDamageOnTeam=%v", champion, flags["highestDamageOnTeam"])
				}
				hasIncorrectDamageClaim = true
				break
			}
			// Also check for "carried" with "damage" context
			if strings.Contains(lowerMsg, "carried") && strings.Contains(lowerMsg, "damage") && !flags["highestDamageInGame"] && !flags["highestDamageOnTeam"] {
				if enableDebug {
					log.Printf("[DEBUG] VALIDATION: Filtering incorrect carry claim - %s claimed to carry with damage but no damage flags", champion)
				}
				hasIncorrectDamageClaim = true
				break
			}
			
			// Check for healing/shielding claims
			if (strings.Contains(lowerMsg, "healing") || strings.Contains(lowerMsg, "shielding") || strings.Contains(lowerMsg, "heal")) && 
			   (strings.Contains(lowerMsg, "most") || strings.Contains(lowerMsg, "amazing") || strings.Contains(lowerMsg, "best")) {
				if !flags["mostHealingShielding"] {
					if enableDebug {
						log.Printf("[DEBUG] VALIDATION: Filtering incorrect healing/shielding claim about %s", champion)
					}
					hasIncorrectHealingClaim = true
					break
				}
			}
			
			// Check for CC claims
			if (strings.Contains(lowerMsg, "crowd control") || strings.Contains(lowerMsg, "cc") || strings.Contains(lowerMsg, "control")) &&
			   (strings.Contains(lowerMsg, "most") || strings.Contains(lowerMsg, "amazing") || strings.Contains(lowerMsg, "best")) {
				// We'd need to add CC flags to the validation map for this to work fully
				// For now, just log if we see potential issues
			}
		}
		// Check if message claims this champion had best vision
		if strings.Contains(lowerMsg, championLower) && 
		   (strings.Contains(lowerMsg, "vision control") || strings.Contains(lowerMsg, "best vision") || strings.Contains(lowerMsg, "dominated vision") || strings.Contains(lowerMsg, "vision mvp") || strings.Contains(lowerMsg, "amazing vision")) {
			if !flags["highestVisionInGame"] && !flags["highestVisionOnTeam"] {
				// Incorrect claim - this champion doesn't have highest vision flags
				if enableDebug {
					log.Printf("[DEBUG] VALIDATION: Filtering incorrect vision claim about %s (flags: highestVisionInGame=%v, highestVisionOnTeam=%v)", champion, flags["highestVisionInGame"], flags["highestVisionOnTeam"])
				}
				hasIncorrectVisionClaim = true
				break
			}
		}
	}
		
		// Check for team color references (BLUE team, RED team, etc.)
		hasTeamColorReference := strings.Contains(lowerMsg, "blue team") || 
		                        strings.Contains(lowerMsg, "red team") ||
		                        strings.Contains(lowerMsg, "team blue") ||
		                        strings.Contains(lowerMsg, "team red")
		
		if enableDebug && (hasInvalidChampion || hasIncorrectDamageClaim || hasIncorrectVisionClaim || hasIncorrectHealingClaim || hasTeamColorReference) {
			log.Printf("[DEBUG] VALIDATION: Filtering message - invalidChamp=%v, damageClaim=%v, visionClaim=%v, healingClaim=%v, teamColor=%v: %s", hasInvalidChampion, hasIncorrectDamageClaim, hasIncorrectVisionClaim, hasIncorrectHealingClaim, hasTeamColorReference, msg)
		}
		
		if !hasInvalidChampion && !hasIncorrectDamageClaim && !hasIncorrectVisionClaim && !hasIncorrectHealingClaim && !hasTeamColorReference {
			validated = append(validated, msg)
		} else if hasTeamColorReference {
			// Try to fix by removing team color references
			fixed := strings.ReplaceAll(msg, "BLUE team", "team")
			fixed = strings.ReplaceAll(fixed, "RED team", "team")
			fixed = strings.ReplaceAll(fixed, "team BLUE", "team")
			fixed = strings.ReplaceAll(fixed, "team RED", "team")
			fixed = strings.ReplaceAll(fixed, "blue team", "team")
			fixed = strings.ReplaceAll(fixed, "red team", "team")
			fixed = strings.ReplaceAll(fixed, "team blue", "team")
			fixed = strings.ReplaceAll(fixed, "team red", "team")
			// If we removed team references completely, just remove "team" if it's standalone
			fixed = strings.ReplaceAll(fixed, " from the team", "")
			fixed = strings.ReplaceAll(fixed, " from team", "")
			fixed = strings.TrimSpace(fixed)
			if len(fixed) > 10 { // Only add if it's still a meaningful message
				validated = append(validated, fixed)
			}
		}
	}
	
	return validated
}

// generateContextualFallbackMessages creates fallback messages based on game context
// This provides better fallbacks than generic messages when LLM fails
func generateContextualFallbackMessages(gameSummaryJSON string) []string {
	var gameSummary map[string]interface{}
	if err := json.Unmarshal([]byte(gameSummaryJSON), &gameSummary); err != nil {
		// If we can't parse, use generic fallbacks
		return []string{
			"ggwp everyone, thanks for the game!",
			"Nice effort team, gl in your next games!",
		}
	}
	
	messages := []string{}
	
	// Check for AFK situations
	afkOnMyTeam, _ := gameSummary["afkOnMyTeam"].(bool)
	afkOnEnemyTeam, _ := gameSummary["afkOnEnemyTeam"].(bool)
	winningTeam, _ := gameSummary["winningTeam"].(string)
	myTeam, _ := gameSummary["myTeam"].(string)
	didWin := winningTeam == myTeam
	
	if afkOnMyTeam && didWin {
		messages = append(messages, "Great job team, that was tough playing 4v5!")
		messages = append(messages, "Sorry for the AFK, opponents - you played well!")
	} else if afkOnMyTeam && !didWin {
		messages = append(messages, "Good effort team despite the disadvantage!")
	} else if afkOnEnemyTeam {
		messages = append(messages, "Sorry for the scuffed game, gl next everyone!")
	} else {
		if didWin {
			messages = append(messages, "ggwp everyone, thanks for the game!")
		} else {
			// More casual tone for losses
			messages = append(messages, "Awe, dang. Sucks to lose, but ggwp everyone!")
		}
	}
	
	// Add a second message if we don't have enough
	if len(messages) < 2 {
		if didWin {
			messages = append(messages, "Well played everyone!")
		} else {
			// Casual, supportive tone for losses focusing on individual highlights
			messages = append(messages, "Rough one, but some great plays out there!")
		}
	}
	
	return messages
}

