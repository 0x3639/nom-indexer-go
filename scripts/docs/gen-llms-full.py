#!/usr/bin/env python3
"""Generate llms-full.txt — every doc page concatenated, for one-shot LLM ingestion.

The output is large but parseable: each page is preceded by a `=== <path> ===`
header so an LLM (or grep) can locate content by source path.

Pass --check to fail (exit 1) if the file on disk would differ from the
regenerated content.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
DOCS = REPO_ROOT / "docs"
OUT = REPO_ROOT / "llms-full.txt"

# Files that are auxiliary content (fragments, generated, OpenAPI placeholder)
# rather than user-readable pages.
SKIP_GLOBS = (
    "schema/fragments/*",
    "schema/_generated.md",
    "api/openapi.yaml",
)


def _skip(rel: Path) -> bool:
    s = str(rel)
    return any(_match(s, pattern) for pattern in SKIP_GLOBS)


def _match(s: str, pattern: str) -> bool:
    if pattern.endswith("/*"):
        return s.startswith(pattern[:-1])
    return s == pattern


def render() -> str:
    out: list[str] = []
    out.append("# nom-indexer-go — full documentation corpus")
    out.append("")
    out.append(
        "Every Markdown page in docs/ concatenated into a single file for LLM "
        "ingestion. Pages are separated by `=== <relative-path> ===` headers. "
        "Schema fragments and generated content are omitted. Regenerated from "
        "scripts/docs/gen-llms-full.py.")
    out.append("")
    out.append("")

    for path in sorted(DOCS.rglob("*.md")):
        rel = path.relative_to(DOCS)
        if _skip(rel):
            continue
        out.append(f"=== docs/{rel} ===")
        out.append("")
        out.append(path.read_text().rstrip())
        out.append("")
        out.append("")

    return "\n".join(out)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()

    new = render()

    if args.check:
        if not OUT.exists():
            sys.stderr.write(
                "llms-full.txt is missing. Run gen-llms-full.py to create it.\n"
            )
            return 1
        existing = OUT.read_text()
        if existing != new:
            sys.stderr.write(
                "llms-full.txt is out of sync with docs/.\n"
                "Run scripts/docs/gen-llms-full.py and commit the result.\n"
            )
            return 1
        return 0

    OUT.write_text(new)
    sys.stdout.write(f"wrote {OUT.relative_to(REPO_ROOT)} ({len(new)} bytes)\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
