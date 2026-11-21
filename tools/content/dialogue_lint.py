#!/usr/bin/env python3
"""Dialogue quality lint (T-016 spec, report mode). Stdlib only.

NOT part of blocking CI: the existing canon is expected to produce many findings (baseline).
Use it to (a) measure the baseline, (b) check that a rewritten batch (--quests MQ-01-05,SQ-01-001) is clean.

    python3 tools/content/dialogue_lint.py [--quests ID,ID] [--strict] [--out build/content/dialogue-lint.json]
Exit 1 with --strict if any ERROR finding exists for the selected quests.
"""
from __future__ import annotations

import argparse
import collections
import json
import os
import re
import sys
import unicodedata

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import lint  # noqa: E402

BANNED = [
    "Я проверю это. Что требуется в первую очередь?", "Проверю это на месте", "Теперь есть с чем сравнить",
    "Зафиксирую результат без догадок", "Задание выполнено. Вот подтверждённые результаты",
    "Пока не все свидетельства собраны", "Есть продвижение, но дело ещё не закрыто",
    "Хорошо, что на этот раз факты дошли до адресата", "Теперь мы знаем больше, чем до начала расследования",
    "На бумаге всё выглядит просто",
]
SECRET = re.compile(r"(цен\w*\s+Договор|Договор\w*.{0,30}плат\w*\s+памят|плат\w*\s+памят|шест\w*\s+хранител|жертв\w*\s+памят)", re.I)
LIMITS = {"NPC": (180, 240), "PLAYER": (110, 160), "SYSTEM": (140, 200)}  # (warn, error)
STAGES = ("START", "OBJ1", "OBJ2", "OBJ3", "PROGRESS", "TURNIN", "DONE")


def norm(t: str) -> str:
    t = unicodedata.normalize("NFC", t).lower().replace("ё", "е")
    return re.sub(r"[\s«»\"'.,!?…:;—-]+", " ", t).strip()


def overlay(content_dir, proposal_dir, tmp):
    """Copy the canon to tmp and replace the sections "### `<ID>`" of 10_DIALOGUES.md with proposal files
    (docs/content/dialogue_expansion/pilot/*.dialogue.md, each holding one drop-in section). Returns replaced IDs."""
    import shutil
    shutil.copytree(content_dir, tmp, dirs_exist_ok=True)
    path = os.path.join(tmp, "10_DIALOGUES.md")
    with open(path, encoding="utf-8") as fh:
        text = fh.read()
    replaced = []
    for fn in sorted(os.listdir(proposal_dir)):
        if not fn.endswith(".dialogue.md"):
            continue
        with open(os.path.join(proposal_dir, fn), encoding="utf-8") as fh:
            sec = fh.read().strip("\n")
        m = re.match(r"### `((?:MQ|SQ)-[\d-]+)`", sec)
        if not m:
            raise SystemExit(f"{fn}: must start with ### `QUEST-ID` — title")
        qid = m.group(1)
        pat = re.compile(r"^### `" + re.escape(qid) + r"`.*?(?=^### |^## |\Z)", re.S | re.M)
        if not pat.search(text):
            raise SystemExit(f"{fn}: section {qid} not found in 10_DIALOGUES.md")
        text = pat.sub(lambda _m: sec + "\n\n", text, count=1)
        replaced.append(qid)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(text)
    return replaced


def run(content_dir, schemas_dir, quests=None):
    res = lint.run(content_dir, schemas_dir, None)
    decl = {d["id"]: d for d in res["decls"]}
    lines = [d for d in res["decls"] if d["kind"] == "dialogue_line"]
    qre = re.compile(r"^DIA\.((?:MQ|SQ)-[\d-]+)\.([A-Z0-9]+)\.")
    by_quest = collections.defaultdict(list)
    for d in lines:
        m = qre.match(d["id"])
        if m:
            by_quest[m.group(1)].append((m.group(2), d))
    sel = set(quests) if quests else set(by_quest)
    # global duplicate counts over NPC+PLAYER lines (normalized)
    cnt = collections.Counter()
    quests_of = collections.defaultdict(set)
    for q, items in by_quest.items():
        for st, d in items:
            if d["fields"].get("speaker") != "SYSTEM":
                n = norm(d["fields"]["text"])
                cnt[n] += 1
                quests_of[n].add(q)
    out = []

    def add(sev, rule, qid, did, msg):
        out.append({"severity": sev, "rule": rule, "quest": qid, "id": did, "message": msg})

    for q in sorted(sel):
        items = by_quest.get(q, [])
        qd = decl.get(q)
        stages = {st for st, _ in items}
        for st in STAGES:
            if st not in stages:
                add("ERROR", "DIA-STAGE", q, None, f"missing stage {st}")
        prog = [d for st, d in items if st == "PROGRESS"]
        if len({d["fields"]["text"] for d in prog}) < 2:
            add("WARN", "DIA-PROGRESS", q, None, "fewer than 2 distinct PROGRESS lines (0/1/2 completed objectives)")
        contact = set(qd["fields"]["npcs"]) if qd else set()
        for st, d in items:
            f = d["fields"]
            t, sp = f["text"], f.get("speaker") or "NPC"
            kind = "NPC" if (sp or "").startswith("NPC-") else sp
            warn, err = LIMITS.get(kind, (180, 240))
            if len(t) > err:
                add("ERROR", "DIA-LENGTH", q, d["id"], f"{len(t)} chars (> {err})")
            elif len(t) > warn:
                add("WARN", "DIA-LENGTH", q, d["id"], f"{len(t)} chars (> {warn})")
            if sp.startswith("NPC-") and contact and sp not in contact:
                add("WARN", "DIA-SPEAKER", q, d["id"], f"{sp} is not among the quest NPCs {sorted(contact)}")
            if sp != "SYSTEM":
                for b in BANNED:
                    if norm(b) in norm(t):
                        add("ERROR", "DIA-STAMP", q, d["id"], f"banned stamp: «{b}»")
                        break
                n = norm(t)
                if cnt[n] >= 8:
                    add("ERROR", "DIA-EXACT", q, d["id"], f"same text in {len(quests_of[n])} quests")
                elif cnt[n] >= 3:
                    add("WARN", "DIA-EXACT", q, d["id"], f"same text in {len(quests_of[n])} quests")
            if t and t[-1] not in ".!?…»\"":
                add("WARN", "DIA-PUNCT", q, d["id"], "no terminal punctuation")
            chap = int(q[3:5])
            if chap <= 3 and SECRET.search(t):
                add("ERROR", "DIA-SECRET", q, d["id"], "possible early reveal of the Covenant secret (chapters I-III)")
    return out, {"quests": len(sel), "lines": len(lines)}


def main(argv=None):
    root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--content-dir", default=os.path.join(root, "docs", "content"))
    ap.add_argument("--schemas", default=os.path.join(root, "content", "schemas"))
    ap.add_argument("--quests", default="")
    ap.add_argument("--proposal-dir", default="", help="overlay *.dialogue.md sections onto 10_DIALOGUES.md before linting")
    ap.add_argument("--strict", action="store_true")
    ap.add_argument("--out", default=os.path.join(root, "build", "content", "dialogue-lint.json"))
    a = ap.parse_args(argv)
    sel = [x for x in a.quests.split(",") if x] or None
    content_dir = a.content_dir
    tmp = None
    if a.proposal_dir:
        import tempfile
        tmp = tempfile.mkdtemp(prefix="dlg_overlay_")
        replaced = overlay(a.content_dir, a.proposal_dir, tmp)
        content_dir = tmp
        sel = sel or replaced
        print(f"overlay: {len(replaced)} section(s) replaced: {', '.join(replaced)}")
    findings, meta = run(content_dir, a.schemas, sel)
    if tmp:
        import shutil
        shutil.rmtree(tmp, ignore_errors=True)
    os.makedirs(os.path.dirname(a.out), exist_ok=True)
    with open(a.out, "w", encoding="utf-8") as fh:
        json.dump({"meta": meta, "findings": findings}, fh, ensure_ascii=False, indent=1)
    by = collections.Counter((f["severity"], f["rule"]) for f in findings)
    for (sev, rule), n in sorted(by.items()):
        print(f"{sev:5} {rule:14} {n}")
    errs = sum(1 for f in findings if f["severity"] == "ERROR")
    bad_q = len({f["quest"] for f in findings if f["severity"] == "ERROR"})
    print(f"dialogue-lint: quests={meta['quests']} errors={errs} (quests with errors: {bad_q}) report={a.out}")
    return 1 if (a.strict and errs) else 0


if __name__ == "__main__":
    sys.exit(main())
