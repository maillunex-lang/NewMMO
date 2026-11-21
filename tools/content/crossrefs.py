"""EX-P02: ID registry, cross-reference resolution, static graph/consistency checks (CT-002..CT-009 subset).

Input: declarations (from profiles) + document texts. Output: crossrefs dict + findings via report.add.
Stdlib only. Typed fields are used for relations; regexes only locate *mentions*, never declare.
"""
from __future__ import annotations

import collections
import re

# namespace -> (id regex, declaring kinds). Mentions of these must resolve to a declaration.
HARD_NAMESPACES = {
    "REG": (r"REG-0[1-6]", ("region",)),
    "FAC": (r"FAC-0[1-8]", ("faction",)),
    "STL": (r"STL-0[1-6]-0[1-3]", ("settlement",)),
    "LOC": (r"LOC-0[1-6]-\d{3}", ("location",)),
    "POI": (r"POI-0[1-6]-\d{3}", ("poi",)),
    "NPC": (r"NPC-\d{3}", ("npc",)),
    "CLS": (r"CLS-0[1-5]", ("class",)),
    "ENM": (r"ENM-0[1-6]-\d{3}", ("enemy",)),
    "MQ": (r"MQ-0[1-6]-\d\d", ("quest_main",)),
    "SQ": (r"SQ-0[1-6]-\d{3}", ("quest_side",)),
    "OBJ": (r"OBJ-0[1-6]-\d\d-\d", ("objective",)),
    "SQO": (r"SQO-0[1-6]-\d{3}-\d\d", ("objective",)),
    "DNG": (r"DNG-0[1-6]-00[12]", ("dungeon",)),
    "EVT": (r"EVT-0[1-6]-00[1-3]", ("event",)),
    "END": (r"END-0[1-3]", ("ending",)),
    "MAT": (r"MAT-0[1-6]-\d{3}", ("material",)),
    "ITM": (r"ITM-0[0-6]-\d{3}", ("item",)),
    "REC": (r"REC-0[1-6]-\d{3}", ("recipe",)),
    "LOOT": (r"LOOT-ENM-0[1-6]-\d{3}", ("loot_table",)),
    "EQ": (r"EQ-0[1-5]-0[1-6]-[A-Z]{2}-0[12]", ("equipment",)),
    "ITEMQ": (r"ITEM-(?:Q-0[1-6]-\d\d|SQ-0[1-6]-\d{3})", ("quest_item",)),
}
# Documented but not individually declared (group-level IDs): tracked, not resolved.
SOFT_NAMESPACES = {"SPAWN": r"SPAWN-\d\d-\d{3}", "SPX": r"SPX-\d\d-\d{3}", "RES": r"RES-\d\d-\d{3}",
                   "RTE": r"RTE-\d\d-\d{3}", "TP": r"TP-\d\d-\d{3}"}

# Sections that only illustrate ID *formats* (examples, not references).
FORMAT_EXAMPLE_SECTIONS = {("11_ITEMS_LOOT.md", "2.")}

AI_LIBRARY = {"melee", "ranged", "caster", "tank", "defender", "skirmish", "boss", "world_boss"}
_ID_LOOKBEHIND = r"(?<![A-Za-z0-9-])"
_ID_LOOKAHEAD = r"(?![A-Za-z0-9-])"


def objective_decls(decls):
    out = []
    for d in decls:
        if d["kind"] in ("quest_main", "quest_side"):
            for o in d["fields"]["objectives"]:
                out.append({"kind": "objective", "id": o["id"], "file": d["file"], "line": o["line"], "fields": {"quest": d["id"]}})
    return out


def build(decls, texts, report):
    """Return crossrefs structure; add findings to report."""
    decls = [d for d in decls if not d["kind"].endswith("_ref")] + objective_decls(decls)
    reg = {d["id"]: d for d in decls}
    # declaration-line set so a declaration is not counted as its own reference
    decl_loc = {(d["file"], d["line"], d["id"]) for d in decls}

    refs: dict[str, list[str]] = collections.defaultdict(list)
    soft: dict[str, int] = collections.Counter()
    unresolved: dict[str, list[str]] = collections.defaultdict(list)
    for fn, text in texts.items():
        in_code = False
        skip = False
        for ln, line in enumerate(text.split("\n"), 1):
            if line.lstrip().startswith(("```", "~~~")):
                in_code = not in_code
            if line.startswith("## "):
                skip = any(fn == f and line[3:].startswith(sec) for f, sec in FORMAT_EXAMPLE_SECTIONS)
            if skip:
                continue
            for ns, (rx, kinds) in HARD_NAMESPACES.items():
                for m in re.finditer(_ID_LOOKBEHIND + "(" + rx + ")" + _ID_LOOKAHEAD, line):
                    i = m.group(1)
                    if (fn, ln, i) in decl_loc:
                        continue
                    refs[i].append(f"{fn}:{ln}")
                    if i not in reg:
                        unresolved[i].append(f"{fn}:{ln}")
            for ns, rx in SOFT_NAMESPACES.items():
                for m in re.finditer(_ID_LOOKBEHIND + "(" + rx + ")" + _ID_LOOKAHEAD, line):
                    soft[m.group(1)] += 1

    for i, where in sorted(unresolved.items()):
        f, l = where[0].rsplit(":", 1)
        report.add("ERROR", "CT002_UNRESOLVED_REF", f"{i} referenced {len(where)}x but never declared (first {where[0]})", f, int(l), i)

    _typed_checks(decls, reg, report)
    _graph_checks(decls, reg, report)

    crossrefs = {
        "schema": "newmmo.content.crossrefs/1",
        "entities": {i: {"kind": d["kind"], "declared_in": f"{d['file']}:{d['line']}",
                         "referenced_from": sorted(set(refs.get(i, [])))} for i, d in sorted(reg.items())},
        "unresolved": {k: v for k, v in sorted(unresolved.items())},
        "soft_group_ids": dict(sorted(soft.items())),
    }
    return crossrefs


def _region(i):
    m = re.search(r"-(\d\d)-", i + "-")
    return m.group(1) if m else None


def _typed_checks(decls, reg, report):
    by_kind = collections.defaultdict(list)
    for d in decls:
        by_kind[d["kind"]].append(d)

    # CT-005 enemies: AI profile known, spawn refs in own region, exactly one loot table each
    loot_by_enemy = collections.Counter(d["fields"].get("enemy") for d in by_kind["loot_table"])
    for e in by_kind["enemy"]:
        f = e["fields"]
        for part in re.split(r"[/,]", f["ai"]):
            if part.strip() not in AI_LIBRARY:
                report.add("ERROR", "CT005_BAD_AI", f"AI profile '{part.strip()}' not in 06 §3 library", e["file"], e["line"], e["id"])
        for sref in f["spawn_refs"]:
            if sref.startswith(("SPAWN", "SPX")) and _region(sref) != _region(e["id"]):
                report.add("ERROR", "CT005_SPAWN_REGION", f"{sref} not in enemy region", e["file"], e["line"], e["id"])
            if sref.startswith("DNG-") and sref not in reg:
                report.add("ERROR", "CT005_BAD_DNG", f"{sref} undeclared", e["file"], e["line"], e["id"])
        if loot_by_enemy.get(e["id"], 0) != 1:
            report.add("ERROR", "CT005_LOOT_LINK", f"expected exactly 1 loot table, found {loot_by_enemy.get(e['id'], 0)}", e["file"], e["line"], e["id"])
    for t in by_kind["loot_table"]:  # CT-009 (static part)
        if t["fields"].get("enemy") not in reg:
            report.add("ERROR", "CT009_LOOT_ENEMY", "loot table enemy missing/undeclared", t["file"], t["line"], t["id"])
        elif _region(t["id"]) != _region(t["fields"]["enemy"]) or t["id"][5:] != t["fields"]["enemy"]:
            report.add("ERROR", "CT009_LOOT_ENEMY", f"table id does not match enemy {t['fields']['enemy']}", t["file"], t["line"], t["id"])
        if not t["fields"].get("percents"):
            report.add("ERROR", "CT009_NO_WEIGHTS", "no drop probabilities parsed", t["file"], t["line"], t["id"])

    # CT-COMBAT (T-038a): every enemy has attack A and signature B with numbers; elites/bosses extra slots;
    # every class has the 12 canonical skills at the canonical unlock levels.
    abil = collections.defaultdict(dict)
    for a in by_kind["ability"]:
        if a["fields"]["enemy"] not in reg:
            report.add("ERROR", "CT_COMBAT_ABILITY_ENEMY", "ability enemy undeclared", a["file"], a["line"], a["id"])
        abil[a["fields"]["enemy"]][a["fields"]["slot"]] = a
    for e in by_kind["enemy"]:
        have = abil.get(e["id"], {})
        for slot in ("A", "B"):
            a = have.get(slot)
            if a is None:
                report.add("ERROR", "CT_COMBAT_NO_ABILITY", f"enemy has no ability {slot}", e["file"], e["line"], e["id"])
            elif "coefficient" not in a["fields"] or "cooldown_s" not in a["fields"]:
                report.add("ERROR", "CT_COMBAT_ABILITY_NUMBERS", f"ability {slot} lacks coefficient/cooldown", a["file"], a["line"], a["id"])
    LEVELS = [1, 3, 6, 10, 15, 20, 25, 30, 35, 40, 50, 60]
    skills = collections.defaultdict(dict)
    for sk in by_kind["skill"]:
        skills[sk["fields"]["class_id"]][sk["id"]] = sk
    for c in by_kind["class"]:
        mine = sorted(skills.get(c["id"], {}).values(), key=lambda x: x["id"])
        if [x["fields"]["level"] for x in mine] != LEVELS:
            report.add("ERROR", "CT_COMBAT_SKILL_LEVELS", f"class skill unlock levels {[x['fields']['level'] for x in mine]} != {LEVELS}", c["file"], c["line"], c["id"])

    # quest evidence: declared ITEM-Q set must equal the set granted by quest rewards (bijection)
    granted = {}
    for q in by_kind["quest_main"]:
        for ev in q["fields"]["reward"].get("evidence", []):
            granted[ev] = q["id"]
    declared = {d["id"]: d for d in by_kind["quest_item"] if d["id"].startswith("ITEM-Q-")}
    for ev, qid in sorted(granted.items()):
        if ev not in declared:
            report.add("ERROR", "CT009_EVIDENCE_UNDECLARED", f"{ev} granted by {qid} but not registered in 11", reg[qid]["file"], reg[qid]["line"], ev)
    for ev, d in sorted(declared.items()):
        if ev not in granted:
            report.add("ERROR", "CT009_EVIDENCE_UNUSED", "registered but no quest grants it", d["file"], d["line"], ev)
        elif d["fields"].get("quest") != granted[ev] or granted[ev] != "MQ-" + ev[7:]:
            report.add("ERROR", "CT009_EVIDENCE_QUEST", f"item says {d['fields'].get('quest')}, granted by {granted[ev]}", d["file"], d["line"], ev)

    # CT-006 dungeons: entrance POI exists in same region; boss/miniboss declared, right tier, same region
    for d in by_kind["dungeon"]:
        f = d["fields"]
        ent = reg.get(f["entrance"])
        if ent is None or ent["kind"] != "poi" or _region(f["entrance"]) != _region(d["id"]):
            report.add("ERROR", "CT006_ENTRANCE", f"entrance {f['entrance']} invalid", d["file"], d["line"], d["id"])
        for role, rng in (("miniboss", "20"), ("boss", "30")):
            b = reg.get(f[role])
            if b is None or b["kind"] != "enemy" or _region(f[role]) != _region(d["id"]) or f[role][7] != rng[0]:
                report.add("ERROR", "CT006_BOSS", f"{role} {f[role]} invalid", d["file"], d["line"], d["id"])

    # CT-007 events: 3 per region, 003 links to existing world boss ENM-RR-401
    for e in by_kind["event"]:
        if e["id"].endswith("003"):
            wb = e["fields"].get("world_boss")
            if wb is None or wb not in reg or _region(wb) != _region(e["id"]):
                report.add("ERROR", "CT007_WORLD_BOSS", f"world boss link {wb} invalid", e["file"], e["line"], e["id"])

    # CT-008 dialogues: every quest has START, TURNIN and DONE lines; PROGRESS expected
    have = collections.defaultdict(set)
    for d in by_kind["dialogue_line"]:
        m = re.match(r"DIA\.((?:MQ|SQ)-[\d-]+)\.([A-Z0-9]+)\.", d["id"])
        if m:
            have[m.group(1)].add(m.group(2))
    for q in by_kind["quest_main"] + by_kind["quest_side"]:
        miss = {"START", "PROGRESS", "TURNIN", "DONE"} - have.get(q["id"], set())
        miss |= {f"OBJ{i + 1}" for i in range(len(q["fields"]["objectives"])) if f"OBJ{i + 1}" not in have.get(q["id"], set())}
        if miss:
            report.add("ERROR", "CT008_DIALOGUE_COVERAGE", f"missing dialogue stages: {sorted(miss)}", q["file"], q["line"], q["id"])
    # region of quest locations vs quest region is not enforced: cross-region mentions are legitimate in canon


def _graph_checks(decls, reg, report):
    for kind, label, ct in (("quest_main", "MQ", "CT003"), ("quest_side", "SQ", "CT004")):
        qs = {d["id"]: d for d in decls if d["kind"] == kind}
        pre = {}
        for i, d in qs.items():
            p = d["fields"].get("prereqs")
            if p is None:
                report.add("ERROR", f"{ct}_NO_UNLOCK", "no «Открытие» unlock rule parsed", d["file"], d["line"], i)
                p = []
            pre[i] = [x for x in p if x != i]
            for x in pre[i]:
                if x not in reg:
                    report.add("ERROR", f"{ct}_PREREQ_MISSING", f"prerequisite {x} undeclared", d["file"], d["line"], i)
        # reachability (AND semantics: all prerequisites must be reachable) -> also detects cycles
        reach, changed = set(), True
        while changed:
            changed = False
            for i, p in pre.items():
                if i not in reach and all(x in reach or (x not in qs and x in reg) for x in p):
                    reach.add(i)
                    changed = True
        for i in sorted(set(qs) - reach):
            report.add("ERROR", f"{ct}_UNREACHABLE", "unreachable or on a prerequisite cycle", qs[i]["file"], qs[i]["line"], i)
        if kind == "quest_main":
            roots = sorted(i for i, p in pre.items() if not p)
            if roots != ["MQ-01-01"]:
                report.add("ERROR", "CT003_ROOTS", f"expected single root MQ-01-01, found {roots}")
            dependents = {x for p in pre.values() for x in p}
            # MQ-06-20 leads to endings, not another MQ; MQ-RR-09/-20 style branch quests may be dead-end only if last of region chain
            for i in sorted(qs):
                if i not in dependents and i != "MQ-06-20":
                    report.add("ERROR", "CT003_DEAD_END", "no quest unlocks after this one (not the final MQ)", qs[i]["file"], qs[i]["line"], i)
            ends = {d["id"] for d in decls if d["kind"] == "ending"}
            if ends != {"END-01", "END-02", "END-03"}:
                report.add("ERROR", "CT003_ENDINGS", f"endings declared: {sorted(ends)}")
