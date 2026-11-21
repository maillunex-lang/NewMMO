import json
import os
import shutil
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, ".."))
import check_dungeon_registry as cdr  # noqa: E402

ROOT = os.path.abspath(os.path.join(HERE, "..", "..", ".."))
SRC = os.path.join(ROOT, "content", "dungeons", "dng-01-001.json")


class DungeonRegistry(unittest.TestCase):
    def _run_with(self, mutate):
        tmp = tempfile.mkdtemp()
        try:
            os.makedirs(os.path.join(tmp, "content", "dungeons"))
            os.symlink(os.path.join(ROOT, "content", "schemas"), os.path.join(tmp, "content", "schemas"))
            d = json.load(open(SRC, encoding="utf-8"))
            mutate(d)
            json.dump(d, open(os.path.join(tmp, "content", "dungeons", "x.json"), "w", encoding="utf-8"))
            return [c for c, _ in cdr.run(tmp, os.path.join(ROOT, "docs", "content"))]
        finally:
            shutil.rmtree(tmp)

    def test_repo_definitions_clean(self):
        self.assertEqual(cdr.run(ROOT, os.path.join(ROOT, "docs", "content")), [])

    def test_unknown_enemy_and_wrong_boss_detected(self):
        def m(d):
            d["rooms"][1]["spawns"][0]["enemy_id"] = "ENM-01-999"
            d["boss_id"] = "ENM-01-302"
        codes = self._run_with(m)
        self.assertIn("DR_ENEMY", codes)
        self.assertIn("DR_HEADER", codes)

    def test_graph_problems_detected(self):
        def m(d):
            d["rooms"].pop()
            d["checkpoint_room_ids"] = ["DNG-01-001-R05"]
            d["puzzle"]["order"] = ["OBJ-DNG-01-001-A"]
        codes = self._run_with(m)
        self.assertIn("DR_ROOMS", codes)
        self.assertIn("DR_CHECKPOINT", codes)
        self.assertIn("DR_PUZZLE", codes)


if __name__ == "__main__":
    unittest.main()
