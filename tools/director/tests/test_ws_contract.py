"""Consistency tests for docs/contracts/websocket-protocol.md (D-003)."""
import json
import os
import re
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", "..", ".."))
sys.path.insert(0, os.path.join(ROOT, "tools", "content"))
import schema as js  # noqa: E402

with open(os.path.join(ROOT, "docs", "contracts", "websocket-envelope.schema.json"), encoding="utf-8") as fh:
    ENV = json.load(fh)["definitions"]
with open(os.path.join(ROOT, "docs", "contracts", "websocket-protocol.md"), encoding="utf-8") as fh:
    PROTO = fh.read()
with open(os.path.join(ROOT, "docs", "content", "14_CONTENT_INTEGRATION.md"), encoding="utf-8") as fh:
    CANON = fh.read()

UUID = "3f2b8c1e-5d4a-4e7b-9a10-0c6d2b7f8e11"
S2C = {"protocol_version": 1, "type": "quest.accept.result", "request_id": UUID, "session_id": "s-1",
       "server_time": "2026-11-21T08:00:00.123Z", "revision": 7, "trace_id": "t-1",
       "payload": {"quest_id": "MQ-01-01", "state": "ACTIVE", "revision": 7}}
C2S = {"protocol_version": 1, "type": "quest.accept.request", "request_id": UUID,
       "payload": {"quest_id": "MQ-01-01", "content_build_id": "0" * 64}}


class WsContract(unittest.TestCase):
    def test_examples_valid(self):
        self.assertEqual(js.validate(C2S, ENV["client_to_server"]), [])
        self.assertEqual(js.validate(S2C, ENV["server_to_client"]), [])

    def test_invalid_variants(self):
        bad = [dict(C2S, protocol_version=2), dict(C2S, request_id="not-a-uuid"),
               {k: v for k, v in C2S.items() if k != "payload"}, dict(C2S, type="Quest")]
        for b in bad:
            self.assertTrue(js.validate(b, ENV["client_to_server"]), b)
        for k in ("session_id", "server_time", "revision", "trace_id"):
            self.assertTrue(js.validate({x: y for x, y in S2C.items() if x != k}, ENV["server_to_client"]), k)
        self.assertTrue(js.validate(dict(S2C, revision=2 ** 53), ENV["server_to_client"]))
        self.assertTrue(js.validate(dict(S2C, server_time="yesterday"), ENV["server_to_client"]))

    def test_all_canon_events_present_in_catalogue(self):
        sec = CANON[CANON.index("### 15.2"):CANON.index("### 15.3")]
        canon_types = set(re.findall(r"^\| `([a-z]+\.[a-z.]+)` \|", sec, re.M))
        self.assertGreaterEqual(len(canon_types), 14)
        cat = PROTO[PROTO.index("## 5."):PROTO.index("## 6.")]
        have = set(re.findall(r"^\| `([a-z]+\.[a-z._]+)`", cat, re.M))
        self.assertEqual(canon_types - have, set(), "every canon §15.2 event must be in the contract")

    def test_canon_directions_match(self):
        sec = CANON[CANON.index("### 15.2"):CANON.index("### 15.3")]
        canon = dict(re.findall(r"^\| `([a-z]+\.[a-z.]+)` \| (C->S|S->C) \|", sec, re.M))
        cat = PROTO[PROTO.index("## 5."):PROTO.index("## 6.")]
        mine = {m[0]: m[1] for m in re.findall(r"^\| `([a-z]+\.[a-z._]+)`[^|]*\| (C→S|S→C) \|", cat, re.M)}
        for t, d in canon.items():
            self.assertEqual(mine[t].replace("→", "->"), d, t)

    def test_error_codes_and_limits_documented(self):
        for token in ("IDEMPOTENCY_CONFLICT", "RATE_LIMITED", "`1013`", "16 KiB", "newmmo.v1", "coder/websocket"):
            self.assertIn(token, PROTO)


if __name__ == "__main__":
    unittest.main()
