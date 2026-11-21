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
        self.assertTrue(res["publishable"])
        self.assertEqual(res["counts"]["settlement"], 18)
        self.assertTrue(all(d["id"].startswith("STL-") for d in res["decls"] if d["kind"] == "settlement"))

    def test_repo_allowlist_is_empty(self):
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
