import json
import os
import shutil
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, ".."))
import check_zone_defs as czd  # noqa: E402

ROOT = os.path.abspath(os.path.join(HERE, "..", "..", ".."))
SRC = os.path.join(ROOT, "content", "zones", "dawn-valley.json")


class ZoneDefs(unittest.TestCase):
    def _codes(self, mutate):
        tmp = tempfile.mkdtemp()
        try:
            os.makedirs(os.path.join(tmp, "content", "zones"))
            os.symlink(os.path.join(ROOT, "content", "schemas"), os.path.join(tmp, "content", "schemas"))
            d = json.load(open(SRC, encoding="utf-8"))
            mutate(d)
            json.dump(d, open(os.path.join(tmp, "content", "zones", "z.json"), "w", encoding="utf-8"))
            return [c for c, _ in czd.run(tmp, os.path.join(ROOT, "docs", "content"))]
        finally:
            shutil.rmtree(tmp)

    def test_repo_zone_is_clean(self):
        self.assertEqual(czd.run(ROOT, os.path.join(ROOT, "docs", "content")), [])

    def test_problems_are_detected(self):
        def m(d):
            d["tick_rate"] = 120
            d["enemies"][0]["enemy_id"] = "ENM-01-999"
            d["enemies"][1]["x"] = 99999
            d["enemies"][2]["spawn_id"] = d["enemies"][3]["spawn_id"]
        codes = self._codes(m)
        for want in ("ZD_TICK", "ZD_ENEMY", "ZD_SPAWN", "ZD_DUP"):
            self.assertIn(want, codes)


if __name__ == "__main__":
    unittest.main()
