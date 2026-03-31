import "uplot/dist/uPlot.min.css";
import uPlot from "uplot";
import UPlotReact from "uplot-react";
import { useRef, useState, useEffect, useMemo } from "react";

interface UPlotAreaChartProps {
  timestamps: number[];
  values: (number | null)[];
  color: string;
  yFormatter: (v: number) => string;
  xFormatter: (ts: number) => string; // receives ms
  yDomain?: [number, number];
  height?: number;
}

function cssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}

function colorWithAlpha(color: string, alpha: number): string {
  // Handles hsl(H, S%, L%) → hsla(H, S%, L%, alpha)
  return color.replace(/^hsl\(/, "hsla(").replace(/\)$/, `, ${alpha})`);
}

function toAlignedData(
  timestamps: number[],
  values: (number | null)[],
): uPlot.AlignedData {
  return [
    timestamps.map((t) => t / 1000), // ms → Unix seconds
    values.map((v) => (v === null ? NaN : v)), // null → NaN for spanGaps
  ];
}

export function UPlotAreaChart({
  timestamps,
  values,
  color,
  yFormatter,
  xFormatter,
  yDomain,
  height = 200,
}: UPlotAreaChartProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const uplotRef = useRef<uPlot | null>(null);
  const [width, setWidth] = useState(300);
  const [tooltip, setTooltip] = useState<{
    left: number;
    top: number;
    value: string;
    time: string;
  } | null>(null);

  // Responsive sizing via ResizeObserver
  useEffect(() => {
    if (!containerRef.current) return;
    const ro = new ResizeObserver((entries) => {
      const w = entries[0]?.contentRect.width;
      if (w && w > 0) setWidth(Math.floor(w));
    });
    ro.observe(containerRef.current);
    return () => ro.disconnect();
  }, []);

  const options = useMemo<uPlot.Options>(() => {
    const borderColor = cssVar("--border");
    const mutedColor = cssVar("--muted-foreground");

    const gradientFn = (u: uPlot): CanvasGradient | string => {
      const { top, height } = u.bbox;
      if (!height || !isFinite(top) || !isFinite(height)) {
        return colorWithAlpha(color, 0.1);
      }
      const grad = u.ctx.createLinearGradient(0, top, 0, top + height);
      grad.addColorStop(0, colorWithAlpha(color, 0.3));
      grad.addColorStop(1, colorWithAlpha(color, 0.02));
      return grad;
    };

    return {
      width,
      height,
      cursor: { show: true },
      padding: [8, 0, 0, 0],
      series: [
        {},
        {
          stroke: color,
          fill: gradientFn as uPlot.Series["fill"],
          width: 1.5,
          spanGaps: true,
          points: { show: false },
          value: (_, v) =>
            v == null || isNaN(v) ? "—" : yFormatter(v),
        },
      ],
      axes: [
        {
          values: (_, ticks) => ticks.map((t) => xFormatter(t * 1000)),
          size: 30,
          gap: 5,
          stroke: mutedColor,
          ticks: { stroke: borderColor, width: 1 },
          grid: { stroke: borderColor, width: 1, dash: [3, 3] },
          font: "11px sans-serif",
        },
        {
          values: (_, ticks) => ticks.map((v) => yFormatter(v)),
          size: 72,
          gap: 5,
          stroke: mutedColor,
          ticks: { stroke: borderColor, width: 1 },
          grid: { stroke: borderColor, width: 1, dash: [3, 3] },
          font: "11px sans-serif",
        },
      ],
      scales: {
        x: { time: true },
        y: yDomain ? { range: () => yDomain } : {},
      },
      hooks: {
        setCursor: [
          (u) => {
            const idx = u.cursor.idx;
            if (idx == null) {
              setTooltip(null);
              return;
            }
            const ts = (u.data[0][idx] as number) * 1000;
            const val = u.data[1][idx] as number | null | undefined;
            if (val == null || isNaN(val as number)) {
              setTooltip(null);
              return;
            }
            const left = u.cursor.left ?? 0;
            const top = u.cursor.top ?? 0;
            setTooltip({
              left: left + u.bbox.left / devicePixelRatio,
              top: top + u.bbox.top / devicePixelRatio,
              value: yFormatter(val as number),
              time: new Date(ts).toLocaleString(),
            });
          },
        ],
      },
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [width, height, color, yDomain]);

  // Live update: call setData directly without re-mounting
  const alignedData = useMemo(
    () => toAlignedData(timestamps, values),
    [timestamps, values],
  );

  useEffect(() => {
    if (uplotRef.current) {
      uplotRef.current.setData(alignedData);
    }
  }, [alignedData]);

  return (
    <div ref={containerRef} className="relative w-full">
      <UPlotReact
        options={options}
        data={alignedData}
        onCreate={(u) => {
          uplotRef.current = u;
        }}
        onDelete={() => {
          uplotRef.current = null;
        }}
      />
      {tooltip && (
        <div
          className="pointer-events-none absolute z-10 rounded-md border bg-popover px-2 py-1.5 text-xs text-popover-foreground shadow-md"
          style={{ left: tooltip.left + 12, top: tooltip.top - 20 }}
        >
          <p className="font-medium">{tooltip.value}</p>
          <p className="text-muted-foreground">{tooltip.time}</p>
        </div>
      )}
    </div>
  );
}
