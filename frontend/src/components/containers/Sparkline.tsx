/** Inline SVG polyline; null values break the line (never drawn as zero). */
export function Sparkline({ values, max, label, className = "text-primary" }: { values: Array<number | null>; max: number; label: string; className?: string }) {
  const W = 240;
  const H = 40;
  const n = Math.max(values.length, 2);
  const segs: string[] = [];
  let cur: string[] = [];
  values.forEach((v, i) => {
    if (v == null) {
      if (cur.length) segs.push(cur.join(" "));
      cur = [];
      return;
    }
    const x = (i / (n - 1)) * W;
    const y = H - 2 - (Math.min(v, max) / (max || 1)) * (H - 4);
    cur.push(`${x.toFixed(1)},${y.toFixed(1)}`);
  });
  if (cur.length) segs.push(cur.join(" "));
  return (
    <svg viewBox={`0 0 ${W} ${H}`} className={`h-10 w-full ${className}`} role="img" aria-label={label} preserveAspectRatio="none">
      <line x1="0" y1={H - 1} x2={W} y2={H - 1} className="stroke-border" strokeWidth="1" />
      {segs.map((pts, i) =>
        pts.includes(" ") ? (
          <polyline key={i} points={pts} fill="none" stroke="currentColor" strokeWidth="1.5" vectorEffect="non-scaling-stroke" />
        ) : (
          <circle key={i} cx={pts.split(",")[0]} cy={pts.split(",")[1]} r="1.5" fill="currentColor" />
        ),
      )}
    </svg>
  );
}
