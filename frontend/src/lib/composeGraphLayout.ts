// Column layout for the Compose service graph. Pure: no React, no bindings.

export interface LayoutSize {
  nodeWidth: number;
  nodeHeight: number;
  columnGap: number;
  rowGap: number;
}
export interface PlacedNode { name: string; column: number; row: number; x: number; y: number }
export interface PlacedEdge { from: string; to: string; path: string; inCycle: boolean }
export interface GraphLayout {
  nodes: PlacedNode[];
  edges: PlacedEdge[];
  width: number;
  height: number;
  cyclic: string[];
}

const DEFAULT_SIZE: LayoutSize = { nodeWidth: 226, nodeHeight: 92, columnGap: 82, rowGap: 20 };

const byName = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0);
const round = (n: number) => Math.round(n * 10) / 10;

interface Analysis {
  deps: Map<string, string[]>; // acyclic dependencies used for layering
  backEdges: { from: string; to: string }[];
  cyclic: string[];
}

/** DFS in name order; an edge into a node still on the stack closes a cycle. */
function analyse(names: string[], deps: Map<string, string[]>): Analysis {
  const state = new Map<string, 1 | 2>(); // 1 = on stack, 2 = done
  const kept = new Map<string, string[]>(names.map((n) => [n, []]));
  const backEdges: { from: string; to: string }[] = [];
  const cyclic = new Set<string>();
  const path: string[] = [];

  const visit = (root: string) => {
    const frames: { name: string; next: number }[] = [{ name: root, next: 0 }];
    state.set(root, 1);
    path.push(root);
    while (frames.length) {
      const f = frames[frames.length - 1];
      const list = deps.get(f.name)!;
      if (f.next >= list.length) {
        state.set(f.name, 2);
        path.pop();
        frames.pop();
        continue;
      }
      const to = list[f.next++];
      const s = state.get(to);
      if (s === 1) {
        backEdges.push({ from: f.name, to });
        for (let i = path.indexOf(to); i < path.length; i++) cyclic.add(path[i]);
      } else {
        kept.get(f.name)!.push(to);
        if (s === undefined) {
          state.set(to, 1);
          path.push(to);
          frames.push({ name: to, next: 0 });
        }
      }
    }
  };
  for (const n of names) if (!state.has(n)) visit(n);
  return { deps: kept, backEdges, cyclic: [...cyclic].sort(byName) };
}

/** Longest path down to a service without dependencies, on the acyclic graph. */
function depths(names: string[], deps: Map<string, string[]>): Map<string, number> {
  const memo = new Map<string, number>();
  const get = (start: string): number => {
    const stack = [start];
    while (stack.length) {
      const n = stack[stack.length - 1];
      if (memo.has(n)) { stack.pop(); continue; }
      const pending = deps.get(n)!.filter((d) => !memo.has(d));
      if (pending.length) { stack.push(...pending); continue; }
      memo.set(n, deps.get(n)!.reduce((m, d) => Math.max(m, memo.get(d)! + 1), 0));
      stack.pop();
    }
    return memo.get(start)!;
  };
  for (const n of names) get(n);
  return memo;
}

function orderRows(columns: string[][], links: Map<string, Set<string>>): void {
  const rowOf = new Map<string, number>();
  const index = () => columns.forEach((c) => c.forEach((n, i) => rowOf.set(n, i)));
  index();
  const sweep = (c: number, neighbourColumn: number) => {
    const key = (n: string) => {
      const rows = [...(links.get(n) ?? [])]
        .filter((m) => columns[neighbourColumn].includes(m))
        .map((m) => rowOf.get(m)!);
      return rows.length ? rows.reduce((a, b) => a + b, 0) / rows.length : rowOf.get(n)!;
    };
    const keys = new Map(columns[c].map((n) => [n, key(n)]));
    columns[c].sort((a, b) => keys.get(a)! - keys.get(b)! || byName(a, b));
    index();
  };
  for (let pass = 0; pass < 2; pass++) {
    for (let c = 1; c < columns.length; c++) sweep(c, c - 1);
    for (let c = columns.length - 2; c >= 0; c--) sweep(c, c + 1);
  }
}

export function layoutGraph(
  input: { name: string; dependsOn: string[] }[],
  size: Partial<LayoutSize> = {},
): GraphLayout {
  const { nodeWidth, nodeHeight, columnGap, rowGap } = { ...DEFAULT_SIZE, ...size };
  const names = [...new Set(input.map((n) => n.name))].sort(byName);
  const known = new Set(names);
  const deps = new Map<string, string[]>(names.map((n) => [n, []]));
  for (const n of input) {
    const list = deps.get(n.name)!;
    for (const d of [...new Set(n.dependsOn)].sort(byName)) {
      if (known.has(d) && !list.includes(d)) list.push(d);
    }
  }

  const { deps: acyclic, backEdges, cyclic } = analyse(names, deps);
  const depth = depths(names, acyclic);
  const maxDepth = names.reduce((m, n) => Math.max(m, depth.get(n)!), 0);
  const columnOf = new Map(names.map((n) => [n, maxDepth - depth.get(n)!]));

  const columns: string[][] = Array.from({ length: names.length ? maxDepth + 1 : 0 }, () => []);
  for (const n of names) columns[columnOf.get(n)!].push(n);

  const links = new Map<string, Set<string>>(names.map((n) => [n, new Set()]));
  for (const [from, list] of acyclic) {
    for (const to of list) { links.get(from)!.add(to); links.get(to)!.add(from); }
  }
  orderRows(columns, links);

  const tallest = columns.reduce((m, c) => Math.max(m, c.length), 0);
  const pos = new Map<string, PlacedNode>();
  columns.forEach((c, ci) => {
    const offset = ((tallest - c.length) * (nodeHeight + rowGap)) / 2;
    c.forEach((name, row) => {
      pos.set(name, {
        name,
        column: ci,
        row,
        x: ci * (nodeWidth + columnGap),
        y: offset + row * (nodeHeight + rowGap),
      });
    });
  });

  const cycleKeys = new Set(backEdges.map((e) => `${e.from}\0${e.to}`));
  const edges: PlacedEdge[] = [];
  for (const from of names) {
    for (const to of deps.get(from)!) {
      const a = pos.get(from)!;
      const b = pos.get(to)!;
      const x1 = a.x + nodeWidth, y1 = a.y + nodeHeight / 2;
      const x2 = b.x, y2 = b.y + nodeHeight / 2;
      const dx = Math.max(40, (x2 - x1) / 2);
      const path = `M ${round(x1)} ${round(y1)} C ${round(x1 + dx)} ${round(y1)}, ${round(x2 - dx)} ${round(y2)}, ${round(x2)} ${round(y2)}`;
      edges.push({ from, to, path, inCycle: cycleKeys.has(`${from}\0${to}`) });
    }
  }

  const cols = columns.length;
  return {
    nodes: names.map((n) => pos.get(n)!),
    edges,
    width: cols ? cols * nodeWidth + (cols - 1) * columnGap : 0,
    height: tallest ? tallest * nodeHeight + (tallest - 1) * rowGap : 0,
    cyclic,
  };
}
