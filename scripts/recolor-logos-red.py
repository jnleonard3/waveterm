#!/usr/bin/env python3
"""Recolor Wave logo greens to the red branding gradient.

Maps stop colors:
  #1F4D22 -> #B91C1C (dark)
  #418D31 -> #EF4444 (mid)
  #58C142 -> #F87171 (light)

Preserves alpha, neutrals, and non-green pixels. Anti-aliased greens are
projected onto the source gradient and remapped to the destination gradient.
"""

from __future__ import annotations

import struct
import sys
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parents[1]

SRC_STOPS = [
    (31, 77, 34),  # #1F4D22
    (65, 141, 49),  # #418D31
    (88, 193, 66),  # #58C142
]
DST_STOPS = [
    (185, 28, 28),  # #B91C1C
    (239, 68, 68),  # #EF4444
    (248, 113, 113),  # #F87171
]


def _dist2(a: tuple[int, int, int], b: tuple[int, int, int]) -> float:
    return (a[0] - b[0]) ** 2 + (a[1] - b[1]) ** 2 + (a[2] - b[2]) ** 2


def _lerp(a: tuple[int, int, int], b: tuple[int, int, int], t: float) -> tuple[int, int, int]:
    return (
        int(round(a[0] + (b[0] - a[0]) * t)),
        int(round(a[1] + (b[1] - a[1]) * t)),
        int(round(a[2] + (b[2] - a[2]) * t)),
    )


def _project_onto_stops(rgb: tuple[int, int, int]) -> tuple[float, float] | None:
    """Return (segment_index, t) for nearest point on the green stop polyline, or None."""
    best_d = None
    best = None
    for i in range(len(SRC_STOPS) - 1):
        a = SRC_STOPS[i]
        b = SRC_STOPS[i + 1]
        ab = (b[0] - a[0], b[1] - a[1], b[2] - a[2])
        ap = (rgb[0] - a[0], rgb[1] - a[1], rgb[2] - a[2])
        ab2 = ab[0] ** 2 + ab[1] ** 2 + ab[2] ** 2
        if ab2 == 0:
            continue
        t = max(0.0, min(1.0, (ap[0] * ab[0] + ap[1] * ab[1] + ap[2] * ab[2]) / ab2))
        proj = (a[0] + ab[0] * t, a[1] + ab[1] * t, a[2] + ab[2] * t)
        d = (rgb[0] - proj[0]) ** 2 + (rgb[1] - proj[1]) ** 2 + (rgb[2] - proj[2]) ** 2
        if best_d is None or d < best_d:
            best_d = d
            best = (float(i), t, d)
    if best is None:
        return None
    # Reject pixels that are not near the green ramp (keeps neutrals / other UI colors).
    if best[2] > 45 ** 2:
        return None
    return best[0], best[1]


def _is_greenish(r: int, g: int, b: int) -> bool:
    if g <= r and g <= b:
        return False
    # Green channel leads; allow soft anti-aliased mixes with black/white.
    if g < 12:
        return False
    return g >= r + 4 and g >= b + 4


def map_pixel(r: int, g: int, b: int, a: int) -> tuple[int, int, int, int]:
    if a == 0:
        return r, g, b, a
    # Keep near-neutral pixels (wordmark text, backgrounds).
    if abs(r - g) <= 3 and abs(g - b) <= 3 and abs(r - b) <= 3:
        return r, g, b, a
    if not _is_greenish(r, g, b):
        return r, g, b, a

    # Scale-aware remapping: treat RGB as scale * green_stop (anti-alias against black)
    # and also project onto the stop polyline for fully opaque logo colors.
    intensity = max(r, g, b)
    if intensity == 0:
        return r, g, b, a

    # Normalize toward unit green chromaticity using g as reference.
    proj = _project_onto_stops((r, g, b))
    if proj is not None:
        seg, t = proj
        i = int(seg)
        dst = _lerp(DST_STOPS[i], DST_STOPS[i + 1], t)
        return dst[0], dst[1], dst[2], a

    # Fallback: darkened / lightened greens via ratio to nearest stop.
    nearest = min(SRC_STOPS, key=lambda s: _dist2((r, g, b), s))
    ni = SRC_STOPS.index(nearest)
    if nearest[1] == 0:
        return r, g, b, a
    scale = g / nearest[1]
    dst = DST_STOPS[ni]
    return (
        max(0, min(255, int(round(dst[0] * scale)))),
        max(0, min(255, int(round(dst[1] * scale)))),
        max(0, min(255, int(round(dst[2] * scale)))),
        a,
    )


def recolor_image(im: Image.Image) -> tuple[Image.Image, int]:
    im = im.convert("RGBA")
    src = list(im.getdata())  # noqa: PIL deprecated but stable enough for one-shot
    out = []
    changed = 0
    for r, g, b, a in src:
        nr, ng, nb, na = map_pixel(r, g, b, a)
        if (nr, ng, nb, na) != (r, g, b, a):
            changed += 1
        out.append((nr, ng, nb, na))
    im.putdata(out)
    return im, changed


def write_png(path: Path) -> None:
    im = Image.open(path)
    out, changed = recolor_image(im)
    out.save(path, optimize=True)
    print(f"recolored {path.relative_to(ROOT)} ({changed} px)")


def write_ico_from_pngs(png_paths: list[Path], out_ico: Path) -> None:
    images = []
    for p in png_paths:
        im = Image.open(p).convert("RGBA")
        images.append(im)
    # Pillow writes multi-size ICO from the largest; pass all sizes.
    images[0].save(out_ico, format="ICO", sizes=[(im.width, im.height) for im in images], append_images=images[1:])
    print(f"wrote {out_ico.relative_to(ROOT)}")


def parse_icns_png_slots(data: bytes) -> list[tuple[bytes, bytes]]:
    assert data[:4] == b"icns"
    flen = struct.unpack(">I", data[4:8])[0]
    pos = 8
    slots = []
    while pos < flen:
        t = data[pos : pos + 4]
        l = struct.unpack(">I", data[pos + 4 : pos + 8])[0]
        payload = data[pos + 8 : pos + l]
        slots.append((t, payload))
        pos += l
    return slots


# Modern ICNS types that store PNG payloads
ICNS_PNG_TYPES = {
    b"ic04",  # 16x16
    b"ic05",  # 32x32
    b"ic07",  # 128x128
    b"ic08",  # 256x256
    b"ic09",  # 512x512
    b"ic10",  # 1024x1024
    b"ic11",  # 16x16@2x
    b"ic12",  # 32x32@2x
    b"ic13",  # 128x128@2x
    b"ic14",  # 256x256@2x
}


def recolor_icns(path: Path) -> None:
    data = path.read_bytes()
    slots = parse_icns_png_slots(data)
    out_slots: list[tuple[bytes, bytes]] = []
    for t, payload in slots:
        if t in ICNS_PNG_TYPES and payload[:8] == b"\x89PNG\r\n\x1a\n":
            from io import BytesIO

            im = Image.open(BytesIO(payload))
            out, changed = recolor_image(im)
            buf = BytesIO()
            out.save(buf, format="PNG", optimize=True)
            out_slots.append((t, buf.getvalue()))
            print(f"  icns {t!r}: recolored {changed} px")
        else:
            out_slots.append((t, payload))

    body = b""
    for t, payload in out_slots:
        body += t + struct.pack(">I", 8 + len(payload)) + payload
    out = b"icns" + struct.pack(">I", 8 + len(body)) + body
    path.write_bytes(out)
    print(f"wrote {path.relative_to(ROOT)}")


def recolor_ico(path: Path) -> None:
    """Recolor each embedded bitmap/PNG in an ICO by round-tripping through Pillow."""
    im = Image.open(path)
    frames = []
    try:
        i = 0
        while True:
            im.seek(i)
            frame, changed = recolor_image(im.copy())
            frames.append(frame)
            print(f"  ico frame {i} ({frame.size}): {changed} px")
            i += 1
    except EOFError:
        pass
    if not frames:
        raise RuntimeError(f"no frames in {path}")
    # Rebuild ICO with common Windows sizes present in original
    sizes = [(f.width, f.height) for f in frames]
    frames[0].save(path, format="ICO", sizes=sizes, append_images=frames[1:])
    print(f"wrote {path.relative_to(ROOT)}")


RASTER_PATHS = [
    "public/logos/wave-logo.png",
    "public/logos/wave-logo-dark.png",
    "public/logos/wave-logo-256.png",
    "public/logos/wave-dark.png",
    "assets/appicon-windows.png",
    "assets/waveterm-logo-with-bg.png",
    "assets/waveterm-logo-with-bg.ico",
    "assets/wave-dark.png",
    "assets/wave-light.png",
    "assets/waveterm-logo-horizontal-dark.png",
    "assets/waveterm-logo-horizontal-light.png",
    "build/icons/16x16.png",
    "build/icons/32x32.png",
    "build/icons/48x48.png",
    "build/icons/64x64.png",
    "build/icons/128x128.png",
    "build/icons/256x256.png",
    "build/icons/512x512.png",
]


def rebuild_packaging_icons_with_png2icons() -> None:
    """Rebuild build/icon.ico and build/icon.icns from assets/appicon-windows.png.

    Requires a one-shot: npm install --no-save png2icons
    """
    import subprocess

    script = r"""
const png2icons = require('png2icons');
const fs = require('fs');
const input = fs.readFileSync('assets/appicon-windows.png');
fs.writeFileSync('build/icon.ico', png2icons.createICO(input, png2icons.BICUBIC, 0, true));
fs.writeFileSync('build/icon.icns', png2icons.createICNS(input, png2icons.BICUBIC, 0));
const withBg = fs.readFileSync('assets/waveterm-logo-with-bg.png');
fs.writeFileSync('assets/waveterm-logo-with-bg.ico', png2icons.createICO(withBg, png2icons.BICUBIC, 0, true));
console.log('png2icons rebuilt build/icon.ico, build/icon.icns, assets/waveterm-logo-with-bg.ico');
"""
    subprocess.check_call(["node", "-e", script], cwd=ROOT)


def main() -> int:
    for rel in RASTER_PATHS:
        path = ROOT / rel
        if not path.exists():
            print(f"skip missing {rel}", file=sys.stderr)
            continue
        if path.suffix.lower() == ".ico":
            # Placeholder; final ICO is rebuilt by png2icons below.
            recolor_ico(path)
        else:
            write_png(path)

    # Prefer png2icons for multi-size packaging icons (full size set + modern ICNS PNG slots).
    try:
        rebuild_packaging_icons_with_png2icons()
    except Exception as exc:
        print(f"png2icons rebuild failed ({exc}); falling back to Pillow ICO + ICNS recolor", file=sys.stderr)
        recolor_ico(ROOT / "build/icon.ico")
        recolor_icns(ROOT / "build/icon.icns")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
