#!/usr/bin/env python3
"""Build client/assets/env/dawn_valley_ground.png as an isometric tessellation (64x32 diamonds).

Grass tiles come from «Isometric Ground Tiles» (Drummyfish, OpenGameArt, CC0); the road follows the
path mask of the previous procedural ground (dirt tiles where the old image was brown). A soft
vignette keeps the old darker border. Usage:
  make_valley_ground.py --tiles <dir with grass_green_64x32.png, dirt_64x32.png> --old tools/spritegen/valley_road_mask.png --out <png>
"""
import argparse
import random
from PIL import Image, ImageChops, ImageFilter
import numpy as np

TW, TH = 64, 32
W, H = 2048, 1400


def tiles(path):
    sheet = Image.open(path).convert("RGBA")
    out = []
    for r in range(sheet.height // TH):
        for c in range(sheet.width // TW):
            t = sheet.crop((c * TW, r * TH, (c + 1) * TW, (r + 1) * TH))
            if np.array(t)[:, :, 3].sum() > 255 * TW * TH * 0.35:
                out.append(t)
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--tiles", required=True)
    ap.add_argument("--old", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--floor-out", help="also write a 760x560 stone floor texture for dungeon rooms")
    a = ap.parse_args()
    grass = tiles(f"{a.tiles}/grass_green_64x32.png")
    grass2 = tiles(f"{a.tiles}/grass_medium_64x32.png")
    dirt = tiles(f"{a.tiles}/dirt_64x32.png")
    # road mask (white = road): tools/spritegen/valley_road_mask.png (taken from the previous procedural ground)
    road = np.array(Image.open(a.old).convert("L")) > 128
    rnd = random.Random(7)
    img = Image.new("RGBA", (W, H), (78, 112, 48, 255))
    for r in range(-2, H // (TH // 2) + 3):
        for c in range(-2, W // TW + 3):
            x = c * TW + (r % 2) * (TW // 2)
            y = r * (TH // 2) - TH // 2
            cx, cy = min(max(x + TW // 2, 0), W - 1), min(max(y + TH // 2, 0), H - 1)
            on_road = road[cy, cx]
            src = dirt if on_road else (grass if rnd.random() < 0.7 else grass2)
            t = rnd.choice(src).resize((TW + 4, TH + 2), Image.LANCZOS)  # slight overlap closes seams
            img.paste(t, (x - 2, y - 1), t)
    # soft vignette (darker border like the previous ground)
    yy, xx = np.mgrid[0:H, 0:W]
    edge = np.minimum(np.minimum(xx, W - 1 - xx) / 90.0, np.minimum(yy, H - 1 - yy) / 90.0).clip(0, 1)
    shade = (0.55 + 0.45 * edge)[:, :, None]
    arr = np.array(img.convert("RGB")).astype(float) * shade
    Image.fromarray(arr.astype("uint8"), "RGB").save(a.out, optimize=True)
    if a.floor_out:
        stone_floor(a.tiles, a.floor_out)


def stone_floor(tiles_dir, out, w=760, h=560):
    stone = tiles(f"{tiles_dir}/stone_path_64x32.png")
    rnd = random.Random(11)
    img = Image.new("RGBA", (w, h), (70, 66, 72, 255))
    for r in range(-2, h // (TH // 2) + 3):
        for c in range(-2, w // TW + 3):
            t = rnd.choice(stone).resize((TW + 4, TH + 2), Image.LANCZOS)
            img.paste(t, (c * TW + (r % 2) * (TW // 2) - 2, r * (TH // 2) - TH // 2 - 1), t)
    img.convert("RGB").save(out, optimize=True)


if __name__ == "__main__":
    main()
