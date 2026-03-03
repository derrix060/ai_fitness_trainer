import aiosqlite
from datetime import datetime
from pathlib import Path


class SessionStore:
    def __init__(self, db_path: Path) -> None:
        self._db_path = db_path

    async def _ensure_tables(self, db: aiosqlite.Connection) -> None:
        await db.execute(
            """
            CREATE TABLE IF NOT EXISTS sessions (
                user_id INTEGER PRIMARY KEY,
                session_id TEXT NOT NULL,
                updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
            )
            """
        )
        await db.execute(
            """
            CREATE TABLE IF NOT EXISTS kv (
                key TEXT PRIMARY KEY,
                value TEXT NOT NULL
            )
            """
        )
        await db.execute(
            """
            CREATE TABLE IF NOT EXISTS analyzed_activities (
                activity_id TEXT PRIMARY KEY,
                analyzed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
            )
            """
        )
        await db.commit()

    async def get_session(self, user_id: int) -> str | None:
        async with aiosqlite.connect(self._db_path) as db:
            await self._ensure_tables(db)
            cursor = await db.execute(
                "SELECT session_id FROM sessions WHERE user_id = ?",
                (user_id,),
            )
            row = await cursor.fetchone()
            return row[0] if row else None

    async def save_session(self, user_id: int, session_id: str) -> None:
        async with aiosqlite.connect(self._db_path) as db:
            await self._ensure_tables(db)
            await db.execute(
                """
                INSERT INTO sessions (user_id, session_id, updated_at)
                VALUES (?, ?, CURRENT_TIMESTAMP)
                ON CONFLICT(user_id) DO UPDATE SET
                    session_id = excluded.session_id,
                    updated_at = CURRENT_TIMESTAMP
                """,
                (user_id, session_id),
            )
            await db.commit()

    async def delete_session(self, user_id: int) -> None:
        async with aiosqlite.connect(self._db_path) as db:
            await self._ensure_tables(db)
            await db.execute(
                "DELETE FROM sessions WHERE user_id = ?",
                (user_id,),
            )
            await db.commit()

    async def get_value(self, key: str) -> str | None:
        async with aiosqlite.connect(self._db_path) as db:
            await self._ensure_tables(db)
            cursor = await db.execute(
                "SELECT value FROM kv WHERE key = ?", (key,)
            )
            row = await cursor.fetchone()
            return row[0] if row else None

    async def set_value(self, key: str, value: str) -> None:
        async with aiosqlite.connect(self._db_path) as db:
            await self._ensure_tables(db)
            await db.execute(
                """
                INSERT INTO kv (key, value) VALUES (?, ?)
                ON CONFLICT(key) DO UPDATE SET value = excluded.value
                """,
                (key, value),
            )
            await db.commit()

    async def get_last_activity_check(self) -> str:
        """Return ISO date of last activity check, or today's date."""
        val = await self.get_value("last_activity_check")
        if val:
            return val
        return datetime.now().strftime("%Y-%m-%d")

    async def get_analyzed_activities(self) -> set[str]:
        """Return set of activity IDs that have already been analyzed."""
        async with aiosqlite.connect(self._db_path) as db:
            await self._ensure_tables(db)
            cursor = await db.execute(
                "SELECT activity_id FROM analyzed_activities"
            )
            rows = await cursor.fetchall()
            return {row[0] for row in rows}

    async def mark_activity_analyzed(self, activity_id: str) -> None:
        async with aiosqlite.connect(self._db_path) as db:
            await self._ensure_tables(db)
            await db.execute(
                """
                INSERT OR IGNORE INTO analyzed_activities
                    (activity_id) VALUES (?)
                """,
                (activity_id,),
            )
            await db.commit()
