#!/usr/bin/env python3
"""Valley props: trees/cottage from CC0 packs (trimmed, scaled), rock and fence generated here.

Inputs (not committed): --plants  «Free Isometric Plants Pack» (Dread Knight, CC0) 'isometric tiles' dir
                         --house  «Western European Medieval Houses» (OpenGameArt, CC0) house1_0.png
Outputs go to client/assets/env/{tree_oak,tree_pine,cottage,rock,fence}.png
"""
import argparse
import math
import random
from PIL import Image, ImageDraw, ImageFilter


def trim_scale(path, width):
    im = Image.open(path).convert("RGBA")
    im = im.crop(im.getbbox())
    h = round(im.height * width / im.width)
    return im.resize((width, h), Image.LANCZOS)


def make_rock(seed, w=72, h=52):
    rnd = random.Random(seed)
    big = Image.new("RGBA", (w * 4, h * 4), (0, 0, 0, 0))
    d = ImageDraw.Draw(big)
    cx, cy = w * 2, h * 2.5
    pts = []
    for i in range(14):
        a = i / 14 * math.tau
        r = (0.86 + rnd.random() * 0.14)
        pts.append((cx + math.cos(a) * w * 1.9 * r, cy + math.sin(a) * h * 1.35 * r * (0.8 if math.sin(a) > 0 else 1.0)))
    d.polygon(pts, fill=(112, 112, 118, 255))
    # lit top facets / dark base
    d.polygon([(x * 0.9 + cx * 0.1, y * 0.88 + cy * 0.05 - h * 0.35) for x, y in pts[:9]], fill=(146, 146, 152, 255))
    d.polygon([(x, y) for x, y in pts[7:13]] + [(cx, cy + h * 0.8)], fill=(84, 84, 92, 255))
    big = big.filter(ImageFilter.GaussianBlur(3))
    return big.resize((w, h), Image.LANCZOS)


def make_fence(w=72, h=40):
    S = 4
    big = Image.new("RGBA", (w * S, h * S), (0, 0, 0, 0))
    d = ImageDraw.Draw(big)
    wood, dark = (138, 98, 58, 255), (92, 62, 36, 255)
    for x in (6, w - 12):
        d.rectangle([x * S, 8 * S, (x + 6) * S, (h - 3) * S], fill=wood, outline=dark, width=S // 2)
    for y in (14, 26):
        d.polygon([(2 * S, y * S), (w * S - 2 * S, (y - 6) * S), (w * S - 2 * S, (y - 1) * S), (2 * S, (y + 5) * S)], fill=wood, outline=dark)
    return big.resize((w, h), Image.LANCZOS)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--plants", required=True)
    ap.add_argument("--house", required=True)
    ap.add_argument("--out", required=True)
    a = ap.parse_args()
    trim_scale(f"{a.plants}/bigtree01.png", 190).save(f"{a.out}/tree_oak.png", optimize=True)
    trim_scale(f"{a.plants}/pine-none03.png", 110).save(f"{a.out}/tree_pine.png", optimize=True)
    trim_scale(a.house, 190).save(f"{a.out}/cottage.png", optimize=True)
    make_rock(3).save(f"{a.out}/rock.png", optimize=True)
    make_fence().save(f"{a.out}/fence.png", optimize=True)


if __name__ == "__main__":
    main()
