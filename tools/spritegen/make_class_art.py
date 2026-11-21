#!/usr/bin/env python3
"""Generates class-coloured hero sheets and skill icons for the client (own work, no third-party art added).

1. client/assets/characters/{berserker,mage,ranger,oracle}_sheet.png: gradient-map recolour of the existing
   CC-BY «Isometric Hero» guardian sheet (derivative of that pack — attribution already in credits).
2. client/assets/ui/skills/<SKILL-ID>.png (96x96): procedurally drawn rounded icons from client/data/skills.json
   (colour by class / element keyword, glyph by effect_kind).
Deterministic; re-run after changing skills.json.  Usage: python3 tools/spritegen/make_class_art.py
"""
import json
import math
import os

import numpy as np
from PIL import Image, ImageDraw, ImageFilter

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
CL = os.path.join(ROOT, "client")

RAMPS = {  # dark, mid, light (RGB)
    "berserker": ((40, 8, 10), (178, 38, 30), (255, 190, 120)),
    "mage": ((22, 14, 70), (96, 84, 220), (190, 235, 255)),
    "ranger": ((10, 38, 20), (52, 150, 70), (200, 245, 150)),
    "oracle": ((70, 48, 18), (226, 186, 90), (255, 250, 225)),
}


def recolour():
    src = Image.open(os.path.join(CL, "assets/characters/guardian_sheet.png")).convert("RGBA")
    arr = np.asarray(src).astype(np.float32) / 255.0
    lum = (0.299 * arr[..., 0] + 0.587 * arr[..., 1] + 0.114 * arr[..., 2])
    lum = np.clip(lum, 0, 1) ** 0.7  # the source is very dark: lift it
    for name, (dark, mid, light) in RAMPS.items():
        d, m, l = (np.array(c, dtype=np.float32) / 255.0 for c in (dark, mid, light))
        t = lum[..., None]
        lo = d + (m - d) * np.clip(t * 2, 0, 1)
        hi = m + (l - m) * np.clip(t * 2 - 1, 0, 1)
        rgb = np.where(t < 0.5, lo, hi)
        out = np.concatenate([rgb, arr[..., 3:4]], axis=-1)
        Image.fromarray((out * 255).astype(np.uint8), "RGBA").save(
            os.path.join(CL, f"assets/characters/{name}_sheet.png"), optimize=True)


CLASS_COL = {"CLS-01": (70, 120, 200), "CLS-02": (190, 60, 50), "CLS-03": (120, 90, 230),
             "CLS-04": (60, 160, 90), "CLS-05": (225, 190, 90)}
ELEMENTS = [  # name keywords -> colour
    (("огн", "пламя", "метеор", "жар", "печать"), (235, 110, 40)),
    (("лед", "холод", "мороз", "снег"), (90, 190, 240)),
    (("молни", "шторм", "гром"), (250, 220, 70)),
    (("яд", "природ", "корн"), (110, 190, 70)),
]


def palette(skill):
    name = skill["name"].lower()
    for keys, col in ELEMENTS:
        if any(k in name for k in keys):
            return col
    return CLASS_COL[skill["id"][:6]]


def glyph(d, kind, s, fg):
    """Draw the glyph on a 4x canvas ImageDraw `d` of size s×s."""
    c = s / 2
    r = s * 0.28
    if kind == "damage":  # eight-point burst
        pts = []
        for i in range(16):
            a = math.pi * i / 8
            rr = r * (1.25 if i % 2 == 0 else 0.55)
            pts.append((c + math.cos(a) * rr, c + math.sin(a) * rr))
        d.polygon(pts, fill=fg)
    elif kind in ("shield", "barrier", "mitigation", "intercept"):
        d.polygon([(c - r, c - r * 0.9), (c + r, c - r * 0.9), (c + r, c + r * 0.1), (c, c + r * 1.2), (c - r, c + r * 0.1)], fill=fg)
    elif kind in ("heal", "cleanse"):
        w = r * 0.38
        d.rectangle([c - w, c - r, c + w, c + r], fill=fg)
        d.rectangle([c - r, c - w, c + r, c + w], fill=fg)
    elif kind in ("haste", "buff", "movement"):
        for dx in (-r * 0.55, r * 0.25):
            d.polygon([(c + dx - r * 0.2, c - r), (c + dx + r * 0.7, c), (c + dx - r * 0.2, c + r), (c + dx + r * 0.05, c)], fill=fg)
    elif kind in ("teleport",):
        d.ellipse([c - r, c - r, c + r, c + r], outline=fg, width=int(s * 0.07))
        d.ellipse([c - r * 0.35, c - r * 0.35, c + r * 0.35, c + r * 0.35], fill=fg)
    elif kind in ("mark", "reveal"):
        d.ellipse([c - r, c - r, c + r, c + r], outline=fg, width=int(s * 0.06))
        d.line([c - r * 1.3, c, c + r * 1.3, c], fill=fg, width=int(s * 0.05))
        d.line([c, c - r * 1.3, c, c + r * 1.3], fill=fg, width=int(s * 0.05))
    elif kind in ("stealth",):
        d.ellipse([c - r, c - r * 0.6, c + r, c + r * 0.6], outline=fg, width=int(s * 0.06))
        d.ellipse([c - r * 0.3, c - r * 0.3, c + r * 0.3, c + r * 0.3], fill=fg)
    else:  # mana / default: diamond
        d.polygon([(c, c - r * 1.1), (c + r * 0.8, c), (c, c + r * 1.1), (c - r * 0.8, c)], fill=fg)


def icon(skill):
    S = 96 * 4
    base = palette(skill)
    img = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    # radial-ish gradient disc: stacked circles from dark rim to bright centre
    for i in range(40):
        t = i / 39
        rad = S / 2 * (1 - 0.82 * t) - 2
        col = tuple(int(base[k] * (0.35 + 0.75 * t)) for k in range(3)) + (255,)
        d.ellipse([S / 2 - rad, S / 2 - rad, S / 2 + rad, S / 2 + rad], fill=col)
    d.ellipse([6, 6, S - 6, S - 6], outline=(240, 220, 150, 255), width=14)  # gold rim
    shadow = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    glyph(ImageDraw.Draw(shadow), skill["effect_kind"], S, (0, 0, 0, 170))
    img.alpha_composite(shadow.filter(ImageFilter.GaussianBlur(10)), (6, 8))
    glyph(d, skill["effect_kind"], S, (255, 255, 255, 245))
    return img.resize((96, 96), Image.LANCZOS)


def icons():
    out = os.path.join(CL, "assets/ui/skills")
    os.makedirs(out, exist_ok=True)
    data = json.load(open(os.path.join(CL, "data/skills.json"), encoding="utf-8"))["classes"]
    n = 0
    for _cls, skills in data.items():
        for s in skills:
            icon(s).save(os.path.join(out, s["id"] + ".png"), optimize=True)
            n += 1
    return n


if __name__ == "__main__":
    recolour()
    print("class sheets: 4, skill icons:", icons())
