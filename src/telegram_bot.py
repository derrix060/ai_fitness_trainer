import logging

from aiogram import Bot, Dispatcher, Router, F
from aiogram.filters import Command
from aiogram.types import Message

from src.claude_client import ClaudeClient
from src.config import Config
from src.session_store import SessionStore

logger = logging.getLogger(__name__)

router = Router()

# These get set in run_bot() before polling starts.
_config: Config
_session_store: SessionStore
_claude: ClaudeClient

MAX_TELEGRAM_LENGTH = 4096


def _split_message(text: str) -> list[str]:
    """Split a long message into chunks that fit within Telegram's limit.

    Splits at paragraph boundaries (double newline) first, then at single
    newlines, then hard-cuts as a last resort.
    """
    if len(text) <= MAX_TELEGRAM_LENGTH:
        return [text]

    chunks: list[str] = []
    remaining = text

    while remaining:
        if len(remaining) <= MAX_TELEGRAM_LENGTH:
            chunks.append(remaining)
            break

        # Try to split at a double newline within limit
        cut = remaining[:MAX_TELEGRAM_LENGTH].rfind("\n\n")
        if cut > 0:
            chunks.append(remaining[:cut])
            remaining = remaining[cut + 2:]
            continue

        # Try single newline
        cut = remaining[:MAX_TELEGRAM_LENGTH].rfind("\n")
        if cut > 0:
            chunks.append(remaining[:cut])
            remaining = remaining[cut + 1:]
            continue

        # Hard cut at limit
        chunks.append(remaining[:MAX_TELEGRAM_LENGTH])
        remaining = remaining[MAX_TELEGRAM_LENGTH:]

    return chunks


@router.message(Command("start"))
async def cmd_start(message: Message) -> None:
    if not _is_allowed(message):
        return
    await message.answer(
        "Hey! I'm your AI fitness coach. Ask me anything about your "
        "training, schedule, nutrition, or recovery. Use /new to start a "
        "fresh conversation."
    )


@router.message(Command("new"))
async def cmd_new(message: Message) -> None:
    if not _is_allowed(message):
        return
    user_id = message.from_user.id
    await _session_store.delete_session(user_id)
    await message.answer("Fresh start! What would you like to work on?")
    logger.info("Session reset for user %d", user_id)


@router.message(F.text)
async def handle_text(message: Message) -> None:
    if not _is_allowed(message):
        return

    user_id = message.from_user.id
    user_text = message.text

    # Show typing indicator
    await message.bot.send_chat_action(chat_id=message.chat.id, action="typing")

    # Get existing session
    session_id = await _session_store.get_session(user_id)

    # Send to Claude
    response_text, new_session_id = await _claude.send_message(
        user_text, session_id
    )

    # Persist session
    if new_session_id:
        await _session_store.save_session(user_id, new_session_id)

    # Send response, splitting if needed
    for chunk in _split_message(response_text):
        await message.answer(chunk)


def _is_allowed(message: Message) -> bool:
    if not message.from_user:
        return False
    if message.from_user.id not in _config.allowed_user_ids:
        logger.debug("Ignored message from unauthorized user %d", message.from_user.id)
        return False
    return True


async def run_bot(config: Config, session_store: SessionStore) -> None:
    global _config, _session_store, _claude

    _config = config
    _session_store = session_store
    _claude = ClaudeClient(model=config.claude_model, timeout=config.claude_timeout)

    bot = Bot(token=config.telegram_bot_token)
    dp = Dispatcher()
    dp.include_router(router)

    # Start morning briefing scheduler
    from src.scheduler import setup_scheduler

    scheduler = setup_scheduler(config, bot, _claude, session_store)
    scheduler.start()

    logger.info("Starting Telegram bot (allowed users: %s)", config.allowed_user_ids)

    try:
        await dp.start_polling(bot)
    finally:
        scheduler.shutdown()
