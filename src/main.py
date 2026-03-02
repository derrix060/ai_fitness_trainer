import asyncio
import logging

from src.config import load_config
from src.session_store import SessionStore
from src.telegram_bot import run_bot


def main() -> None:
    config = load_config()

    logging.basicConfig(
        level=config.log_level,
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    )

    config.data_dir.mkdir(parents=True, exist_ok=True)

    session_store = SessionStore(config.data_dir / "sessions.db")
    asyncio.run(run_bot(config, session_store))


if __name__ == "__main__":
    main()
