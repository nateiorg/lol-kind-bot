# Non-Functional Requirements

1. **Performance**
   - Time from `EndOfGame` detection to quip availability: ideally **≤ 1–2 seconds**.
   - Polling overhead must be minimal and not impact system performance.

2. **Stability**
   - Recover gracefully from:
     - League client restarts.
     - LCU unavailability.
     - LLM server unavailability.

3. **Safety & Tone**
   - Generated output must be:
     - Non-toxic.
     - Non-discriminatory.
     - Focused on sportsmanship and positivity.

4. **Extensibility**
   - Designed to allow future enhancements:
     - Additional tagging types (objective control, role-specific nuances).
     - Additional LLM providers or remote hosted options.
     - Localization (multi-language support).

