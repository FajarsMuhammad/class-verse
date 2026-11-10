#!/usr/bin/env python3
"""Generate original sporty sprites. Requires Pillow only for asset authoring."""
import argparse
import io
import json
import re
from pathlib import Path

from PIL import Image, ImageDraw

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "src/assets/avatars/sporty"
TOKENS = (ROOT / "src/styles/tokens.css").read_text()
COLORS = dict(re.findall(r"--([\w-]+):\s*(#[\da-fA-F]{6});", TOKENS))
STYLES = ("runner", "cap", "headphones", "varsity", "ponytail", "hoodie", "glasses")
PALETTE = ("nila", "mangga", "zamrud", "laut", "koral", "grafit")
DIRECTIONS = ("down", "left", "right", "up")
WIDTH, HEIGHT = 32, 48
COLUMNS = 5


def frame(style, color, direction, phase):
    """Idle at column 0, four alternating walk poses at columns 1–4."""
    image = Image.new("RGBA", (WIDTH, HEIGHT))
    draw = ImageDraw.Draw(image)
    ink = COLORS["color-ink"]
    white = COLORS["color-surface"]
    kit = COLORS[f"avatar-kit-{color}"]
    light = COLORS[f"avatar-kit-{color}-light"]
    skin = COLORS["map-wood" if style in ("varsity", "ponytail") else "map-path"]
    # Walk displacement never moves the ground anchor (16, 46).
    bob = -1 if phase in (2, 4) else 0
    stride = 1 if phase == 1 else -1 if phase == 3 else 0

    def rect(x, y, w, h, fill, body=False):
        y += bob if body else 0
        draw.rectangle((x, y, x + w - 1, y + h - 1), fill=fill)

    def poly(points, fill, body=False):
        draw.polygon([(x, y + (bob if body else 0)) for x, y in points], fill=fill)

    rect(7, 46, 19, 1, ink + "22")
    if direction in ("down", "up"):
        back = direction == "up"
        if style == "ponytail" and not back:
            poly([(8, 7), (4, 7), (4, 12), (2, 12), (2, 21), (5, 21), (5, 26), (8, 26)], ink, True)
            rect(5, 8, 4, 2, kit, True)
        # Legs alternate independently; shoes remain inside their own frame.
        for x, leg in ((10, stride), (18, -stride)):
            rect(x, 34, 5, 9 - abs(leg), ink)
            rect(x, 35, 1, 6, light)
            rect(x - 1, 42 - leg, 7, 4, white)
            rect(x, 43 - leg, 4, 1, kit)
            rect(x - 1, 45 - leg, 7, 1, ink)
        sleeve = white if style == "varsity" else kit
        for x, swing in ((6, -stride), (23, stride)):
            rect(x, 24 + swing, 3, 11, sleeve, True)
            rect(x, 35 + swing, 3, 4, skin, True)
            rect(x, 32 + swing, 3, 2, kit, True)
        poly([(11, 21), (20, 21), (22, 24), (22, 34), (9, 34), (9, 24)], kit, True)
        rect(10, 34, 12, 1, ink, True)
        if back:
            rect(10, 23, 3, 10, light, True)
            rect(19, 23, 2, 10, light, True)
            rect(12, 25, 8, 2, white, True)
        elif style == "cap":
            rect(10, 23, 2, 11, white, True)
            rect(20, 23, 2, 11, white, True)
            rect(15, 28, 3, 4, white, True)
        elif style != "hoodie":
            rect(11, 23, 3, 10, light, True)
            rect(16, 23, 1, 11, white, True)
            rect(10, 22, 4, 2, white, True)
            rect(19, 22, 3, 2, white, True)
            rect(19, 28, 2, 1, white, True)
        if style == "hoodie":
            poly([(11, 21), (20, 21), (20, 24), (18, 26), (13, 26), (11, 24)], light, True)
            if not back:
                rect(13, 26, 1, 3, white, True)
                rect(19, 26, 1, 3, white, True)
                rect(13, 31, 7, 1, light, True)
        rect(14, 18, 5, 4, skin, True)
        rect(9, 6, 14, 12, ink, True)
        if not back:
            poly([(11, 8), (21, 8), (23, 11), (23, 17), (21, 19), (12, 19), (10, 17), (10, 11)], skin, True)
            rect(8, 13, 3, 4, skin, True)
            rect(23, 13, 2, 4, skin, True)
            poly([(9, 12), (9, 7), (11, 5), (20, 5), (23, 8), (23, 11), (19, 11), (19, 9), (16, 9), (16, 11), (12, 11), (12, 13)], ink, True)
            rect(14, 13, 1, 2, ink, True)
            rect(21, 13, 1, 2, ink, True)
            rect(17, 18, 3, 1, COLORS["color-accent-ink"], True)
            if style == "glasses":
                # Open lenses preserve skin and eyes; bridge connects both rims.
                for x in (11, 19):
                    rect(x, 12, 6, 1, ink, True)
                    rect(x, 16, 6, 1, ink, True)
                    rect(x, 13, 1, 3, ink, True)
                    rect(x + 5, 13, 1, 3, ink, True)
                rect(17, 13, 2, 1, ink, True)
                rect(9, 13, 2, 1, ink, True)
        else:
            poly([(11, 4), (20, 4), (23, 7), (23, 17), (21, 20), (11, 20), (8, 17), (8, 8)], ink, True)
        if style == "runner":
            rect(9, 9, 15, 2, white, True)
            rect(10, 9, 3, 2, kit, True)
        if style == "cap":
            poly([(9, 8), (9, 5), (12, 3), (20, 3), (23, 6), (23, 9), (27, 9), (27, 10), (10, 10)], kit, True)
            rect(12, 4, 2, 4, light, True)
            if back:
                rect(14, 8, 5, 2, ink, True)
            else:
                rect(20, 9, 9, 2, ink, True)
        if style == "headphones":
            poly([(7, 16), (7, 7), (10, 4), (12, 3), (20, 3), (24, 7), (25, 16), (23, 16), (22, 9), (20, 6), (12, 6), (9, 9), (9, 16)], ink, True)
            for x in (6, 24):
                rect(x, 11, 4, 8, ink, True)
                rect(x + 1, 12, 2, 5, light, True)
        if style == "ponytail" and back:
            rect(12, 9, 8, 11, ink, True)
            rect(14, 18, 5, 9, ink, True)
            rect(13, 13, 7, 2, kit, True)
    else:
        # Author the right side, mirror the whole frame for the left side.
        if style == "ponytail":
            poly([(10, 7), (5, 8), (5, 18), (7, 18), (7, 25), (10, 25)], ink, True)
            rect(7, 9, 4, 2, kit, True)
        for x, swing in ((12, stride * 3), (16, -stride * 3)):
            rect(x + swing, 34, 4, 10 - abs(stride), ink)
            rect(x + swing, 35, 1, 6, light)
            rect(x + swing, 42 - abs(stride), 7, 4, white)
            rect(x + swing, 43 - abs(stride), 4, 1, kit)
            rect(x + swing, 45 - abs(stride), 7, 1, ink)
        rect(11, 22, 10, 13, kit, True)
        rect(11, 23, 2, 10, light, True)
        rect(11, 34, 10, 1, ink, True)
        if style == "hoodie":
            rect(10, 21, 5, 5, light, True)
        rect(14, 18, 5, 4, skin, True)
        rect(10, 7, 12, 12, ink, True)
        poly([(15, 8), (21, 8), (23, 11), (23, 13), (25, 13), (25, 15), (23, 15), (23, 18), (20, 20), (15, 19)], skin, True)
        poly([(10, 14), (10, 7), (13, 4), (20, 4), (23, 7), (23, 11), (18, 10), (17, 14)], ink, True)
        rect(21, 12, 1, 2, ink, True)
        rect(20, 18, 3, 1, COLORS["color-accent-ink"], True)
        if style == "glasses":
            rect(13, 12, 6, 1, ink, True)
            rect(19, 11, 5, 1, ink, True)
            rect(19, 15, 5, 1, ink, True)
            rect(19, 12, 1, 3, ink, True)
            rect(23, 12, 1, 3, ink, True)
        if style == "runner":
            rect(10, 9, 13, 2, white, True)
        if style == "cap":
            poly([(10, 8), (10, 5), (13, 3), (20, 3), (23, 6), (23, 9), (29, 9), (29, 11), (16, 11)], kit, True)
            rect(11, 5, 2, 3, light, True)
            rect(23, 10, 6, 1, ink, True)
        if style == "headphones":
            rect(13, 3, 3, 11, ink, True)
            rect(12, 11, 6, 8, ink, True)
            rect(13, 12, 4, 5, light, True)
        sleeve = white if style == "varsity" else kit
        arm = stride * 2
        rect(15 + arm, 24, 4, 11, sleeve, True)
        rect(15 + arm, 33, 4, 2, white if style != "varsity" else kit, True)
        rect(15 + arm, 35, 4, 4, skin, True)
        if direction == "left":
            image = image.transpose(Image.Transpose.FLIP_LEFT_RIGHT)
    return image


def generate():
    output = {}
    for style in STYLES:
        for color in PALETTE:
            key = f"{style}-{color}"
            sheet = Image.new("RGBA", (WIDTH * COLUMNS, HEIGHT * len(DIRECTIONS)))
            for row, direction in enumerate(DIRECTIONS):
                for column in range(COLUMNS):
                    sheet.paste(frame(style, color, direction, column), (column * WIDTH, row * HEIGHT))
            for name, image in ((key, sheet), (f"{key}-idle", frame(style, color, "down", 0))):
                data = io.BytesIO()
                image.save(data, format="PNG", optimize=True)
                output[f"{name}.png"] = data.getvalue()
    manifest = {
        "frameWidth": WIDTH, "frameHeight": HEIGHT, "columns": COLUMNS,
        "rows": list(DIRECTIONS), "anchor": {"x": 16, "y": 46},
        "idleColumn": 0, "walkColumns": [1, 2, 3, 4], "frameRate": 8,
        "styles": list(STYLES), "colors": list(PALETTE),
    }
    output["manifest.json"] = (json.dumps(manifest, indent=2) + "\n").encode()
    return output


def check(output):
    assert len(output) == len(STYLES) * len(PALETTE) * 2 + 1
    for style in STYLES:
        # Clothing colors must not change skin or reuse another color's artwork.
        assert len({frame(style, c, "down", 0).tobytes() for c in PALETTE}) == len(PALETTE)
        assert len({frame(style, c, "down", 0).getpixel((18, 15)) for c in PALETTE}) == 1
        for color in PALETTE:
            key = f"{style}-{color}"
            sheet = Image.open(io.BytesIO(output[f"{key}.png"]))
            assert sheet.size == (160, 192) and sheet.mode == "RGBA"
            assert sheet.getpixel((0, 0))[3] == 0
            for row, direction in enumerate(DIRECTIONS):
                frames = [sheet.crop((i * WIDTH, row * HEIGHT, (i + 1) * WIDTH, (row + 1) * HEIGHT)) for i in range(COLUMNS)]
                assert len({f.tobytes() for f in frames}) >= 4, (key, direction)
                for f in frames:
                    left, top, right, bottom = f.getbbox()
                    assert 0 < left < right < WIDTH and 0 < top < bottom < HEIGHT
                if direction == "left":
                    assert frames[1].tobytes() == frame(style, color, "right", 1).transpose(Image.Transpose.FLIP_LEFT_RIGHT).tobytes()
            idle = Image.open(io.BytesIO(output[f"{key}-idle.png"]))
            assert idle.tobytes() == frame(style, color, "down", 0).tobytes()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="verify generated artwork and committed files without writing")
    args = parser.parse_args()
    assets = generate()
    check(assets)
    if args.check:
        for name, data in assets.items():
            assert (OUT / name).read_bytes() == data, f"stale asset: {name}"
    else:
        OUT.mkdir(parents=True, exist_ok=True)
        for name, data in assets.items():
            (OUT / name).write_bytes(data)
    count = len(STYLES) * len(PALETTE)
    print(f"{'Checked' if args.check else 'Generated'} {count} sheets, {count * COLUMNS * len(DIRECTIONS)} frames, {count} idle portraits and manifest")
