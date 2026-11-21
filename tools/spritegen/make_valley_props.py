#!/usr/bin/env python3
"""Valley props: trees/cottage from CC0 packs (trimmed, scaled), rock and fence generated here.

Inputs (not committed): --plants  «Free Isometric Plants Pack» (Dread Knight, CC0) 'isometric tiles' dir
                         --house  «Western European Medieval Houses» (OpenGameArt, CC0) house1_0.png
Outputs go to client/assets/env/{tree_oak,tree_pine,cottage,rock,fence,lantern,crystal}.png
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


def glow(size, center, radius, color):
    g = Image.new("RGBA", size, (0, 0, 0, 0))
    d = ImageDraw.Draw(g)
    for i in range(radius, 0, -2):
        a = int(110 * (1 - i / radius) ** 2)
        d.ellipse([center[0] - i, center[1] - i, center[0] + i, center[1] + i], fill=color + (a,))
    return g


def make_lantern(w=40, h=72):
    S = 4
    big = Image.new("RGBA", (w * S, h * S), (0, 0, 0, 0))
    big.alpha_composite(glow(big.size, (20 * S, 18 * S), 17 * S, (255, 190, 90)))
    d = ImageDraw.Draw(big)
    iron, dark = (58, 52, 58, 255), (30, 28, 32, 255)
    d.rectangle([18 * S, 26 * S, 22 * S, 66 * S], fill=iron, outline=dark)        # post
    d.polygon([(12 * S, 68 * S), (28 * S, 68 * S), (24 * S, 62 * S), (16 * S, 62 * S)], fill=iron, outline=dark)  # base
    d.polygon([(11 * S, 12 * S), (29 * S, 12 * S), (25 * S, 5 * S), (15 * S, 5 * S)], fill=iron, outline=dark)    # cap
    d.rectangle([13 * S, 12 * S, 27 * S, 26 * S], fill=(255, 214, 120, 255), outline=dark, width=S // 2)           # glass
    d.rectangle([16 * S, 15 * S, 24 * S, 23 * S], fill=(255, 245, 190, 255))                                      # flame
    return big.resize((w, h), Image.LANCZOS)


def make_crystal(w=64, h=96):
    S = 4
    big = Image.new("RGBA", (w * S, h * S), (0, 0, 0, 0))
    big.alpha_composite(glow(big.size, (32 * S, 52 * S), 30 * S, (90, 210, 255)))
    d = ImageDraw.Draw(big)
    # shadow ellipse
    d.ellipse([14 * S, 82 * S, 50 * S, 92 * S], fill=(0, 0, 0, 70))
    def shard(cx, base, top, half, light, dark):
        d.polygon([(cx - half, base), (cx, top), (cx, base + 4 * S)], fill=dark)
        d.polygon([(cx + half, base), (cx, top), (cx, base + 4 * S)], fill=light)
    shard(32 * S, 80 * S, 8 * S, 13 * S, (150, 235, 255, 255), (60, 160, 215, 255))
    shard(17 * S, 82 * S, 38 * S, 8 * S, (120, 215, 250, 255), (45, 135, 195, 255))
    shard(47 * S, 82 * S, 46 * S, 8 * S, (120, 215, 250, 255), (45, 135, 195, 255))
    d.polygon([(30 * S, 30 * S), (32 * S, 10 * S), (34 * S, 30 * S)], fill=(230, 250, 255, 200))
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
    make_lantern().save(f"{a.out}/lantern.png", optimize=True)
    make_crystal().save(f"{a.out}/crystal.png", optimize=True)


if __name__ == "__main__":
    main()
