# Google Calendar Setup

This guide covers setting up the Google Calendar integration, including multi-account support for querying personal and work calendars simultaneously.

The bot uses [@cocal/google-calendar-mcp](https://www.npmjs.com/package/@cocal/google-calendar-mcp) as its MCP server.

## Prerequisites

- [Node.js](https://nodejs.org/) (for `npx`)
- A Google account
- A Google Cloud project with the Calendar API enabled

## 1. Create a Google Cloud project

1. Go to [console.cloud.google.com](https://console.cloud.google.com)
2. Create a **New Project** (e.g. "Calendar MCP")
3. Go to **APIs & Services** > **Library**, search for **Google Calendar API**, and click **Enable**

## 2. Configure the OAuth consent screen

1. Go to **APIs & Services** > **OAuth consent screen**
2. Select **External**, click **Create**
3. Fill in the required fields (app name, your email)
4. Add scope: `https://www.googleapis.com/auth/calendar`
5. Add your Google email(s) as test users (all accounts you want to connect)

> **Tip:** While in test mode, tokens expire after 7 days. To avoid weekly re-authentication, go to the OAuth consent screen and click **Publish App** to move to production.

## 3. Create OAuth credentials

1. Go to **APIs & Services** > **Credentials**
2. Click **Create Credentials** > **OAuth client ID**
3. Select **Desktop app** as the application type
4. Click **Create**, then **Download JSON**
5. Save the file to `./config/gcp-oauth.keys.json`

## 4. Authenticate your Google account

Run the auth command to complete the browser-based OAuth flow:

```bash
GOOGLE_OAUTH_CREDENTIALS=./config/gcp-oauth.keys.json \
GOOGLE_CALENDAR_MCP_TOKEN_PATH=./config/gcal-tokens.json \
  npx -y @cocal/google-calendar-mcp auth
```

Your browser will open asking you to authorize Google Calendar access. After granting access, tokens are saved to `./config/gcal-tokens.json`.

That's it for a single-account setup. The bot can now read and write to all calendars in that account.

## Multi-account setup

The MCP server supports connecting multiple Google accounts (e.g. personal + work) and querying them in a single request.

Each account is identified by a **nickname** you choose (e.g. `personal`, `work`). All tokens are stored in the same `gcal-tokens.json` file.

### Adding accounts

Authenticate each account by passing a nickname as the last argument:

```bash
# Personal account — sign in with your personal Google account when the browser opens
GOOGLE_OAUTH_CREDENTIALS=./config/gcp-oauth.keys.json \
GOOGLE_CALENDAR_MCP_TOKEN_PATH=./config/gcal-tokens.json \
  npx -y @cocal/google-calendar-mcp auth personal
```

```bash
# Work account — sign in with your work Google account when the browser opens
GOOGLE_OAUTH_CREDENTIALS=./config/gcp-oauth.keys.json \
GOOGLE_CALENDAR_MCP_TOKEN_PATH=./config/gcal-tokens.json \
  npx -y @cocal/google-calendar-mcp auth work
```

Repeat for as many accounts as you need.

### How multi-account queries work

- **Read-only tools** (e.g. `list-calendars`, `list-events`): when no `account` parameter is specified, results from all accounts are merged automatically.
- **Write tools** (e.g. `create-event`): auto-select the account with the appropriate permissions, or you can specify the `account` parameter explicitly.
- You can also pass an array of account nicknames to query a subset: `account: ["personal", "work"]`.

### Token file structure

After authenticating multiple accounts, `gcal-tokens.json` looks like this:

```json
{
  "personal": {
    "access_token": "...",
    "refresh_token": "...",
    "scope": "https://www.googleapis.com/auth/calendar",
    "token_type": "Bearer",
    "expiry_date": 1772563200430
  },
  "work": {
    "access_token": "...",
    "refresh_token": "...",
    "scope": "https://www.googleapis.com/auth/calendar",
    "token_type": "Bearer",
    "expiry_date": 1772563216835
  }
}
```

### Removing an account

To remove an account, delete its key from `gcal-tokens.json` and restart the bot.

## Verifying the setup

Test that the MCP server can connect and list your calendars:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list-calendars","arguments":{}}}' | \
  GOOGLE_OAUTH_CREDENTIALS=./config/gcp-oauth.keys.json \
  GOOGLE_CALENDAR_MCP_TOKEN_PATH=./config/gcal-tokens.json \
  npx -y @cocal/google-calendar-mcp 2>/dev/null | python3 -m json.tool
```

You should see a list of calendars from all authenticated accounts.

## Docker notes

No extra Docker configuration is needed. The existing `docker-compose.yml` already mounts `./config:/app/config` and `.mcp.json` references the correct paths inside the container:

```json
{
  "google-calendar": {
    "command": "npx",
    "args": ["-y", "@cocal/google-calendar-mcp"],
    "env": {
      "GOOGLE_OAUTH_CREDENTIALS": "/app/config/gcp-oauth.keys.json",
      "GOOGLE_CALENDAR_MCP_TOKEN_PATH": "/app/config/gcal-tokens.json"
    }
  }
}
```

After authenticating on your host machine, rebuild and restart:

```bash
docker compose build
docker compose up -d
```

## Troubleshooting

**"Google Calendar API has not been used in project ... before or it is disabled"**
Enable the Calendar API in your Google Cloud project: **APIs & Services** > **Library** > **Google Calendar API** > **Enable**. Wait a minute for it to propagate.

**Tokens expire after 7 days**
Your OAuth app is in test mode. Go to Google Cloud Console > **APIs & Services** > **OAuth consent screen** > **Publish App** to move to production.

**"No token file found" on container startup**
Run the auth command on your host machine first (see step 4 above). The token file is mounted into the container via Docker volumes.

**Work account shows "Access Not Configured" error**
Make sure the work account's email is added as a test user in the OAuth consent screen, or publish the app to production.
