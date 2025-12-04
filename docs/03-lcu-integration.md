# LCU Connection & Game State Monitoring

The application must:

1. **Locate the lockfile**
   - Default path: `C:\Riot Games\League of Legends\lockfile`.
   - Allow override through environment variable: `LOL_LOCKFILE_PATH`.
   - Poll periodically (e.g., every 3 seconds) until the lockfile exists.
   - Handle League client being opened/closed at any time.

2. **Parse the lockfile**
   - Lockfile format: `processName:PID:port:password:protocol`.
   - Extract:
     - `port` (int)
     - `password` (string)
     - `protocol` (string; e.g., `https`)

3. **Construct LCU Client**
   - Base URL: `{protocol}://127.0.0.1:{port}`.
   - Use HTTP client:
     - TLS: trust self-signed certs (local only).
     - Authorization: `Basic` header with `riot:<password>` (Base64 encoded).
   - Provide a helper `GET` function for endpoints.

4. **Poll gameflow phase**
   - Endpoint: `/lol-gameflow/v1/gameflow-phase`.
   - Poll frequency: ~3 seconds (configurable).
   - Track current and last phase.
   - Detect transition into `EndOfGame` phase.
   - Debounce: Only trigger EoG processing once within a configurable cooldown (e.g., 30 seconds).

5. **Connection Error Handling**
   - On request failure (client down, network issue, etc.):
     - Log error.
     - Attempt to re-read lockfile and recreate client.
     - Continue polling until League is available again.

