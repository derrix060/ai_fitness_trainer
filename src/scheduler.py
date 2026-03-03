import logging
import re
from datetime import datetime, timedelta

from aiogram import Bot
from apscheduler.schedulers.asyncio import AsyncIOScheduler
from apscheduler.triggers.cron import CronTrigger
from apscheduler.triggers.interval import IntervalTrigger

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

ACTIVITY_ANALYSIS_PROMPT = """\
Check Intervals.icu for any activities from the last 24 hours \
(use oldest={since_date}).

{skip_clause}

For EACH new activity, start your response with ANALYZED:<activity_id> on its \
own line (e.g. ANALYZED:i129194330), then provide a detailed analysis:

1. **Activity summary**: type, duration, distance, average HR, average power/pace
2. **Training classification**: what kind of training was this? (recovery, endurance/zone 2, tempo/sweetspot, threshold, VO2max, anaerobic, sprint, strength, etc.). Explain WHY you classified it this way based on the intensity distribution, HR zones, and power/pace data.
3. **Performance assessment**: how well did it go? Compare to recent similar activities. Look at pacing consistency, cardiac drift, power/pace decoupling, RPE vs actual intensity.
4. **Scientific context**: use WebSearch to find relevant exercise science research (peer-reviewed papers, systematic reviews) that supports your analysis. For example, if it was a zone 2 ride, cite research on mitochondrial adaptations; if it was intervals, cite research on the specific protocol's effectiveness.
5. **Key takeaways**: 2-3 actionable insights for future training.

For every scientific claim, include a reference with author, year, journal, and the finding. Format references as a numbered list at the end.

If there are NO new activities to analyze, respond with exactly: NO_NEW_ACTIVITIES
"""


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

    async def activity_check() -> None:
        since_date = (
            datetime.now() - timedelta(hours=24)
        ).strftime("%Y-%m-%dT%H:%M")
        analyzed = await session_store.get_analyzed_activities()
        logger.info(
            "Checking for new activities since %s (skip %d already analyzed)",
            since_date,
            len(analyzed),
        )

        if analyzed:
            skip_clause = (
                "Skip these activity IDs (already analyzed): "
                + ", ".join(sorted(analyzed))
            )
        else:
            skip_clause = ""

        for user_id in config.allowed_user_ids:
            session_id = await session_store.get_session(user_id)

            prompt = ACTIVITY_ANALYSIS_PROMPT.format(
                since_date=since_date,
                skip_clause=skip_clause,
            )
            response_text, new_session_id = await claude.send_message(
                prompt, session_id
            )

            if new_session_id:
                await session_store.save_session(user_id, new_session_id)

            if "NO_NEW_ACTIVITIES" in response_text:
                logger.info("No new activities for user %d", user_id)
                continue

            # Extract analyzed activity IDs from response
            for match in re.finditer(r"ANALYZED:(i\d+)", response_text):
                aid = match.group(1)
                await session_store.mark_activity_analyzed(aid)
                logger.info("Marked activity %s as analyzed", aid)

            # Strip ANALYZED: markers before sending to user
            clean_response = re.sub(
                r"^ANALYZED:i\d+\n?", "", response_text, flags=re.MULTILINE
            ).strip()

            if clean_response:
                logger.info(
                    "Sending activity analysis to user %d", user_id
                )
                for chunk in _split_message(clean_response):
                    await bot.send_message(chat_id=user_id, text=chunk)

    scheduler.add_job(
        morning_briefing,
        CronTrigger(
            hour=config.briefing_hour,
            minute=config.briefing_minute,
        ),
        id="morning_briefing",
        replace_existing=True,
    )

    scheduler.add_job(
        activity_check,
        IntervalTrigger(minutes=30),
        id="activity_check",
        replace_existing=True,
    )

    logger.info(
        "Morning briefing scheduled at %02d:%02d %s",
        config.briefing_hour,
        config.briefing_minute,
        config.timezone,
    )
    logger.info("Activity check scheduled every 30 minutes")

    return scheduler
