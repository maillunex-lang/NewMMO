"""EX-P01 tests (CT-001, CT-002 static). Run: python3 -m unittest discover -s tools/content/tests -v"""
import json
import os
import shutil
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, ".."))
import lint  # noqa: E402
import md_ast  # noqa: E402
import schema as js  # noqa: E402

ROOT = os.path.abspath(os.path.join(HERE, "..", "..", ".."))
CANON = os.path.join(ROOT, "docs", "content")
SCHEMAS = os.path.join(ROOT, "content", "schemas")
ALLOW = os.path.join(ROOT, "content", "lint_allowlist.json")


class Sandbox(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="ct_")
        self.dir = os.path.join(self.tmp, "content")
        shutil.copytree(CANON, self.dir)

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def edit(self, fn, old, new, count=1):
        p = os.path.join(self.dir, fn)
        with open(p, encoding="utf-8") as fh:
            t = fh.read()
        self.assertIn(old, t, "test precondition: pattern must exist in canon")
        with open(p, "w", encoding="utf-8") as fh:
            fh.write(t.replace(old, new, count))

    def run_lint(self, allow=ALLOW):
        return lint.run(self.dir, SCHEMAS, allow)

    @staticmethod
    def errors(res, code=None):
        return [i for i in res["report"].items if i["severity"] == "ERROR" and (code is None or i["code"] == code)]


class RealCanon(Sandbox):
    def test_canon_passes_with_allowlist(self):  # CT-001 + CT-002
        res = self.run_lint()
        self.assertEqual(self.errors(res), [])
        self.assertTrue(res["publishable"])

    def test_manifest_has_sha256_for_14_files(self):  # CT-001
        res = self.run_lint()
        self.assertEqual(len(res["manifest_files"]), 14)
        for f in res["manifest_files"]:
            self.assertRegex(f["sha256"], r"^[0-9a-f]{64}$")

    def test_golden_counts(self):
        c = self.run_lint()["counts"]
        for k, v in {"faction": 8, "location": 48, "npc": 32, "class": 5, "enemy": 96, "quest_main": 120,
                     "quest_side": 120, "dungeon": 12, "event": 18, "ending": 3}.items():
            self.assertEqual(c[k], v, k)

    def test_no_id_collisions_without_allowlist(self):  # C-SET-001 RESOLVED (STL-*)
        res = self.run_lint(allow=None)
        self.assertEqual(self.errors(res, "DUPLICATE_DECLARATION"), [])
        self.assertEqual(res["counts"]["settlement"], 18)
        self.assertTrue(all(d["id"].startswith("STL-") for d in res["decls"] if d["kind"] == "settlement"))

    def test_repo_allowlist_is_empty(self):  # C-SET-001, C-ITEMQ-001 RESOLVED
        with open(ALLOW, encoding="utf-8") as fh:
            self.assertEqual(json.load(fh)["entries"], [])

    def _dup_with_allow(self, expires):
        self.edit("08_QUESTS_MAIN.md", "#### `MQ-01-02` — Два свидетеля", "#### `MQ-01-01` — Два свидетеля")
        p = os.path.join(self.tmp, "allow.json")
        with open(p, "w") as fh:
            json.dump({"entries": [{"code": "DUPLICATE_DECLARATION", "id": "MQ-01-01", "conflict": "T-1",
                                    "owner": "t", "expires": expires}]}, fh)
        return self.run_lint(allow=p)

    def test_allowlist_downgrades_until_expiry(self):
        res = self._dup_with_allow("2999-01-01")
        self.assertFalse(self.errors(res, "DUPLICATE_DECLARATION"))
        self.assertTrue(any(i["code"] == "DUPLICATE_DECLARATION" and i["severity"] == "WARN" for i in res["report"].items))

    def test_expired_allowlist_becomes_error(self):
        res = self._dup_with_allow("2000-01-01")
        self.assertTrue(self.errors(res, "ALLOWLIST_EXPIRED"))
        self.assertFalse(res["publishable"])


class Negative(Sandbox):
    def test_missing_file(self):  # CT-001
        os.remove(os.path.join(self.dir, "09_QUESTS_SIDE.md"))
        res = self.run_lint()
        self.assertTrue(self.errors(res, "CT001_MISSING"))
        self.assertFalse(res["publishable"])

    def test_bad_encoding(self):  # CT-001
        with open(os.path.join(self.dir, "01_WORLD.md"), "wb") as fh:
            fh.write(b"# x\n\xff\xfe\n")
        self.assertTrue(self.errors(self.run_lint(), "CT001_ENCODING"))

    def test_duplicate_quest_declaration(self):  # CT-002
        self.edit("08_QUESTS_MAIN.md", "#### `MQ-01-02` — Два свидетеля", "#### `MQ-01-01` — Два свидетеля")
        res = self.run_lint()
        self.assertTrue(any(i["id"] == "MQ-01-01" for i in self.errors(res, "DUPLICATE_DECLARATION")))

    def test_prose_mention_is_not_a_declaration(self):  # profile parser, not global regex
        self.edit("01_WORLD.md", "# LUNEX MMORPG — 01 WORLD", "# LUNEX MMORPG — 01 WORLD\n\nСм. `NPC-101` и `MQ-01-01`.")
        res = self.run_lint()
        self.assertEqual(self.errors(res), [])

    def test_missing_reward_fails_schema(self):
        self.edit("08_QUESTS_MAIN.md", "**Награда:** 100 XP, 12 золота, +20 репутации `FAC-08`.", "**Награда:** см. ниже.")
        res = self.run_lint()
        self.assertTrue(any(i["id"] == "MQ-01-01" for i in self.errors(res, "SCHEMA_INVALID")))

    def test_non_ascii_hyphen_in_id(self):
        self.edit("02_FACTIONS.md", "FAC-01 — Союз", "FAC‑01 — Союз")
        self.assertTrue(self.errors(self.run_lint(), "ID_NON_ASCII_HYPHEN"))

    def test_dlg_is_warning_only(self):  # C-DIA-001 A
        self.edit("01_WORLD.md", "# LUNEX MMORPG — 01 WORLD", "# LUNEX MMORPG — 01 WORLD\n\nOld DLG-QUEST-01.")
        res = self.run_lint()
        self.assertTrue(any(i["code"] == "DLG_OBSOLETE" and i["severity"] == "WARN" for i in res["report"].items))
        self.assertEqual(self.errors(res), [])

    def test_dialogue_unknown_quest(self):
        self.edit("10_DIALOGUES.md", "`DIA.MQ-01-01.PROGRESS.01`", "`DIA.MQ-01-99.PROGRESS.01`")
        self.assertTrue(self.errors(self.run_lint(), "SCHEMA_INVALID") or self.errors(self.run_lint(), "DIA_UNKNOWN_QUEST"))

    def test_dungeon_level_mismatch_03_vs_12(self):
        self.edit("12_DUNGEONS.md", "| `DNG-01-001` Склеп Первого Огня | `POI-01-006` | 7 |", "| `DNG-01-001` Склеп Первого Огня | `POI-01-006` | 8 |")
        self.assertTrue(self.errors(self.run_lint(), "DNG_LEVEL_MISMATCH"))


class CrossrefsAndGraphs(Sandbox):  # CT-002..CT-009 (static)
    def codes(self, res):
        return {i["code"] for i in self.errors(res)}

    def test_canon_resolves_all_refs(self):
        res = self.run_lint(allow=None)
        xr = res["crossrefs"]
        self.assertEqual(xr["unresolved"], {})
        self.assertEqual(len(xr["entities"]), 4925)
        self.assertEqual(xr["entities"]["NPC-802"]["kind"], "npc")
        self.assertTrue(xr["entities"]["NPC-802"]["referenced_from"])
        self.assertTrue(res["publishable"])

    def test_evidence_registry_bijection(self):  # C-ITEMQ-001 A
        res = self.run_lint(allow=None)
        self.assertEqual(sum(1 for d in res["decls"] if d["kind"] == "quest_item"), 36)
        self.edit("11_ITEMS_LOOT.md", "| `ITEM-Q-01-02` |", "| `ITEM-Q-01-09` |")
        codes = self.codes(self.run_lint(allow=None))
        self.assertTrue({"CT009_EVIDENCE_UNDECLARED", "CT009_EVIDENCE_UNUSED"} <= codes)

    def test_unresolved_npc_reference(self):
        self.edit("01_WORLD.md", "# LUNEX MMORPG — 01 WORLD", "# LUNEX MMORPG — 01 WORLD\n\nТут `NPC-999` и `LOC-01-099`.")
        res = self.run_lint()
        ids = {i["id"] for i in self.errors(res, "CT002_UNRESOLVED_REF")}
        self.assertEqual(ids, {"NPC-999", "LOC-01-099"})

    def test_old_set_id_not_a_settlement(self):
        self.edit("03_LOCATIONS.md", "| STL-01-01 | Астервиль |", "| SET-01-01 | Астервиль |")
        self.assertEqual(self.run_lint()["counts"]["settlement"], 17)

    def test_ct003_prereq_cycle_and_dead_end(self):
        self.edit("08_QUESTS_MAIN.md", "**Открытие:** `MQ-01-02` завершён", "**Открытие:** `MQ-01-04` завершён")
        res = self.run_lint()
        self.assertIn("CT003_UNREACHABLE", self.codes(res))

    def test_ct003_missing_prereq(self):
        self.edit("08_QUESTS_MAIN.md", "**Открытие:** `MQ-01-02` завершён", "**Открытие:** `MQ-01-77` завершён")
        self.assertIn("CT003_PREREQ_MISSING", self.codes(self.run_lint()))

    def test_ct004_sq_unknown_prereq(self):
        self.edit("09_QUESTS_SIDE.md", "**Открытие:** `completed(MQ-01-03)`", "**Открытие:** `completed(MQ-01-88)`")
        self.assertIn("CT004_PREREQ_MISSING", self.codes(self.run_lint()))

    def test_ct005_bad_ai(self):
        self.edit("06_ENEMIES.md", "| 1 | `melee` | физический | `SPAWN-01-001` |", "| 1 | `flying` | физический | `SPAWN-01-001` |")
        self.assertIn("CT005_BAD_AI", self.codes(self.run_lint()))

    def test_ct005_spawn_wrong_region(self):
        self.edit("06_ENEMIES.md", "| 1 | `melee` | физический | `SPAWN-01-001` |", "| 1 | `melee` | физический | `SPAWN-02-001` |")
        self.assertIn("CT005_SPAWN_REGION", self.codes(self.run_lint()))

    def test_ct006_dungeon_boss_invalid(self):
        self.edit("12_DUNGEONS.md", "`ENM-01-301` Криптарий", "`ENM-01-399` Криптарий")
        self.assertTrue({"CT006_BOSS", "CT002_UNRESOLVED_REF"} & self.codes(self.run_lint()))

    def test_ct007_world_boss_link(self):
        self.edit("13_WORLD_EVENTS.md", "`EVT-01-003` / `ENM-01-401`", "`EVT-01-003` / `ENM-02-401`")
        self.assertIn("CT007_WORLD_BOSS", self.codes(self.run_lint()))

    def test_ct008_dialogue_coverage(self):
        self.edit("10_DIALOGUES.md", "`DIA.MQ-01-01.TURNIN.01`", "`DIA.MQ-01-01.OBJ3.77`")
        self.assertIn("CT008_DIALOGUE_COVERAGE", self.codes(self.run_lint()))

    def test_ct009_loot_probability_over_100(self):
        self.edit("11_ITEMS_LOOT.md", "| 65% | 8% | 1.00% UNCOMMON | нет |", "| 165% | 8% | 1.00% UNCOMMON | нет |")
        self.assertIn("SCHEMA_INVALID", self.codes(self.run_lint()))

    def test_ct009_loot_table_for_unknown_enemy(self):
        self.edit("11_ITEMS_LOOT.md", "Трактовый грабитель (`ENM-01-001`)", "Трактовый грабитель (`ENM-01-099`)")
        self.assertTrue({"CT009_LOOT_ENEMY", "CT002_UNRESOLVED_REF"} & self.codes(self.run_lint()))


class RepoSync(unittest.TestCase):
    def test_client_data_matches_canon(self):
        import check_client_sync
        f = check_client_sync.run(ROOT, CANON)
        self.assertEqual([x for x in f if x[0] == "ERROR"], [])

    def test_client_sync_detects_drift(self):
        import check_client_sync
        tmp = tempfile.mkdtemp(prefix="cs_")
        try:
            shutil.copytree(os.path.join(ROOT, "client", "data"), os.path.join(tmp, "client", "data"))
            shutil.copytree(os.path.join(ROOT, "content"), os.path.join(tmp, "content"))
            qp = os.path.join(tmp, "client", "data", "quests", "mq_01_01_to_05.json")
            with open(qp, encoding="utf-8") as fh:
                q = json.load(fh)
            q["quests"][0]["reward"]["xp"] = 999
            q["quests"][1]["objectives"][0]["id"] = "OBJ-01-02-9"
            with open(qp, "w", encoding="utf-8") as fh:
                json.dump(q, fh, ensure_ascii=False)
            codes = {x[1] for x in check_client_sync.run(tmp, CANON) if x[0] == "ERROR"}
            self.assertTrue({"CS_REWARD", "CS_OBJ"} <= codes)
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    def test_registry_drift_detected(self):
        import check_client_sync
        tmp = tempfile.mkdtemp(prefix="cs_")
        try:
            shutil.copytree(os.path.join(ROOT, "client", "data"), os.path.join(tmp, "client", "data"))
            shutil.copytree(os.path.join(ROOT, "content"), os.path.join(tmp, "content"))
            rp = os.path.join(tmp, "content", "interactables", "reg-01.json")
            with open(rp, encoding="utf-8") as fh:
                r = json.load(fh)
            r["interactables"][1]["target_id"] = "POI-OTHER"
            r["interactables"][9]["instances"] = ["only-one"]
            r["interactables"][2]["canon_poi"] = "POI-01-099"
            with open(rp, "w", encoding="utf-8") as fh:
                json.dump(r, fh, ensure_ascii=False)
            codes = {x[1] for x in check_client_sync.run(tmp, CANON) if x[0] == "ERROR"}
            self.assertTrue({"CS_REG_DRIFT", "CS_REG_INST", "CS_REG_CANON"} <= codes)
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    def test_docs_links(self):
        import check_docs_links
        self.assertEqual(check_docs_links.check(ROOT), [])


class MiniSchema(unittest.TestCase):
    def test_keywords(self):
        s = {"type": "object", "required": ["a"], "properties": {"a": {"type": "integer", "minimum": 1},
             "b": {"type": "string", "pattern": "^x"}, "c": {"enum": ["u", "v"]}, "d": {"type": "array", "minItems": 1}}}
        self.assertEqual(js.validate({"a": 1, "b": "xy", "c": "u", "d": [1]}, s), [])
        self.assertEqual(len(js.validate({"a": 0, "b": "y", "c": "z", "d": []}, s)), 4)
        self.assertEqual(len(js.validate({}, s)), 1)
        self.assertTrue(js.validate({"a": True}, s))  # bool is not integer

    def test_markdown_ast_tables_and_fences(self):
        b = md_ast.parse("# T\n\n| A | B |\n|---|---|\n| `X-1` | y |\n\n```\n| `FAKE-1` | z |\n```\n- item\n")
        self.assertEqual([x.kind for x in b], ["heading", "table", "code", "list_item"])
        self.assertEqual(b[1].rows[0][1], ["`X-1`", "y"])
        self.assertEqual(b[1].rows[0][0], 5)


if __name__ == "__main__":
    unittest.main()
