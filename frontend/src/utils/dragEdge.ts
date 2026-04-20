// Drag-to-page-flip edge detection.
//
// When the user drags a container and lingers near the viewport's left/right edge,
// we switch pages. Edge-zone detection is isolated here as a pure function so it
// can be unit-tested without touching the DOM / React.

export const EDGE_THRESHOLD_PX = 48
export const EDGE_DWELL_MS = 600

export type EdgeZone = 'left' | 'right' | null

export function detectEdgeZone(
  clientX: number,
  viewportWidth: number,
  threshold: number = EDGE_THRESHOLD_PX,
): EdgeZone {
  // Defensive: bail on nonsensical viewport or threshold values so callers never
  // get a false positive during window resize / SSR / headless test contexts.
  if (!Number.isFinite(clientX)) return null
  if (!Number.isFinite(viewportWidth) || viewportWidth <= 0) return null
  if (!Number.isFinite(threshold) || threshold <= 0) return null
  // Overlapping threshold on a tiny viewport: left wins (arbitrary but deterministic).
  if (clientX < threshold) return 'left'
  if (clientX > viewportWidth - threshold) return 'right'
  return null
}
