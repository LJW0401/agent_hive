import { describe, it, expect } from 'vitest'

// Test the split pane ratio constraints (unit-level logic test)
describe('Split pane constraints', () => {
  const MAX_TODO_RATIO = 2 / 3

  const clampRatio = (ratio: number) => Math.min(MAX_TODO_RATIO, Math.max(0, ratio))

  it('should clamp ratio to 0 when dragged fully left', () => {
    expect(clampRatio(-0.1)).toBe(0)
    expect(clampRatio(0)).toBe(0)
  })

  it('should allow hiding todo (ratio near 0)', () => {
    const ratio = clampRatio(0.01)
    expect(ratio).toBeLessThan(0.02)
  })

  it('should not exceed 2/3 max ratio', () => {
    expect(clampRatio(0.7)).toBeCloseTo(MAX_TODO_RATIO)
    expect(clampRatio(0.9)).toBeCloseTo(MAX_TODO_RATIO)
    expect(clampRatio(1.0)).toBeCloseTo(MAX_TODO_RATIO)
  })

  it('should allow normal ratios within range', () => {
    expect(clampRatio(0.25)).toBeCloseTo(0.25)
    expect(clampRatio(0.5)).toBeCloseTo(0.5)
    expect(clampRatio(MAX_TODO_RATIO)).toBeCloseTo(MAX_TODO_RATIO)
  })
})
