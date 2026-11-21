"""Minimal Markdown block parser (stdlib only).

Produces a flat list of blocks with 1-based source lines. Supports exactly what
the canon docs use: headings, pipe tables, list items, paragraphs, code fences.
Code fences are kept as opaque blocks so examples inside them are never
mistaken for declarations.
"""
from __future__ import annotations

import re
from dataclasses import dataclass, field

HEADING_RE = re.compile(r"^(#{1,6})\s+(.*?)\s*#*\s*$")
LIST_RE = re.compile(r"^(\s*)(?:[-*+]|\d+[.)])\s+(.*)$")
FENCE_RE = re.compile(r"^\s*(```|~~~)")
TABLE_SEP_RE = re.compile(r"^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$")


@dataclass
class Block:
    kind: str  # heading | table | list_item | paragraph | code
    line: int
    text: str = ""
    level: int = 0
    header: list[str] = field(default_factory=list)
    rows: list[tuple[int, list[str]]] = field(default_factory=list)  # (line, cells)
    end_line: int = 0


def split_row(line: str) -> list[str]:
    s = line.strip()
    if s.startswith("|"):
        s = s[1:]
    if s.endswith("|"):
        s = s[:-1]
    return [c.strip() for c in s.split("|")]


def parse(text: str) -> list[Block]:
    lines = text.split("\n")
    blocks: list[Block] = []
    i, n = 0, len(lines)
    while i < n:
        raw = lines[i]
        if not raw.strip():
            i += 1
            continue
        if FENCE_RE.match(raw):
            start = i
            i += 1
            while i < n and not FENCE_RE.match(lines[i]):
                i += 1
            blocks.append(Block("code", start + 1, "\n".join(lines[start + 1 : i]), end_line=min(i + 1, n)))
            i += 1
            continue
        m = HEADING_RE.match(raw)
        if m:
            blocks.append(Block("heading", i + 1, m.group(2), level=len(m.group(1)), end_line=i + 1))
            i += 1
            continue
        if raw.lstrip().startswith("|"):
            start = i
            tbl = [lines[i]]
            i += 1
            while i < n and lines[i].lstrip().startswith("|"):
                tbl.append(lines[i])
                i += 1
            b = Block("table", start + 1, end_line=start + len(tbl))
            body_start = 1
            b.header = split_row(tbl[0])
            if len(tbl) > 1 and TABLE_SEP_RE.match(tbl[1]):
                body_start = 2
            for k in range(body_start, len(tbl)):
                b.rows.append((start + k + 1, split_row(tbl[k])))
            blocks.append(b)
            continue
        m = LIST_RE.match(raw)
        if m:
            blocks.append(Block("list_item", i + 1, m.group(2), end_line=i + 1))
            i += 1
            continue
        start = i
        buf = [raw]
        i += 1
        while i < n and lines[i].strip() and not HEADING_RE.match(lines[i]) and not LIST_RE.match(lines[i]) \
                and not lines[i].lstrip().startswith("|") and not FENCE_RE.match(lines[i]):
            buf.append(lines[i])
            i += 1
        blocks.append(Block("paragraph", start + 1, "\n".join(buf), end_line=start + len(buf)))
    return blocks


def strip_ticks(s: str) -> str:
    s = s.strip()
    if len(s) >= 2 and s.startswith("`") and s.endswith("`"):
        s = s[1:-1]
    return s.strip()


def sections(blocks: list[Block]):
    """Yield (heading_block_or_None, [blocks until next heading of <= level])."""
    cur_head = None
    cur: list[Block] = []
    out = []
    for b in blocks:
        if b.kind == "heading":
            out.append((cur_head, cur))
            cur_head, cur = b, []
        else:
            cur.append(b)
    out.append((cur_head, cur))
    return out
