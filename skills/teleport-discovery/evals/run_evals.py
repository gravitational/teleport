#!/usr/bin/env python3
from __future__ import annotations
"""Eval runner for teleport-discovery skill.

Runs the skill via claude -p, grades with skill-creator's grader agent,
and aggregates into benchmark.json.

Usage:
    python evals/run_evals.py
    python evals/run_evals.py --eval-ids 1
    python evals/run_evals.py --no-grade
"""

import argparse
import json
import subprocess
import sys
import time
from pathlib import Path

from _claude_util import (
    claude_cmd,
    claude_env,
    read_model,
    result_cost,
    run_claude_stream,
    sum_tokens,
)

SKILL_DIR = Path(__file__).resolve().parent.parent
EVALS_JSON = SKILL_DIR / "evals" / "evals.json"
SKILL_MD = SKILL_DIR / "SKILL.md"


def _find_plugin_asset(relative: str) -> Path | None:
    """Locate a skill-creator asset under ~/.claude/plugins without hardcoding the marketplace path.

    Keeps run_evals.py self-contained (it gets copied into archives) while
    matching the rglob discovery used in hyperskill/preflight.py.
    """
    matches = list(Path.home().joinpath(".claude/plugins").rglob(relative))
    return matches[0] if matches else None


GRADER_AGENT = _find_plugin_asset("skill-creator/agents/grader.md")
AGGREGATE_SCRIPT = _find_plugin_asset("skill-creator/scripts/aggregate_benchmark.py")


def log(msg: str) -> None:
    print(msg, file=sys.stderr, flush=True)


def _cmd() -> list[str]:
    """Claude CLI command with the skill's preferred model resolved."""
    return claude_cmd(read_model(SKILL_MD))


def grade_run(run_dir: Path, expectations: list[str], timeout: int = 360) -> dict | None:
    grading_path = run_dir / "grading.json"

    if not GRADER_AGENT or not GRADER_AGENT.exists():
        log(f"  WARNING: grader agent not found under ~/.claude/plugins")
        return None

    # Strip [outcome] prefix — classification is for the improver, not the grader
    def strip_prefix(e):
        return e[len("[outcome] "):] if e.startswith("[outcome] ") else e

    expectations_text = "\n".join(f"- {strip_prefix(e)}" for e in expectations)
    prompt = f"""{GRADER_AGENT.read_text()}

---

## Parameters

- **expectations**:
{expectations_text}

- **transcript_path**: {run_dir / "outputs" / "transcript.jsonl"}
- **outputs_dir**: {run_dir / "outputs"}

Grade these expectations following the process above. Write grading.json to {grading_path}."""

    log(f"  Grading {len(expectations)} expectations...")
    start = time.time()

    try:
        result = subprocess.run(
            _cmd(),
            input=prompt, capture_output=True, text=True,
            cwd=str(run_dir), env=claude_env(), timeout=timeout,
        )
        log(f"  Grading complete in {time.time() - start:.0f}s (exit={result.returncode})")
        if result.returncode != 0 and result.stderr:
            log(f"  Grading stderr: {result.stderr[:300]}")
    except subprocess.TimeoutExpired:
        log(f"  Grading timed out after {timeout}s")
        return None

    if not grading_path.exists():
        log(f"  WARNING: grader did not write {grading_path}")
        return None

    try:
        grading = json.loads(grading_path.read_text())
    except json.JSONDecodeError:
        log(f"  WARNING: invalid JSON in {grading_path}")
        return None

    # The aggregator crashes on null values throughout grading.json.
    # Normalize: replace nulls with type-appropriate defaults before writing back.
    def strip_nulls(obj):
        if isinstance(obj, dict):
            return {k: strip_nulls(v) for k, v in obj.items() if v is not None}
        if isinstance(obj, list):
            return [strip_nulls(v) for v in obj]
        return obj

    grading = strip_nulls(grading)

    # Ensure user_notes_summary has expected structure
    grading.setdefault("user_notes_summary", {"uncertainties": [], "needs_review": [], "workarounds": []})

    exps = grading.get("expectations", [])
    passed = sum(1 for e in exps if e.get("passed"))
    grading["summary"] = {
        "passed": passed, "failed": len(exps) - passed,
        "total": len(exps), "pass_rate": passed / len(exps) if exps else 0.0,
    }
    grading_path.write_text(json.dumps(grading, indent=2))

    s = grading["summary"]
    log(f"  Grade: {s['passed']}/{s['total']} passed ({s['pass_rate']:.0%})")
    return grading


def run_single_eval(
    eval_config: dict, iteration_dir: Path, timeout: int, skip_grade: bool,
) -> dict:
    eval_id = eval_config["id"]
    prompt = eval_config["prompt"]
    expectations = eval_config.get("expectations", [])

    run_dir = iteration_dir / f"eval-{eval_id}" / "with_skill" / "run-1"
    outputs_dir = run_dir / "outputs"
    outputs_dir.mkdir(parents=True, exist_ok=True)

    context = eval_config.get("context", {})
    context_lines = [f"- {k}: {v}" for k, v in context.items()]

    full_prompt = f"{(SKILL_DIR / 'SKILL.md').read_text()}\n\n---\n\n{prompt}"
    if context_lines:
        full_prompt += "\n\n" + "\n".join(context_lines)

    log(f"  Prompt: {prompt[:100]}{'...' if len(prompt) > 100 else ''}")

    stderr_path = run_dir / "err.log"
    outcome = run_claude_stream(
        full_prompt,
        cwd=outputs_dir,
        stderr_path=stderr_path,
        timeout=timeout,
        log=log,
        model=read_model(SKILL_MD),
    )

    if stderr_path.exists():
        if stderr_path.stat().st_size > 0:
            log(f"  stderr: {stderr_path.read_text()[:300]}")
        else:
            stderr_path.unlink()

    log(f"  Execution: {outcome.duration:.0f}s (exit={outcome.exit_code})")

    (run_dir / "timing.json").write_text(json.dumps({
        "total_tokens": sum_tokens(outcome.result_event.get("usage", {})),
        "duration_ms": int(outcome.duration * 1000),
        "total_duration_seconds": round(outcome.duration, 1),
        "cost_usd": result_cost(outcome.result_event),
        "num_turns": outcome.result_event.get("num_turns", 0),
        "exit_code": outcome.exit_code,
    }, indent=2))

    (outputs_dir / "metrics.json").write_text(json.dumps({
        "tool_calls": outcome.tool_calls,
        "total_tool_calls": sum(outcome.tool_calls.values()),
        "errors_encountered": outcome.errors,
    }, indent=2))

    (outputs_dir / "transcript.jsonl").write_text(
        "\n".join(json.dumps(e) for e in outcome.events) + "\n"
    )

    grading = None
    if not skip_grade and expectations:
        grading = grade_run(run_dir, expectations)

    return {
        "eval_id": eval_id,
        "duration_seconds": round(outcome.duration, 1),
        "exit_code": outcome.exit_code,
        "run_dir": str(run_dir),
        "grading_pass_rate": grading.get("summary", {}).get("pass_rate", 0) if grading else None,
    }


def run_aggregation(iteration_dir: Path, skill_name: str) -> bool:
    if not AGGREGATE_SCRIPT or not AGGREGATE_SCRIPT.exists():
        log(f"  WARNING: aggregate script not found under ~/.claude/plugins")
        return False
    result = subprocess.run(
        [sys.executable, str(AGGREGATE_SCRIPT), str(iteration_dir), "--skill-name", skill_name],
        capture_output=True, text=True, timeout=30,
    )
    if result.returncode == 0:
        log(f"  {result.stdout.strip()}")
        return True
    log(f"  WARNING: aggregation failed:\n{result.stderr[:300]}")
    return False


def main():
    parser = argparse.ArgumentParser(description="Eval runner for teleport-discovery skill")
    parser.add_argument("--evals", default=str(EVALS_JSON), help="Path to evals.json")
    parser.add_argument("--workspace", default=None, help="Workspace directory")
    parser.add_argument("--iteration", type=int, default=1, help="Iteration number")
    parser.add_argument("--eval-ids", type=int, nargs="*", help="Run only these eval IDs")
    parser.add_argument("--no-grade", action="store_true", help="Skip grading step")
    parser.add_argument("--timeout", type=int, default=None, help="Per-eval timeout")
    args = parser.parse_args()

    evals_path = Path(args.evals)
    if not evals_path.exists():
        log(f"Error: {evals_path} not found")
        sys.exit(1)

    config = json.loads(evals_path.read_text())
    if not isinstance(config, dict) or "evals" not in config:
        log(f"Error: {evals_path} missing 'evals' key")
        sys.exit(1)

    evals = config["evals"]
    skill_name = config.get("skill_name", "teleport-discovery")
    exec_config = config.get("execution", {})
    timeout = args.timeout or exec_config.get("timeout_seconds", 120)

    if args.eval_ids:
        evals = [e for e in evals if e["id"] in args.eval_ids]
        if not evals:
            log(f"Error: no evals matched IDs {args.eval_ids}")
            sys.exit(1)

    workspace = Path(args.workspace) if args.workspace else SKILL_DIR.parent / f"{skill_name}-workspace"
    iteration_dir = workspace / f"iteration-{args.iteration}"
    iteration_dir.mkdir(parents=True, exist_ok=True)

    log(f"Skill:     {skill_name}")
    log(f"Workspace: {iteration_dir}")
    log(f"Evals:     {len(evals)} (IDs: {[e['id'] for e in evals]})")
    log(f"Timeout:   {timeout}s  Grading: {'off' if args.no_grade else 'on'}")

    results = []
    for i, eval_config in enumerate(evals):
        eval_id = eval_config["id"]
        log(f"\n{'=' * 60}")
        log(f"Eval {eval_id}  ({i + 1}/{len(evals)})")
        log(f"{'=' * 60}")
        result = run_single_eval(eval_config, iteration_dir, timeout, args.no_grade)
        results.append(result)
        pass_rate = result.get("grading_pass_rate")
        grade_str = f"  grade={pass_rate:.0%}" if pass_rate is not None else ""
        log(f"  Done in {result['duration_seconds']}s  (exit={result['exit_code']}){grade_str}")

    if not args.no_grade:
        run_aggregation(iteration_dir, skill_name)

    graded = [r for r in results if r.get("grading_pass_rate") is not None]
    summary = {
        "skill_name": skill_name,
        "iteration": args.iteration,
        "evals": results,
        "total": len(results),
        "completed": sum(1 for r in results if r["exit_code"] == 0),
    }
    if graded:
        summary["avg_pass_rate"] = round(
            sum(r["grading_pass_rate"] for r in graded) / len(graded), 4
        )

    summary_path = iteration_dir / "run_summary.json"
    summary_path.write_text(json.dumps(summary, indent=2))

    log(f"\n{'=' * 60}")
    log(f"Summary: {summary['completed']}/{summary['total']} completed")
    if graded:
        log(f"Average pass rate: {summary['avg_pass_rate']:.0%}")
    log(f"Results: {summary_path}")

    print(json.dumps(summary, indent=2))


if __name__ == "__main__":
    main()
