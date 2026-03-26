import type { GroupMemberView, NodeView, TunnelView } from "@/lib/api";

const dateFormatter = new Intl.DateTimeFormat("zh-CN", {
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
});

const relativeFormatter = new Intl.RelativeTimeFormat("zh-CN", {
  numeric: "auto",
});

export function formatDateTime(value?: string | null) {
  if (!value) {
    return "未记录";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "未记录";
  }

  return dateFormatter.format(date);
}

export function formatRelativeTime(value?: string | null) {
  if (!value) {
    return "未记录";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "未记录";
  }

  const diffSeconds = Math.round((date.getTime() - Date.now()) / 1000);
  const abs = Math.abs(diffSeconds);

  if (abs < 60) {
    return relativeFormatter.format(diffSeconds, "second");
  }
  if (abs < 3600) {
    return relativeFormatter.format(Math.round(diffSeconds / 60), "minute");
  }
  if (abs < 86400) {
    return relativeFormatter.format(Math.round(diffSeconds / 3600), "hour");
  }
  return relativeFormatter.format(Math.round(diffSeconds / 86400), "day");
}

export function formatLatency(latency?: number | null) {
  if (latency === undefined || latency === null) {
    return "未探测";
  }
  return `${latency} ms`;
}

export function formatNodeStatus(status?: string) {
  switch (status) {
    case "healthy":
      return "可用";
    case "unreachable":
      return "不可达";
    default:
      return "未知";
  }
}

export function formatTunnelStatus(status?: string) {
  switch (status) {
    case "running":
      return "运行中";
    case "stopped":
      return "已停止";
    case "starting":
      return "启动中";
    case "degraded":
      return "降级";
    case "error":
      return "异常";
    default:
      return "未知";
  }
}

export function nodeStatusTone(status?: string) {
  switch (status) {
    case "healthy":
      return "success";
    case "unreachable":
      return "danger";
    default:
      return "muted";
  }
}

export function tunnelStatusTone(status?: string) {
  switch (status) {
    case "running":
      return "success";
    case "starting":
      return "warn";
    case "degraded":
      return "warn";
    case "error":
      return "danger";
    case "stopped":
      return "muted";
    default:
      return "muted";
  }
}

export function parseEventDetail(detailJSON: string) {
  try {
    return JSON.parse(detailJSON) as Record<string, unknown>;
  } catch {
    return { raw: detailJSON };
  }
}

export function safeRegex(pattern: string) {
  try {
    return new RegExp(pattern);
  } catch {
    return null;
  }
}

export function inferRegion(value: string) {
  const normalized = value.toLowerCase();
  if (/香港|hong kong|\bhk\b/.test(normalized)) {
    return "HK";
  }
  if (/日本|japan|tokyo|osaka|\bjp\b/.test(normalized)) {
    return "JP";
  }
  if (/新加坡|singapore|\bsg\b/.test(normalized)) {
    return "SG";
  }
  if (/美国|united states|usa|los angeles|new york|\bus\b/.test(normalized)) {
    return "US";
  }
  if (/台湾|taiwan|\btw\b/.test(normalized)) {
    return "TW";
  }
  return "—";
}

export function countHealthyNodes(items: Array<NodeView | GroupMemberView>) {
  return items.filter((item) => item.enabled && item.last_status === "healthy").length;
}

export function countRunningTunnels(items: TunnelView[]) {
  return items.filter((item) => item.status === "running").length;
}
