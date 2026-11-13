import Phaser from 'phaser'
import { cssToken } from './tiles'
import { roomLayout } from './roomLayout'
import { pill } from './worldArt'

const color = (token: string) => Phaser.Display.Color.HexStringToColor(cssToken(token)).color

// Stations are static scenery, not simulated experiments or recording tools.
export function drawStations(scene: Phaser.Scene, key: string) {
  const layout = roomLayout(key)
  if (!layout) return
  if (!layout.stations.length) return
  const g = scene.add.graphics()
  for (const s of layout.stations) {
    const zone = s.kind.endsWith('zone')
    const accent = key === 'staff' ? '--kv-text-muted' : '--kv-subject-ipa'
    g.fillStyle(color(zone ? accent : '--kv-surface'), zone ? 0.07 : 1)
    if (s.kind === 'dome') {
      g.fillStyle(color('--kv-ink')).fillRoundedRect(s.x, s.y, s.w, s.h, { tl: s.w / 2, tr: s.w / 2, bl: 24, br: 24 })
      g.fillStyle(color('--kv-star'))
      for (let y = 30; y < s.h - 20; y += 31) {
        for (let x = 30; x < s.w - 20; x += 37) {
          if (y > 110 || Math.abs(x - s.w / 2) < y + 40) g.fillCircle(s.x + x, s.y + y, 1.5)
        }
      }
    } else {
      const fill = s.kind === 'ex-body' ? '--kv-library-wall' : s.kind === 'ex-circuit' ? '--kv-star' : s.kind === 'vrpod' ? '--kv-subject-kreatif' : '--kv-surface'
      if (!zone) g.fillStyle(color(fill), s.kind === 'ex-circuit' ? 0.3 : 1)
      const radius = s.kind === 'vrpod' || s.kind === 'deck' ? s.w / 2 : 18
      g.fillRoundedRect(s.x, s.y, s.w, s.h, radius)
      g.lineStyle(zone ? 2 : 3, color(zone ? accent : '--kv-ink'))
      if (zone) {
        for (let x = s.x + 18; x < s.x + s.w - 18; x += 12) {
          g.lineBetween(x, s.y, x + 7, s.y)
          g.lineBetween(x, s.y + s.h, x + 7, s.y + s.h)
        }
        for (let y = s.y + 18; y < s.y + s.h - 18; y += 12) {
          g.lineBetween(s.x, y, s.x, y + 7)
          g.lineBetween(s.x + s.w, y, s.x + s.w, y + 7)
        }
      } else g.strokeRoundedRect(s.x, s.y, s.w, s.h, radius)
      if (s.kind === 'ex-solar') {
        g.lineStyle(2, color('--kv-ink'), 0.3)
        for (const r of [22, 38, 53]) g.strokeCircle(s.x + s.w / 2, s.y + s.h / 2, r)
        g.fillStyle(color('--kv-star')).fillCircle(s.x + s.w / 2, s.y + s.h / 2, 12)
      } else if (!zone) {
        const symbol = ({ 'ex-body': '人', 'ex-volcano': '△', 'ex-circuit': 'ϟ', vrpod: 'VR', deck: '⌕' } as Record<string, string>)[s.kind]
        if (symbol) scene.add.text(s.x + s.w / 2, s.y + s.h / 2, symbol, {
          fontFamily: cssToken('--kv-font-ui'), fontSize: '36px', fontStyle: 'bold', color: cssToken('--kv-ink'),
        }).setOrigin(0.5).setDepth(-8)
      }
    }
    pill(scene, s.x + s.w / 2, s.y, s.label, 100000).setName(`station-${s.id}`)
  }
  g.generateTexture('room-stations', layout.width, layout.height)
  scene.add.image(0, 0, 'room-stations').setOrigin(0).setDepth(-9)
  g.destroy()
}
