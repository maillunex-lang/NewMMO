#!/usr/bin/env python3
"""Checks content/dungeons/*.json against canon (12_DUNGEONS.md header table + 11 loot tables + 06 enemies).

Errors: unknown dungeon/boss/miniboss/enemy/loot/POI/LOC/MAT ids, header mismatch with the canon dungeon
declaration, room graph problems (duplicate rooms/objects, bad checkpoints/puzzle), reward tables not
matching boss/miniboss. Exit 1 on any error.
"""
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
    loot_by_enemy = {d["fields"].get("enemy"): d["id"] for d in res["decls"] if d["kind"] == "loot_table"}
    errs = []

    def err(code, msg):
        errs.append((code, msg))

    files = sorted(glob.glob(os.path.join(root, "content", "dungeons", "*.json")))
    if not files:
        err("DR_NONE", "no dungeon definitions")
    for f in files:
        d = json.load(open(f, encoding="utf-8"))
        did = d.get("dungeon_id", "?")
        name = os.path.basename(f)
        c = decl.get(did)
        if not c or c["kind"] != "dungeon":
            err("DR_ID", f"{name}: {did} is not a canon dungeon")
            continue
        cf = c["fields"]
        for key, canon in (("boss_id", "boss"), ("miniboss_id", "miniboss"), ("entrance_poi_id", "entrance")):
            if d.get(key) != cf.get(canon):
                err("DR_HEADER", f"{name}: {key}={d.get(key)} != canon {cf.get(canon)}")
        if d.get("entry_level") != cf.get("level"):
            err("DR_HEADER", f"{name}: entry_level={d.get('entry_level')} != canon {cf.get('level')}")
        if d.get("region_id") != c.get("region") and d.get("region_id") not in (did[4:6] and "REG-" + did[4:6],):
            err("DR_REGION", f"{name}: region_id {d.get('region_id')}")
        for k in ("location_id", "entrance_poi_id", "first_clear_material_id"):
            if d.get(k) not in decl:
                err("DR_REF", f"{name}: {k} {d.get(k)} undeclared in canon")
        rooms = d.get("rooms", [])
        if len(rooms) != 7:
            err("DR_ROOMS", f"{name}: {len(rooms)} rooms, canon says 7")
        want = [f"{did}-R{i:02d}" for i in range(1, 8)]
        if [r.get("id") for r in rooms] != want:
            err("DR_ROOMS", f"{name}: room ids {[r.get('id') for r in rooms]} != {want}")
        spawn_ids, objs = set(), {}
        for r in rooms:
            for s in r.get("spawns", []):
                if s["id"] in spawn_ids:
                    err("DR_SPAWN", f"{name}: duplicate spawn {s['id']}")
                spawn_ids.add(s["id"])
                if s["enemy_id"] not in decl or decl[s["enemy_id"]]["kind"] != "enemy":
                    err("DR_ENEMY", f"{name}: {s['id']} enemy {s['enemy_id']} not canon")
                elif s["enemy_id"] not in loot_by_enemy:
                    err("DR_LOOT", f"{name}: enemy {s['enemy_id']} has no loot table")
            for o in r.get("objects", []) + r.get("optional_objects", []):
                if o in objs:
                    err("DR_OBJ", f"{name}: object {o} in {objs[o]} and {r['id']}")
                objs[o] = r["id"]
        mini, boss = d.get("miniboss_id"), d.get("boss_id")
        spawn_enemies = {s["enemy_id"] for r in rooms for s in r.get("spawns", [])}
        for e in (mini, boss):
            if e not in spawn_enemies:
                err("DR_BOSS", f"{name}: {e} is not spawned in any room")
        tables = d.get("rewards", {}).get("tables", [])
        if sorted(tables) != sorted(x for x in (loot_by_enemy.get(mini), loot_by_enemy.get(boss)) if x):
            err("DR_REWARD", f"{name}: reward tables {tables} != boss/miniboss tables")
        cps = d.get("checkpoint_room_ids", [])
        if not cps or cps[0] != want[0] or any(c not in want for c in cps):
            err("DR_CHECKPOINT", f"{name}: checkpoints {cps}")
        pz = d.get("puzzle", {})
        if pz and any(objs.get(o) != pz.get("room") for o in pz.get("order", [])):
            err("DR_PUZZLE", f"{name}: puzzle objects are not all in {pz.get('room')}")
    return errs


def main(argv=None):
    root = os.path.abspath(os.path.join(HERE, "..", ".."))
    errs = run(root, os.path.join(root, "docs", "content"))
    for code, msg in errs:
        print(f"ERROR {code} {msg}")
    print(f"dungeon-registry: {len(errs)} error(s)")
    return 1 if errs else 0


if __name__ == "__main__":
    sys.exit(main())
