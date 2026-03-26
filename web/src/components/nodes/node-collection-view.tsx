import { LayoutGrid, Rows3 } from "lucide-react";
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
    <div className="inline-flex rounded-2xl border border-white/10 bg-white/5 p-1">
      <ModeButton active={mode === "grid"} label="网格视图" onClick={() => onChange("grid")}>
        <LayoutGrid className="h-4 w-4" />
      </ModeButton>
      <ModeButton active={mode === "table"} label="表格视图" onClick={() => onChange("table")}>
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
      <div className="rounded-[20px] border border-white/10 bg-white/4 px-4 py-10 text-center text-sm text-[var(--muted-foreground)]">
        {emptyMessage || "暂无节点"}
      </div>
    );
  }

  if (mode === "table") {
    return (
      <Table className="rounded-[20px] border-white/10">
        <TableElement>
          <TableHead>
            <tr>
              <TableHeaderCell>节点名称</TableHeaderCell>
              <TableHeaderCell>状态</TableHeaderCell>
              <TableHeaderCell>IP 地址</TableHeaderCell>
              <TableHeaderCell>地区</TableHeaderCell>
              <TableHeaderCell>最后在线</TableHeaderCell>
            </tr>
          </TableHead>
          <TableBody>
            {items.map((item) => (
              <TableRow
                className={cn(
                  onSelect ? "cursor-pointer" : "",
                  selectedId === item.id ? "bg-violet-500/10" : "",
                )}
                key={item.id}
                onClick={onSelect ? () => onSelect(item) : undefined}
              >
                <TableCell>{item.name}</TableCell>
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
                        "h-2.5 w-2.5 rounded-full",
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
                <TableCell>{item.server || "—"}</TableCell>
                <TableCell>{inferRegion(`${item.name} ${item.source_kind || ""}`)}</TableCell>
                <TableCell>{formatRelativeTime(item.last_checked_at)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </TableElement>
      </Table>
    );
  }

  return (
    <div className="grid gap-4 md:grid-cols-2 2xl:grid-cols-3">
      {items.map((item) => {
        const selectable = Boolean(onSelect);
        const content = (
          <>
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <p className="line-clamp-2 text-xl font-semibold leading-8 text-white">{item.name}</p>
                <p className="mt-1 text-xs uppercase tracking-[0.18em] text-[var(--muted-foreground)]">
                  {item.protocol.toUpperCase()} / {formatNodeSource(item.source_kind).toUpperCase()}
                </p>
              </div>
              <span className={cn("shrink-0 text-lg font-semibold", nodeMetricClass(item))}>
                {nodeMetricValue(item)}
              </span>
            </div>

            <div className="mt-auto flex items-end justify-between gap-3">
              <div className="text-sm text-[var(--muted-foreground)]">
                最近探测 {formatRelativeTime(item.last_checked_at)}
              </div>
              <Badge tone={nodeCollectionTone(item)}>{nodeCollectionStatus(item)}</Badge>
            </div>
          </>
        );

        if (selectable) {
          return (
            <button
              className={cn(
                "grid min-h-[156px] gap-4 rounded-[24px] border px-5 py-4 text-left transition-colors cursor-pointer",
                selectedId === item.id
                  ? "border-violet-400/35 bg-[rgba(17,63,88,0.42)] shadow-[0_0_0_1px_rgba(14,165,233,0.15)_inset]"
                  : "border-white/10 bg-white/5 hover:border-white/20 hover:bg-white/7",
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
              "grid min-h-[156px] gap-4 rounded-[24px] border px-5 py-4 text-left transition-colors",
              selectedId === item.id
                ? "border-violet-400/35 bg-[rgba(17,63,88,0.42)] shadow-[0_0_0_1px_rgba(14,165,233,0.15)_inset]"
                : "border-white/10 bg-white/5 hover:border-white/20 hover:bg-white/7",
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
        "inline-flex items-center gap-2 rounded-xl px-3 py-2 text-sm transition-colors",
        active ? "bg-violet-500/85 text-white" : "text-[var(--muted-foreground)] hover:text-white",
      )}
      onClick={onClick}
      type="button"
    >
      {children}
      {label}
    </button>
  );
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
