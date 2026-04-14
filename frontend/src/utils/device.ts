const MOBILE_RE = /iPhone|iPod|Android.*Mobile|webOS|BlackBerry|Opera Mini|IEMobile/i

export function isMobile(): boolean {
  return MOBILE_RE.test(navigator.userAgent)
}
