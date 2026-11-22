import { describe, expect, it } from 'vitest'
import { selectGraph } from './graphSelection'

const e = (from: string, to: string) => ({ from, to })

describe('selectGraph', () => {
  it('returns null for no selection or a missing service', () => {
    expect(selectGraph(null, ['a'], [])).toBeNull()
    expect(selectGraph('z', ['a'], [])).toBeNull()
  })

  it('chain: middle service has one need and one dependent', () => {
    const s = selectGraph('b', ['a', 'b', 'c'], [e('a', 'b'), e('b', 'c')])!
    expect(s.needs).toEqual(['c'])
    expect(s.neededBy).toEqual(['a'])
    expect([...s.litNodes].sort()).toEqual(['a', 'b', 'c'])
    expect(s.isLitEdge('a', 'b')).toBe(true)
    expect(s.isLitEdge('b', 'c')).toBe(true)
  })

  it('diamond counts direct connections only', () => {
    const nodes = ['top', 'l', 'r', 'bottom']
    const edges = [e('top', 'l'), e('top', 'r'), e('l', 'bottom'), e('r', 'bottom')]
    const top = selectGraph('top', nodes, edges)!
    expect(top.needs).toHaveLength(2)
    expect(top.neededBy).toHaveLength(0)
    expect(top.litNodes.has('bottom')).toBe(false)
    expect(top.isLitEdge('l', 'bottom')).toBe(false)
    const bottom = selectGraph('bottom', nodes, edges)!
    expect(bottom.neededBy).toHaveLength(2)
    expect(bottom.needs).toHaveLength(0)
  })

  it('independent service lights only itself', () => {
    const s = selectGraph('x', ['a', 'b', 'x'], [e('a', 'b')])!
    expect(s.exists).toBe(true)
    expect(s.needs).toEqual([])
    expect(s.neededBy).toEqual([])
    expect([...s.litNodes]).toEqual(['x'])
    expect(s.isLitEdge('a', 'b')).toBe(false)
  })

  it('cycle: no duplicates, self-edge ignored', () => {
    const s = selectGraph('a', ['a', 'b'], [e('a', 'b'), e('b', 'a'), e('a', 'a')])!
    expect(s.needs).toEqual(['b'])
    expect(s.neededBy).toEqual(['b'])
    expect(s.litNodes.size).toBe(2)
  })
})
