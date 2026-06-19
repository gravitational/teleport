"""Vendored helpers for invoking the `claude` CLI from within an eval adapter.

This file duplicates a subset of the top-level `hyperskill.claude` and
`hyperskill.stream_tap` modules on purpose: the eval adapter is copied into
per-iteration archives and must run standalone (without the `hyperskill`
package on `sys.path`).

Keep this module aligned with `hyperskill/claude.py` and the stream-json
schema contract documented in `hyperskill/stream_tap.py`.
"""

from __future__ import annotations

import json
import os
import re
import subprocess
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Callable


# ---------------------------------------------------------------- commands


def read_model(skill_md: Path | None = None) -> str:
    """Resolve the model for `claude -p`.

    Prefers `model:` in the provided `SKILL.md` frontmatter (if any), then
    falls back to `~/.claude/settings.json`. Returns `""` if neither sets one.
    """
    if skill_md and skill_md.exists():
        text = skill_md.read_text()
        m = re.match(r"^---\n(.*?)\n---", text, re.DOTALL)
        if m:
            for line in m.group(1).splitlines():
                if line.strip().startswith("model:"):
                    value = line.split(":", 1)[1].strip()
                    if value:
                        return value

    settings = Path.home() / ".claude" / "settings.json"
    if settings.exists():
        try:
            return json.loads(settings.read_text()).get("model", "")
        except (json.JSONDecodeError, TypeError):
            pass

    return ""


def claude_cmd(model: str | None = None) -> list[str]:
    """Standard `claude -p` command used for eval runs and grading."""
    cmd = [
        "claude", "-p",
        "--output-format", "stream-json",
        "--verbose",
        "--no-session-persistence",
        "--disable-slash-commands",
        "--setting-sources", "",
        "--permission-mode", "auto",
    ]
    resolved = model if model is not None else read_model()
    if resolved:
        cmd.extend(["--model", resolved])
    return cmd


def claude_env() -> dict[str, str]:
    """`os.environ` minus `CLAUDE*` keys to avoid leaking parent context."""
    return {k: v for k, v in os.environ.items() if not k.startswith("CLAUDE")}


# --------------------------------------------------------- event-field helpers


def result_cost(event: dict) -> float:
    """Cost from a `result` event (accepts either cost_usd or total_cost_usd)."""
    return event.get("cost_usd", event.get("total_cost_usd", 0))


def sum_tokens(usage: dict) -> int:
    """Total tokens from a `result.usage` dict."""
    return (
        usage.get("input_tokens", 0)
        + usage.get("cache_creation_input_tokens", 0)
        + usage.get("cache_read_input_tokens", 0)
        + usage.get("output_tokens", 0)
    )


def _format_tool_summary(name: str, inp: dict) -> str:
    """Short human-readable summary of a tool call for log lines."""
    if name == "Bash":
        return f"{name}: {inp.get('command', '').split(chr(10), 1)[0][:60]}"
    if name in ("Write", "Read", "Edit"):
        return f"{name}: {Path(inp.get('file_path', '')).name}"
    return name


# ------------------------------------------------- stream runner


@dataclass
class StreamOutcome:
    """Everything the eval runner needs from one `claude -p` execution."""

    events: list[dict] = field(default_factory=list)
    tool_calls: dict[str, int] = field(default_factory=dict)
    errors: int = 0
    result_event: dict = field(default_factory=dict)
    duration: float = 0.0
    exit_code: int = 0


def run_claude_stream(
    prompt: str,
    *,
    cwd: Path,
    stderr_path: Path,
    timeout: int,
    log: Callable[[str], None],
    model: str | None = None,
) -> StreamOutcome:
    """Spawn `claude -p`, feed `prompt` via stdin, parse stream-json stdout.

    Returns a populated `StreamOutcome`. Exit codes: 0 on success, -1 if the
    subprocess had to be killed on timeout, otherwise the value claude
    returned.
    """
    out = StreamOutcome()
    step = 0
    start = time.time()

    with open(stderr_path, "w") as stderr_file:
        process = subprocess.Popen(
            claude_cmd(model),
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=stderr_file,
            cwd=str(cwd), env=claude_env(),
        )
        assert process.stdin is not None and process.stdout is not None

        with process.stdin as stdin:
            stdin.write(prompt.encode("utf-8"))

        try:
            with process.stdout as stdout:
                for raw_line in stdout:
                    line = raw_line.decode("utf-8", errors="replace").strip()
                    if not line:
                        continue
                    try:
                        event = json.loads(line)
                    except json.JSONDecodeError:
                        continue
                    out.events.append(event)
                    step = _log_event(event, log, step, out.tool_calls)
                    out.errors += _count_tool_errors(event)
                    if event.get("type") == "result":
                        out.result_event = event
                        log(
                            f"  Result: {event.get('num_turns', 0)} turns, "
                            f"${result_cost(event):.2f}, {time.time() - start:.0f}s"
                        )
            process.wait(timeout=max(1, timeout - (time.time() - start)))
            out.exit_code = process.returncode
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()
            out.exit_code = -1
            log(f"  TIMEOUT after {timeout}s")

    out.duration = time.time() - start
    return out


def _log_event(
    event: dict, log: Callable[[str], None], step: int, tool_calls: dict[str, int],
) -> int:
    if event.get("type") != "assistant":
        return step
    for block in event.get("message", {}).get("content", []):
        btype = block.get("type", "")
        if btype == "text":
            text = block.get("text", "").strip()
            if text:
                step += 1
                log(f"  [{step}] {text.split(chr(10), 1)[0][:80]}")
        elif btype == "tool_use":
            step += 1
            name = block.get("name", "")
            tool_calls[name] = tool_calls.get(name, 0) + 1
            log(f"  [{step}] {_format_tool_summary(name, block.get('input', {}))}")
    return step


def _count_tool_errors(event: dict) -> int:
    if event.get("type") != "user":
        return 0
    return sum(
        1 for block in event.get("message", {}).get("content", [])
        if block.get("type") == "tool_result" and block.get("is_error")
    )
