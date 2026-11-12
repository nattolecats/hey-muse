#!/usr/bin/env python3
"""Make the Japanese font the screen falls back to, echod/internal/lib/cjk/NotoSansJP-subset.ttf, from
Google's Noto Sans JP.

    curl -LO 'https://raw.githubusercontent.com/google/fonts/main/ofl/notosansjp/NotoSansJP%5Bwght%5D.ttf'
    uv run --with fonttools tools/fonts/make_jp_font.py --font 'NotoSansJP[wght].ttf'

The font as it comes is 9.6 MB and every weight at once. This keeps one weight, Medium, which stands
beside both the regular and the bold of the screen's own font, and only what JIS X 0208 has: the kana,
the 6355 kanji of its two levels, and its punctuation and symbols, with the full width forms. Latin
letters are left out, since the screen's own font draws those.

Noto Sans JP is under the SIL Open Font License, which is beside the font as OFL.txt.
"""
import argparse
import pathlib

from fontTools import subset
from fontTools.ttLib import TTFont
from fontTools.varLib import instancer

ROOT = pathlib.Path(__file__).resolve().parents[2]
WEIGHT = 500


def jis_x_0208():
    """Every character JIS X 0208 has, by way of EUC-JP, which is that table with both high bits set."""
    out = set()
    for row in range(1, 95):
        for cell in range(1, 95):
            try:
                out.update(ord(c) for c in bytes([0xA0 + row, 0xA0 + cell]).decode("euc_jp"))
            except UnicodeDecodeError:
                pass
    return out


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--font", required=True, help="NotoSansJP[wght].ttf")
    ap.add_argument("--out", default=str(ROOT / "echod" / "internal" / "lib" / "cjk" / "NotoSansJP-subset.ttf"))
    a = ap.parse_args()

    font = instancer.instantiateVariableFont(TTFont(a.font), {"wght": WEIGHT})
    keep = jis_x_0208()
    keep.update(range(0x3000, 0x3100))  # punctuation and kana, whole
    keep.update(range(0xFF00, 0xFFF0))  # full width and half width forms
    keep -= set(range(0x20, 0x250))     # Latin is the screen's own font's

    opts = subset.Options()
    opts.layout_features = []  # one glyph a character is all the screen draws
    opts.hinting = False
    opts.name_IDs = [0, 1, 2, 3, 4, 5, 6, 13, 14]  # who made it and its license stay with it
    opts.notdef_outline = True
    s = subset.Subsetter(opts)
    s.populate(unicodes=sorted(keep))
    s.subset(font)
    font.save(a.out)
    have = sum(1 for c in keep if c in font.getBestCmap())
    print("%s: %d characters, %.1f MB" % (a.out, have, pathlib.Path(a.out).stat().st_size / 1048576))


if __name__ == "__main__":
    main()
