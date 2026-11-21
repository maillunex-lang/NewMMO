#!/usr/bin/env python3
"""Shrinks the embedded textures of GLB files for mobile (Pillow only, no third-party tools).
Meshy exports 4K PNG textures (5-8 MB per model); this rewrites images as <=MAX px (JPEG q85 when opaque, PNG when
alpha is used) and repacks the binary chunk. Geometry/skeleton/animations are untouched.
usage: optimize_glb.py <max_px> file.glb [file.glb ...]   (in place)"""
import io
import json
import struct
import sys

from PIL import Image


def optimize(path, max_px):
    data = open(path, "rb").read()
    magic, ver, _ = struct.unpack("<4sII", data[:12])
    assert magic == b"glTF" and ver == 2
    off = 12
    js = bin_chunk = None
    while off < len(data):
        ln, typ = struct.unpack("<II", data[off:off + 8])
        chunk = data[off + 8:off + 8 + ln]
        if typ == 0x4E4F534A:
            js = json.loads(chunk)
        elif typ == 0x004E4942:
            bin_chunk = chunk
        off += 8 + ln
    views = js["bufferViews"]
    new = {}
    for i, img in enumerate(js.get("images", [])):
        v = views[img["bufferView"]]
        raw = bin_chunk[v.get("byteOffset", 0):v.get("byteOffset", 0) + v["byteLength"]]
        im = Image.open(io.BytesIO(raw))
        im.load()
        if max(im.size) > max_px:
            k = max_px / max(im.size)
            im = im.resize((max(1, round(im.width * k)), max(1, round(im.height * k))), Image.LANCZOS)
        out = io.BytesIO()
        has_alpha = im.mode in ("RGBA", "LA") and im.getchannel("A").getextrema()[0] < 250
        if has_alpha:
            im.save(out, "PNG", optimize=True)
            img["mimeType"] = "image/png"
        else:
            im.convert("RGB").save(out, "JPEG", quality=85, optimize=True)
            img["mimeType"] = "image/jpeg"
        new[img["bufferView"]] = out.getvalue()
    pieces = []
    pos = 0
    for i, v in enumerate(views):
        blob = new.get(i, bin_chunk[v.get("byteOffset", 0):v.get("byteOffset", 0) + v["byteLength"]])
        pad = (-pos) % 4
        pieces.append(b"\0" * pad)
        pos += pad
        v["byteOffset"] = pos
        v["byteLength"] = len(blob)
        pieces.append(blob)
        pos += len(blob)
    binb = b"".join(pieces) + b"\0" * ((-pos) % 4)
    js["buffers"][0]["byteLength"] = len(binb)
    jb = json.dumps(js, separators=(",", ":")).encode()
    jb += b" " * ((-len(jb)) % 4)
    outb = struct.pack("<4sII", b"glTF", 2, 12 + 8 + len(jb) + 8 + len(binb))
    outb += struct.pack("<II", len(jb), 0x4E4F534A) + jb + struct.pack("<II", len(binb), 0x004E4942) + binb
    open(path, "wb").write(outb)
    return len(data), len(outb)


if __name__ == "__main__":
    mx = int(sys.argv[1])
    for p in sys.argv[2:]:
        a, b = optimize(p, mx)
        print(f"{p}: {a/1e6:.1f} MB -> {b/1e6:.2f} MB")
