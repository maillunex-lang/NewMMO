#!/usr/bin/env python3
"""UI / FX concept images through Meshy text-to-image (nano-banana, 3 credits each).
usage: make_images.py   (set of images below -> client/assets/ui/gen/*.png and client/assets/fx/*.png)"""
import os
import sys
import time

sys.path.insert(0, os.path.dirname(__file__))
import meshy  # noqa: E402

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
S = "vibrant saturated painterly fantasy MMO game art, clean, no text, no letters"
IMAGES = {
    "ui/gen/panel": f"ornate dark navy blue UI panel background with a thin gold filigree border and subtle parchment texture inside, flat front view, seamless center, {S}",
    "ui/gen/button": f"fantasy UI button plate, deep blue with gold trim and small gems on the ends, flat front view on dark background, {S}",
    "ui/gen/portrait_frame": f"round golden ornate portrait frame with a small shield crest on top, empty dark center, flat front view, {S}",
    "ui/gen/login_bg": f"epic fantasy valley at sunrise, golden light rays, rolling green hills, a village with red roofs, a distant castle, wide landscape illustration, {S}",
    "ui/gen/logo": f"fantasy game logo emblem, a glowing blue moon crystal with golden wings, dark background, centered, {S}",
    "ui/gen/loading_bg": f"magical glowing forest path with fireflies at dusk, wide landscape, {S}",
    "ui/gen/bar_frame": f"horizontal ornate golden frame for a health bar, dark empty inside, flat front view, {S}",
    "ui/gen/quest_banner": f"parchment scroll banner with golden rolled ends, empty center, flat front view, {S}",
    "fx/rune_a": f"glowing magic circle sigil on pure black background, top down view, intricate runes, cyan and violet light, {S}",
    "fx/rune_b": f"glowing golden holy sun-sigil magic circle on pure black background, top down view, {S}",
    "fx/rune_c": f"glowing red-orange fire sigil magic circle on pure black background, top down view, {S}",
    "fx/burst": f"bright magical energy burst explosion with sparks, centered, on pure black background, {S}",
    "fx/slash": f"glowing white-blue crescent sword slash trail arc on pure black background, {S}",
    "fx/sparkle": f"cluster of glowing golden sparkles and stars on pure black background, {S}",
}
jobs = {}
for name, prompt in IMAGES.items():
    code, r = meshy.call("POST", "/v1/text-to-image", {"ai_model": "nano-banana", "prompt": prompt})
    if code < 300:
        jobs[name] = r["result"]
        print("queued", name)
    else:
        print("ERR", name, code, r)
done = set()
while len(done) < len(jobs):
    for name, tid in jobs.items():
        if name in done:
            continue
        r = meshy.call("GET", f"/v1/text-to-image/{tid}")[1]
        if r.get("status") in ("SUCCEEDED", "FAILED"):
            done.add(name)
            if r["status"] == "SUCCEEDED":
                path = os.path.join(ROOT, "client/assets", name + ".png")
                os.makedirs(os.path.dirname(path), exist_ok=True)
                meshy.download(r["image_urls"][0], path)
                print("saved", name)
    time.sleep(5)
print("balance", meshy.balance())
