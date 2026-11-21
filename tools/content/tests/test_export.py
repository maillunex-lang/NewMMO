"""Export bundle tests (D-001). Run: python3 -m unittest discover -s tools/content/tests -v"""
import hashlib
import json
import os
import shutil
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, ".."))
import export as ex  # noqa: E402

ROOT = os.path.abspath(os.path.join(HERE, "..", "..", ".."))
CANON = os.path.join(ROOT, "docs", "content")
SCHEMAS = os.path.join(ROOT, "content", "schemas")
ALLOW = os.path.join(ROOT, "content", "lint_allowlist.json")


class Export(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="ex_")

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def run_export(self, name, content=CANON):
        out = os.path.join(self.tmp, name)
        m, rep = ex.export(content, SCHEMAS, ALLOW, out)
        return m, out

    def test_bundle_consistent(self):
        m, out = self.run_export("a")
        self.assertEqual(m["entity_count"], 4925)
        with open(os.path.join(out, "entities.jsonl"), "rb") as fh:
            raw = fh.read()
        self.assertEqual(hashlib.sha256(raw).hexdigest(), m["entities_sha256"])
        lines = raw.decode("utf-8").splitlines()
        self.assertEqual(len(lines), m["entity_count"])
        ents = [json.loads(x) for x in lines]
        self.assertEqual(len({e["id"] for e in ents}), len(ents), "entity ids are unique inside a build")
        self.assertEqual(ents, sorted(ents, key=lambda e: (e["kind"], e["id"])))
        for e in ents:
            self.assertEqual(set(e), {"schema_version", "id", "kind", "region_id", "source_file", "source_line", "payload"})
        self.assertEqual(sum(m["counts"].values()), m["entity_count"])
        self.assertEqual(m["build_id"], m["manifest_sha256"])
        with open(os.path.join(out, "manifest.json"), "rb") as fh:
            mraw = fh.read()
        self.assertEqual(hashlib.sha256(mraw).hexdigest(), m["build_id"], "build_id = sha256(raw manifest.json bytes)")
        body = json.loads(mraw.decode("utf-8"))
        self.assertNotIn("build_id", body)
        self.assertNotIn("manifest_sha256", body)
        self.assertEqual(body["entities_sha256"], m["entities_sha256"])
        self.assertTrue(mraw.endswith(b"\n") and mraw.count(b"\n") == 1, "single-line canonical manifest + LF")
        with open(os.path.join(out, "BUILD_ID"), "rb") as fh:
            self.assertEqual(fh.read(), m["build_id"].encode("ascii") + b"\n")
        with open(os.path.join(out, "crossrefs.json"), "rb") as fh:
            self.assertEqual(hashlib.sha256(fh.read()).hexdigest(), m["crossrefs_sha256"])

    def test_deterministic(self):
        m1, o1 = self.run_export("a")
        m2, o2 = self.run_export("b")
        self.assertEqual(m1["build_id"], m2["build_id"])
        for f in ("entities.jsonl", "manifest.json", "crossrefs.json", "BUILD_ID"):
            with open(os.path.join(o1, f), "rb") as a, open(os.path.join(o2, f), "rb") as b:
                self.assertEqual(a.read(), b.read(), f)

    def test_canon_change_changes_build_id(self):
        m1, _ = self.run_export("a")
        c2 = os.path.join(self.tmp, "canon")
        shutil.copytree(CANON, c2)
        p = os.path.join(c2, "08_QUESTS_MAIN.md")
        with open(p, encoding="utf-8") as fh:
            t = fh.read()
        with open(p, "w", encoding="utf-8") as fh:
            fh.write(t.replace("100 XP, 12 золота", "101 XP, 12 золота", 1))
        m2, _ = self.run_export("b", c2)
        self.assertNotEqual(m1["build_id"], m2["build_id"])

    def test_lint_error_blocks_export(self):
        c2 = os.path.join(self.tmp, "canon")
        shutil.copytree(CANON, c2)
        os.remove(os.path.join(c2, "09_QUESTS_SIDE.md"))
        out = os.path.join(self.tmp, "blocked")
        m, rep = ex.export(c2, SCHEMAS, ALLOW, out)
        self.assertIsNone(m)
        self.assertFalse(os.path.exists(out), "nothing written when lint has errors")

    def test_quest_and_dialogue_entities(self):
        _, out = self.run_export("a")
        with open(os.path.join(out, "entities.jsonl"), encoding="utf-8") as fh:
            ents = {json.loads(x)["id"]: json.loads(x) for x in fh}
        q = ents["MQ-01-01"]
        self.assertEqual((q["kind"], q["region_id"]), ("quest_main", "REG-01"))
        self.assertEqual(q["payload"]["reward"]["xp"], 100)
        self.assertEqual(ents["OBJ-01-01-1"]["payload"]["quest_id"], "MQ-01-01")
        self.assertEqual(ents["DIA.MQ-01-01.START.01"]["region_id"], "REG-01")
        self.assertEqual(ents["CH-02"]["region_id"], "REG-02")
        self.assertEqual(ents["STL-01-01"]["kind"], "settlement")


if __name__ == "__main__":
    unittest.main()
