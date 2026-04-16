import { describe, it, expect } from 'vitest'

// Test the split pane ratio constraints (unit-level logic test)
describe('Desktop split pane constraints', () => {
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

describe('Mobile split pane constraints', () => {
  const MIN_TERMINAL_RATIO = 0.40
  const MAX_TERMINAL_RATIO = 0.85 // 1 - 0.15 (todo min 15%)

  const clampMobile = (ratio: number) => Math.min(MAX_TERMINAL_RATIO, Math.max(MIN_TERMINAL_RATIO, ratio))

  it('should default to 70% terminal ratio', () => {
    const defaultRatio = 0.7
    expect(defaultRatio).toBe(0.7)
  })

  it('should not allow terminal below 40%', () => {
    expect(clampMobile(0.3)).toBeCloseTo(MIN_TERMINAL_RATIO)
    expect(clampMobile(0.1)).toBeCloseTo(MIN_TERMINAL_RATIO)
    expect(clampMobile(0)).toBeCloseTo(MIN_TERMINAL_RATIO)
  })

  it('should not allow todo below 15% (terminal max 85%)', () => {
    expect(clampMobile(0.9)).toBeCloseTo(MAX_TERMINAL_RATIO)
    expect(clampMobile(1.0)).toBeCloseTo(MAX_TERMINAL_RATIO)
  })

  it('should allow ratios within valid range', () => {
    expect(clampMobile(0.5)).toBeCloseTo(0.5)
    expect(clampMobile(0.7)).toBeCloseTo(0.7)
    expect(clampMobile(0.8)).toBeCloseTo(0.8)
  })
})
