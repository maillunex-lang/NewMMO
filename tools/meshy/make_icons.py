#!/usr/bin/env python3
"""Skill icons through Meshy text-to-image (nano-banana, 3 credits each): first N skills of every class.
Round-masked, 128 px, saved to client/assets/ui/skills/<ID>.png (replaces the procedural placeholder).
usage: make_icons.py [per_class=6]"""
import json
import os
import sys
import time

from PIL import Image, ImageDraw

sys.path.insert(0, os.path.dirname(__file__))
import meshy  # noqa: E402

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
OUT = os.path.join(ROOT, "client/assets/ui/skills")
CLS = {"CLS-01": "knight guardian", "CLS-02": "berserker warrior", "CLS-03": "arcane mage", "CLS-04": "ranger hunter", "CLS-05": "light oracle priest"}
KIND = {"damage": "attack", "shield": "protective shield", "heal": "healing", "haste": "speed buff", "buff": "power buff", "teleport": "teleport",
        "mark": "target mark", "cleanse": "purifying", "movement": "dash", "stealth": "stealth", "reveal": "reveal", "mana": "mana", "barrier": "barrier",
        "mitigation": "damage reduction", "intercept": "guarding"}
N = int(sys.argv[1]) if len(sys.argv) > 1 else 6
START = int(sys.argv[2]) if len(sys.argv) > 2 else 0
skills = json.load(open(os.path.join(ROOT, "client/data/skills.json"), encoding="utf-8"))["classes"]
jobs = {}
for cls, lst in skills.items():
    for s in lst[START:N]:
        prompt = (f"fantasy MMO skill icon, {KIND.get(s['effect_kind'], 'magic')} ability named '{s['name']}' for a {CLS[cls]}, "
                  "one centered glowing emblem inside a round golden frame, vibrant saturated colors, painterly stylized game art, dark background, no text, no letters")
        code, r = meshy.call("POST", "/v1/text-to-image", {"ai_model": "nano-banana", "prompt": prompt})
        if code < 300:
            jobs[s["id"]] = r["result"]
            print("queued", s["id"], s["name"])
done = {}
while len(done) < len(jobs):
    for sid, tid in jobs.items():
        if sid in done:
            continue
        r = meshy.call("GET", f"/v1/text-to-image/{tid}")[1]
        if r.get("status") in ("SUCCEEDED", "FAILED"):
            done[sid] = r
            if r["status"] == "SUCCEEDED":
                tmp = f"/tmp/claude-0/ico_{sid}.png"
                meshy.download(r["image_urls"][0], tmp)
                im = Image.open(tmp).convert("RGBA")
                w = im.width
                c = w // 2
                rad = int(w * 0.445)
                mask = Image.new("L", (w * 2, w * 2), 0)
                ImageDraw.Draw(mask).ellipse([c * 2 - rad * 2, c * 2 - rad * 2, c * 2 + rad * 2, c * 2 + rad * 2], fill=255)
                mask = mask.resize((w, w), Image.LANCZOS)
                im.putalpha(mask)
                im = im.crop((c - rad, c - rad, c + rad, c + rad)).resize((128, 128), Image.LANCZOS)
                im.save(os.path.join(OUT, sid + ".png"), optimize=True)
                print("saved", sid)
            else:
                print("FAILED", sid)
    time.sleep(5)
print("balance", meshy.balance())
