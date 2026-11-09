export interface LabelBox {
  x: number // centre
  y: number // top
  w: number
  h: number
}

// stackLabels pushes overlapping name tags down so every name stays readable
// when avatars stand together. Returns the new top of each box, same order.
export function stackLabels(boxes: LabelBox[]): number[] {
  const order = boxes.map((_, i) => i).sort((a, b) => boxes[a]!.y - boxes[b]!.y)
  const placed: LabelBox[] = []
  const tops = boxes.map((b) => b.y)
  for (const i of order) {
    const b = { ...boxes[i]! }
    let moved = true
    while (moved) {
      moved = false
      for (const p of placed) {
        const overlapX = Math.abs(p.x - b.x) < (p.w + b.w) / 2
        const overlapY = b.y < p.y + p.h && p.y < b.y + b.h
        if (overlapX && overlapY) {
          b.y = p.y + p.h
          moved = true
        }
      }
    }
    tops[i] = b.y
    placed.push(b)
  }
  return tops
}
