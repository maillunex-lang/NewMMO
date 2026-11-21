#!/usr/bin/env python3
"""Generate original procedural 2D sprites for Lunex Stage 1 (interim art)."""
from __future__ import annotations

import math
import os
from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter

ROOT = Path(__file__).resolve().parents[2] / "client" / "assets"


def ensure(p: Path) -> Path:
    p.parent.mkdir(parents=True, exist_ok=True)
    return p


def new_rgba(w: int, h: int) -> Image.Image:
    return Image.new("RGBA", (w, h), (0, 0, 0, 0))


def disc(draw: ImageDraw.ImageDraw, xy, r, fill):
    x, y = xy
    draw.ellipse((x - r, y - r, x + r, y + r), fill=fill)


def soft_blob(img: Image.Image, cx, cy, rx, ry, color, blur=1.2):
    layer = new_rgba(*img.size)
    d = ImageDraw.Draw(layer)
    d.ellipse((cx - rx, cy - ry, cx + rx, cy + ry), fill=color)
    if blur:
        layer = layer.filter(ImageFilter.GaussianBlur(blur))
    return Image.alpha_composite(img, layer)


def save(img: Image.Image, rel: str):
    path = ensure(ROOT / rel)
    img.save(path, "PNG")
    print("wrote", path.relative_to(ROOT.parent.parent))


def make_shadow(w=48, h=18) -> Image.Image:
    img = new_rgba(w, h)
    d = ImageDraw.Draw(img)
    d.ellipse((2, 2, w - 3, h - 3), fill=(0, 0, 0, 90))
    return img.filter(ImageFilter.GaussianBlur(1.5))


def make_grass_tile(size=64) -> Image.Image:
    img = Image.new("RGBA", (size, size), (34, 72, 48, 255))
    d = ImageDraw.Draw(img)
    for i in range(0, size, 8):
        for j in range(0, size, 8):
            shade = 8 if (i // 8 + j // 8) % 2 == 0 else -6
            c = (34 + shade, 78 + shade, 50 + shade, 255)
            d.rectangle((i, j, i + 7, j + 7), fill=c)
    for n in range(40):
        x = (n * 17) % size
        y = (n * 29) % size
        d.point((x, y), fill=(70, 120, 55, 180))
    return img


def make_path_tile(size=64) -> Image.Image:
    img = Image.new("RGBA", (size, size), (92, 74, 52, 255))
    d = ImageDraw.Draw(img)
    for n in range(25):
        x = (n * 13) % (size - 6)
        y = (n * 19) % (size - 6)
        d.ellipse((x, y, x + 5, y + 4), fill=(110, 92, 68, 255))
    return img


def make_tree() -> Image.Image:
    img = new_rgba(96, 128)
    img = soft_blob(img, 48, 112, 22, 8, (0, 0, 0, 70), 2)
    d = ImageDraw.Draw(img)
    d.polygon([(42, 70), (54, 70), (58, 118), (38, 118)], fill=(78, 48, 28, 255))
    for (cx, cy, r, col) in [
        (48, 48, 34, (28, 92, 52, 255)),
        (30, 58, 22, (22, 78, 44, 255)),
        (66, 56, 24, (36, 110, 60, 255)),
        (48, 34, 20, (48, 130, 72, 230)),
    ]:
        disc(d, (cx, cy), r, col)
    return img


def make_rock() -> Image.Image:
    img = new_rgba(80, 56)
    img = soft_blob(img, 40, 46, 26, 8, (0, 0, 0, 70), 1.5)
    d = ImageDraw.Draw(img)
    d.polygon([(8, 36), (18, 12), (44, 4), (70, 18), (72, 40), (40, 48), (12, 44)], fill=(98, 108, 118, 255))
    d.polygon([(20, 28), (30, 14), (48, 12), (42, 30)], fill=(130, 140, 150, 180))
    return img


def make_crystal() -> Image.Image:
    img = new_rgba(64, 96)
    img = soft_blob(img, 32, 84, 16, 6, (0, 0, 0, 60), 1.2)
    d = ImageDraw.Draw(img)
    d.polygon([(32, 8), (48, 40), (40, 82), (24, 82), (16, 40)], fill=(64, 210, 230, 210))
    d.polygon([(32, 8), (40, 40), (32, 78), (24, 40)], fill=(180, 250, 255, 160))
    return img


def make_cottage() -> Image.Image:
    img = new_rgba(140, 120)
    img = soft_blob(img, 70, 108, 50, 10, (0, 0, 0, 70), 2)
    d = ImageDraw.Draw(img)
    d.rectangle((30, 50, 110, 100), fill=(138, 104, 72, 255))
    d.polygon([(22, 54), (70, 18), (118, 54)], fill=(120, 58, 48, 255))
    d.rectangle((58, 68, 82, 100), fill=(60, 40, 28, 255))
    d.rectangle((40, 62, 54, 74), fill=(180, 210, 230, 220))
    d.rectangle((90, 62, 104, 74), fill=(180, 210, 230, 220))
    return img


def make_lantern() -> Image.Image:
    img = new_rgba(40, 72)
    d = ImageDraw.Draw(img)
    d.rectangle((18, 24, 22, 62), fill=(70, 55, 40, 255))
    d.rectangle((10, 18, 30, 36), fill=(210, 170, 70, 230))
    d.rectangle((14, 22, 26, 32), fill=(255, 230, 140, 255))
    img = soft_blob(img, 20, 26, 14, 12, (255, 200, 80, 60), 3)
    return img


def make_fence() -> Image.Image:
    img = new_rgba(72, 40)
    d = ImageDraw.Draw(img)
    for x in (8, 34, 60):
        d.rectangle((x, 8, x + 6, 34), fill=(96, 70, 42, 255))
    d.rectangle((4, 12, 68, 18), fill=(110, 82, 50, 255))
    d.rectangle((4, 24, 68, 30), fill=(110, 82, 50, 255))
    return img


def dir_vector(i: int) -> tuple[float, float]:
    # 0=E,1=SE,2=S,3=SW,4=W,5=NW,6=N,7=NE
    ang = i * math.pi / 4
    return math.cos(ang), math.sin(ang)


def draw_humanoid(frame: Image.Image, dx: float, dy: float, palette, bob=0, attack=0.0, kind="player"):
    d = ImageDraw.Draw(frame)
    w, h = frame.size
    cx, cy = w // 2, h // 2 + 8 + bob
    # shadow
    d.ellipse((cx - 14, cy + 18, cx + 14, cy + 28), fill=(0, 0, 0, 70))
    body = palette["body"]
    accent = palette["accent"]
    head = palette["head"]
    # legs
    lx = int(3 * dx)
    d.rectangle((cx - 8 + lx, cy + 6, cx - 2 + lx, cy + 20), fill=palette["leg"])
    d.rectangle((cx + 2 - lx, cy + 6, cx + 8 - lx, cy + 20), fill=palette["leg"])
    # body
    d.ellipse((cx - 12, cy - 10, cx + 12, cy + 12), fill=body)
    if kind == "player":
        d.polygon([(cx - 10, cy - 6), (cx + 10, cy - 2), (cx + 8, cy + 10), (cx - 12, cy + 8)], fill=accent)
    # head
    d.ellipse((cx - 8, cy - 26, cx + 8, cy - 10), fill=head)
    # facing cue
    eye_x = cx + int(4 * dx)
    d.ellipse((eye_x - 2, cy - 20, eye_x + 2, cy - 16), fill=(30, 30, 40, 255))
    # weapon / staff
    wx = cx + int(16 * dx)
    wy = cy - int(6 * dy) - int(8 * attack)
    if kind == "player":
        d.line((cx + 8, cy, wx, wy - 10), fill=(200, 180, 120, 255), width=3)
        d.polygon([(wx, wy - 18), (wx + 6, wy - 8), (wx - 6, wy - 8)], fill=(180, 200, 220, 255))
    elif kind == "npc":
        d.line((cx + 6, cy + 4, wx, cy + 16), fill=(120, 90, 50, 255), width=3)
    elif kind == "bandit":
        d.line((cx + 6, cy, wx, wy), fill=(160, 160, 170, 255), width=3)


def make_actor_sheet(name: str, palette: dict, kind: str, out_rel: str, frame_w=64, frame_h=64):
    dirs = 8
    idle_frames = 4
    run_frames = 6
    attack_frames = 4
    cols = idle_frames + run_frames + attack_frames
    sheet = new_rgba(frame_w * cols, frame_h * dirs)
    for di in range(dirs):
        dx, dy = dir_vector(di)
        # invert y for screen (N is up => negative screen y)
        sdx, sdy = dx, -dy
        for fi in range(idle_frames):
            fr = new_rgba(frame_w, frame_h)
            bob = int(math.sin(fi / idle_frames * math.pi * 2) * 1)
            draw_humanoid(fr, sdx, sdy, palette, bob=bob, kind=kind)
            sheet.paste(fr, (fi * frame_w, di * frame_h), fr)
        for fi in range(run_frames):
            fr = new_rgba(frame_w, frame_h)
            bob = int(math.sin(fi / run_frames * math.pi * 2) * 3)
            draw_humanoid(fr, sdx, sdy, palette, bob=bob, kind=kind)
            sheet.paste(fr, ((idle_frames + fi) * frame_w, di * frame_h), fr)
        for fi in range(attack_frames):
            fr = new_rgba(frame_w, frame_h)
            atk = math.sin(fi / max(attack_frames - 1, 1) * math.pi)
            draw_humanoid(fr, sdx, sdy, palette, bob=0, attack=atk, kind=kind)
            sheet.paste(fr, ((idle_frames + run_frames + fi) * frame_w, di * frame_h), fr)
    meta = {
        "frame_w": frame_w,
        "frame_h": frame_h,
        "directions": dirs,
        "idle": idle_frames,
        "run": run_frames,
        "attack": attack_frames,
        "name": name,
    }
    save(sheet, out_rel)
    return meta


def make_slime_sheet() -> None:
    dirs, frames = 8, 6
    fw = fh = 48
    sheet = new_rgba(fw * frames, fh * dirs)
    for di in range(dirs):
        dx, dy = dir_vector(di)
        for fi in range(frames):
            fr = new_rgba(fw, fh)
            d = ImageDraw.Draw(fr)
            squash = 1.0 + 0.12 * math.sin(fi / frames * math.pi * 2)
            cx, cy = 24, 28
            rx, ry = int(16 * squash), int(14 / squash)
            d.ellipse((cx - 14, cy + 10, cx + 14, cy + 18), fill=(0, 0, 0, 60))
            d.ellipse((cx - rx, cy - ry, cx + rx, cy + ry), fill=(70, 190, 90, 230))
            d.ellipse((cx - 6 + int(4 * dx), cy - 4 - int(2 * dy), cx - 2 + int(4 * dx), cy), fill=(20, 40, 20, 255))
            d.ellipse((cx + 2 + int(4 * dx), cy - 4 - int(2 * dy), cx + 6 + int(4 * dx), cy), fill=(20, 40, 20, 255))
            sheet.paste(fr, (fi * fw, di * fh), fr)
    save(sheet, "monsters/slime_sheet.png")


def make_wolf_sheet() -> None:
    dirs, frames = 8, 6
    fw, fh = 64, 48
    sheet = new_rgba(fw * frames, fh * dirs)
    for di in range(dirs):
        dx, dy = dir_vector(di)
        for fi in range(frames):
            fr = new_rgba(fw, fh)
            d = ImageDraw.Draw(fr)
            cx, cy = 32, 26
            stride = int(math.sin(fi / frames * math.pi * 2) * 3)
            d.ellipse((cx - 16, cy + 12, cx + 16, cy + 20), fill=(0, 0, 0, 55))
            body = [(cx - 18, cy), (cx + 10, cy - 8), (cx + 20, cy), (cx + 8, cy + 10), (cx - 16, cy + 8)]
            d.polygon([(x + int(dx * 2), y + stride // 2) for x, y in body], fill=(90, 70, 55, 255))
            hx, hy = cx + int(16 * dx), cy - 4 - int(4 * dy)
            d.ellipse((hx - 8, hy - 6, hx + 8, hy + 6), fill=(70, 55, 42, 255))
            d.polygon([(hx + int(8 * dx), hy), (hx + int(14 * dx), hy + 2), (hx + int(8 * dx), hy + 4)], fill=(40, 30, 25, 255))
            sheet.paste(fr, (fi * fw, di * fh), fr)
    save(sheet, "monsters/wolf_sheet.png")


def make_hit_vfx() -> None:
    img = new_rgba(48, 48)
    d = ImageDraw.Draw(img)
    d.ellipse((8, 8, 40, 40), outline=(255, 220, 120, 220), width=3)
    d.line((12, 12, 36, 36), fill=(255, 240, 180, 255), width=2)
    d.line((36, 12, 12, 36), fill=(255, 240, 180, 255), width=2)
    save(img, "vfx/hit_spark.png")


def make_loot_icon() -> None:
    img = new_rgba(32, 32)
    d = ImageDraw.Draw(img)
    d.rectangle((6, 10, 26, 26), fill=(160, 110, 50, 255))
    d.rectangle((10, 6, 22, 12), fill=(120, 80, 40, 255))
    d.rectangle((14, 14, 18, 20), fill=(220, 190, 80, 255))
    save(img, "ui/loot_bag.png")


def make_portrait(path: str, color) -> None:
    img = new_rgba(72, 72)
    d = ImageDraw.Draw(img)
    d.rounded_rectangle((2, 2, 70, 70), radius=10, fill=(20, 24, 32, 255), outline=(198, 164, 86, 255), width=2)
    d.ellipse((18, 14, 54, 50), fill=color)
    d.ellipse((28, 28, 34, 34), fill=(20, 20, 30, 255))
    d.ellipse((40, 28, 46, 34), fill=(20, 20, 30, 255))
    save(img, path)


def main():
    save(make_grass_tile(), "env/grass_tile.png")
    save(make_path_tile(), "env/path_tile.png")
    save(make_tree(), "env/tree_oak.png")
    save(make_rock(), "env/rock.png")
    save(make_crystal(), "env/crystal.png")
    save(make_cottage(), "env/cottage.png")
    save(make_lantern(), "env/lantern.png")
    save(make_fence(), "env/fence.png")
    save(make_shadow(), "env/shadow.png")

    player = {
        "body": (70, 90, 140, 255),
        "accent": (198, 164, 86, 255),
        "head": (220, 185, 150, 255),
        "leg": (40, 45, 70, 255),
    }
    npc = {
        "body": (90, 70, 110, 255),
        "accent": (160, 120, 70, 255),
        "head": (230, 200, 170, 255),
        "leg": (50, 40, 45, 255),
    }
    bandit = {
        "body": (110, 50, 50, 255),
        "accent": (80, 80, 80, 255),
        "head": (210, 170, 140, 255),
        "leg": (40, 30, 30, 255),
    }
    make_actor_sheet("guardian", player, "player", "characters/guardian_sheet.png")
    make_actor_sheet("elder_mira", npc, "npc", "npc/elder_mira_sheet.png")
    make_actor_sheet("bandit", bandit, "bandit", "monsters/bandit_sheet.png")
    make_slime_sheet()
    make_wolf_sheet()
    make_hit_vfx()
    make_loot_icon()
    make_portrait("ui/portrait_player.png", (70, 90, 140, 255))
    make_portrait("ui/portrait_mira.png", (90, 70, 110, 255))
    print("Stage 1 assets generated under", ROOT)


if __name__ == "__main__":
    main()
