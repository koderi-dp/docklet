export interface GraphSelection {
  exists: boolean
  needs: string[]
  neededBy: string[]
  litNodes: Set<string>
  isLitEdge: (from: string, to: string) => boolean
}

// Direct neighbours of the selected service: what it needs and what needs it.
export function selectGraph(
  selected: string | null,
  nodes: readonly string[],
  edges: readonly { from: string; to: string }[],
): GraphSelection | null {
  if (selected === null || !nodes.includes(selected)) return null
  const needs = new Set<string>()
  const neededBy = new Set<string>()
  for (const edge of edges) {
    if (edge.from === selected && edge.to !== selected) needs.add(edge.to)
    if (edge.to === selected && edge.from !== selected) neededBy.add(edge.from)
  }
  return {
    exists: true,
    needs: [...needs],
    neededBy: [...neededBy],
    litNodes: new Set([selected, ...needs, ...neededBy]),
    isLitEdge: (from, to) => from === selected || to === selected,
  }
}
