#!/usr/bin/env python3
"""Client data <-> canon sync check (EX-P02 follow-up). Stdlib only.

Compares client/data quests+dialogues with canon declarations produced by lint, and
resolves canonical IDs mentioned in client scripts/scenes. ERROR = client contradicts canon.
Known non-canon stub IDs (C-POI-001: POI-BOARD-01 ...) are reported as INFO, not errors.

    python3 tools/content/check_client_sync.py [--client client] [--content-dir docs/content]
"""
from __future__ import annotations

import argparse
import glob
import json
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import lint  # noqa: E402

STUB_POI = re.compile(r"^POI-[A-Z][A-Z0-9-]*$")  # non-numeric gameplay stub ids (C-POI-001)
ID_RE = re.compile(r"(?<![A-Za-z0-9-])(?:NPC-\d{3}|MQ-\d\d-\d\d|SQ-\d\d-\d{3}|OBJ-\d\d-\d\d-\d|LOC-\d\d-\d{3}|FAC-0\d|DIA\.[A-Za-z0-9_.-]+[A-Za-z0-9])")


def run(root, content_dir):
    res = lint.run(content_dir, os.path.join(root, "content", "schemas"), None)
    decl = {d["id"]: d for d in res["decls"]}
    findings = []  # (severity, code, message)

    def add(sev, code, msg):
        findings.append((sev, code, msg))

    qfiles = sorted(glob.glob(os.path.join(root, "client", "data", "quests", "*.json")))
    dfiles = sorted(glob.glob(os.path.join(root, "client", "data", "dialogues", "*.json")))
    flows_ids = set()
    for qf in qfiles:
        with open(qf, encoding="utf-8") as fh:
            data = json.load(fh)
        for k in ("region", "reputation_faction"):
            if data.get(k) not in decl:
                add("ERROR", "CS_HEADER", f"{os.path.basename(qf)}: {k}={data.get(k)} not in canon")
        for q in data["quests"]:
            c = decl.get(q["id"])
            n = os.path.basename(qf)
            if c is None or c["kind"] != "quest_main":
                add("ERROR", "CS_QUEST", f"{n}: {q['id']} not a canon main quest")
                continue
            f = c["fields"]
            if q["title"] != f["title"]:
                add("ERROR", "CS_TITLE", f"{q['id']}: client '{q['title']}' != canon '{f['title']}'")
            if q.get("location") not in f["locations"][:1] + [q.get("location")] or q.get("location") not in decl:
                add("ERROR", "CS_LOC", f"{q['id']}: location {q.get('location')} undeclared")
            elif q["location"] != f["locations"][0]:
                add("ERROR", "CS_LOC", f"{q['id']}: location {q['location']} != canon first LOC {f['locations'][0]}")
            if q.get("contact") not in decl:
                add("ERROR", "CS_NPC", f"{q['id']}: contact {q.get('contact')} undeclared")
            cobj = [o["id"] for o in f["objectives"]]
            kobj = [o["id"] for o in q["objectives"]]
            if cobj != kobj:
                add("ERROR", "CS_OBJ", f"{q['id']}: objectives client {kobj} != canon {cobj}")
            for o in q["objectives"]:
                t = o.get("target", "")
                if t in decl:
                    continue
                if STUB_POI.match(t):
                    add("INFO", "CS_STUB_POI", f"{q['id']}/{o['id']}: gameplay stub target {t} (C-POI-001, map in later stage)")
                else:
                    add("ERROR", "CS_TARGET", f"{q['id']}/{o['id']}: target {t} unknown")
            rw, crw = q["reward"], f["reward"]
            if rw.get("xp") != crw.get("xp") or rw.get("gold") != crw.get("gold"):
                add("ERROR", "CS_REWARD", f"{q['id']}: xp/gold client {rw.get('xp')}/{rw.get('gold')} != canon {crw.get('xp')}/{crw.get('gold')}")
            crep = sum(r["delta"] for r in crw.get("reputation", []))
            if rw.get("reputation", 0) != crep:
                add("ERROR", "CS_REWARD", f"{q['id']}: reputation client {rw.get('reputation')} != canon {crep}")
            ev = rw.get("evidence")
            if ev != (crw.get("evidence") or [None])[0]:
                add("ERROR", "CS_EVIDENCE", f"{q['id']}: evidence client {ev} != canon {crw.get('evidence')}")
            for fl in rw.get("flags", []):
                if fl not in f["flags"]:
                    add("ERROR", "CS_FLAG", f"{q['id']}: flag {fl} not in canon flags")
    for df in dfiles:
        with open(df, encoding="utf-8") as fh:
            data = json.load(fh)
        n = os.path.basename(df)
        for did, ln in data["lines"].items():
            c = decl.get(did)
            if c is None:
                add("ERROR", "CS_DIA_MISSING", f"{n}: {did} not in canon 10")
                continue
            if ln.get("speaker") != c["fields"].get("speaker"):
                add("ERROR", "CS_DIA_SPEAKER", f"{did}: client {ln.get('speaker')} != canon {c['fields'].get('speaker')}")
            # canon text is the source of truth; client must be equal or an explicit prefix-free substring edit
            ct, kt = c["fields"]["text"], ln.get("text", "")
            if kt != ct:
                if kt.rstrip(".") == ct.rstrip("."):
                    add("INFO", "CS_DIA_PUNCT", f"{did}: only terminal period differs (canon lacks it)")
                else:
                    add("ERROR", "CS_DIA_TEXT", f"{did}: client text differs from canon")
        for qid, fl in data["flows"].items():
            if qid not in decl:
                add("ERROR", "CS_FLOW", f"{n}: flow for unknown quest {qid}")
            for stage, ids in fl.items():
                if isinstance(ids, dict):  # state-dependent variants keyed by completed-objective count
                    ids = [x for v in ids.values() for x in v]
                for i in ids:
                    flows_ids.add(i)
                    if i not in data["lines"]:
                        add("ERROR", "CS_FLOW_LINE", f"{qid}/{stage}: {i} not in client lines")
    # interaction registry (D-002) must cover every client objective and point at canon places
    rpath = os.path.join(root, "content", "interactables", "reg-01.json")
    if os.path.exists(rpath):
        with open(rpath, encoding="utf-8") as fh:
            reg = json.load(fh)["interactables"]
        canon_objs = {o["id"] for d in res["decls"] if d["kind"] in ("quest_main", "quest_side") for o in d["fields"]["objectives"]}
        by_obj = {}
        for e in reg:
            if e["objective_id"] in by_obj:
                add("ERROR", "CS_REG_DUP", f"registry: duplicate objective {e['objective_id']}")
            by_obj[e["objective_id"]] = e
            for k in ("canon_poi", "canon_loc"):
                if e[k] not in decl:
                    add("ERROR", "CS_REG_CANON", f"registry {e['objective_id']}: {k} {e[k]} not in canon")
            if e["objective_id"] not in canon_objs:
                add("ERROR", "CS_REG_OBJ", f"registry: objective {e['objective_id']} not in canon")
            if e["count"] > 1 and (not e["instances"] or len(e["instances"]) != e["count"]):
                add("ERROR", "CS_REG_INST", f"registry {e['objective_id']}: count {e['count']} needs {e['count']} distinct instances")
        for qf in qfiles:
            with open(qf, encoding="utf-8") as fh:
                data = json.load(fh)
            for q in data["quests"]:
                for o in q["objectives"]:
                    r = by_obj.get(o["id"])
                    if r is None:
                        add("ERROR", "CS_REG_MISSING", f"{o['id']}: no registry entry")
                    elif (r["event"], r["target_id"], r["count"]) != (o["event"], o["target"], o.get("count", 1)):
                        add("ERROR", "CS_REG_DRIFT", f"{o['id']}: registry {r['event']}/{r['target_id']}/{r['count']} != client {o['event']}/{o['target']}/{o.get('count', 1)}")
    # player-facing names table must mirror canon NPC names
    npath = os.path.join(root, "client", "data", "names.json")
    if os.path.exists(npath):
        with open(npath, encoding="utf-8") as fh:
            names = json.load(fh).get("npc", {})
        for nid, nm in names.items():
            c = decl.get(nid)
            if not c or c["kind"] != "npc":
                add("ERROR", "CS_NAME_ID", f"names.json: {nid} is not a canon NPC")
            elif c["fields"].get("name") != nm:
                add("ERROR", "CS_NAME", f"names.json: {nid} '{nm}' != canon '{c['fields'].get('name')}'")
        for d in decl.values():
            if d["kind"] == "npc" and d["id"] not in names:
                add("ERROR", "CS_NAME_MISSING", f"names.json: canon NPC {d['id']} missing")
    # client skill table must mirror canon skills (id, name, level, target, cost, cooldown, effect kind)
    spath = os.path.join(root, "client", "data", "skills.json")
    if os.path.exists(spath):
        client_sk = {x["id"]: x for lst in json.load(open(spath, encoding="utf-8"))["classes"].values() for x in lst}
        canon_sk = {d["id"]: d for d in res["decls"] if d["kind"] == "skill"}
        for sid, c in canon_sk.items():
            x = client_sk.get(sid)
            f = c["fields"]
            if x is None:
                add("ERROR", "CS_SKILL_MISSING", f"skills.json: {sid} missing")
            elif (x["name"], x["level"], x["target"], x["cost"], x["cooldown_s"], x["effect_kind"]) != \
                    (f["name"], f["level"], f["target"], f["cost"], f["cooldown_s"], f.get("effect_kind", "")):
                add("ERROR", "CS_SKILL", f"skills.json: {sid} differs from canon")
        for sid in client_sk.keys() - canon_sk.keys():
            add("ERROR", "CS_SKILL_ID", f"skills.json: {sid} is not a canon skill")
    # CC-BY (attribution) sources listed in assets/LICENSES.md must be credited in client/data/credits.txt
    lic_path = os.path.join(root, "client", "assets", "LICENSES.md")
    cred_path = os.path.join(root, "client", "data", "credits.txt")
    if os.path.exists(lic_path):
        credits = open(cred_path, encoding="utf-8").read() if os.path.exists(cred_path) else ""
        for line in open(lic_path, encoding="utf-8"):
            if line.startswith("|") and "CC-BY" in line:
                urls = re.findall(r"https://[^\s|)`]+", line)
                src = [u for u in urls if "creativecommons.org" not in u]
                if not src:
                    add("ERROR", "CS_CREDITS", f"LICENSES.md CC-BY row without source URL: {line[:60]}")
                for u in src:
                    if u.rstrip(".,") not in credits:
                        add("ERROR", "CS_CREDITS", f"CC-BY source {u} is not credited in client/data/credits.txt")
    # canonical IDs mentioned in client code/scenes
    for path in glob.glob(os.path.join(root, "client", "**", "*"), recursive=True):
        if not path.endswith((".gd", ".tscn", ".tres", ".cfg")) or os.sep + ".godot" + os.sep in path:
            continue
        with open(path, encoding="utf-8", errors="replace") as fh:
            for ln, line in enumerate(fh, 1):
                for m in ID_RE.finditer(line):
                    if m.group(0) not in decl:
                        add("ERROR", "CS_CODE_ID", f"{os.path.relpath(path, root)}:{ln}: {m.group(0)} not in canon")
    return findings


def main(argv=None):
    root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", default=root)
    ap.add_argument("--content-dir", default=os.path.join(root, "docs", "content"))
    a = ap.parse_args(argv)
    f = run(a.root, a.content_dir)
    for sev, code, msg in f:
        print(f"{sev} {code} {msg}")
    errs = sum(1 for x in f if x[0] == "ERROR")
    print(f"client-sync: {errs} error(s), {sum(1 for x in f if x[0] == 'INFO')} info")
    return 1 if errs else 0


if __name__ == "__main__":
    sys.exit(main())
