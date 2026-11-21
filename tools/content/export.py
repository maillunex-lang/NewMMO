#!/usr/bin/env python3
"""Content export bundle for the server loader (EX-P03 / contract D-001). Stdlib only.

Runs lint first; any ERROR => nothing is written, exit 1 (publish blocked).
Output dir (default build/content/export/):
    manifest.json   - metadata + hashes (see docs/contracts/content-export.md)
    entities.jsonl  - one canonical JSON entity per line, sorted by (kind, id)
    crossrefs.json  - ID registry + references (copy of lint output)
Deterministic: same canon => byte-identical files and the same build_id.

    python3 tools/content/export.py [--out build/content/export]
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import lint  # noqa: E402

SCHEMA_VERSION = 1
FORMAT = "newmmo.content.export"
MIN_CLIENT_VERSION = "0.0.1"


def canon_json(obj) -> str:
    return json.dumps(obj, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def sha256_hex(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def region_of(entity_id: str, fields: dict):
    if entity_id.startswith("REG-"):
        return entity_id
    if re.match(r"^(MQ|SQ|OBJ|SQO|LOC|POI|STL|ENM|DNG|EVT|MAT|ITM|REC|LOOT-ENM|ITEM-Q|ITEM-SQ|SPAWN)-0[1-6]-", entity_id):
        return "REG-" + re.search(r"-(0[1-6])-", entity_id).group(1)
    m = re.match(r"^DIA\.(?:MQ|SQ)-(0[1-6])-", entity_id)
    if m:
        return "REG-" + m.group(1)
    m = re.match(r"^DIA\.REG-(0[1-6])\.", entity_id)
    if m:
        return "REG-" + m.group(1)
    m = re.match(r"^EQ-0[1-5]-(0[1-6])-", entity_id)
    if m:
        return "REG-" + m.group(1)
    for key in ("region", "location"):  # typed field fallback (chapters, settlements, NPCs)
        v = fields.get(key)
        m = isinstance(v, str) and re.search(r"(REG-0[1-6])|LOC-(0[1-6])-", v)
        if m:
            return m.group(1) or "REG-" + m.group(2)
    return None


def build_entities(decls):
    """Entities = declarations (not *_ref mirrors) + quest objectives."""
    rows = []
    for d in decls:
        if d["kind"].endswith("_ref"):
            continue
        rows.append(d)
        if d["kind"] in ("quest_main", "quest_side"):
            for o in d["fields"]["objectives"]:
                rows.append({"kind": "objective", "id": o["id"], "file": d["file"], "line": o["line"],
                             "fields": {"quest_id": d["id"], "text": o["text"]}})
    out = []
    for d in rows:
        out.append({
            "schema_version": SCHEMA_VERSION,
            "id": d["id"],
            "kind": d["kind"],
            "region_id": region_of(d["id"], d["fields"]),
            "source_file": d["file"],
            "source_line": d["line"],
            "payload": d["fields"],
        })
    out.sort(key=lambda e: (e["kind"], e["id"]))
    return out


def export(content_dir, schemas_dir, allowlist, out_dir):
    res = lint.run(content_dir, schemas_dir, allowlist)
    rep = res["report"]
    if not res["publishable"]:
        return None, rep
    entities = build_entities(res["decls"])
    lines = [canon_json(e) for e in entities]
    entities_bytes = ("\n".join(lines) + "\n").encode("utf-8")
    xrefs_bytes = (json.dumps(res["crossrefs"], ensure_ascii=False, indent=1, sort_keys=True) + "\n").encode("utf-8")
    counts = {}
    for e in entities:
        counts[e["kind"]] = counts.get(e["kind"], 0) + 1
    body = {
        "format": FORMAT,
        "schema_version": SCHEMA_VERSION,
        "minimum_client_version": MIN_CLIENT_VERSION,
        "source_files": [{"file": f["file"], "sha256": f["sha256"]} for f in sorted(res["manifest_files"], key=lambda x: x["file"])],
        "entity_count": len(entities),
        "counts": dict(sorted(counts.items())),
        "entities_sha256": sha256_hex(entities_bytes),
        "crossrefs_sha256": sha256_hex(xrefs_bytes),
        "lint": {"errors": 0, "warnings": rep.count("WARN")},
    }
    manifest_sha = sha256_hex(canon_json(body).encode("utf-8"))
    manifest = dict(body, manifest_sha256=manifest_sha, build_id=manifest_sha)
    os.makedirs(out_dir, exist_ok=True)
    with open(os.path.join(out_dir, "entities.jsonl"), "wb") as fh:
        fh.write(entities_bytes)
    with open(os.path.join(out_dir, "crossrefs.json"), "wb") as fh:
        fh.write(xrefs_bytes)
    with open(os.path.join(out_dir, "manifest.json"), "w", encoding="utf-8") as fh:
        json.dump(manifest, fh, ensure_ascii=False, indent=2, sort_keys=True)
        fh.write("\n")
    return manifest, rep


def main(argv=None):
    root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--content-dir", default=os.path.join(root, "docs", "content"))
    ap.add_argument("--schemas", default=os.path.join(root, "content", "schemas"))
    ap.add_argument("--allowlist", default=os.path.join(root, "content", "lint_allowlist.json"))
    ap.add_argument("--out", default=os.path.join(root, "build", "content", "export"))
    a = ap.parse_args(argv)
    manifest, rep = export(a.content_dir, a.schemas, a.allowlist, a.out)
    if manifest is None:
        for it in rep.items:
            if it["severity"] == "ERROR":
                print(f"ERROR {it['code']} {it['file']}:{it['line']} {it['id'] or ''} {it['message']}")
        print("export: BLOCKED by lint errors; nothing written")
        return 1
    print(f"export: build_id={manifest['build_id']} entities={manifest['entity_count']} -> {a.out}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
