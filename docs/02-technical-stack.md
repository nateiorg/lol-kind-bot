# Technical Stack

## Language & Platform

- Primary implementation language: **Go**
- Minimum Go version: **1.22+** (or latest stable).
- Target OS: **Windows 10/11**.
- Distribution format: single executable (`lol-kind-bot.exe`), plus optional config file(s).

## External Components

1. **League of Legends Client (LCU) API**
   - Uses the local League client's **lockfile** to obtain:
     - Protocol (HTTP/HTTPS)
     - Port
     - Auth password
   - Uses authenticated HTTPS against the LCU:
     - Base URL: `{protocol}://127.0.0.1:{port}`.
     - Basic auth: `"riot:<password>"` (Base64 encoded).

2. **LLM Runtime (local)**
   - Example: **Ollama** or similar local LLM server.
   - Provides an HTTP endpoint (e.g., `http://localhost:11434/api/generate`).
   - Configurable model name (e.g., `llama3.1`, `qwen-7b`, etc.).
   - Non-streaming responses are acceptable for v1.

