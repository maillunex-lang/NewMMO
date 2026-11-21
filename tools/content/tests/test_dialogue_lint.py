import os
import re
import shutil
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, ".."))
import dialogue_lint as dl  # noqa: E402

ROOT = os.path.abspath(os.path.join(HERE, "..", "..", ".."))
CANON = os.path.join(ROOT, "docs", "content")
SCHEMAS = os.path.join(ROOT, "content", "schemas")


class DialogueLint(unittest.TestCase):
    def test_baseline_flags_templated_canon(self):
        f, meta = dl.run(CANON, SCHEMAS, ["MQ-01-01"])
        rules = {x["rule"] for x in f if x["severity"] == "ERROR"}
        self.assertTrue({"DIA-STAMP", "DIA-EXACT"} <= rules)
        self.assertEqual(meta["quests"], 1)
        self.assertFalse([x for x in f if x["rule"] in ("DIA-STAGE", "DIA-SECRET")])

    def test_rewritten_quest_passes(self):
        tmp = tempfile.mkdtemp(prefix="dl_")
        try:
            c = os.path.join(tmp, "c")
            shutil.copytree(CANON, c)
            p = os.path.join(c, "10_DIALOGUES.md")
            with open(p, encoding="utf-8") as fh:
                lines = fh.read().split("\n")
            n = 0
            for i, ln in enumerate(lines):
                m = re.match(r"^(- `DIA\.MQ-01-01\.[A-Z0-9]+\.\d+`.*?«)(.*?)(».*)$", ln)
                if m:
                    n += 1
                    lines[i] = f"{m.group(1)}Уникальная живая реплика номер {n}, не похожая на соседние.{m.group(3)}"
            with open(p, "w", encoding="utf-8") as fh:
                fh.write("\n".join(lines))
            f, _ = dl.run(c, SCHEMAS, ["MQ-01-01"])
            self.assertEqual([x for x in f if x["severity"] == "ERROR"], [])
            self.assertTrue([x for x in f if x["rule"] == "DIA-PROGRESS"])  # single PROGRESS still warned
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    def test_secret_marker_in_early_chapter(self):
        tmp = tempfile.mkdtemp(prefix="dl_")
        try:
            c = os.path.join(tmp, "c")
            shutil.copytree(CANON, c)
            p = os.path.join(c, "10_DIALOGUES.md")
            with open(p, encoding="utf-8") as fh:
                t = fh.read()
            t = t.replace("«Проверю это на месте.»", "«Цена Договора — память города.»", 1)
            with open(p, "w", encoding="utf-8") as fh:
                fh.write(t)
            f, _ = dl.run(c, SCHEMAS, ["MQ-01-01", "MQ-01-02"])
            self.assertTrue([x for x in f if x["rule"] == "DIA-SECRET"])
        finally:
            shutil.rmtree(tmp, ignore_errors=True)


if __name__ == "__main__":
    unittest.main()
