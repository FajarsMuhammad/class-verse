import type Phaser from 'phaser'

// Static floor geometry must not be tessellated on every WebGL frame.
export function freezeGround(scene: Phaser.Scene, graphics: Phaser.GameObjects.Graphics, width: number, height: number) {
  graphics.generateTexture('world-ground', width, height)
  scene.add.image(0, 0, 'world-ground').setOrigin(0).setDepth(-10)
  graphics.destroy()
}
