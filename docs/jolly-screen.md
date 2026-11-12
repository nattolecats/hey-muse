# The Jolly screen

A Show can draw a character walking about a room in place of the clock, with the time and the date in
a corner of the wall behind it. The room has depth. The character is drawn smaller and higher up the
further back it goes, it turns the way it is walking, and now and then it comes up to the glass to
look out.

It is a clock style, named **Jolly**, on the Show 5 and Show 8. The Spot and the Dot do not draw it.

**Not tried on a device yet.** It was drawn and looked at on a computer only. See
[What is not known](#what-is-not-known).

The character is yours to supply and is not in this repository. Until one is installed, the style is
the room and the time, and says "No character installed".

## 1. Make the sprite sheets

You need one picture of the character seen from four sides, two across and two down: front and right
on top, back and left under them, on a plain pale background. "Right" is the character facing the
right of the picture. Words over each view do no harm.

You also need [uv](https://docs.astral.sh/uv/).

```sh
uv run --with numpy --with pillow --with opencv-python-headless \
    tools/muse-avatar/make_roam_sprites.py --sheet turnaround.png --preview build/jolly-preview
```

That writes five sheets and `manifest.json` to `build/jolly/`, which git ignores, and each animation
as a GIF to `build/jolly-preview/` so you can check the cut-out before copying anything:

| File | What it holds |
|---|---|
| `idle.png` | the character at rest, facing out: it breathes and blinks |
| `walk_front.png` | walking toward you |
| `walk_back.png` | walking away |
| `walk_left.png`, `walk_right.png` | walking across |

There is one still picture of each side, so the walking is made by moving that picture: it bobs with
each step, rocks from foot to foot and lifts the foot it is not standing on. A character in a T-pose
walks with its arms out.

If the views are laid out another way, say so with `--order`, for example
`--order front,back,left,right`. If some of the floor's shadow is left under the feet, lower
`--shadow` from its 190. The set takes about 13 MB of the device's memory.

## 2. Put them on the device

```sh
scp -r build/jolly root@<address>:/data/misc/techo5/
ssh root@<address> killall techo5
```

The daemon reads the set once as it starts and logs `Jolly character loaded`. A set it cannot read is
logged as `no Jolly character installed`, with the reason. To take the character off again, delete
`/data/misc/techo5/jolly` and restart the daemon.

## 3. Choose the style

Swipe left or right across the clock until **Jolly** shows, or pick it under Settings, Display, Clock
style.

## A set from a real model

The device plays whatever is in the five sheets. If you have a 3D model of the character, render its
own walk and rest loops and lay them out the same way, and it walks properly with no change to the
device: square frames in rows, a paletted PNG per animation whose palette carries each color's alpha,
and a `manifest.json` like the one the tool writes. In each frame the feet stand 95% of the way down
and the character is no wider than 92% of the frame, since the screen counts on both to place it and
its shadow.

## What is not known

- **How much of the processor it takes.** The clock is otherwise drawn once a second. This style
  draws the whole screen 12 times a second for as long as it is up. On a computer a frame takes about
  2.5 ms, which suggests a few tens of milliseconds on the Show, but it was not measured there, and
  nor was whether the wake word is heard as well while it runs. If it is too much, lower `fps` in
  `manifest.json` on the device: the character walks at the same speed, in fewer frames.
- **How it looks on the panel.** The room's colors were chosen on a computer screen.
