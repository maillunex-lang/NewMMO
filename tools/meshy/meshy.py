#!/usr/bin/env python3
"""Tiny Meshy API client for the Lunex 3D asset pipeline (no secrets in the repo: the environment's network
proxy injects the Authorization header for api.meshy.ai; locally set MESHY_API_KEY to use your own key).

  meshy.py balance
  meshy.py text3d "<prompt>" [--polycount N] [--out name]    # preview task, prints task id
  meshy.py refine <preview_task_id> "<texture prompt>"        # PBR texture pass
  meshy.py rig <task_id>                                     # humanoid auto-rig
  meshy.py anim <rig_task_id> <action_id>
  meshy.py wait <endpoint> <task_id>                         # poll until SUCCEEDED/FAILED
  meshy.py get <endpoint> <task_id> [--download name]        # print JSON, optionally download the GLB
Endpoints: text-to-3d (v2), rigging (v1), animations (v1).
Every call's credit delta is appended to docs/research/3d-asset-budget.md by `ledger`.
"""
import json
import os
import subprocess
import sys
import time

BASE = "https://api.meshy.ai/openapi"


def call(method, path, body=None):
    cmd = ["curl", "-sS", "-m", "120", "-X", method, BASE + path, "-H", "Content-Type: application/json", "-w", "\n%{http_code}"]
    if os.environ.get("MESHY_API_KEY"):
        cmd += ["-H", "Authorization: Bearer " + os.environ["MESHY_API_KEY"]]
    if body is not None:
        cmd += ["-d", json.dumps(body)]
    out = subprocess.run(cmd, capture_output=True, text=True).stdout
    text, _, code = out.rpartition("\n")
    try:
        return int(code), json.loads(text)
    except ValueError:
        return int(code or 0), {"raw": text}


def balance():
    return call("GET", "/v1/balance")[1].get("balance")


def version(endpoint):
    return "v2" if endpoint == "text-to-3d" else "v1"


def get(endpoint, task_id):
    return call("GET", f"/{version(endpoint)}/{endpoint}/{task_id}")[1]


def wait(endpoint, task_id, every=10, limit=900):
    t0 = time.time()
    while time.time() - t0 < limit:
        r = get(endpoint, task_id)
        st = r.get("status")
        print(f"  {endpoint} {task_id[:8]} {st} {r.get('progress')}%", flush=True)
        if st in ("SUCCEEDED", "FAILED", "CANCELED"):
            return r
        time.sleep(every)
    return r


def download(url, path):
    os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
    subprocess.run(["curl", "-sS", "-L", "-m", "300", "-o", path, url], check=True)
    return os.path.getsize(path)


def main(a):
    if a[0] == "balance":
        print(balance())
    elif a[0] == "text3d":
        poly = int(a[a.index("--polycount") + 1]) if "--polycount" in a else 8000
        body = {"mode": "preview", "prompt": a[1], "ai_model": "latest", "topology": "triangle", "target_polycount": poly, "should_remesh": True}
        code, r = call("POST", "/v2/text-to-3d", body)
        print(code, r)
    elif a[0] == "refine":
        body = {"mode": "refine", "preview_task_id": a[1], "enable_pbr": True}
        if len(a) > 2:
            body["texture_prompt"] = a[2]
        print(*call("POST", "/v2/text-to-3d", body))
    elif a[0] == "rig":
        print(*call("POST", "/v1/rigging", {"input_task_id": a[1]}))
    elif a[0] == "anim":
        print(*call("POST", "/v1/animations", {"rig_task_id": a[1], "action_id": int(a[2])}))
    elif a[0] == "wait":
        print(json.dumps(wait(a[1], a[2]), indent=1)[:1500])
    elif a[0] == "get":
        r = get(a[1], a[2])
        print(json.dumps(r, indent=1)[:2000])
        if "--download" in a and r.get("model_urls", {}).get("glb"):
            print("downloaded", download(r["model_urls"]["glb"], a[a.index("--download") + 1]))


if __name__ == "__main__":
    main(sys.argv[1:])
