/** "Off", "90 s", "2 min", "2 min 30 s": one wording for the menu and the dialog. */
export function formatRest(seconds: number): string {
  if (seconds <= 0) return 'Off'
  if (seconds < 60 || (seconds < 120 && seconds % 60 !== 0)) return `${seconds} s`
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return s === 0 ? `${m} min` : `${m} min ${s} s`
}
