import { Activity, GitBranch, LogOut, Radio, ServerCog } from "lucide-react";
import type { ReactNode } from "react";
import { NavLink, useLocation } from "react-router-dom";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { useSession } from "@/hooks/use-session";
import { useShellMetrics } from "@/hooks/use-shell-metrics";

const navigation = [
  { to: "/workspace", label: "工作区", icon: GitBranch },
  { to: "/nodes", label: "节点", icon: Radio },
  { to: "/subscriptions", label: "订阅", icon: ServerCog },
];

const routeMeta: Record<
  string,
  {
    title: string;
    description: string;
  }
> = {
  "/nodes": {
    title: "节点编排",
    description: "手动节点、导入节点与探测状态统一收口。",
  },
  "/workspace": {
    title: "动态分组管理",
    description: "分组、隧道、成员状态在同一工作区完成联动管理。",
  },
  "/subscriptions": {
    title: "订阅刷新",
    description: "订阅源状态、失败信息与刷新结果集中展示。",
  },
};

function topRoute(pathname: string) {
  if (pathname.startsWith("/workspace")) {
    return "/workspace";
  }
  return pathname in routeMeta ? pathname : "/nodes";
}

type AppShellProps = {
  children: ReactNode;
  hideHeader?: boolean;
};

export function AppShell({ children, hideHeader = false }: AppShellProps) {
  const session = useSession();
  const metrics = useShellMetrics();
  const location = useLocation();
  const currentRoute = topRoute(location.pathname);
  const meta = routeMeta[currentRoute];

  return (
    <div className="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
      <div className="pointer-events-none fixed inset-0 bg-[radial-gradient(circle_at_top_right,rgba(139,92,246,0.16),transparent_34%),radial-gradient(circle_at_bottom_left,rgba(96,165,250,0.12),transparent_30%)]" />
      <div className="relative mx-auto flex min-h-screen max-w-[1560px] flex-col gap-4 px-4 py-4 sm:px-6 lg:flex-row lg:gap-5 lg:py-6">
        <aside className="hidden w-[300px] shrink-0 lg:flex lg:flex-col lg:gap-4">
          <Card className="overflow-hidden p-6">
            <div className="space-y-4">
              <div className="space-y-3">
                <Badge tone="info">Control Plane</Badge>
                <div>
                  <h1 className="font-display text-3xl font-semibold text-white">SimplePool</h1>
                </div>
              </div>
              <div className="grid gap-3">
                {navigation.map((item) => {
                  const Icon = item.icon;
                  return (
                    <NavLink
                      className={({ isActive }) =>
                        cn(
                          "flex cursor-pointer items-center justify-between rounded-2xl border px-4 py-3 text-sm transition-colors",
                          isActive
                            ? "border-violet-400/35 bg-violet-500/14 text-white"
                            : "border-white/10 bg-white/5 text-[var(--muted-foreground)] hover:border-white/20 hover:bg-white/8 hover:text-white",
                        )
                      }
                      key={item.to}
                      to={item.to}
                    >
                      <span className="flex items-center gap-3">
                        <Icon className="h-4 w-4" />
                        {item.label}
                      </span>
                    </NavLink>
                  );
                })}
              </div>
            </div>
          </Card>

          <Card className="p-5">
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-xs uppercase tracking-[0.24em] text-[var(--muted-foreground)]">
                    Runtime
                  </p>
                  <h2 className="mt-2 text-lg font-semibold text-white">全局状态栏</h2>
                </div>
                <Activity className="h-5 w-5 text-sky-300" />
              </div>
              <div className="grid gap-3">
                <MetricLine label="后台状态" value={metrics.readyStatus === "ready" ? "在线" : metrics.readyStatus} />
                <MetricLine label="活跃隧道" value={`${metrics.activeTunnelCount}`} />
                <MetricLine label="健康节点" value={`${metrics.healthyNodeCount}`} />
              </div>
            </div>
          </Card>
        </aside>

        <main className="flex min-h-[calc(100vh-2rem)] flex-1 flex-col gap-4">
          {hideHeader ? null : (
            <Card className="p-5 sm:p-6">
              <div className="flex flex-col gap-4">
                <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                  <div className="space-y-2">
                    <Badge tone={metrics.readyStatus === "ready" ? "success" : "warn"}>
                      {metrics.readyStatus === "ready" ? "Backend Ready" : "Backend Degraded"}
                    </Badge>
                    <div>
                      <h2 className="font-display text-3xl font-semibold text-white">{meta.title}</h2>
                      <p className="mt-2 max-w-2xl text-sm leading-6 text-[var(--muted-foreground)]">
                        {meta.description}
                      </p>
                    </div>
                  </div>
                  <div className="flex flex-wrap items-center gap-3">
                    <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
                      <p className="text-xs uppercase tracking-[0.16em] text-[var(--muted-foreground)]">
                        当前管理员
                      </p>
                      <p className="mt-1 text-sm font-medium text-white">
                        {session.status === "authenticated" ? session.user.username : "未登录"}
                      </p>
                    </div>
                    <Button onClick={() => void session.logout()} variant="secondary">
                      <LogOut className="h-4 w-4" />
                      退出
                    </Button>
                  </div>
                </div>

                <div className="grid gap-3 md:grid-cols-3">
                  <StatusCard label="当前后台" value={metrics.readyStatus === "ready" ? "在线" : "降级"} tone={metrics.readyStatus === "ready" ? "success" : "warn"} />
                  <StatusCard label="活跃隧道" value={`${metrics.activeTunnelCount}`} tone="info" />
                  <StatusCard label="节点可用数" value={`${metrics.healthyNodeCount}`} tone="warn" />
                </div>

                <nav className="flex gap-2 overflow-x-auto lg:hidden">
                  {navigation.map((item) => (
                    <NavLink
                      className={({ isActive }) =>
                        cn(
                          "flex shrink-0 cursor-pointer items-center gap-2 rounded-full border px-4 py-2 text-sm transition-colors",
                          isActive
                            ? "border-violet-400/35 bg-violet-500/14 text-white"
                            : "border-white/10 bg-white/6 text-[var(--muted-foreground)]",
                        )
                      }
                      key={item.to}
                      to={item.to}
                    >
                      <item.icon className="h-4 w-4" />
                      {item.label}
                    </NavLink>
                  ))}
                </nav>
              </div>
            </Card>
          )}

          <section className="flex-1">{children}</section>
        </main>
      </div>
    </div>
  );
}

function StatusCard({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone: "info" | "success" | "warn";
}) {
  const tones = {
    info: "border-violet-400/20 bg-violet-500/12",
    success: "border-emerald-400/20 bg-emerald-400/10",
    warn: "border-amber-400/20 bg-amber-400/10",
  };

  return (
    <div className={cn("rounded-[24px] border px-4 py-4", tones[tone])}>
      <p className="text-xs uppercase tracking-[0.18em] text-[var(--muted-foreground)]">{label}</p>
      <p className="mt-3 text-2xl font-semibold text-white">{value}</p>
    </div>
  );
}

function MetricLine({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between rounded-2xl border border-white/10 bg-white/5 px-4 py-3">
      <span className="text-sm text-[var(--muted-foreground)]">{label}</span>
      <span className="text-sm font-medium text-white">{value}</span>
    </div>
  );
}

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <Card className="flex min-h-[240px] flex-col items-center justify-center p-8 text-center">
      <div className="max-w-md space-y-3">
        <h3 className="text-xl font-semibold text-white">{title}</h3>
        <p className="text-sm leading-6 text-[var(--muted-foreground)]">{description}</p>
        {action ? <div className="pt-2">{action}</div> : null}
      </div>
    </Card>
  );
}

export function PanelTitle({
  eyebrow,
  title,
  description,
}: {
  eyebrow: string;
  title: string;
  description?: string;
}) {
  return (
    <div className="space-y-2">
      <p className="text-xs uppercase tracking-[0.24em] text-[var(--muted-foreground)]">{eyebrow}</p>
      <div className="space-y-2">
        <h3 className="text-2xl font-semibold text-white">{title}</h3>
        {description ? <p className="max-w-3xl text-sm leading-6 text-[var(--muted-foreground)]">{description}</p> : null}
      </div>
    </div>
  );
}
