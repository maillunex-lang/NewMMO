#!/usr/bin/env python3
"""Checks content/zones/*.json (T-038b): canon enemy ids exist in the zone's region, tick rate/speed/interest sane,
spawn ids unique, spawns inside bounds, player spawn inside bounds. Exit 1 on any error."""
import glob
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import lint  # noqa: E402


def run(root, content_dir):
    res = lint.run(content_dir, os.path.join(root, "content", "schemas"), None)
    decl = {d["id"]: d for d in res["decls"]}
    errs = []
    files = sorted(glob.glob(os.path.join(root, "content", "zones", "*.json")))
    if not files:
        errs.append(("ZD_NONE", "no zone definitions"))
    for f in files:
        d = json.load(open(f, encoding="utf-8"))
        n = os.path.basename(f)
        b = d.get("bounds", {})
        if not (5 <= d.get("tick_rate", 0) <= 30):
            errs.append(("ZD_TICK", f"{n}: tick_rate {d.get('tick_rate')} outside 5..30"))
        if d.get("player_speed", 0) <= 0 or d.get("interest_radius", 0) <= 0:
            errs.append(("ZD_NUMBERS", f"{n}: player_speed/interest_radius must be > 0"))
        if not (b.get("max_x", 0) > b.get("min_x", 0) and b.get("max_y", 0) > b.get("min_y", 0)):
            errs.append(("ZD_BOUNDS", f"{n}: invalid bounds"))
        inside = lambda x, y: b["min_x"] <= x <= b["max_x"] and b["min_y"] <= y <= b["max_y"]  # noqa: E731
        ps = d.get("player_spawn", {})
        if not inside(ps.get("x", -1), ps.get("y", -1)):
            errs.append(("ZD_SPAWN", f"{n}: player_spawn outside bounds"))
        seen = set()
        for e in d.get("enemies", []):
            if e["spawn_id"] in seen:
                errs.append(("ZD_DUP", f"{n}: duplicate spawn {e['spawn_id']}"))
            seen.add(e["spawn_id"])
            c = decl.get(e["enemy_id"])
            if not c or c["kind"] != "enemy":
                errs.append(("ZD_ENEMY", f"{n}: {e['spawn_id']} enemy {e['enemy_id']} is not canon"))
            elif c.get("region") not in (None, d.get("region_id")) and not e["enemy_id"].startswith("ENM-" + d["region_id"][4:]):
                errs.append(("ZD_REGION", f"{n}: {e['enemy_id']} not in region {d['region_id']}"))
            if not inside(e["x"], e["y"]):
                errs.append(("ZD_SPAWN", f"{n}: {e['spawn_id']} outside bounds"))
    return errs


def main():
    root = os.path.abspath(os.path.join(HERE, "..", ".."))
    errs = run(root, os.path.join(root, "docs", "content"))
    for code, msg in errs:
        print(f"ERROR {code} {msg}")
    print(f"zone-defs: {len(errs)} error(s)")
    return 1 if errs else 0


if __name__ == "__main__":
    sys.exit(main())
