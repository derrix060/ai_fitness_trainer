import asyncio
import json
import logging

logger = logging.getLogger(__name__)


class ClaudeClient:
    def __init__(self, model: str = "sonnet", timeout: int = 120) -> None:
        self._model = model
        self._timeout = timeout

    async def send_message(
        self,
        user_text: str,
        session_id: str | None = None,
    ) -> tuple[str, str]:
        """Send a message to Claude CLI and return (response_text, session_id)."""
        cmd = [
            "claude",
            "-p",
            user_text,
            "--output-format", "stream-json",
            "--verbose",
            "--model", self._model,
            "--allowedTools",
            "mcp__intervals-icu__*,mcp__google-calendar__*,WebSearch,Read,Edit",
            "--dangerously-skip-permissions",
        ]

        if session_id:
            cmd.extend(["--resume", session_id])

        logger.info(
            ">>> User: %s (session=%s)",
            user_text[:200],
            session_id or "new",
        )

        try:
            proc = await asyncio.create_subprocess_exec(
                *cmd,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )

            result_data = None
            buf = b""

            while True:
                chunk = await proc.stdout.read(65536)
                if not chunk:
                    break
                buf += chunk
                while b"\n" in buf:
                    line, buf = buf.split(b"\n", 1)
                    text = line.decode(errors="replace").strip()
                    if not text:
                        continue

                    try:
                        event = json.loads(text)
                    except json.JSONDecodeError:
                        continue

                    event_type = event.get("type")

                    if event_type == "system":
                        mcp = event.get("mcp_servers", [])
                        statuses = ", ".join(
                            f"{s['name']}={s['status']}" for s in mcp
                        )
                        if statuses:
                            logger.info("    MCP: %s", statuses)

                    elif event_type == "assistant":
                        msg = event.get("message", {})
                        for block in msg.get("content", []):
                            if block.get("type") == "tool_use":
                                tool_name = block.get("name", "?")
                                tool_input = json.dumps(
                                    block.get("input", {}), ensure_ascii=False
                                )
                                logger.info(
                                    "    Tool call: %s(%s)",
                                    tool_name,
                                    tool_input[:200],
                                )
                            elif block.get("type") == "text":
                                snippet = block.get("text", "")[:200]
                                if snippet:
                                    logger.info("    Thinking: %s", snippet)

                    elif event_type == "tool_result":
                        tool_name = event.get("tool_name", "?")
                        content = event.get("content", "")
                        if isinstance(content, str):
                            snippet = content[:150]
                        else:
                            snippet = json.dumps(content, ensure_ascii=False)[:150]
                        logger.info("    Tool result: %s -> %s", tool_name, snippet)

                    elif event_type == "result":
                        result_data = event

            await asyncio.wait_for(proc.wait(), timeout=self._timeout)

        except asyncio.TimeoutError:
            proc.kill()
            await proc.wait()
            logger.error("Claude subprocess timed out after %ds", self._timeout)
            return (
                "Sorry, the request timed out. Please try again.",
                session_id or "",
            )

        if proc.returncode != 0:
            if session_id:
                logger.warning(
                    "Claude failed with session %s, retrying without session",
                    session_id,
                )
                return await self.send_message(user_text, session_id=None)
            logger.error("Claude subprocess failed (rc=%d)", proc.returncode)
            return (
                "Sorry, something went wrong processing your message. Please try again.",
                "",
            )

        if not result_data:
            logger.error("No result event received from Claude")
            return (
                "Sorry, I received an unexpected response. Please try again.",
                session_id or "",
            )

        response_text = result_data.get("result", "")
        new_session_id = result_data.get("session_id", session_id or "")

        if not response_text:
            logger.warning("Empty result from Claude")
            response_text = "I didn't generate a response. Please try rephrasing."

        duration = result_data.get("duration_ms", 0)
        num_turns = result_data.get("num_turns", 0)
        cost = result_data.get("total_cost_usd", 0)

        logger.info(
            "<<< Claude (%d turns, %.1fs, $%.4f, session=%s)",
            num_turns,
            duration / 1000,
            cost,
            new_session_id,
        )
        logger.info("<<< Response: %s", response_text[:500])

        return response_text, new_session_id
