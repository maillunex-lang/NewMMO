"""Per-document profile extractors: AST -> declaration records.

A declaration is a record {kind, id, file, line, fields}. Prose mentions are
NOT declarations; each profile only reads the structures its document owns.
"""
from __future__ import annotations

import re

from md_ast import Block, strip_ticks

Decl = dict


def D(kind, id_, file, line, **fields) -> Decl:
    return {"kind": kind, "id": id_, "file": file, "line": line, "fields": fields}


def _in_section(blocks, pred):
    """Blocks under the first-level-2 heading whose text satisfies pred."""
    out, on = [], False
    for b in blocks:
        if b.kind == "heading" and b.level <= 2:
            on = pred(b.text)
            continue
        if on:
            out.append(b)
    return out


def _tables(blocks):
    return [b for b in blocks if b.kind == "table"]


def _rows(blocks, id_re, kind, file, columns):
    """Declare one record per table row whose first cell is an ID matching id_re.

    columns maps header text (lowercased) -> field name; unmapped headers are ignored.
    """
    rx = re.compile(id_re)
    out = []
    hdr = []
    for t in _tables(blocks):
        rows = list(t.rows)
        if rx.fullmatch(strip_ticks(t.header[0])):
            # table split by blank lines: its first line is a data row, reuse previous header
            rows.insert(0, (t.line, t.header))
        else:
            hdr = [h.strip().lower() for h in t.header]
        for line, cells in rows:
            if not cells:
                continue
            cid = strip_ticks(cells[0])
            if not rx.fullmatch(cid):
                continue
            f = {}
            for h, c in zip(hdr, cells):
                if h in columns:
                    f[columns[h]] = strip_ticks(c)
            if kind == "loot_table":
                f["row_text"] = " | ".join(cells)
            out.append(D(kind, cid, file, line, **f))
    return out


def _int(s):
    m = re.search(r"-?\d+", s or "")
    return int(m.group()) if m else None


def p_factions(b, f):
    r = _rows(_in_section(b, lambda t: t.startswith("2.")), r"FAC-0[1-8]", "faction", f,
              {"фракция": "name", "тип": "type", "база/регион": "base"})
    return r


def p_locations(b, f):
    out = _rows(_in_section(b, lambda t: t.startswith("2.")), r"REG-0[1-6]", "region", f,
                {"регион": "name", "уровни": "levels", "ключевой город": "capital", "сюжетный узел": "story_node"})
    sec3 = _in_section(b, lambda t: t.startswith("3."))
    out += _rows(sec3, r"LOC-\d\d-\d{3}", "location", f,
                 {"название / роль": "name", "тип": "type", "pvp": "pvp"})
    cur_reg = None
    for blk in b:
        if blk.kind == "heading":
            m = re.match(r"(REG-\d\d)", blk.text)
            cur_reg = m.group(1) if m else cur_reg if blk.level > 3 else None
        elif blk.kind == "list_item" and cur_reg:
            m = re.match(r"`(POI-\d\d-\d{3})`\s*[—-]\s*(.*)", blk.text)
            if m:
                out.append(D("poi", m.group(1), f, blk.line, description=m.group(2), region=cur_reg))
    out += _rows(_in_section(b, lambda t: t.startswith("5.")), r"STL-\d\d-\d\d", "settlement", f,
                 {"название": "name", "регион": "region", "роль": "role"})
    out += _rows(_in_section(b, lambda t: t.startswith("6.")), r"DNG-\d\d-00[12]", "dungeon_ref", f,
                 {"подземелье": "name", "регион": "region", "уровень входа": "level"})
    for d in out:
        if d["kind"] == "dungeon_ref":
            d["fields"]["level"] = _int(d["fields"].get("level"))
    return out


def p_characters(b, f):
    return _rows(_in_section(b, lambda t: t.startswith("3.")), r"NPC-\d{3}", "npc", f,
                 {"имя": "name", "fac": "faction", "регион / основная зона": "location", "функция": "role",
                  "первое появление": "first_chapter"})


def p_classes(b, f):
    out = _rows(_in_section(b, lambda t: t.startswith("2.")), r"CLS-0[1-5]", "class", f,
                {"класс": "name", "роль": "role", "ресурс": "resource"})
    skills = _rows(_in_section(b, lambda t: t.startswith("4.")), r"CLS-0[1-5]-SK\d\d", "skill", f,
                   {"навык": "name", "уровень": "level", "цель": "target", "стоимость": "cost", "cd, с": "cooldown",
                    "базовый эффект": "effect", "правило": "rule"})
    for d in skills:
        fl = d["fields"]
        fl["class_id"] = d["id"][:6]
        fl["level"] = _int(fl.get("level"))
        fl["cost"] = _int(fl.get("cost"))
        fl["cooldown_s"] = _num(fl.pop("cooldown", ""))
        eff = fl.pop("effect", "")
        fl["effect_raw"] = eff
        k = re.match(r"([a-z_]+):\+?(\d+(?:\.\d+)?)%?(?:\*(\w+))?", eff)
        m = re.match(r"(\d+(?:\.\d+)?)\*(AP|SP)\b", eff)
        if k:
            fl["effect_kind"], fl["coefficient"] = k.group(1), float(k.group(2))
            if k.group(3):
                fl["scale"] = k.group(3)
        elif m:
            fl["effect_kind"], fl["coefficient"], fl["scale"] = "damage", float(m.group(1)), m.group(2)
        elif re.match(r"[a-z_]+$", eff):
            fl["effect_kind"] = eff
    return out + skills


_AB_ID = re.compile(r"`(AB-ENM-(\d\d)-(\d{3})-([A-Z]))`")
_NUMRE = r"(\d+(?:[.,]\d+)?)"


def _num(s):
    m = re.search(r"\d+(?:[.,]\d+)?", s or "")
    return float(m.group().replace(",", ".")) if m else None


def _abilities(blocks, f):
    """AB-ENM-RR-NNN-X from the §5 enemy cards: coefficient/scale, cooldown (A = attack interval),
    telegraph; the remaining mechanics stay as `text` for hand-written boss scripts."""
    out = []
    for b in blocks:
        if b.kind not in ("list_item", "paragraph") or "AB-ENM-" not in b.text:
            continue
        ms = list(_AB_ID.finditer(b.text))
        for i, m in enumerate(ms):
            seg = b.text[m.end(): ms[i + 1].start() if i + 1 < len(ms) else len(b.text)]
            aid, rr, nnn, slot = m.groups()
            fl = {"enemy": f"ENM-{rr}-{nnn}", "slot": slot}
            c = re.search(_NUMRE + r"\*(AP|SP)(?:\s*(?:\||или)\s*(?:" + _NUMRE + r"\*)?(AP|SP))?", seg)
            if c:
                fl["coefficient"] = float(c.group(1).replace(",", "."))
                a2 = c.group(4)
                fl["scale"] = c.group(2) if not a2 or a2 == c.group(2) else "AP|SP"
            cd = re.search(r"CD\s*" + _NUMRE + r"\s*с", seg) or re.search(r"(?:каждые|интервал)\s*" + _NUMRE + r"\s*с", seg)
            if not cd and slot == "A":
                cd = re.search(r"(?:^|[\s,])" + _NUMRE + r"\s*с\b", seg)
            if cd:
                fl["cooldown_s"] = float(cd.group(1).replace(",", "."))
            tg = (re.search(r"(?:телеграф|подготовк\w*|предупреждени\w*)\s*" + _NUMRE + r"\s*с", seg)
                  or re.search(_NUMRE + r"\s*с\s*телеграф", seg))
            if tg:
                fl["telegraph_s"] = float(tg.group(1).replace(",", "."))
            fl["text"] = re.sub(r"\s+", " ", seg).strip(" ;.,")[:240]
            out.append(D("ability", aid, f, b.line, **fl))
    return out


def p_enemies(b, f):
    out = []
    for t in _tables(_in_section(b, lambda t: t.startswith("5."))):
        pass
    sec = _in_section(b, lambda t: t.startswith("5."))
    out = _rows(sec, r"ENM-\d\d-\d{3}", "enemy", f,
                {"имя": "name", "тип": "threat", "уровень": "level", "ai": "ai", "элемент": "element",
                 "основной спавн": "spawn"})
    for d in out:
        d["fields"]["level"] = _int(d["fields"].get("level"))
        d["fields"]["spawn_refs"] = re.findall(r"(?:SPAWN|SPX|DNG|EVT)-\d\d-\d{3}", d["fields"].pop("spawn", ""))
    return out + _abilities(sec, f)


def p_story(b, f):
    out = []
    for blk in b:
        if blk.kind != "heading":
            continue
        m = re.match(r"Глава (\d)\.\s*«(.+?)»\s*·\s*`(REG-\d\d)`", blk.text)
        if m:
            out.append(D("chapter", f"CH-{int(m.group(1)):02d}", f, blk.line,
                         title=m.group(2), region=m.group(3)))
        m = re.match(r"`(END-0[1-3])`\s*—\s*«(.+?)»", blk.text)
        if m:
            out.append(D("ending", m.group(1), f, blk.line, title=m.group(2)))
        m = re.match(r"`(SCN-[A-Z0-9-]+)`\s*—\s*(.*)", blk.text)
        if m:
            out.append(D("scene", m.group(1), f, blk.line, title=m.group(2)))
    return out


_REWARD_XP = re.compile(r"(\d+)\s*XP")
_REWARD_GOLD = re.compile(r"(\d+)\s*золот")
_REWARD_REP = re.compile(r"([+-]\d+)\s*репутации\s*`(FAC-\d\d)`")


def _quest_blocks(blocks, head_re, kind, file, obj_re, flag_re):
    out = []
    cur = None
    for blk in blocks:
        if blk.kind == "heading":
            if cur:
                out.append(cur)
                cur = None
            m = re.match(head_re, blk.text)
            if m and blk.level == 4:
                cur = D(kind, m.group(1), file, blk.line, title=m.group(2).strip(),
                        objectives=[], flags=[], npcs=[], locations=[], reward={}, _body=[])
            continue
        if cur is None:
            continue
        txt = blk.text
        cur["fields"]["_body"].append(txt)
        if "**Открытие" in txt and "prereqs" not in cur["fields"]:
            seg = re.split(r"\*\*(?:Начало|Цели)", txt[txt.index("**Открытие"):])[0]
            seg = re.sub(r"`QF-[^`]*`|`SQF-[^`]*`", "", seg)
            cur["fields"]["prereqs"] = sorted(set(re.findall(r"(?<![A-Za-z0-9-])(?:MQ-\d\d-\d\d|SQ-\d\d-\d{3})(?![A-Za-z0-9-])", seg)))
        if blk.kind == "list_item":
            m = re.match(obj_re, txt)
            if m:
                cur["fields"]["objectives"].append({"id": m.group(1), "text": m.group(2).strip(), "line": blk.line})
        if "Награда:" in txt:
            rw = cur["fields"]["reward"]
            m = _REWARD_XP.search(txt)
            if m:
                rw["xp"] = int(m.group(1))
            m = _REWARD_GOLD.search(txt)
            if m:
                rw["gold"] = int(m.group(1))
            rw["evidence"] = sorted(set(re.findall(r"ITEM-Q-\d\d-\d\d", txt)))
            rw["reputation"] = [{"faction": fa, "delta": int(d)} for d, fa in _REWARD_REP.findall(txt)]
    if cur:
        out.append(cur)
    for q in out:
        body = "\n".join(q["fields"].pop("_body"))
        fl = q["fields"]
        m = re.search(r"(?:Уровень допуска|Минимальный уровень):\*\*\s*(\d+)", body)
        if m:
            fl["min_level"] = int(m.group(1))
        fl["flags"] = sorted(set(re.findall(flag_re, body)))
        fl["npcs"] = sorted(set(re.findall(r"NPC-\d{3}", body)))
        fl["locations"] = sorted(set(re.findall(r"LOC-\d\d-\d{3}", body)))
        # dedupe objective ids keep order
        seen, objs = set(), []
        for o in fl["objectives"]:
            if o["id"] not in seen:
                seen.add(o["id"])
                objs.append(o)
        fl["objectives"] = objs
    return out


def p_quests_main(b, f):
    return _quest_blocks(b, r"`(MQ-\d\d-\d\d)`\s*—\s*(.*)", "quest_main", f,
                         r"`(OBJ-\d\d-\d\d-\d)`:?\s*(.*)", r"QF-MQ-\d\d-\d\d-[A-Z0-9]+")


def p_quests_side(b, f):
    return _quest_blocks(b, r"`(SQ-\d\d-\d{3})`\s*—\s*(.*)", "quest_side", f,
                         r"`(SQO-\d\d-\d{3}-\d\d)`:?\s*(.*)", r"SQ[FR]-\d\d-\d{3}-[A-Z0-9]+")


_DIA_HEAD = re.compile(r"`(DIA\.[^`]*)`")
_DIA_TEXT = re.compile(r"«(.*?)»")
_DIA_SPEAKER = re.compile(r"·\s*`([^`]+)`")


def p_dialogues(b, f):
    out, template_seen = [], False
    for blk in b:
        if blk.kind != "list_item" or not blk.text.startswith("`DIA."):
            continue
        h = _DIA_HEAD.match(blk.text)
        did, rest = h.group(1), blk.text[h.end():]
        tm = _DIA_TEXT.search(rest)
        if "*" in did:
            # template row repeated under every NPC; declare once, not per NPC
            if not template_seen:
                out.append(D("dialogue_template", did, f, blk.line, text=tm.group(1) if tm else ""))
                template_seen = True
            continue
        sm = _DIA_SPEAKER.search(rest.split("«")[0])
        speaker = sm.group(1) if sm else None
        if speaker is None:
            m = re.match(r"DIA\.NPC\.(NPC-\d{3})\.", did)
            if m:
                speaker = m.group(1)
            elif did.endswith(".PLAYER"):
                speaker = "PLAYER"
        fields = {"text": tm.group(1) if tm else ""}
        if speaker:
            fields["speaker"] = speaker
        if not tm:
            fields["malformed"] = True
        out.append(D("dialogue_line", did, f, blk.line, **fields))
    return out


def p_items(b, f):
    out = []
    out += _rows(b, r"MAT-\d\d-\d{3}", "material", f, {"название": "name", "качество": "quality"})
    out += _rows(b, r"ITM-\d\d-\d{3}", "item", f, {"название": "name", "качество": "quality"})
    out += _rows(b, r"EQ-\d\d-\d\d-[A-Z]{2}-\d\d", "equipment", f,
                 {"класс": "class_name", "слот": "slot", "название": "name", "базовый уровень": "level"})
    out += _rows(b, r"REC-\d\d-\d{3}", "recipe", f, {})
    out += _rows(b, r"LOOT-ENM-\d\d-\d{3}", "loot_table", f,
                 {"противник": "enemy_raw", "категория": "category", "золото": "gold_raw", "mat-001": "mat1_raw",
                  "mat-003": "mat3_raw", "экипировка": "equip_raw", "особая награда": "special_raw"})
    out += _rows(b, r"ITEM-Q-\d\d-\d\d", "quest_item", f, {"квест": "quest", "название": "name", "тип": "type"})
    out += _rows(b, r"ITEM-SQ-\d\d-\d{3}", "quest_item", f, {"название": "name"})
    for d in out:
        if d["kind"] == "loot_table":
            d["fields"]["percents"] = [float(x) for x in re.findall(r"(\d+(?:\.\d+)?)\s*%", d["fields"].pop("row_text", ""))]
            m = re.search(r"ENM-\d\d-\d{3}", d["fields"].pop("enemy_raw", ""))
            if m:
                d["fields"]["enemy"] = m.group()
            fl = d["fields"]
            g = re.match(r"\s*(\d+)\s*[–-]\s*(\d+)", fl.pop("gold_raw", ""))
            if g:
                fl["gold_min"], fl["gold_max"] = int(g.group(1)), int(g.group(2))
            for key, raw in (("mat1_pct", fl.pop("mat1_raw", "")), ("mat3_pct", fl.pop("mat3_raw", ""))):
                m2 = re.match(r"\s*(\d+(?:\.\d+)?)\s*%", raw)
                if m2:
                    fl[key] = float(m2.group(1))
            sp = re.search(r"`(MAT-\d\d-\d{3})`\s*(\d+(?:\.\d+)?)\s*%", fl.pop("special_raw", ""))
            if sp:
                fl["special_item"], fl["special_pct"] = sp.group(1), float(sp.group(2))
            fl.pop("equip_raw", None)
        if d["kind"] == "equipment":
            d["fields"]["level"] = _int(d["fields"].get("level"))
    for blk in b:
        if blk.kind == "list_item":
            m = re.match(r"`(SET-\d\d-\d\d)`\s*[—-]\s*(.*)", blk.text)
            if m:
                out.append(D("item_set", m.group(1), f, blk.line, description=m.group(2)[:160]))
    return out


def p_dungeons(b, f):
    out = _rows(_in_section(b, lambda t: t.startswith("2.")) + [x for x in b if x.kind == "table"][:0],
                r"`?DNG-\d\d-00[12]`?", "dungeon", f, {})
    # catalog first cell is "`DNG-..` Name" -> parse by regex instead
    out = []
    for t in _tables(_in_section(b, lambda t: t.startswith("2."))):
        for line, cells in t.rows:
            m = re.match(r"`(DNG-\d\d-00[12])`\s*(.*)", cells[0])
            if m and len(cells) >= 6:
                ents = re.findall(r"ENM-\d\d-\d{3}", cells[4] + " " + cells[5])
                out.append(D("dungeon", m.group(1), f, line, name=m.group(2).strip(),
                             entrance=strip_ticks(cells[1]), level=_int(cells[2]), format=cells[3],
                             miniboss=ents[0] if ents else None,
                             boss=ents[1] if len(ents) > 1 else None))
    return out


def p_events(b, f):
    out = []
    for blk in b:
        if blk.kind == "heading" and blk.level == 4:
            m = re.match(r"`(EVT-\d\d-00[1-3])`\s*—\s*«(.*?)»", blk.text)
            if m:
                out.append(D("event", m.group(1), f, blk.line, title=m.group(2)))
    boss = {}
    for t in _tables(_in_section(b, lambda t: t.startswith("2."))):
        for _line, cells in t.rows:
            m = re.search(r"`(EVT-\d\d-003)`\s*/\s*`(ENM-\d\d-401)`", cells[3] if len(cells) > 3 else "")
            if m:
                boss[m.group(1)] = m.group(2)
    for d in out:
        if d["id"] in boss:
            d["fields"]["world_boss"] = boss[d["id"]]
    return out


PROFILES = {
    "01_WORLD.md": lambda b, f: [],
    "02_FACTIONS.md": p_factions,
    "03_LOCATIONS.md": p_locations,
    "04_CHARACTERS.md": p_characters,
    "05_CLASSES.md": p_classes,
    "06_ENEMIES.md": p_enemies,
    "07_STORY.md": p_story,
    "08_QUESTS_MAIN.md": p_quests_main,
    "09_QUESTS_SIDE.md": p_quests_side,
    "10_DIALOGUES.md": p_dialogues,
    "11_ITEMS_LOOT.md": p_items,
    "12_DUNGEONS.md": p_dungeons,
    "13_WORLD_EVENTS.md": p_events,
}
