"""T-038a: enemy abilities and class skills extracted from canon (kinds `ability`, `skill`) + CT-COMBAT checks."""
import os
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
from test_content import Sandbox  # noqa: E402


class CombatData(Sandbox):
    def decl(self, res, id_):
        return next(d for d in res["decls"] if d["id"] == id_)

    def codes(self, res):
        return {i["code"] for i in res["report"].items if i["severity"] == "ERROR"} if hasattr(res["report"], "items") \
            and not callable(res["report"].items) else {i["code"] for i in res["report"]["items"] if i["severity"] == "ERROR"}

    def test_real_canon_volumes_and_values(self):
        res = self.run_lint()
        kinds = {}
        for d in res["decls"]:
            kinds[d["kind"]] = kinds.get(d["kind"], 0) + 1
        self.assertEqual(kinds["skill"], 60)
        self.assertEqual(kinds["ability"], 258)
        b = self.decl(res, "AB-ENM-01-001-B")["fields"]
        self.assertEqual((b["coefficient"], b["scale"], b["cooldown_s"], b["telegraph_s"]), (1.05, "AP", 14.0, 0.7))
        a = self.decl(res, "AB-ENM-01-001-A")["fields"]
        self.assertEqual((a["coefficient"], a["cooldown_s"]), (0.6, 2.0))
        sk = self.decl(res, "CLS-01-SK02")["fields"]
        self.assertEqual((sk["effect_kind"], sk["coefficient"], sk["scale"], sk["level"]), ("shield", 0.18, "MaxHP", 3))
        heal = self.decl(res, "CLS-05-SK02")["fields"]
        self.assertEqual((heal["effect_kind"], heal["scale"]), ("heal", "SP"))  # not mistaken for damage

    def test_missing_ability_numbers_is_an_error(self):
        self.edit("06_ENEMIES.md", "«Рывок клинка», `1.05*AP` или `1.05*SP` (физический), CD 14 с;",
                  "«Рывок клинка», (физический);")
        res = self.run_lint()
        self.assertIn("CT_COMBAT_ABILITY_NUMBERS", self.codes(res))

    def test_wrong_skill_unlock_level_is_an_error(self):
        self.edit("05_CLASSES.md", "| `CLS-01-SK02` | Железная клятва | 3 |", "| `CLS-01-SK02` | Железная клятва | 4 |")
        res = self.run_lint()
        self.assertTrue({"SCHEMA_INVALID", "CT_COMBAT_SKILL_LEVELS"} & self.codes(res))


if __name__ == "__main__":
    unittest.main()
