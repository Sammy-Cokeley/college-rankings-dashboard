import { describe, expect, it } from 'vitest'
import { splitName } from '../utils/names'

describe('splitName', () => {
  it('splits on the first space into first/last', () => {
    expect(splitName('Luke Lilledahl')).toEqual({ first: 'Luke', last: 'Lilledahl' })
  })

  it('keeps a hyphenated first name intact', () => {
    expect(splitName('Marc-Anthony McGowan')).toEqual({ first: 'Marc-Anthony', last: 'McGowan' })
  })

  it('puts everything past the first space on the last line, not just the second word', () => {
    expect(splitName('Jo Jo Smith')).toEqual({ first: 'Jo', last: 'Jo Smith' })
  })

  it('returns an empty last for a single-word name', () => {
    expect(splitName('Prodigy')).toEqual({ first: 'Prodigy', last: '' })
  })
})
