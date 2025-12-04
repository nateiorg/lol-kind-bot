# Project Overview

**Working name:** `lol-kind-bot`  
**Goal:**  
A background application that runs on the user's Windows PC, connects to the League of Legends client, listens for the end-of-game event, analyzes post-game stats (including AFKs/leavers), and generates short, positive, context-aware chat messages the player can copy into the post-game chat.

**Key points:**

- Runs in the background with a **system tray icon**.
- Written in **Go**.
- Uses a **local LLM** (via a local HTTP API such as Ollama) leveraging the user's **RTX 5090**.
- Provides a **UI for changing settings** (e.g., configuration window launched from tray menu).
- Does **not** automate in-game UI interactions; it only reads local APIs and presents messages to the user.

