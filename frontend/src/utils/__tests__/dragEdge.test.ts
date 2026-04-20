import { describe, it, expect } from 'vitest'
import { detectEdgeZone } from '../dragEdge'

describe('detectEdgeZone', () => {
  // Smoke: happy path across the three zones.
  it('returns the correct zone for left / center / right', () => {
    expect(detectEdgeZone(10, 1000, 48)).toBe('left')
    expect(detectEdgeZone(500, 1000, 48)).toBeNull()
    expect(detectEdgeZone(990, 1000, 48)).toBe('right')
  })

  // Edge (边界值): exactly on the threshold boundaries.
  it('treats the threshold line as "outside" the edge zone', () => {
    // clientX === threshold: strictly less than → not left
    expect(detectEdgeZone(48, 1000, 48)).toBeNull()
    // clientX === viewportWidth - threshold: strictly greater than → not right
    expect(detectEdgeZone(952, 1000, 48)).toBeNull()
    // Just inside
    expect(detectEdgeZone(47, 1000, 48)).toBe('left')
    expect(detectEdgeZone(953, 1000, 48)).toBe('right')
  })

  // Edge (非法输入): malformed arguments must not trigger false positives.
  it('returns null for invalid viewport or threshold', () => {
    expect(detectEdgeZone(10, 0, 48)).toBeNull()
    expect(detectEdgeZone(10, -100, 48)).toBeNull()
    expect(detectEdgeZone(10, 1000, 0)).toBeNull()
    expect(detectEdgeZone(10, 1000, -5)).toBeNull()
    expect(detectEdgeZone(Number.NaN, 1000, 48)).toBeNull()
    expect(detectEdgeZone(10, Number.NaN, 48)).toBeNull()
  })

  // Edge (边界值): tiny viewport where edge zones would overlap.
  it('is deterministic when left/right zones overlap on a tiny viewport', () => {
    // 50px viewport with 48px threshold: clientX=25 would satisfy both; left wins.
    expect(detectEdgeZone(25, 50, 48)).toBe('left')
  })
})
