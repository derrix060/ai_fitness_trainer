import logging

from aiogram import Bot
from apscheduler.schedulers.asyncio import AsyncIOScheduler
from apscheduler.triggers.cron import CronTrigger

from src.claude_client import ClaudeClient
from src.config import Config
from src.session_store import SessionStore
from src.telegram_bot import _split_message

logger = logging.getLogger(__name__)

MORNING_BRIEFING_PROMPT = (
    "Good morning! Please give me my daily training briefing:\n"
    "1. Check my training plan for today on Intervals.icu — what workout is scheduled?\n"
    "2. Check my recent wellness data (sleep, soreness, HRV, mood).\n"
    "3. Look at my recent training load (ATL/CTL/TSB) and fatigue.\n"
    "4. Based on all this, tell me what I should do today.\n"
    "5. Ask me how I'm feeling and if anything needs adjusting.\n"
    "Keep it concise — this is my morning check-in."
)


def setup_scheduler(
    config: Config,
    bot: Bot,
    claude: ClaudeClient,
    session_store: SessionStore,
) -> AsyncIOScheduler:
    scheduler = AsyncIOScheduler(timezone=config.timezone)

    async def morning_briefing() -> None:
        for user_id in config.allowed_user_ids:
            logger.info("Sending morning briefing to user %d", user_id)

            session_id = await session_store.get_session(user_id)

            response_text, new_session_id = await claude.send_message(
                MORNING_BRIEFING_PROMPT, session_id
            )

            if new_session_id:
                await session_store.save_session(user_id, new_session_id)

            for chunk in _split_message(response_text):
                await bot.send_message(chat_id=user_id, text=chunk)

            logger.info("Morning briefing sent to user %d", user_id)

    scheduler.add_job(
        morning_briefing,
        CronTrigger(
            hour=config.briefing_hour,
            minute=config.briefing_minute,
        ),
        id="morning_briefing",
        replace_existing=True,
    )

    logger.info(
        "Morning briefing scheduled at %02d:%02d %s",
        config.briefing_hour,
        config.briefing_minute,
        config.timezone,
    )

    return scheduler
