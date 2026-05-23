#!/usr/bin/env python3
"""Generate llms.txt at the repo root from mkdocs.yml.

llms.txt is the emerging convention (https://llmstxt.org) for giving LLMs
a single hash-formatted index of a project's documentation. We walk the
mkdocs nav, pull the first H1 from each referenced page as the title and
the next non-empty line as the one-sentence summary, and emit a flat
hierarchy that any LLM can read raw.

Pass --check to fail (exit 1) if the file on disk would differ from the
regenerated content; the CI uses this to enforce that llms.txt is committed.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    sys.stderr.write(
        "PyYAML is required. `pip install pyyaml` or run inside the CI venv.\n"
    )
    sys.exit(2)


REPO_ROOT = Path(__file__).resolve().parents[2]
MKDOCS = REPO_ROOT / "mkdocs.yml"
DOCS = REPO_ROOT / "docs"
OUT = REPO_ROOT / "llms.txt"


def _yaml_safe_load(path: Path) -> dict:
    """mkdocs.yml uses !!python/name: tags for mermaid; treat as opaque."""

    class _Loader(yaml.SafeLoader):
        pass

    def _ignore_python_name(
        loader: yaml.SafeLoader, tag_suffix: str, node: yaml.Node
    ) -> str:
        return f"<python-callable:{tag_suffix}>"

    # add_multi_constructor handles the variable suffix after `python/name:`.
    _Loader.add_multi_constructor(
        "tag:yaml.org,2002:python/name:", _ignore_python_name
    )

    return yaml.load(path.read_text(), Loader=_Loader)


def _flatten_nav(nav: list, depth: int = 0) -> list[tuple[int, str, str | None]]:
    """Walk an mkdocs nav list, emitting (depth, label, page_path_or_None)."""

    out: list[tuple[int, str, str | None]] = []
    for entry in nav:
        if isinstance(entry, str):
            out.append((depth, entry, entry))
            continue
        if not isinstance(entry, dict):
            continue
        for label, value in entry.items():
            if isinstance(value, str):
                out.append((depth, str(label), value))
            elif isinstance(value, list):
                out.append((depth, str(label), None))
                out.extend(_flatten_nav(value, depth + 1))
    return out


def _page_summary(path: Path) -> tuple[str, str]:
    """Pull (H1 title, one-line summary) from a Markdown page."""

    if not path.exists():
        return ("(missing)", "Page does not exist on disk.")
    title = ""
    summary = ""
    h1_re = re.compile(r"^#\s+(.+?)\s*$")
    with path.open() as fh:
        for line in fh:
            if not title:
                m = h1_re.match(line)
                if m:
                    title = m.group(1).strip()
                continue
            stripped = line.strip()
            if not stripped:
                continue
            if stripped.startswith("#") or stripped.startswith("<!--"):
                continue
            if stripped.startswith("|") or stripped.startswith("-"):
                continue
            summary = re.sub(r"\s+", " ", stripped)
            # Trim trailing punctuation at sentence-ish boundary.
            if len(summary) > 240:
                summary = summary[:237].rstrip() + "..."
            break
    if not title:
        title = path.stem.replace("_", " ").replace("-", " ")
    if not summary:
        summary = "TODO."
    return (title, summary)


def render() -> str:
    mkdocs_cfg = _yaml_safe_load(MKDOCS)
    site_name = mkdocs_cfg.get("site_name", "nom-indexer-go")
    site_desc = mkdocs_cfg.get("site_description", "")
    nav = mkdocs_cfg.get("nav", [])

    out: list[str] = []
    out.append(f"# {site_name}")
    out.append("")
    if site_desc:
        out.append(f"> {site_desc}")
        out.append("")
    out.append(
        "Hash-formatted flat index of every documentation page, generated from "
        "mkdocs.yml. See https://llmstxt.org for the convention.")
    out.append("")

    current_section = None

    for depth, label, page in _flatten_nav(nav):
        if page is None:
            heading = "## " if depth == 0 else "### "
            out.append(f"{heading}{label}")
            out.append("")
            current_section = label
            continue

        page_path = DOCS / page
        title, summary = _page_summary(page_path)
        rel = f"docs/{page}"
        out.append(f"- [{label}]({rel}): {summary}")

    if current_section is None:
        # No sub-headings — flat list. That's fine.
        pass

    out.append("")
    return "\n".join(out)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--check",
        action="store_true",
        help="Exit 1 if llms.txt would differ from the regenerated content.",
    )
    args = parser.parse_args()

    new = render()

    if args.check:
        if not OUT.exists():
            sys.stderr.write("llms.txt is missing. Run gen-llms-txt.py to create it.\n")
            return 1
        existing = OUT.read_text()
        if existing != new:
            sys.stderr.write(
                "llms.txt is out of sync with mkdocs.yml / docs pages.\n"
                "Run scripts/docs/gen-llms-txt.py and commit the result.\n"
            )
            return 1
        return 0

    OUT.write_text(new)
    sys.stdout.write(f"wrote {OUT.relative_to(REPO_ROOT)} ({len(new)} bytes)\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
