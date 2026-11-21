import json
import os
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, ".."))
import lane_check as lc  # noqa: E402

with open(lc.LANES, encoding="utf-8") as fh:
    CFG = json.load(fh)
T = {"Lane": ["client"], "Task": ["T-001"]}


class LaneCheck(unittest.TestCase):
    def test_glob(self):
        self.assertTrue(lc.matches("client/scripts/a.gd", ["client/**"]))
        self.assertFalse(lc.matches("clientx/a", ["client/**"]))
        self.assertTrue(lc.matches("docs/ROADMAP_EXECUTION.md", ["docs/ROADMAP*.md"]))
        self.assertTrue(lc.matches("a/b/c.json", ["**/c.json"]))

    def test_lanes_do_not_overlap(self):
        lanes = {k: v["write"] for k, v in CFG["lanes"].items() if k != "director"}
        probes = ["client/x", "server/x", "deploy/x", "docs/research/x", "docs/reviews/x"]
        for p in probes:
            self.assertLessEqual(sum(lc.matches(p, w) for w in lanes.values()), 1, p)

    def test_ok(self):
        self.assertEqual(lc.check_commit(CFG, ["client/scripts/x.gd"], T), [])

    def test_missing_lane(self):
        self.assertTrue(lc.check_commit(CFG, ["client/x"], {}))

    def test_missing_task(self):
        self.assertTrue(lc.check_commit(CFG, ["client/x"], {"Lane": ["client"]}))

    def test_cross_lane(self):
        v = lc.check_commit(CFG, ["server/main.go"], T)
        self.assertTrue(any("outside lane" in x for x in v))

    def test_protected_paths(self):
        for f in ["docs/content/08_QUESTS_MAIN.md", "AGENTS.md", ".github/workflows/ci.yml", "tools/content/lint.py",
                  "client/data/quests/mq_01_01_to_05.json", "content/schemas/npc.schema.json"]:
            v = lc.check_commit(CFG, [f], T)
            self.assertTrue(v, f)

    def test_extra_write_grants_protected(self):
        t = dict(T, **{"Extra-Write": ["client/data/dialogues/**"]})
        self.assertEqual(lc.check_commit(CFG, ["client/data/dialogues/a.json"], t), [])
        self.assertTrue(lc.check_commit(CFG, ["client/data/quests/a.json"], t))

    def test_own_task_research_note_allowed_everywhere(self):
        self.assertEqual(lc.check_commit(CFG, ["docs/research/T-001-notes.md"], T), [])
        self.assertTrue(lc.check_commit(CFG, ["docs/research/T-002-notes.md"], T))  # someone else's task
        srv = {"Lane": ["server"], "Task": ["T-005"]}
        self.assertEqual(lc.check_commit(CFG, ["docs/research/T-005-go-health.md", "server/main.go"], srv), [])

    def test_director_unrestricted(self):
        self.assertEqual(lc.check_commit(CFG, ["docs/content/x.md", "AGENTS.md"], {"Lane": ["director"]}), [])

    def test_unknown_lane(self):
        self.assertTrue(lc.check_commit(CFG, ["x"], {"Lane": ["root"]}))

    def test_extra_write_comma_separated(self):
        tr = lc.parse_trailers("m\n\nLane: client\nTask: T-015\nExtra-Write: tools/android/**, tools/smoke_stage1.sh")
        self.assertEqual(tr["Extra-Write"], ["tools/android/**", "tools/smoke_stage1.sh"])
        self.assertEqual(lc.check_commit(CFG, ["tools/android/a.sh", "tools/smoke_stage1.sh", "client/x"], tr), [])
        self.assertTrue(lc.check_commit(CFG, ["tools/content/lint.py"], tr))

    def test_trailer_parse(self):
        tr = lc.parse_trailers("msg\n\nLane: server\nTask: T-002\nExtra-Write: a/**\nCo-Authored-By: x")
        self.assertEqual(tr, {"Lane": ["server"], "Task": ["T-002"], "Extra-Write": ["a/**"]})


if __name__ == "__main__":
    unittest.main()
