import { ArrowUpRight, LayoutGrid, Rows3 } from "lucide-react";
import type { ReactNode } from "react";
import { Badge } from "@/components/ui/badge";
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
}: {
  items: NodeCollectionItem[];
  mode: NodeCollectionViewMode;
  selectedId?: string | null;
  onSelect?: (item: NodeCollectionItem) => void;
  emptyMessage?: string;
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
            </tr>
          </TableHead>
          <TableBody>
            {items.map((item) => (
              <TableRow
                className={cn(
                  onSelect ? "cursor-pointer" : "",
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
                <TableCell className={cn("font-medium", nodeMetricClass(item))}>{nodeMetricValue(item)}</TableCell>
                <TableCell>{nodeRegion(item)}</TableCell>
                <TableCell>{formatRelativeTime(item.last_checked_at)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </TableElement>
      </Table>
    );
  }

  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
      {items.map((item) => {
        const selectable = Boolean(onSelect);
        const content = (
          <>
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0 space-y-3">
                <span className="inline-flex h-7 min-w-10 items-center justify-center border border-white/18 bg-white/90 px-2 text-[11px] font-semibold tracking-[0.12em] text-slate-900">
                  {nodeRegion(item)}
                </span>
                <p className="line-clamp-2 text-[1.02rem] font-semibold leading-7 text-white">{item.name}</p>
              </div>
            </div>

            <div className="grid gap-1">
              <p className="text-sm text-white/88">
                {item.protocol.toUpperCase()} / {formatNodeSource(item.source_kind)}
              </p>
              <p className="text-sm text-[var(--muted-foreground)]">
                最近探测 {formatRelativeTime(item.last_checked_at)}
              </p>
            </div>

            <div className="mt-auto flex items-end justify-between gap-3">
              <span className={cn("inline-flex items-center gap-1 text-xl font-semibold", nodeMetricClass(item))}>
                {nodeMetricValue(item)}
                {showHealthyTrend(item) ? <ArrowUpRight className="h-4 w-4" /> : null}
              </span>
              <Badge className="tracking-[0.08em]" tone={nodeCollectionTone(item)}>
                {nodeCollectionStatus(item)}
              </Badge>
            </div>
          </>
        );

        if (selectable) {
          return (
            <button
              className={cn(
                "grid min-h-[168px] gap-5 border px-5 py-4 text-left transition-colors",
                selectedId === item.id
                  ? "border-violet-400/35 bg-[linear-gradient(180deg,rgba(42,38,74,0.95),rgba(26,31,58,0.92))] shadow-[0_0_0_1px_rgba(167,139,250,0.16)_inset]"
                  : "border-white/10 bg-[linear-gradient(180deg,rgba(39,41,72,0.9),rgba(33,36,66,0.88))] hover:border-white/18 hover:bg-[linear-gradient(180deg,rgba(45,47,81,0.92),rgba(36,39,72,0.9))]",
              )}
              key={item.id}
              onClick={() => onSelect?.(item)}
              type="button"
            >
              {content}
            </button>
          );
        }

        return (
          <div
            className={cn(
              "grid min-h-[168px] gap-5 border px-5 py-4 text-left transition-colors",
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

function showHealthyTrend(item: NodeCollectionItem) {
  return item.enabled && item.last_status === "healthy" && item.last_latency_ms !== undefined && item.last_latency_ms !== null;
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

export function nodeMetricValue(item: NodeCollectionItem) {
  if (!item.enabled) {
    return "禁用";
  }
  if (item.last_status === "unreachable") {
    return "超时";
  }
  return formatLatency(item.last_latency_ms);
}

export function nodeMetricClass(item: NodeCollectionItem) {
  if (!item.enabled) {
    return "text-white/55";
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
