import { describe, it, expect } from 'vitest'
import { moveContainerToPage } from '../layout'
import type { LayoutEntry } from '../../api'

const P = 4

function L(containerId: string, page: number, position: number): LayoutEntry {
  return { containerId, page, position }
}

function byId(entries: LayoutEntry[]): Record<string, { page: number; position: number }> {
  return Object.fromEntries(entries.map((e) => [e.containerId, { page: e.page, position: e.position }]))
}

describe('moveContainerToPage', () => {
  // Smoke: forward move to a full next page — swap with the front slot.
  it('swaps with position 0 when moving forward into a full page', () => {
    const layout = [
      L('A', 0, 0), L('B', 0, 1), L('C', 0, 2), L('D', 0, 3),
      L('E', 1, 0), L('F', 1, 1), L('G', 1, 2), L('H', 1, 3),
    ]
    const next = byId(moveContainerToPage(layout, 'D', 1, P))
    expect(next.D).toEqual({ page: 1, position: 0 })
    expect(next.E).toEqual({ page: 0, position: 3 })
    // Others unchanged
    expect(next.A).toEqual({ page: 0, position: 0 })
    expect(next.F).toEqual({ page: 1, position: 1 })
  })

  // Smoke: backward move to a full prev page — swap with the tail slot.
  it('swaps with position pageSize-1 when moving backward into a full page', () => {
    const layout = [
      L('A', 0, 0), L('B', 0, 1), L('C', 0, 2), L('D', 0, 3),
      L('E', 1, 0), L('F', 1, 1), L('G', 1, 2), L('H', 1, 3),
    ]
    const next = byId(moveContainerToPage(layout, 'E', 0, P))
    expect(next.E).toEqual({ page: 0, position: 3 })
    expect(next.D).toEqual({ page: 1, position: 0 })
  })

  // Smoke: moving into a non-full target compacts the source page.
  it('places at next free slot and compacts the source page when target has room', () => {
    const layout = [L('A', 0, 0), L('B', 0, 1), L('C', 0, 2), L('D', 0, 3), L('E', 1, 0)]
    const next = byId(moveContainerToPage(layout, 'A', 1, P))
    expect(next.A).toEqual({ page: 1, position: 1 })
    expect(next.B).toEqual({ page: 0, position: 0 })
    expect(next.C).toEqual({ page: 0, position: 1 })
    expect(next.D).toEqual({ page: 0, position: 2 })
    expect(next.E).toEqual({ page: 1, position: 0 })
  })

  // Edge (边界值): empty target page.
  it('places at position 0 when target page is empty', () => {
    const layout = [L('A', 0, 0), L('B', 0, 1), L('C', 0, 2), L('D', 0, 3)]
    const next = byId(moveContainerToPage(layout, 'D', 1, P))
    expect(next.D).toEqual({ page: 1, position: 0 })
    expect(next.A).toEqual({ page: 0, position: 0 })
    expect(next.B).toEqual({ page: 0, position: 1 })
    expect(next.C).toEqual({ page: 0, position: 2 })
  })

  // Edge (非法输入): unknown container id is a no-op.
  it('returns layout unchanged when container id is not found', () => {
    const layout = [L('A', 0, 0), L('B', 0, 1)]
    const next = moveContainerToPage(layout, 'ZZ', 1, P)
    expect(next).toEqual(layout)
  })

  // Edge (边界值): moving to the same page is a no-op.
  it('returns layout unchanged when target is the source page', () => {
    const layout = [L('A', 0, 0), L('B', 0, 1)]
    const next = moveContainerToPage(layout, 'A', 0, P)
    expect(next).toEqual(layout)
  })

  // Edge (异常恢复): regression of the reported bug — after the move, layout
  // must reflect the change after a round-trip through setLayout; compactLayout
  // no longer silently reverts it.
  it('regression: move + reverse move leaves both pages back in original order', () => {
    const layout = [
      L('A', 0, 0), L('B', 0, 1), L('C', 0, 2), L('D', 0, 3),
      L('E', 1, 0), L('F', 1, 1), L('G', 1, 2), L('H', 1, 3),
    ]
    const afterForward = moveContainerToPage(layout, 'D', 1, P)
    // D should have actually moved to page 1, not bounced back.
    expect(byId(afterForward).D).toEqual({ page: 1, position: 0 })
    // And reversing it returns everyone home.
    const afterBack = byId(moveContainerToPage(afterForward, 'D', 0, P))
    expect(afterBack.D).toEqual({ page: 0, position: 3 })
    expect(afterBack.E).toEqual({ page: 1, position: 0 })
  })
})
