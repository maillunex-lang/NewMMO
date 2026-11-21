#!/usr/bin/env python3
"""EX-P01 content lint: docs/content/01..13 -> declarations -> schema -> report.

Python 3 stdlib only. Exit code 1 if any ERROR remains (publish blocked).

    python3 tools/content/lint.py [--content-dir docs/content] [--schemas content/schemas]
                                  [--allowlist content/lint_allowlist.json] [--out build/content]
"""
from __future__ import annotations

import argparse
import collections
import datetime
import hashlib
import json
import os
import re
import sys
import unicodedata

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import md_ast  # noqa: E402
import profiles  # noqa: E402
import schema as js  # noqa: E402

SOURCES = list(profiles.PROFILES.keys())  # 01..13
ORCHESTRATOR = "14_CONTENT_INTEGRATION.md"

# Golden volumes (14 §3 / §17.1 step 5): kind -> expected declaration count.
GOLDEN = {"faction": 8, "region": 6, "location": 48, "npc": 32, "class": 5, "enemy": 96,
          "quest_main": 120, "quest_side": 120, "dungeon": 12, "event": 18,
          "chapter": 6, "ending": 3, "equipment": 270, "item_set": 30, "material": 36, "loot_table": 96}

# Namespaces whose mentions are checked against declarations (WARN, full crossrefs = EX-P02).
REF_NAMESPACES = {
    "FAC": r"FAC-0[1-8]", "NPC": r"NPC-\d{3}", "LOC": r"LOC-\d\d-\d{3}", "ENM": r"ENM-\d\d-\d{3}",
    "MQ": r"MQ-\d\d-\d\d", "SQ": r"SQ-\d\d-\d{3}", "DNG": r"DNG-\d\d-00\d", "EVT": r"EVT-\d\d-00\d",
    "CLS": r"CLS-0\d", "POI": r"POI-\d\d-\d{3}",
}
KIND_OF_NS = {"FAC": "faction", "NPC": "npc", "LOC": "location", "ENM": "enemy", "MQ": "quest_main",
              "SQ": "quest_side", "DNG": "dungeon", "EVT": "event", "CLS": "class", "POI": "poi"}
BAD_DASH = re.compile(r"\b(FAC|NPC|LOC|POI|DNG|ENM|CLS|MQ|SQ|OBJ|SQO|EVT|MAT|ITM|EQ|SET|REC|LOOT|DIA|REG)[‐-―−]")
DLG_RE = re.compile(r"\bDLG-[A-Za-z0-9_.-]+")


class Report:
    def __init__(self, allow):
        self.items: list[dict] = []
        self.allow = allow

    def add(self, sev, code, msg, file=None, line=None, id_=None):
        for a in self.allow:
            if a["code"] == code and a.get("id") in (None, id_):
                if a["expires"] >= datetime.date.today().isoformat():
                    self.items.append({"severity": "WARN", "code": code, "message": msg, "file": file,
                                       "line": line, "id": id_,
                                       "allowlisted": {k: a[k] for k in ("conflict", "owner", "expires")}})
                    return
                self.items.append({"severity": "ERROR", "code": "ALLOWLIST_EXPIRED",
                                   "message": f"allowlist entry for {code} {id_} expired {a['expires']}",
                                   "file": file, "line": line, "id": id_})
        self.items.append({"severity": sev, "code": code, "message": msg, "file": file, "line": line, "id": id_})

    def count(self, sev):
        return sum(1 for i in self.items if i["severity"] == sev)


def load_allowlist(path):
    if not path or not os.path.exists(path):
        return []
    with open(path, encoding="utf-8") as fh:
        data = json.load(fh)
    for e in data.get("entries", []):
        for k in ("code", "conflict", "owner", "expires"):
            if k not in e:
                raise SystemExit(f"allowlist entry missing '{k}': {e}")
    return data.get("entries", [])


def load_schemas(path):
    out = {}
    for fn in sorted(os.listdir(path)):
        if fn.endswith(".schema.json"):
            with open(os.path.join(path, fn), encoding="utf-8") as fh:
                out[fn[: -len(".schema.json")]] = json.load(fh)
    return out


def run(content_dir, schemas_dir, allowlist_path=None):
    rep = Report(load_allowlist(allowlist_path))
    schemas = load_schemas(schemas_dir)
    manifest_files, decls, texts = [], [], {}

    # --- CT-001: presence, readability, sha256, encoding ---
    for fn in SOURCES + [ORCHESTRATOR]:
        p = os.path.join(content_dir, fn)
        if not os.path.isfile(p):
            rep.add("ERROR", "CT001_MISSING", f"required source missing: {fn}", fn)
            continue
        with open(p, "rb") as fh:
            raw = fh.read()
        try:
            text = raw.decode("utf-8")
        except UnicodeDecodeError as e:
            rep.add("ERROR", "CT001_ENCODING", f"not valid UTF-8: {e}", fn)
            continue
        if "\r" in text:
            rep.add("WARN", "CT001_CRLF", "CRLF line endings normalized to LF", fn)
            text = text.replace("\r\n", "\n").replace("\r", "\n")
        if unicodedata.normalize("NFC", text) != text:
            rep.add("WARN", "CT001_NFC", "text is not NFC; normalized before parsing", fn)
            text = unicodedata.normalize("NFC", text)
        if not text.lstrip().startswith("# "):
            rep.add("ERROR", "CT001_NO_TITLE", "document does not start with an H1 title", fn, 1)
        texts[fn] = text
        manifest_files.append({"file": fn, "sha256": hashlib.sha256(raw).hexdigest(), "bytes": len(raw),
                               "lines": text.count("\n") + 1, "role": "orchestrator" if fn == ORCHESTRATOR else "canon"})

    # --- text-level hygiene ---
    for fn, text in texts.items():
        for ln, line in enumerate(text.split("\n"), 1):
            if BAD_DASH.search(line):
                rep.add("ERROR", "ID_NON_ASCII_HYPHEN", "ID followed by a non-ASCII dash (must be ASCII '-')", fn, ln)
            if fn != "14_CONTENT_INTEGRATION.md" or True:
                for m in DLG_RE.finditer(line):
                    rep.add("WARN", "DLG_OBSOLETE", f"obsolete dialogue id shorthand {m.group()} (C-DIA-001 A: DIA.* only)", fn, ln, m.group())

    # --- profile parsing ---
    for fn in SOURCES:
        if fn not in texts:
            continue
        blocks = md_ast.parse(texts[fn])
        recs = profiles.PROFILES[fn](blocks, fn)
        decls += recs

    # --- schema validation per declaration ---
    for d in decls:
        sc = schemas.get(d["kind"])
        if sc is None:
            rep.add("ERROR", "SCHEMA_MISSING", f"no schema for kind '{d['kind']}'", d["file"], d["line"], d["id"])
            continue
        for e in js.validate({k: d[k] for k in ("kind", "id", "file", "line", "fields")}, sc):
            rep.add("ERROR", "SCHEMA_INVALID", e, d["file"], d["line"], d["id"])
        if d["kind"] == "dialogue_line" and d["fields"].get("malformed"):
            rep.add("ERROR", "DIA_MALFORMED", "dialogue line does not match `DIA.*` · `SPEAKER`: «text»", d["file"], d["line"], d["id"])

    # --- CT-002 (declaration part): duplicates across all declarations ---
    seen: dict[str, dict] = {}
    for d in decls:
        if d["kind"].endswith("_ref"):  # catalog mirrors, not declarations (checked for consistency below)
            continue
        prev = seen.get(d["id"])
        if prev is None:
            seen[d["id"]] = d
        else:
            rep.add("ERROR", "DUPLICATE_DECLARATION",
                    f"{d['id']} declared as {prev['kind']} ({prev['file']}:{prev['line']}) and as {d['kind']}",
                    d["file"], d["line"], d["id"])

    # --- golden volumes ---
    counts = collections.Counter(d["kind"] for d in decls)
    for kind, want in GOLDEN.items():
        if counts.get(kind, 0) != want:
            rep.add("ERROR", "GOLDEN_COUNT", f"{kind}: expected {want}, parsed {counts.get(kind, 0)}")
    # 20 MQ / 20 SQ / 2 DNG / 3 EVT per region
    per = collections.defaultdict(collections.Counter)
    for d in decls:
        m = re.match(r"(?:MQ|SQ|DNG|EVT)-(\d\d)-", d["id"])
        if m and d["kind"] in ("quest_main", "quest_side", "dungeon", "event"):
            per[d["kind"]][m.group(1)] += 1
    for kind, want in (("quest_main", 20), ("quest_side", 20), ("dungeon", 2), ("event", 3)):
        for r in ("01", "02", "03", "04", "05", "06"):
            if per[kind][r] != want:
                rep.add("ERROR", "GOLDEN_REGION_COUNT", f"{kind} REG-{r}: expected {want}, parsed {per[kind][r]}")

    # --- cross-document consistency that needs no registry (03 vs 12) ---
    by = {(d["kind"], d["id"]): d for d in decls}
    for d in decls:
        if d["kind"] == "dungeon":
            ref = by.get(("dungeon_ref", d["id"]))
            if ref is None:
                rep.add("ERROR", "DNG_NOT_IN_03", f"{d['id']} is in 12 but not in 03 §6", d["file"], d["line"], d["id"])
            elif ref["fields"].get("level") != d["fields"].get("level"):
                rep.add("ERROR", "DNG_LEVEL_MISMATCH", f"{d['id']} level 03={ref['fields'].get('level')} vs 12={d['fields'].get('level')}",
                        d["file"], d["line"], d["id"])

    # --- quest structure: objective ids belong to quest, objectives declared once ---
    for d in decls:
        if d["kind"] == "quest_main":
            rr, ss = d["id"][3:5], d["id"][6:8]
            for o in d["fields"]["objectives"]:
                if not o["id"].startswith(f"OBJ-{rr}-{ss}-"):
                    rep.add("ERROR", "OBJ_FOREIGN", f"{o['id']} does not belong to {d['id']}", d["file"], o["line"], o["id"])
        if d["kind"] == "quest_side":
            rr, sss = d["id"][3:5], d["id"][6:9]
            for o in d["fields"]["objectives"]:
                if not o["id"].startswith(f"SQO-{rr}-{sss}-"):
                    rep.add("ERROR", "OBJ_FOREIGN", f"{o['id']} does not belong to {d['id']}", d["file"], o["line"], o["id"])

    # --- dialogue lines reference existing quests (light check; full crossrefs = EX-P02) ---
    qids = {d["id"] for d in decls if d["kind"] in ("quest_main", "quest_side")}
    npcs = {d["id"] for d in decls if d["kind"] == "npc"}
    for d in decls:
        if d["kind"] != "dialogue_line":
            continue
        m = re.match(r"DIA\.((?:MQ|SQ)-[\d-]+)\.", d["id"])
        if m and m.group(1) not in qids:
            rep.add("ERROR", "DIA_UNKNOWN_QUEST", f"dialogue refers to undeclared quest {m.group(1)}", d["file"], d["line"], d["id"])
        sp = d["fields"].get("speaker")
        if sp and sp.startswith("NPC-") and sp not in npcs:
            rep.add("ERROR", "DIA_UNKNOWN_SPEAKER", f"speaker {sp} is not a declared NPC", d["file"], d["line"], d["id"])

    # --- reference scan (WARN only; resolution policy is EX-P02) ---
    declared = {d["id"] for d in decls}
    unresolved = collections.defaultdict(list)
    for fn, text in texts.items():
        for ln, line in enumerate(text.split("\n"), 1):
            for ns, rx in REF_NAMESPACES.items():
                for m in re.finditer(r"(?<![A-Za-z0-9-])(" + rx + r")(?![A-Za-z0-9-])", line):
                    if m.group(1) not in declared:
                        unresolved[m.group(1)].append(f"{fn}:{ln}")
    for rid, where in sorted(unresolved.items()):
        rep.add("WARN", "REF_UNRESOLVED", f"{rid} mentioned {len(where)}x, no declaration (first: {where[0]}); crossref policy = EX-P02",
                where[0].split(":")[0], int(where[0].split(":")[1]), rid)

    publishable = rep.count("ERROR") == 0
    return {
        "decls": decls, "report": rep, "counts": counts, "manifest_files": manifest_files,
        "unresolved_refs": {k: len(v) for k, v in unresolved.items()}, "publishable": publishable,
    }


def write_outputs(res, out_dir):
    os.makedirs(out_dir, exist_ok=True)
    rep = res["report"]
    summary = {"errors": rep.count("ERROR"), "warnings": rep.count("WARN"), "publishable": res["publishable"],
               "counts": dict(sorted(res["counts"].items()))}
    def dump(name, obj, **kw):
        with open(os.path.join(out_dir, name), "w", encoding="utf-8") as fh:
            json.dump(obj, fh, ensure_ascii=False, **kw)

    dump("validation-report.json", {"summary": summary, "items": rep.items}, indent=2)
    h = hashlib.sha256()
    for f in sorted(res["manifest_files"], key=lambda x: x["file"]):
        h.update(f["sha256"].encode())
    dump("content.manifest.json", {"schema": "newmmo.content.manifest/1", "stage": "EX-P01", "content_hash": h.hexdigest(),
                                   "publishable": res["publishable"], "files": res["manifest_files"],
                                   "counts": summary["counts"]}, indent=2)
    dump("declarations.json", res["decls"])


def main(argv=None):
    root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--content-dir", default=os.path.join(root, "docs", "content"))
    ap.add_argument("--schemas", default=os.path.join(root, "content", "schemas"))
    ap.add_argument("--allowlist", default=os.path.join(root, "content", "lint_allowlist.json"))
    ap.add_argument("--out", default=os.path.join(root, "build", "content"))
    ap.add_argument("-q", "--quiet", action="store_true")
    a = ap.parse_args(argv)
    res = run(a.content_dir, a.schemas, a.allowlist)
    write_outputs(res, a.out)
    rep = res["report"]
    if not a.quiet:
        for it in rep.items:
            if it["severity"] == "ERROR":
                print(f"ERROR {it['code']} {it['file']}:{it['line']} {it['id'] or ''} {it['message']}")
        warn = collections.Counter(i["code"] for i in rep.items if i["severity"] == "WARN")
        print("counts:", dict(sorted(res["counts"].items())))
        print("warnings by code:", dict(warn))
    print(f"content-lint: {rep.count('ERROR')} error(s), {rep.count('WARN')} warning(s) -> "
          f"{'PUBLISHABLE' if res['publishable'] else 'BLOCKED'}; report in {a.out}")
    return 0 if res["publishable"] else 1


if __name__ == "__main__":
    sys.exit(main())
