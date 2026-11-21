#!/usr/bin/env python3
"""Runs the asset manifest through Meshy: preview -> refine (PBR) -> download GLB into client/assets/models.
State (task ids, credits) is kept in tools/meshy/state.json so the run is resumable. No secrets involved.
usage: batch.py preview [names...] | refine [names...] | fetch [names...] | status"""
import json
import os
import sys
import time

sys.path.insert(0, os.path.dirname(__file__))
import meshy  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
MAN = json.load(open(os.path.join(HERE, "assets.json"), encoding="utf-8"))
STATE_P = os.path.join(HERE, "state.json")
STATE = json.load(open(STATE_P)) if os.path.exists(STATE_P) else {}


def save():
    json.dump(STATE, open(STATE_P, "w"), indent=1, sort_keys=True)


def names(a):
    return a if a else list(MAN["assets"].keys())


def preview(ns):
    for n in ns:
        if "preview" in STATE.get(n, {}):
            continue
        a = MAN["assets"][n]
        before = meshy.balance()
        code, r = meshy.call("POST", "/v2/text-to-3d", {"mode": "preview", "prompt": a["prompt"] + ". " + MAN["style"], "ai_model": "latest",
                                                         "topology": "triangle", "target_polycount": a["poly"], "should_remesh": True})
        if code >= 300:
            print(n, "ERR", code, r)
            continue
        STATE.setdefault(n, {})["preview"] = r["result"]
        STATE[n]["spent"] = int(r.get("consumed_credits", 0))
        print(n, "preview", r["result"], "credits", r.get("consumed_credits"), "balance_before", before)
        save()


def wait_all(ns, key, endpoint):
    pending = [n for n in ns if key in STATE.get(n, {}) and not STATE[n].get(key + "_done")]
    while pending:
        for n in list(pending):
            r = meshy.get(endpoint, STATE[n][key])
            if r.get("status") in ("SUCCEEDED", "FAILED", "CANCELED"):
                STATE[n][key + "_done"] = r["status"]
                STATE[n][key + "_urls"] = r.get("model_urls", {})
                print(n, key, r["status"])
                pending.remove(n)
                save()
        if pending:
            time.sleep(10)


def refine(ns):
    for n in ns:
        s = STATE.get(n, {})
        if s.get("preview_done") != "SUCCEEDED" or "refine" in s:
            continue
        a = MAN["assets"][n]
        code, r = meshy.call("POST", "/v2/text-to-3d", {"mode": "refine", "preview_task_id": s["preview"], "enable_pbr": True,
                                                         "texture_prompt": a.get("tex", "") + ", vibrant saturated colors, hand-painted"})
        if code >= 300:
            print(n, "refine ERR", code, r)
            continue
        s["refine"] = r["result"]
        s["spent"] = s.get("spent", 0) + int(r.get("consumed_credits", 0))
        print(n, "refine", r["result"], r.get("consumed_credits"))
        save()


def fetch(ns):
    for n in ns:
        s = STATE.get(n, {})
        url = s.get("refine_urls", {}).get("glb")
        if s.get("refine_done") == "SUCCEEDED" and url:
            p = os.path.join(ROOT, "client", "assets", "models", n + ".glb")
            print(n, "->", p, meshy.download(url, p), "bytes")


if __name__ == "__main__":
    cmd, ns = sys.argv[1], names(sys.argv[2:])
    if cmd == "preview":
        preview(ns)
        wait_all(ns, "preview", "text-to-3d")
    elif cmd == "refine":
        refine(ns)
        wait_all(ns, "refine", "text-to-3d")
    elif cmd == "fetch":
        fetch(ns)
    elif cmd == "status":
        print(json.dumps(STATE, indent=1))
        print("balance", meshy.balance())
