import type { LayoutEntry } from '../api'

// Move a single container to another page without re-packing unrelated pages.
//
// Policy:
//   - Target page has a free slot → place at the next free position; compact the
//     source page so the vacated slot is filled.
//   - Target page is full → swap with the container in the edge-facing slot
//     (position 0 when moving forward, position PAGE_SIZE-1 when moving back);
//     the displaced container takes the moved container's original slot.
//
// Why not re-use compactLayout here: compactLayout sorts everything into a
// single sequence and re-pages by index, which silently undoes any cross-page
// move when pages aren't exactly full. This local variant preserves user intent.
export function moveContainerToPage(
  layout: LayoutEntry[],
  containerId: string,
  targetPage: number,
  pageSize: number,
): LayoutEntry[] {
  const current = layout.find((e) => e.containerId === containerId)
  if (!current) return layout
  const sourcePage = current.page
  if (sourcePage === targetPage) return layout

  const next = layout.map((e) => ({ ...e }))
  const moved = next.find((e) => e.containerId === containerId)!
  const targetEntries = next.filter(
    (e) => e.page === targetPage && e.containerId !== containerId,
  )

  if (targetEntries.length < pageSize) {
    const taken = new Set(targetEntries.map((e) => e.position))
    let pos = 0
    while (taken.has(pos) && pos < pageSize) pos++
    moved.page = targetPage
    moved.position = pos
    // Compact source page so the gap left behind is filled, but only this page.
    const sourceEntries = next
      .filter((e) => e.page === sourcePage)
      .sort((a, b) => a.position - b.position)
    sourceEntries.forEach((e, i) => {
      e.position = i
    })
  } else {
    // Full: swap with the slot on the incoming side of the target page.
    const swapPos = targetPage > sourcePage ? 0 : pageSize - 1
    const swapWith = targetEntries.find((e) => e.position === swapPos)!
    const savedSourcePos = moved.position
    moved.page = targetPage
    moved.position = swapPos
    swapWith.page = sourcePage
    swapWith.position = savedSourcePos
  }

  return next
}
