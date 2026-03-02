# AI Fitness Trainer

Personal AI fitness coach that runs as a Telegram bot, powered by [Claude Code](https://code.claude.com/) (Max subscription). It integrates with [Intervals.icu](https://intervals.icu) for training data, Google Calendar for scheduling, and web search for fitness research.

Built for triathletes (swim/bike/run) and weight lifters.

## Features

- **Data-driven coaching** — queries your Intervals.icu data (activities, training load, wellness, power curves) before giving advice
- **Calendar-aware** — checks Google Calendar to schedule workouts around your life
- **Morning briefing** — configurable daily cron that sends your training plan, checks wellness, and asks how you're feeling
- **Self-improving** — give feedback ("I'm vegetarian", "I have a knee injury") and the coach remembers it permanently by editing its own instructions
- **Conversation memory** — sessions persist across container restarts via SQLite
- **Single-user** — whitelist-based access, responds only to your Telegram account

## Architecture

```
Telegram --> aiogram bot (Python) --> claude -p subprocess --> Anthropic API
                                          |
                        +-----------------+-----------------+
                        |                 |                 |
                  intervals-mcp    google-cal MCP      WebSearch
                        |                 |                 |
                  Intervals.icu    Google Calendar        Web
```

All components run inside a single Docker container. Claude Code CLI authenticates via OAuth token (Max subscription — no API key billing).

## Prerequisites

- Docker and Docker Compose
- A [Claude Max subscription](https://claude.ai) (for Claude Code access)
- A Telegram account
- An [Intervals.icu](https://intervals.icu) account with training data

## Setup

### 1. Get your Claude Code OAuth token

This token lets Claude Code run headless inside Docker using your Max subscription.

```bash
# Install Claude Code if you haven't already
curl -fsSL https://claude.ai/install.sh | bash

# Generate a long-lived OAuth token for headless use
claude setup-token
```

Copy the token (`sk-ant-oat01-...`) for use in the `.env` file.

### 2. Create a Telegram bot

1. Open Telegram and message [@BotFather](https://t.me/BotFather)
2. Send `/newbot` and follow the prompts (pick a name and a username ending in `bot`)
3. Copy the bot token (format: `123456789:AAH...`)

Then get your numeric user ID:

1. Message [@userinfobot](https://t.me/userinfobot) on Telegram
2. It replies with your numeric ID (e.g. `7064223637`)

### 3. Get Intervals.icu credentials

1. Log in to [intervals.icu](https://intervals.icu)
2. Go to **Settings** (gear icon)
3. Scroll to the bottom — **Developer Settings** section
4. Copy your **API Key**
5. Your **Athlete ID** is shown there too (starts with `i`, e.g. `i518868`). It's also visible in the URL when you view your profile.

The bot uses [intervals-mcp](https://github.com/derrix060/intervals-mcp), a Go-based MCP server that exposes 144 tools for the full Intervals.icu API. It's built from source during `docker build`.

### 4. Google Calendar setup (optional)

<details>
<summary>Click to expand Google Calendar setup instructions</summary>

#### Create a Google Cloud project

1. Go to [console.cloud.google.com](https://console.cloud.google.com)
2. Create a **New Project** (e.g. "Calendar MCP")
3. Go to **APIs & Services** > **Library**, search for "Google Calendar API", and **Enable** it

#### Configure OAuth consent screen

1. Go to **APIs & Services** > **OAuth consent screen**
2. Select **External**, click **Create**
3. Fill in the app name and your email
4. Add scope: `https://www.googleapis.com/auth/calendar.events`
5. Add your Google email as a test user

#### Create OAuth credentials

1. Go to **APIs & Services** > **Credentials**
2. Click **Create Credentials** > **OAuth client ID**
3. Select **Desktop app** as the application type
4. Click **Create**, then **Download JSON**
5. Save the file to `./config/gcp-oauth.keys.json`

On first run, the MCP server opens a browser for OAuth consent (one-time). Run it locally first if using Docker:

```bash
npx -y @cocal/google-calendar-mcp
```

Then mount the resulting token into the container.

</details>

### 5. Configure environment

```bash
cp .env.example .env
```

Edit `.env` with your values:

```bash
# Required
TELEGRAM_BOT_TOKEN=123456:ABC-DEF...
ALLOWED_TELEGRAM_USER_IDS=7064223637
CLAUDE_CODE_OAUTH_TOKEN=sk-ant-oat01-...
INTERVALS_API_KEY=your-api-key
INTERVALS_ATHLETE_ID=i12345
TZ=Europe/Lisbon

# Optional
CLAUDE_MODEL=sonnet          # sonnet/opus/haiku
CLAUDE_TIMEOUT=120           # seconds
BRIEFING_HOUR=7              # morning briefing time (0-23)
BRIEFING_MINUTE=0            # (0-59)
LOG_LEVEL=INFO
```

### 6. Build and run

```bash
docker compose build
docker compose up -d
```

Check the logs:

```bash
docker compose logs -f
```

## Usage

| Command | Description |
|---------|-------------|
| `/start` | Welcome message |
| `/new` | Reset conversation (start fresh session) |
| Any text | Chat with your AI coach |

### Example messages

- "What were my last 3 activities?"
- "How's my training load looking this week?"
- "I have a half marathon in 8 weeks, help me plan"
- "What should I eat before a long ride?"
- "Remember that I'm vegetarian" (self-improving — saved permanently)

### Morning briefing

The bot sends a daily check-in at the configured time (default 07:00). It:

1. Checks your Intervals.icu training plan for the day
2. Reviews recent wellness data (sleep, HRV, soreness)
3. Looks at your training load and fatigue
4. Recommends what to do
5. Asks how you're feeling

### Customizing the coach

Edit `CLAUDE.md` to change the coach's personality, knowledge areas, or behavior. Changes take effect on the next message — no rebuild needed.

The coach also edits its own `CLAUDE.md` when you give it feedback (e.g. dietary preferences, injury history, race goals). Check the "Athlete Profile" and "Learned Preferences" sections at the bottom of the file to see what it's learned.

## Project structure

```
ai_fitness_trainer/
├── .env.example              # Template for secrets
├── .mcp.json                 # MCP server config (intervals-icu + google calendar)
├── CLAUDE.md                 # Coach personality + self-improving memory
├── Dockerfile                # Multi-stage: Go + Python + Node.js + Claude CLI
├── docker-compose.yml
├── pyproject.toml
└── src/
    ├── main.py               # Entrypoint
    ├── config.py             # Settings from env vars
    ├── telegram_bot.py       # aiogram handlers + whitelist
    ├── claude_client.py      # claude -p subprocess wrapper (stream-json)
    ├── session_store.py      # SQLite session persistence
    └── scheduler.py          # Morning briefing cron (apscheduler)
```

## Troubleshooting

**Bot doesn't respond**: Check `docker compose logs -f` for errors. Common issues:
- Invalid `CLAUDE_CODE_OAUTH_TOKEN` — regenerate with `claude setup-token`
- Wrong `ALLOWED_TELEGRAM_USER_IDS` — must be numeric, not username

**Session errors after restart**: The bot auto-recovers by starting a fresh session if `--resume` fails.

**OAuth token expired**: Re-run `claude setup-token` on your host, update `.env`, and `docker compose restart`.

**Morning briefing not sending**: Check that `TZ` is set correctly and the container clock matches. Verify with `docker compose exec bot date`.
