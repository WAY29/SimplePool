import { GitBranch, LogOut, Radio, ServerCog } from "lucide-react";
import type { ReactNode } from "react";
import { NavLink, useLocation } from "react-router-dom";
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
    chromeTitle: string;
  }
> = {
  "/nodes": {
    chromeTitle: "SimplePool Node Management",
  },
  "/workspace": {
    chromeTitle: "SimplePool Workspace",
  },
  "/subscriptions": {
    chromeTitle: "SimplePool Subscription Management",
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

export function AppShell({ children }: AppShellProps) {
  const session = useSession();
  const metrics = useShellMetrics();
  const location = useLocation();
  const currentRoute = topRoute(location.pathname);
  const meta = routeMeta[currentRoute] ?? routeMeta["/nodes"];

  return (
    <div className="min-h-screen bg-[var(--background)] text-[var(--foreground)]">
      <div className="pointer-events-none fixed inset-0 bg-[radial-gradient(circle_at_top_left,rgba(24,190,141,0.12),transparent_28%),radial-gradient(circle_at_top_right,rgba(82,112,255,0.18),transparent_34%),radial-gradient(circle_at_bottom_left,rgba(60,130,246,0.12),transparent_30%)]" />

      <div className="relative mx-auto max-w-[1600px] px-0 py-0 sm:px-0 lg:px-0 lg:py-0">
        <div className="overflow-hidden border border-white/8 bg-[linear-gradient(180deg,rgba(8,14,26,0.98),rgba(6,11,21,0.98))] shadow-[0_36px_120px_rgba(2,8,20,0.5)]">
          <header className="border-b border-white/10 bg-[linear-gradient(180deg,rgba(23,104,85,0.7),rgba(16,25,41,0.96))]">
            <div className="grid gap-4 px-5 py-3 lg:grid-cols-[220px_minmax(0,1fr)_auto] lg:items-center">
              <div className="flex items-center gap-3">
                <div className="flex h-10 w-10 items-center justify-center bg-[radial-gradient(circle_at_30%_30%,rgba(111,99,255,0.95),rgba(59,52,132,0.9))] shadow-[0_12px_28px_rgba(84,74,188,0.24)]">
                  <span className="h-4 w-4 bg-white/90 shadow-[0_0_16px_rgba(255,255,255,0.45)]" />
                </div>
                <div>
                  <p className="text-[11px] uppercase tracking-[0.28em] text-white/45">Control Plane</p>
                  <h1 className="text-[1.95rem] font-semibold leading-none text-white">SimplePool</h1>
                </div>
              </div>

              <div className="flex flex-col gap-3 lg:items-center">
                <p className="text-center text-sm font-medium tracking-[0.04em] text-white/88">{meta.chromeTitle}</p>
                <div className="flex flex-wrap items-center gap-x-8 gap-y-2 lg:justify-center">
                  <ReadyPill ready={metrics.readyStatus === "ready"} />
                  <HeaderMetric label="Group Count" value={`${metrics.groupCount}`} />
                  <HeaderMetric label="Active Tunnels" value={`${metrics.activeTunnelCount}`} />
                  <HeaderMetric label="Available Nodes" value={`${metrics.healthyNodeCount}`} />
                </div>
              </div>

              <div className="flex flex-wrap items-center justify-start gap-3 lg:justify-end">
                <div className="border border-white/10 bg-white/4 px-3 py-2 text-sm text-white/75">
                  当前管理员:{" "}
                  <span className="font-semibold text-white">
                    {session.status === "authenticated" ? session.user.username : "未登录"}
                  </span>
                </div>
                <Button
                  className="border-white/10 bg-transparent px-3"
                  onClick={() => void session.logout()}
                  size="sm"
                  variant="secondary"
                >
                  <LogOut className="h-4 w-4" />
                  退出
                </Button>
              </div>
            </div>

            <nav className="flex gap-2 overflow-x-auto border-t border-white/10 px-4 py-2 lg:hidden">
              {navigation.map((item) => (
                <NavLink
                  className={({ isActive }) =>
                    cn(
                      "flex shrink-0 items-center gap-2 border px-4 py-2 text-sm transition-colors",
                      isActive
                        ? "border-white/16 bg-white/10 text-white"
                        : "border-white/10 bg-white/5 text-[var(--muted-foreground)] hover:text-white",
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
          </header>

          <div className="lg:grid lg:grid-cols-[210px_minmax(0,1fr)]">
            <aside className="hidden border-r border-white/10 bg-[linear-gradient(180deg,rgba(9,15,26,0.96),rgba(7,12,22,0.98))] lg:flex lg:min-h-[calc(100vh-110px)] lg:flex-col">
              <nav className="flex flex-col gap-1 px-3 py-4">
                {navigation.map((item) => {
                  const Icon = item.icon;
                  return (
                    <NavLink
                      className={({ isActive }) =>
                        cn(
                          "flex items-center gap-3 border px-4 py-3 text-sm transition-colors",
                          isActive
                            ? "border-white/8 bg-white/10 text-white"
                            : "border-transparent text-[var(--muted-foreground)] hover:border-white/8 hover:bg-white/4 hover:text-white",
                        )
                      }
                      key={item.to}
                      to={item.to}
                    >
                      <Icon className="h-4 w-4" />
                      {item.label}
                    </NavLink>
                  );
                })}
              </nav>
            </aside>

            <main className="min-h-[calc(100vh-110px)] bg-[linear-gradient(180deg,rgba(10,15,28,0.96),rgba(7,12,22,0.98))] px-4 py-4 sm:px-5 sm:py-5 lg:px-6 lg:py-6">
              {children}
            </main>
          </div>
        </div>
      </div>
    </div>
  );
}

function ReadyPill({ ready }: { ready: boolean }) {
  return (
    <div
      className={cn(
        "inline-flex items-center gap-2 border px-3 py-1.5 text-sm font-medium",
        ready
          ? "border-emerald-400/25 bg-emerald-400/12 text-emerald-200"
          : "border-amber-400/25 bg-amber-400/12 text-amber-100",
      )}
      >
      <span
        className={cn(
          "h-2.5 w-2.5",
          ready ? "bg-emerald-300 shadow-[0_0_12px_rgba(110,231,183,0.75)]" : "bg-amber-300",
        )}
      />
      {ready ? "BACKEND READY" : "BACKEND DEGRADED"}
    </div>
  );
}

function HeaderMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="inline-flex items-center gap-2 text-sm text-white/78">
      <span className="text-white/45">{label}:</span>
      <span className="font-semibold text-white">{value}</span>
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
