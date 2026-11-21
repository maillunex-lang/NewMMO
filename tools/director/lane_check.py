#!/usr/bin/env python3
"""Lane gate for multi-agent commits. Stdlib only.

Every commit in the range must carry trailers:
    Lane: client|server|research|director
    Task: T-NNN           (required for non-director lanes)
    Extra-Write: path/glob (optional; director-issued tasks may grant extra paths — must be listed in docs/director/TASKS.md)
Every lane may also write its own research note `docs/research/<Task>-*.md`.
A commit may only touch files matching its lane's `write` globs; `protected` paths are director-only.

    python3 tools/director/lane_check.py --range <base>..<head>
"""
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
LANES = os.path.join(ROOT, "docs", "director", "lanes.json")


def glob_re(pat: str):
    out, i = "", 0
    while i < len(pat):
        if pat.startswith("**/", i):
            out += "(?:.*/)?"
            i += 3
        elif pat.startswith("**", i):
            out += ".*"
            i += 2
        elif pat[i] == "*":
            out += "[^/]*"
            i += 1
        elif pat[i] == "?":
            out += "[^/]"
            i += 1
        else:
            out += re.escape(pat[i])
            i += 1
    return re.compile("^" + out + "$")


def matches(path, pats):
    return any(glob_re(p).match(path) for p in pats)


def parse_trailers(msg: str):
    tr = {}
    for line in msg.splitlines():
        m = re.match(r"^(Lane|Task|Extra-Write):\s*(\S.*?)\s*$", line)
        if m:
            if m.group(1) == "Extra-Write":  # several globs may be comma-separated on one line
                tr.setdefault("Extra-Write", []).extend(x.strip() for x in m.group(2).split(",") if x.strip())
            else:
                tr.setdefault(m.group(1), []).append(m.group(2))
    return tr


def check_commit(cfg, files, trailers):
    """Return list of violation strings for one commit."""
    v = []
    lane = (trailers.get("Lane") or [None])[0]
    if lane is None:
        return ["missing `Lane:` trailer"]
    if lane not in cfg["lanes"]:
        return [f"unknown lane '{lane}'"]
    if lane == "director":
        return v
    if not trailers.get("Task") or not re.fullmatch(r"T-\d{3,}", trailers["Task"][0]):
        v.append("missing/invalid `Task: T-NNN` trailer")
    own = list(cfg["lanes"][lane]["write"])
    extra = list(trailers.get("Extra-Write", []))
    # every lane may write its own task's research note: docs/research/T-NNN-*.md (research-first rule)
    tid = trailers["Task"][0] if trailers.get("Task") else None
    if tid and re.fullmatch(r"T-\d{3,}", tid):
        extra.append(f"docs/research/{tid}-*.md")
    for f in files:
        if matches(f, cfg["protected"]) and not matches(f, extra):
            v.append(f"{f}: protected path (director only)")
        elif not matches(f, own) and not matches(f, extra):
            v.append(f"{f}: outside lane '{lane}' write set {own}")
    return v


def git(*a):
    return subprocess.run(["git", *a], cwd=ROOT, check=True, capture_output=True, text=True).stdout


def main(argv=None):
    ap = argparse.ArgumentParser()
    ap.add_argument("--range", required=True)
    a = ap.parse_args(argv)
    with open(LANES, encoding="utf-8") as fh:
        cfg = json.load(fh)
    shas = git("rev-list", "--reverse", "--no-merges", a.range).split()
    bad = 0
    for sha in shas:
        msg = git("show", "-s", "--format=%B", sha)
        files = [f for f in git("diff-tree", "--no-commit-id", "--name-only", "-r", "--root", sha).splitlines() if f]
        for viol in check_commit(cfg, files, parse_trailers(msg)):
            print(f"LANE_VIOLATION {sha[:8]} {viol}")
            bad += 1
    print(f"lane-check: {len(shas)} commit(s), {bad} violation(s)")
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
