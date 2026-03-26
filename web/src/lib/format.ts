import type { GroupMemberView, NodeView, TunnelView } from "@/lib/api";

type AvailableNodeLike = Pick<NodeView, "enabled" | "last_status"> | Pick<GroupMemberView, "enabled" | "last_status">;

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

const regionMatchers = [
  { code: "HK", pattern: /香港|hong kong|\bhk\b/ },
  { code: "JP", pattern: /日本|japan|tokyo|osaka|\bjp\b/ },
  { code: "SG", pattern: /新加坡|singapore|\bsg\b/ },
  { code: "US", pattern: /美国|united states|usa|los angeles|new york|\bus\b/ },
  { code: "TW", pattern: /台湾|taiwan|\btw\b/ },
  { code: "FR", pattern: /法国|france|paris|\bfr\b/ },
  { code: "CA", pattern: /加拿大|canada|toronto|vancouver|montreal|\bca\b/ },
  { code: "GB", pattern: /英国|united kingdom|england|london|\buk\b|\bgb\b/ },
  { code: "DE", pattern: /德国|germany|frankfurt|berlin|\bde\b/ },
  { code: "KR", pattern: /韩国|korea|seoul|busan|\bkr\b/ },
  { code: "AU", pattern: /澳大利亚|australia|sydney|melbourne|\bau\b/ },
  { code: "NL", pattern: /荷兰|netherlands|amsterdam|\bnl\b/ },
] as const;

export function inferRegion(value: string) {
  const normalized = value.toLowerCase();
  for (const matcher of regionMatchers) {
    if (matcher.pattern.test(normalized)) {
      return matcher.code;
    }
  }
  return "—";
}

export function formatRegionFlag(region?: string) {
  switch (region) {
    case "HK":
      return "🇭🇰";
    case "JP":
      return "🇯🇵";
    case "SG":
      return "🇸🇬";
    case "US":
      return "🇺🇸";
    case "TW":
      return "🇹🇼";
    case "FR":
      return "🇫🇷";
    case "CA":
      return "🇨🇦";
    case "GB":
      return "🇬🇧";
    case "DE":
      return "🇩🇪";
    case "KR":
      return "🇰🇷";
    case "AU":
      return "🇦🇺";
    case "NL":
      return "🇳🇱";
    case "AUTO":
    case "—":
    default:
      return "🌐";
  }
}

export function countHealthyNodes(items: Array<NodeView | GroupMemberView>) {
  return items.filter((item) => item.enabled && item.last_status === "healthy").length;
}

export function isAvailableNode(item: AvailableNodeLike) {
  return item.enabled && item.last_status !== "unreachable";
}

export function countAvailableNodes(items: AvailableNodeLike[]) {
  return items.filter((item) => isAvailableNode(item)).length;
}

export function countRunningTunnels(items: TunnelView[]) {
  return items.filter((item) => item.status === "running").length;
}
