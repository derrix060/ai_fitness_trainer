import aiosqlite
from pathlib import Path


class SessionStore:
    def __init__(self, db_path: Path) -> None:
        self._db_path = db_path

    async def _ensure_table(self, db: aiosqlite.Connection) -> None:
        await db.execute(
            """
            CREATE TABLE IF NOT EXISTS sessions (
                user_id INTEGER PRIMARY KEY,
                session_id TEXT NOT NULL,
                updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
            )
            """
        )
        await db.commit()

    async def get_session(self, user_id: int) -> str | None:
        async with aiosqlite.connect(self._db_path) as db:
            await self._ensure_table(db)
            cursor = await db.execute(
                "SELECT session_id FROM sessions WHERE user_id = ?",
                (user_id,),
            )
            row = await cursor.fetchone()
            return row[0] if row else None

    async def save_session(self, user_id: int, session_id: str) -> None:
        async with aiosqlite.connect(self._db_path) as db:
            await self._ensure_table(db)
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
            await self._ensure_table(db)
            await db.execute(
                "DELETE FROM sessions WHERE user_id = ?",
                (user_id,),
            )
            await db.commit()
