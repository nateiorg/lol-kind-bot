# LLM Integration Requirements

## Prompt Construction

The app must send a structured prompt to the LLM including:

1. **System-style instruction text** (embedded in prompt):

   - The LLM acts as a **League of Legends post-game sportsmanship coach**.
   - Rules:
     - Never insult, blame, or criticize any player.
     - No sarcasm, no passive-aggressive comments.
     - Highlight what players did well (damage, vision, KDA, CS, etc.).
     - AFK rules:
       - If `AfkOnMyTeam == true`, do not mention AFK player(s). Instead:
         - Praise the remaining four teammates.
         - Include at least one apology to the enemy team.
       - If `AfkOnEnemyTeam == true`, avoid gloating and show empathy.
     - Messages:
       - 1–2 sentences each.
       - Under 150 characters each.
       - Casual gamer language allowed ("ggwp", "nice damage", etc.) but wholesome.
       - No profanity, slurs, or toxicity.
       - Each message must appear on its **own line**, no bullet points or numbering.

2. **Game summary JSON**
   - Append serialized `gameSummary` as a JSON block after the instructions.
   - Prompt example shape:

     - Instructions text.
     - `Game summary JSON:\n<json>`

3. **Response format expectation**
   - The LLM should output:
     - 2–3 chat messages.
     - Each message as one line of text (no bullets, no numbering).
   - Post-processing:
     - Split response on newline.
     - Trim whitespace and remove leading `- `, `* `, etc.
     - Filter out empty lines.

## LLM HTTP API Call

1. **Endpoint (example with Ollama):**
   - `POST http://localhost:11434/api/generate`

2. **Request body:**
   - JSON with fields:
     - `model` (string): configured model name.
     - `prompt` (string): the full prompt text (instructions + JSON).
     - `stream` (bool): `false` for v1.

3. **Response handling:**
   - Parse the JSON response body.
   - Extract the generated content (e.g., `response` field).
   - Derive messages as described.

4. **Error handling:**
   - If HTTP call fails or returns non-2xx:
     - Log error with reason.
     - Fall back to one or more basic template messages such as:
       - "ggwp everyone, thanks for the game!"
       - "Nice effort team, gl in your next games!"

