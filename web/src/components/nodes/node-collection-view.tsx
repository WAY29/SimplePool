import { Clock3, LayoutGrid, LoaderCircle, Rows3 } from "lucide-react";
import type { ReactNode } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableElement,
  TableHead,
  TableHeaderCell,
  TableRow,
} from "@/components/ui/table";
import {
  formatLatency,
  formatNodeStatus,
  formatRegionFlag,
  formatRelativeTime,
  inferRegion,
  nodeStatusTone,
} from "@/lib/format";
import { cn } from "@/lib/utils";
import type { NodeCollectionViewMode } from "@/hooks/use-persisted-view-mode";

export type NodeCollectionItem = {
  id: string;
  name: string;
  protocol: string;
  source_kind?: string;
  server?: string;
  server_port?: number;
  enabled: boolean;
  last_status: string;
  last_latency_ms?: number | null;
  last_checked_at?: string | null;
};

export type NodeProbeState = {
  probing: boolean;
  latencyMS?: number | null;
  success?: boolean;
};

export function NodeViewModeSwitch({
  mode,
  onChange,
}: {
  mode: NodeCollectionViewMode;
  onChange: (mode: NodeCollectionViewMode) => void;
}) {
  return (
    <div className="inline-flex border border-white/10 bg-white/5 p-1 shadow-[0_12px_28px_rgba(2,8,20,0.16)]">
      <ModeButton active={mode === "grid"} label="网格视图" onClick={() => onChange("grid")}>
        <LayoutGrid className="h-4 w-4" />
      </ModeButton>
      <ModeButton active={mode === "table"} label="列表视图" onClick={() => onChange("table")}>
        <Rows3 className="h-4 w-4" />
      </ModeButton>
    </div>
  );
}

export function NodeCollectionView({
  items,
  mode,
  selectedId,
  onSelect,
  emptyMessage,
  probeStates,
  onProbe,
}: {
  items: NodeCollectionItem[];
  mode: NodeCollectionViewMode;
  selectedId?: string | null;
  onSelect?: (item: NodeCollectionItem) => void;
  emptyMessage?: string;
  probeStates?: Record<string, NodeProbeState>;
  onProbe?: (item: NodeCollectionItem) => void;
}) {
  if (items.length === 0) {
    return (
      <div className="border border-white/10 bg-white/4 px-4 py-10 text-center text-sm text-[var(--muted-foreground)]">
        {emptyMessage || "暂无节点"}
      </div>
    );
  }

  if (mode === "table") {
    return (
      <Table className="border-white/10 bg-[rgba(8,13,24,0.72)]">
        <TableElement>
          <TableHead>
            <tr>
              <TableHeaderCell>节点</TableHeaderCell>
              <TableHeaderCell>状态</TableHeaderCell>
              <TableHeaderCell>延迟</TableHeaderCell>
              <TableHeaderCell>地区</TableHeaderCell>
              <TableHeaderCell>最近探测</TableHeaderCell>
              {onProbe ? <TableHeaderCell className="w-[84px] text-right">探测</TableHeaderCell> : null}
            </tr>
          </TableHead>
          <TableBody>
            {items.map((item) => (
              <TableRow
                className={cn(
                  onSelect ? "cursor-pointer" : "",
                  probeStates?.[item.id]?.probing ? "opacity-55" : "",
                  selectedId === item.id ? "bg-[rgba(109,77,243,0.14)]" : "",
                )}
                key={item.id}
                onClick={onSelect ? () => onSelect(item) : undefined}
              >
                <TableCell>
                  <div className="grid gap-1">
                    <span className="font-medium text-white">{item.name}</span>
                    <span className="text-xs uppercase tracking-[0.16em] text-[var(--muted-foreground)]">
                      {item.protocol.toUpperCase()} / {formatNodeSource(item.source_kind).toUpperCase()}
                    </span>
                  </div>
                </TableCell>
                <TableCell>
                  <span
                    className={cn(
                      "inline-flex items-center gap-2",
                      item.enabled ? "" : "text-white/55",
                      item.enabled && item.last_status === "healthy"
                        ? "text-emerald-200"
                        : item.last_status === "unreachable"
                          ? "text-rose-200"
                          : "text-slate-200",
                    )}
                  >
                    <span
                      className={cn(
                        "h-2.5 w-2.5",
                        item.enabled && item.last_status === "healthy"
                          ? "bg-emerald-400"
                          : item.last_status === "unreachable"
                            ? "bg-rose-400"
                            : "bg-slate-400",
                      )}
                    />
                    {nodeCollectionStatus(item)}
                  </span>
                </TableCell>
                <TableCell className={cn("font-medium", nodeMetricClass(item, probeStates?.[item.id]))}>
                  <span className="whitespace-nowrap">{nodeMetricValue(item, probeStates?.[item.id])}</span>
                </TableCell>
                <TableCell>
                  <span className="inline-flex items-center gap-2 whitespace-nowrap">
                    <span aria-hidden="true" className="emoji-flag text-base leading-none">
                      {formatRegionFlag(nodeRegion(item))}
                    </span>
                    <span>{nodeRegion(item)}</span>
                  </span>
                </TableCell>
                <TableCell>{formatRelativeTime(item.last_checked_at)}</TableCell>
                {onProbe ? (
                  <TableCell className="text-right">
                    <ProbeActionButton
                      item={item}
                      onProbe={onProbe}
                      probing={Boolean(probeStates?.[item.id]?.probing)}
                    />
                  </TableCell>
                ) : null}
              </TableRow>
            ))}
          </TableBody>
        </TableElement>
      </Table>
    );
  }

  return (
    <div className="grid gap-3 [grid-template-columns:repeat(auto-fill,minmax(180px,1fr))]">
      {items.map((item) => {
        const selectable = Boolean(onSelect);
        const probeState = probeStates?.[item.id];
        const content = (
          <>
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0 space-y-2">
                <span
                  aria-label={`${nodeRegion(item)} 旗帜`}
                  className="emoji-flag inline-flex h-8 w-8 items-center justify-center rounded-full border border-white/14 bg-white/10 text-base shadow-[0_10px_24px_rgba(2,8,20,0.2)]"
                >
                  {formatRegionFlag(nodeRegion(item))}
                </span>
                <p className="line-clamp-2 text-[0.95rem] font-semibold leading-6 text-white">{item.name}</p>
              </div>
              {onProbe ? (
                <ProbeActionButton item={item} onProbe={onProbe} probing={Boolean(probeState?.probing)} />
              ) : null}
            </div>

            <div className="min-h-0" />

            <div className="mt-auto flex items-end justify-between gap-3">
              <div className="grid gap-1">
                <p className="text-[11px] uppercase tracking-[0.12em] text-[var(--muted-foreground)]">
                  最近探测
                </p>
                <p className="text-xs text-white/78">
                  {formatRelativeTime(item.last_checked_at)}
                </p>
              </div>
              <div className="flex flex-col items-end gap-2">
                <span className={cn("inline-flex items-center gap-1 whitespace-nowrap text-xs font-semibold", nodeMetricClass(item, probeState))}>
                  {nodeMetricValue(item, probeState)}
                </span>
                <Badge className="tracking-[0.08em]" tone={nodeCollectionTone(item)}>
                  {nodeCollectionStatus(item)}
                </Badge>
              </div>
            </div>
          </>
        );

        if (selectable) {
          return (
            <div
              aria-pressed={selectedId === item.id}
              className={cn(
                "grid min-h-[144px] gap-4 rounded-[24px] border px-4 py-3.5 text-left transition-colors",
                probeState?.probing ? "opacity-55" : "",
                selectedId === item.id
                  ? "border-violet-400/35 bg-[linear-gradient(180deg,rgba(42,38,74,0.95),rgba(26,31,58,0.92))] shadow-[0_0_0_1px_rgba(167,139,250,0.16)_inset]"
                  : "border-white/10 bg-[linear-gradient(180deg,rgba(39,41,72,0.9),rgba(33,36,66,0.88))] hover:border-white/18 hover:bg-[linear-gradient(180deg,rgba(45,47,81,0.92),rgba(36,39,72,0.9))]",
              )}
              key={item.id}
              onClick={() => onSelect?.(item)}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  onSelect?.(item);
                }
              }}
              role="button"
              tabIndex={0}
            >
              {content}
            </div>
          );
        }

        return (
          <div
            className={cn(
              "grid min-h-[144px] gap-4 rounded-[24px] border px-4 py-3.5 text-left transition-colors",
              probeState?.probing ? "opacity-55" : "",
              selectedId === item.id
                ? "border-violet-400/35 bg-[linear-gradient(180deg,rgba(42,38,74,0.95),rgba(26,31,58,0.92))] shadow-[0_0_0_1px_rgba(167,139,250,0.16)_inset]"
                : "border-white/10 bg-[linear-gradient(180deg,rgba(39,41,72,0.9),rgba(33,36,66,0.88))] hover:border-white/18 hover:bg-[linear-gradient(180deg,rgba(45,47,81,0.92),rgba(36,39,72,0.9))]",
            )}
            key={item.id}
          >
            {content}
          </div>
        );
      })}
    </div>
  );
}

function ProbeActionButton({
  item,
  probing,
  onProbe,
}: {
  item: NodeCollectionItem;
  probing: boolean;
  onProbe: (item: NodeCollectionItem) => void;
}) {
  return (
    <Button
      aria-label={`测试 ${item.name} 延迟`}
      className="h-8 w-8 rounded-full border-white/10 bg-black/20 p-0 text-white/75 hover:bg-white/10 hover:text-white"
      disabled={probing || !item.enabled}
      onClick={(event) => {
        event.stopPropagation();
        onProbe(item);
      }}
      size="sm"
      type="button"
      variant="ghost"
    >
      {probing ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Clock3 className="h-4 w-4" />}
    </Button>
  );
}

function ModeButton({
  active,
  label,
  children,
  onClick,
}: {
  active: boolean;
  label: string;
  children: ReactNode;
  onClick: () => void;
}) {
  return (
    <button
      className={cn(
        "inline-flex items-center gap-2 border px-3 py-2 text-sm transition-colors",
        active
          ? "border-white/10 bg-white/10 text-white"
          : "border-transparent text-[var(--muted-foreground)] hover:text-white",
      )}
      onClick={onClick}
      type="button"
    >
      {children}
      {label}
    </button>
  );
}

function nodeRegion(item: NodeCollectionItem) {
  const region = inferRegion(`${item.name} ${item.source_kind || ""}`);
  return region === "—" ? "AUTO" : region;
}

export function formatNodeSource(sourceKind?: string) {
  switch (sourceKind) {
    case "subscription":
      return "订阅";
    case "manual":
      return "手动";
    default:
      return sourceKind || "未知";
  }
}

export function nodeMetricValue(item: NodeCollectionItem, probeState?: NodeProbeState) {
  if (!item.enabled) {
    return "禁用";
  }
  if (probeState?.probing) {
    return "探测中";
  }
  if (item.last_status === "unreachable") {
    return "超时";
  }
  if (probeState?.latencyMS !== undefined) {
    return formatLatency(probeState.latencyMS);
  }
  return formatLatency(item.last_latency_ms);
}

export function nodeMetricClass(item: NodeCollectionItem, probeState?: NodeProbeState) {
  if (!item.enabled) {
    return "text-white/55";
  }
  if (probeState?.probing) {
    return "text-slate-300";
  }
  if (probeState?.success === true) {
    return "text-emerald-300";
  }
  if (probeState?.success === false) {
    return "text-rose-300";
  }
  if (item.last_status === "healthy") {
    return "text-emerald-300";
  }
  if (item.last_status === "unreachable") {
    return "text-rose-300";
  }
  return "text-sky-300";
}

export function nodeCollectionStatus(item: NodeCollectionItem) {
  if (!item.enabled) {
    return "已禁用";
  }
  return formatNodeStatus(item.last_status);
}

export function nodeCollectionTone(item: NodeCollectionItem): "success" | "danger" | "muted" {
  if (!item.enabled) {
    return "muted";
  }
  return nodeStatusTone(item.last_status) as "success" | "danger" | "muted";
}
