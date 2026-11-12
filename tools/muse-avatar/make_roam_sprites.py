#!/usr/bin/env python3
"""Make the sprite set for a TECHO5 Show's Jolly clock style from one picture of a character seen from
four sides: front, right, back and left, two across and two down, on a plain pale background.

    uv run --with numpy --with pillow --with opencv-python-headless tools/muse-avatar/make_roam_sprites.py \\
        --sheet turnaround.png

The sheets go to build/jolly (--out), in the form echod's lib/avatar reads: the character at rest, and
walking toward you, away, left and right. Copy the folder to /data/misc/techo5/jolly on the device.
The picture, like the character, is not part of this repository.

There is one still picture of each side, so the walking is made by moving that picture: it bobs with
each step, rocks from foot to foot, and lifts the foot it is not standing on. At rest it breathes and
blinks. A set drawn from a real model of the character, in the same five sheets, plays as well.
"""
import argparse
import json
import math
import pathlib
import sys

import cv2
import numpy as np
from PIL import Image

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
import make_video_sprites as mvs  # noqa: E402

ROOT = pathlib.Path(__file__).resolve().parents[2]
COLUMNS = 6
# Where the character goes in a frame, which the screen counts on (display/jolly_style.go): its feet
# this far down, and no wider or taller than these shares of the frame.
FOOT, WIDE, TALL = 0.95, 0.92, 0.80
# Frames are worked on at this many times their size and brought down at the end, for a clean edge.
OVER = 2


def views(path, order, shadow):
    """The picture's four views, each cut out and cropped to the character, by name."""
    rgb = np.asarray(Image.open(path).convert("RGB")).astype(np.float32)
    # The background is rarely pure white. Whatever it is at the edges becomes white.
    edge = np.concatenate([rgb[0], rgb[-1], rgb[:, 0], rgb[:, -1]])
    rgb = np.clip(rgb * (255.0 / np.maximum(np.median(edge, axis=0), 1.0)), 0, 255)
    # The shadow the character casts on the floor is pale and has no color; fur and skin have some.
    lo, hi = rgb.min(axis=2), rgb.max(axis=2)
    rgb[(hi - lo <= 12) & (lo >= shadow)] = 255.0

    h, w = rgb.shape[:2]
    out = {}
    for i, name in enumerate(order):
        y, x = (i // 2) * (h // 2), (i % 2) * (w // 2)
        cut = mvs.cut_out(rgb[y:y + h // 2, x:x + w // 2].astype(np.uint8), 1)
        # The character is the one largest thing there: a word written over it is not part of it.
        n, labels, stats, _ = cv2.connectedComponentsWithStats((cut[..., 3] > 8).astype(np.uint8), connectivity=8)
        if n < 2:
            sys.exit("nothing to cut out in the %s view" % name)
        keep = 1 + int(np.argmax(stats[1:, cv2.CC_STAT_AREA]))
        cut[labels != keep] = 0
        bx, by, bw, bh = (stats[keep, k] for k in (cv2.CC_STAT_LEFT, cv2.CC_STAT_TOP, cv2.CC_STAT_WIDTH, cv2.CC_STAT_HEIGHT))
        out[name] = cut[by:by + bh, bx:bx + bw]
    return out


def premultiplied(rgba):
    f = rgba.astype(np.float32)
    f[..., :3] *= f[..., 3:] / 255.0
    return f


def bend(pre, rise_left, rise_right, swing):
    """The character with a foot lifted, or its feet swung: only what is below the body moves.

    rise_left and rise_right are how far the foot on that side of the picture comes up, and swing how
    far both feet go sideways, as shares of the character's height.
    """
    h, w = pre.shape[:2]
    ys, xs = np.mgrid[0:h, 0:w].astype(np.float32)
    low = np.clip((ys - 0.78 * h) / (0.22 * h), 0, 1)
    left = 1.0 / (1.0 + np.exp((xs - w / 2.0) / (0.03 * w)))
    rise = (rise_left * left + rise_right * (1 - left)) * h
    return cv2.remap(pre, xs - swing * h * low, ys + rise * low, cv2.INTER_LINEAR, borderMode=cv2.BORDER_CONSTANT, borderValue=0)


def pose(pre, scale, cell, lift=0.0, tilt=0.0, stretch=1.0):
    """One frame: the character scaled and stood in a cell, lifted, tilted about its feet and stretched.

    lift is a share of the character's height, tilt in degrees clockwise, stretch how much taller it
    is drawn, and it is drawn that much narrower.
    """
    h, w = pre.shape[:2]
    side = cell * OVER
    k = scale * OVER
    t = math.radians(tilt)
    # From the picture to the cell: its feet's middle to the origin, stretch, turn, and to its place.
    sx, sy = k / stretch, k * stretch
    m = np.array([[math.cos(t) * sx, -math.sin(t) * sy, 0], [math.sin(t) * sx, math.cos(t) * sy, 0]], dtype=np.float64)
    m[:, 2] = -m[:, :2] @ np.array([w / 2.0, float(h)])
    m[0, 2] += side / 2.0
    m[1, 2] += FOOT * side - lift * h * k
    big = cv2.warpAffine(pre, m, (side, side), flags=cv2.INTER_LINEAR, borderMode=cv2.BORDER_CONSTANT, borderValue=0)
    small = cv2.resize(big, (cell, cell), interpolation=cv2.INTER_AREA)
    alpha = small[..., 3:]
    color = np.where(alpha > 1.0, small[..., :3] * 255.0 / np.maximum(alpha, 1.0), 0)
    return Image.fromarray(np.dstack([np.clip(color, 0, 255), np.clip(alpha, 0, 255)]).astype(np.uint8), "RGBA")


def walk_facing(pre, scale, cell, n):
    """Walking toward or away: rocking from foot to foot, the other foot off the floor."""
    out = []
    for i in range(n):
        s = math.sin(2 * math.pi * i / n)
        bent = bend(pre, 0.05 * max(s, 0), 0.05 * max(-s, 0), 0)
        out.append(pose(bent, scale, cell, lift=0.02 * abs(s), tilt=3.5 * s, stretch=1 + 0.02 * math.cos(4 * math.pi * i / n)))
    return out


def walk_side(pre, scale, cell, n, forward):
    """Walking across: leaning the way it goes (forward is 1 to the right, -1 to the left), its feet
    swinging under it."""
    out = []
    for i in range(n):
        s = math.sin(2 * math.pi * i / n)
        bent = bend(pre, 0.02 * abs(s), 0.02 * abs(s), 0.035 * s * forward)
        out.append(pose(bent, scale, cell, lift=0.025 * abs(s), tilt=forward * (2.5 + 2.0 * s),
                        stretch=1 + 0.02 * math.cos(4 * math.pi * i / n)))
    return out


def eyes_shut(rgba):
    """The front view with its eyes closed: each eye painted over with the skin around it, and a line
    where it was. The picture as it is, if no two eyes are found."""
    h, w = rgba.shape[:2]
    dark = ((rgba[..., :3].max(axis=2) < 80) & (rgba[..., 3] > 200)).astype(np.uint8)
    dark[int(0.5 * h):] = 0
    n, labels, stats, mids = cv2.connectedComponentsWithStats(dark, connectivity=8)
    found = sorted(range(1, n), key=lambda k: -stats[k, cv2.CC_STAT_AREA])[:2]
    # Two eyes are much of a size and at much the same height; anything else is not a pair of eyes.
    if len(found) < 2 or stats[found[1], cv2.CC_STAT_AREA] * 3 < stats[found[0], cv2.CC_STAT_AREA] \
            or abs(mids[found[0]][1] - mids[found[1]][1]) > 0.05 * h:
        return rgba
    mask = cv2.dilate(np.isin(labels, found).astype(np.uint8) * 255, np.ones((7, 7), np.uint8))
    out = rgba.copy()
    out[..., :3] = cv2.inpaint(np.ascontiguousarray(rgba[..., :3]), mask, 5, cv2.INPAINT_TELEA)
    for k in found:
        x, y, bw, bh = (int(stats[k, c]) for c in (cv2.CC_STAT_LEFT, cv2.CC_STAT_TOP, cv2.CC_STAT_WIDTH, cv2.CC_STAT_HEIGHT))
        ink = tuple(int(v) for v in rgba[labels == k][:, :3].mean(axis=0))
        cy = y + bh // 2 + bh // 6
        cv2.line(out, (x, cy), (x + bw - 1, cy), ink + (255,), max(2, bh // 6), cv2.LINE_AA)
    return out


def rest(front, scale, cell, n):
    """At rest, facing out: breathing, and a blink near the end of the loop."""
    open_, shut = premultiplied(front), premultiplied(eyes_shut(front))
    out = []
    for i in range(n):
        blink = n >= 12 and n - 6 <= i < n - 4
        out.append(pose(shut if blink else open_, scale, cell, stretch=1 + 0.012 * math.sin(2 * math.pi * i / n)))
    return out


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--sheet", required=True, help="the picture of the four views")
    ap.add_argument("--order", default="front,right,back,left",
                    help="which view is where, left to right and then down; right is the character facing right")
    ap.add_argument("--out", default=str(ROOT / "build" / "jolly"), help="where the sheets go")
    ap.add_argument("--cell", type=int, default=400, help="a frame's side, in pixels")
    ap.add_argument("--fps", type=float, default=12, help="frames a second")
    ap.add_argument("--steps", type=int, default=12, help="frames in one walking loop, two steps")
    ap.add_argument("--rest", type=int, default=36, help="frames in the loop at rest")
    ap.add_argument("--shadow", type=int, default=190,
                    help="how dark the colorless shadow on the floor gets, 0 to 255; lower takes more of it out")
    ap.add_argument("--preview", help="also write each animation here as a GIF, on a dark ground")
    a = ap.parse_args()

    order = [v.strip() for v in a.order.split(",")]
    if sorted(order) != ["back", "front", "left", "right"]:
        sys.exit("--order names front, right, back and left, each once")
    sheet = pathlib.Path(a.sheet).expanduser()
    if not sheet.is_file():
        sys.exit("no picture at %s" % sheet)
    cut = views(sheet, order, a.shadow)
    # One scale for all four, so the character is one size whichever way it turns.
    scale = min(WIDE * a.cell / max(v.shape[1] for v in cut.values()), TALL * a.cell / max(v.shape[0] for v in cut.values()))
    pre = {name: premultiplied(v) for name, v in cut.items()}
    animations = {
        "idle": rest(cut["front"], scale, a.cell, a.rest),
        "walk_front": walk_facing(pre["front"], scale, a.cell, a.steps),
        "walk_back": walk_facing(pre["back"], scale, a.cell, a.steps),
        "walk_left": walk_side(pre["left"], scale, a.cell, a.steps, -1),
        "walk_right": walk_side(pre["right"], scale, a.cell, a.steps, 1),
    }

    out = pathlib.Path(a.out)
    out.mkdir(parents=True, exist_ok=True)
    mvs.COLUMNS = COLUMNS
    manifest = {"cell": a.cell, "fps": a.fps, "animations": {}}
    for name, cells in animations.items():
        made = mvs.sheet(cells, a.cell)
        made.save(out / (name + ".png"), optimize=True, transparency=made.info["transparency"])
        manifest["animations"][name] = {"frames": len(cells), "columns": COLUMNS, "loop": True}
        print("%-10s %3d frames  %6.1f KB" % (name, len(cells), (out / (name + ".png")).stat().st_size / 1024))
        if a.preview:
            pathlib.Path(a.preview).mkdir(parents=True, exist_ok=True)
            shown = []
            for c in cells:
                ground = Image.new("RGBA", (a.cell, a.cell), (0x2b, 0x28, 0x24, 255))
                ground.alpha_composite(c)
                shown.append(ground.convert("RGB"))
            shown[0].save(pathlib.Path(a.preview) / (name + ".gif"), save_all=True, append_images=shown[1:],
                          duration=round(1000 / a.fps), loop=0)
    (out / "manifest.json").write_text(json.dumps(manifest, indent=1) + "\n")
    decoded = sum(-(-v["frames"] // COLUMNS) * COLUMNS * a.cell * a.cell for v in manifest["animations"].values())
    print("decoded on the device: %.1f MB" % (decoded / 1048576))


if __name__ == "__main__":
    main()
