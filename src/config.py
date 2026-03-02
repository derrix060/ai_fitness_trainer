import os
from dataclasses import dataclass, field
from pathlib import Path

from dotenv import load_dotenv

load_dotenv()


@dataclass(frozen=True)
class Config:
    # Telegram
    telegram_bot_token: str = field(repr=False)
    allowed_user_ids: set[int] = field(default_factory=set)

    # Claude
    claude_model: str = "sonnet"
    claude_timeout: int = 120

    # Intervals.icu (passed to MCP server via env)
    intervals_api_key: str = field(default="", repr=False)
    intervals_athlete_id: str = ""

    # Paths
    data_dir: Path = Path("data")

    # Morning briefing cron
    briefing_hour: int = 7
    briefing_minute: int = 0
    timezone: str = ""

    # Logging
    log_level: str = "INFO"


def load_config() -> Config:
    allowed_ids_raw = os.environ.get("ALLOWED_TELEGRAM_USER_IDS", "")
    allowed_ids = set()
    for uid in allowed_ids_raw.split(","):
        uid = uid.strip()
        if uid:
            allowed_ids.add(int(uid))

    return Config(
        telegram_bot_token=os.environ["TELEGRAM_BOT_TOKEN"],
        allowed_user_ids=allowed_ids,
        claude_model=os.environ.get("CLAUDE_MODEL", "sonnet"),
        claude_timeout=int(os.environ.get("CLAUDE_TIMEOUT", "120")),
        intervals_api_key=os.environ.get("INTERVALS_API_KEY", ""),
        intervals_athlete_id=os.environ.get("INTERVALS_ATHLETE_ID", ""),
        data_dir=Path(os.environ.get("DATA_DIR", "data")),
        briefing_hour=int(os.environ.get("BRIEFING_HOUR", "7")),
        briefing_minute=int(os.environ.get("BRIEFING_MINUTE", "0")),
        timezone=os.environ["TZ"],
        log_level=os.environ.get("LOG_LEVEL", "INFO"),
    )
