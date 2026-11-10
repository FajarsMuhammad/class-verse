# Sporty avatar asset pack — US-04

Original pixel art for Kelasverse, based on the approved sporty concept. No Gather assets are included.
Profile and Phaser use this pack via `sporty-<style>-<color>` keys. Existing SVG avatars and saved legacy keys remain supported without forced conversion.

## Files and playback

- Forty-two transparent PNG spritesheets: seven styles × six clothing colors.
- Colors: `nila`, `mangga`, `zamrud`, `laut`, `koral`, `grafit`; base/light panel pairs live in `tokens.css`.
- Each sheet is **160 × 192 px**, containing **5 columns × 4 rows**.
- Each frame is **32 × 48 px**, with no margin or spacing.
- Rows, top to bottom: **down, left, right, up**.
- Column **0** is idle, facing the row's direction.
- Columns **1, 2, 3, 4** loop at **8 fps** while moving. Reset to idle when stopped; do not animate idle.
- Phaser frame index: `row * 5 + column`.
- Ground anchor in each frame: **(16, 46)**. Normalized origin: **(0.5, 46 / 48)**.
- Render with nearest-neighbor filtering, preserving the **2:3 aspect ratio**. Proposed screen size is 32 × 48 px,
  not 48 × 48 px. A 2× display is 64 × 96 px.
- `*-idle.png`: matching down-facing portrait for profile previews.
- `manifest.json`: frame dimensions, row order, animation columns, fps and all six colors.

Styles: `runner`, `cap`, `headphones`, `varsity`, `ponytail`, `hoodie`, `glasses`.
Glasses have open dark rims at the front, a temple at the side, and no lenses on the back of the head.
Headphones have **no boom microphone** in any direction. Skin examples are independent of clothing color.
The left-facing pose mirrors the right-facing pose; front/back have separately drawn faces, hair and clothing.

## Preview and regenerate

Open `docs/concepts/avatars-sporty-colors.html` for the style/color picker preview (no account persistence).
Open `docs/concepts/avatars-sporty-animation.html` for the original two-color direction gallery,
through a static HTTP server rooted at the repository.
Preview has direction buttons, pause/resume, all twelve variants, and `prefers-reduced-motion` support.
`docs/concepts/avatars-sporty-walk.gif` shows the four directions without requiring the preview page.

Authoring requires Python 3 and Pillow (already available in the development environment; not a runtime dependency):

```sh
python3 classverse-frontend/scripts/generate_sporty_avatars.py
python3 classverse-frontend/scripts/generate_sporty_avatars.py --check
```

The generator reads existing color tokens from `src/styles/tokens.css`. Its checks cover sheet dimensions,
transparency, frame bounds, distinct walk poses, mirrored directions, portrait matching and stale output.

## Integration

Backend/OpenAPI explicitly allow all 42 sporty keys alongside legacy keys. The profile's style and color
controls combine into one saved key. Old choices stay unchanged unless the user selects a new style/color.
Phaser selects frames from rendered position changes rather than held keys alone, so blocked movement
returns to idle. Reduced motion keeps the correct directional idle frame. Movement, collision and voice
rules are unchanged.
