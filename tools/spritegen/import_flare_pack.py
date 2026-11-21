#!/usr/bin/env python3
"""Repack the OpenGameArt «Isometric Hero and Creatures» sheets (CC-BY 3.0) into the game's actor grid.

Source sheets are 2048x2048 = 8x8 cells of 256 px: rows are directions in Flare order
(0=W,1=NW,2=N,3=NE,4=E,5=SE,6=S,7=SW), columns 0-3 a 4-frame run cycle, 4 attack, 5 special, 6 hit,
7 the death pose. Equipment sheets (male_heavy, male_longsword, ...) are transparent layers that are
composited over male_base.

Output (game grid, see client/scripts/actors/directional_animator.gd): rows in game order
(0=E,1=SE,2=S,3=SW,4=W,5=NW,6=N,7=NE) -> game row g takes Flare row (g+4)%8;
columns: idle (1 frame) | run (4 frames) | attack (3 frames: attack, special, attack).

Usage: import_flare_pack.py --src <dir with the unzipped PNGs> --out client/assets
Pillow required. Inputs are NOT committed (see client/assets/LICENSES.md for the source URL).
"""
import argparse
import os
from PIL import Image

CELL = 256
IDLE, RUN, ATTACK = [0], [0, 1, 2, 3], [4, 5, 4]  # col 4 attack, 5 special, 6 hit, 7 dead are single frames

# name -> (layers over/instead of base, frame size px, crop box in the 256 cell)
STD_CROP = (32, 32, 224, 224)
ACTORS = {
    "characters/guardian_sheet.png": (["male_base", "male_heavy", "male_shield", "male_longsword"], 96, STD_CROP),
    "npc/elder_mira_sheet.png": (["magician"], 96, STD_CROP),
    "npc/feira_niss_sheet.png": (["male_base", "male_light", "male_staff"], 96, STD_CROP),
    "npc/ilra_venn_sheet.png": (["male_base", "male_heavy", "male_longbow"], 96, STD_CROP),
    "monsters/bandit_sheet.png": (["male_base", "male_light", "male_longsword"], 96, STD_CROP),
    "monsters/slime_sheet.png": (["slime"], 96, STD_CROP),
    "monsters/wolf_sheet.png": (["werewolf"], 96, STD_CROP),
    "monsters/goblin_sheet.png": (["goblin"], 96, STD_CROP),
    "monsters/skeleton_sheet.png": (["skeleton"], 112, (16, 16, 240, 240)),
    "monsters/zombie_sheet.png": (["zombie"], 96, STD_CROP),
    "monsters/ogre_sheet.png": (["ogre"], 144, (0, 0, 256, 256)),
}


def cell(img, row, col):
    return img.crop((col * CELL, row * CELL, (col + 1) * CELL, (row + 1) * CELL))


def build(src, layers, size, crop):
    sheets = [Image.open(os.path.join(src, name + ".png")).convert("RGBA") for name in layers]
    cols = IDLE + RUN + ATTACK
    out = Image.new("RGBA", (size * len(cols), size * 8), (0, 0, 0, 0))
    for g in range(8):
        flare_row = (g + 4) % 8
        for i, c in enumerate(cols):
            frame = cell(sheets[0], flare_row, c)
            for layer in sheets[1:]:
                frame = Image.alpha_composite(frame, cell(layer, flare_row, c))
            frame = frame.crop(crop).resize((size, size), Image.LANCZOS)
            out.paste(frame, (i * size, g * size))
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--src", required=True)
    ap.add_argument("--out", required=True)
    a = ap.parse_args()
    for rel, (layers, size, crop) in ACTORS.items():
        path = os.path.join(a.out, rel)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        build(a.src, layers, size, crop).save(path, optimize=True)
        print(rel, size)


if __name__ == "__main__":
    main()
