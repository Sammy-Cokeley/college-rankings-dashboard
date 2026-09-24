export interface SplitName {
  first: string
  last: string
}

// splitName breaks a raw_source_string name into a first line/last line pair
// for the mobile sticky Wrestler column (two rows instead of one truncated,
// ellipsized line). Splits on the FIRST space only, so a hyphenated first
// name ("Marc-Anthony McGowan") or a middle name/suffix ("Jo Jo Smith" ->
// "Jo" / "Jo Smith") stays a clean two-line split rather than guessing at
// name structure. A single-word name (no space) has an empty last line.
export function splitName(name: string): SplitName {
  const i = name.indexOf(' ')
  if (i === -1) return { first: name, last: '' }
  return { first: name.slice(0, i), last: name.slice(i + 1) }
}
