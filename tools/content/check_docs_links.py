#!/usr/bin/env python3
"""Check that repo paths referenced by docs exist (markdown links + backticked paths). Stdlib only.

Skips URLs, anchors, globs (*), placeholders (<...>), res:// paths, generated dirs (build/), and
paths explicitly planned (listed under a "planned"/"создавать" note are not auto-detected: use --allow).
"""
from __future__ import annotations

import argparse
import glob
import os
import re
import sys

LINK_RE = re.compile(r"\[[^\]]*\]\(([^)\s]+)\)")
TICK_RE = re.compile(r"`([^`\n]+)`")
PATH_HINT = re.compile(r"^(?:\.{0,2}/)?(?:docs|client|server|tools|content|assets|deploy|\.github|\.cursor)/[\w./-]+$|^[\w-]+\.(?:md|json|gd|py|go|yml|yaml|tscn|sh)$")
SKIP_PREFIX = ("http://", "https://", "mailto:", "#", "res://", "user://", "build/", "dist/")
SKIP_DIRS = (os.sep + ".git" + os.sep, os.sep + ".godot" + os.sep, os.sep + "build" + os.sep, os.sep + "node_modules" + os.sep)


GENERATED = {"crossrefs.json", "validation-report.json", "declarations.json", "content.manifest.json"}


def check(root, allow=()):
    bad = []
    basenames = set()
    for dp, dn, fn in os.walk(root):
        dn[:] = [d for d in dn if d not in (".git", ".godot", "node_modules")]
        basenames.update(fn)
    for md in glob.glob(os.path.join(root, "**", "*.md"), recursive=True):
        if any(s in md for s in SKIP_DIRS):
            continue
        rel = os.path.relpath(md, root)
        if rel.startswith("docs/director/prompts/"):
            continue  # executor prompts name output files that do not exist yet
        if re.match(r"docs/content/\d\d_", rel):
            continue  # canon specs describe planned/future structure; owner-approved, validated by lint.py instead
        base = os.path.dirname(md)
        with open(md, encoding="utf-8") as fh:
            in_code = False
            for ln, line in enumerate(fh, 1):
                if line.lstrip().startswith("```"):
                    in_code = not in_code
                    continue
                if in_code:
                    continue
                cands = [(m, True) for m in LINK_RE.findall(line)] + [(m, False) for m in TICK_RE.findall(line)]
                arrow = line.find("→")
                for raw, is_link in cands:
                    t = raw.split("#")[0].strip()
                    if "NNN" in t or (arrow >= 0 and line.find(raw) > arrow):
                        continue  # placeholder, or planned output after «→»
                    if not t or t.startswith(SKIP_PREFIX) or any(c in t for c in "*<>{}$ ") or ".." in t.replace("../", "") or t.endswith("/") and not is_link:
                        continue
                    if not is_link and not PATH_HINT.match(t):
                        continue
                    t = t.rstrip("/")
                    if "/" not in t and (t in basenames or t in GENERATED):
                        continue
                    rels = [os.path.normpath(os.path.join(base, t)), os.path.normpath(os.path.join(root, t))]
                    if not any(os.path.exists(r) for r in rels) and t not in allow:
                        bad.append((os.path.relpath(md, root), ln, t))
    return bad


def main(argv=None):
    root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", default=root)
    ap.add_argument("--allow", nargs="*", default=[])
    a = ap.parse_args(argv)
    bad = check(a.root, a.allow)
    for f, ln, t in bad:
        print(f"ERROR DOC_BROKEN_PATH {f}:{ln} {t}")
    print(f"docs-links: {len(bad)} broken reference(s)")
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
