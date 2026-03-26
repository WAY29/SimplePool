import {
  createContext,
  type ReactNode,
  useContext,
  useEffect,
  useState,
} from "react";
import { api } from "@/lib/api";
import { countHealthyNodes, countRunningTunnels } from "@/lib/format";
import { useSession } from "@/hooks/use-session";

type ShellMetrics = {
  readyStatus: string;
  activeTunnelCount: number;
  healthyNodeCount: number;
  refresh: () => Promise<void>;
};

const ShellMetricsContext = createContext<ShellMetrics | null>(null);

export function ShellMetricsProvider({ children }: { children: ReactNode }) {
  const session = useSession();
  const [metrics, setMetrics] = useState<Omit<ShellMetrics, "refresh">>({
    readyStatus: "unknown",
    activeTunnelCount: 0,
    healthyNodeCount: 0,
  });

  async function refresh() {
    if (session.status !== "authenticated") {
      setMetrics({
        readyStatus: "unknown",
        activeTunnelCount: 0,
        healthyNodeCount: 0,
      });
      return;
    }

    try {
      const [ready, nodes, tunnels] = await Promise.all([
        api.ready(),
        api.nodes.list(session.token),
        api.tunnels.list(session.token),
      ]);
      setMetrics({
        readyStatus: ready.status,
        activeTunnelCount: countRunningTunnels(tunnels),
        healthyNodeCount: countHealthyNodes(nodes),
      });
    } catch {
      setMetrics((current) => ({
        ...current,
        readyStatus: "degraded",
      }));
    }
  }

  useEffect(() => {
    if (session.status === "authenticated") {
      void refresh();
      return;
    }
    setMetrics({
      readyStatus: "unknown",
      activeTunnelCount: 0,
      healthyNodeCount: 0,
    });
  }, [session.status]);

  return (
    <ShellMetricsContext.Provider
      value={{
        ...metrics,
        refresh,
      }}
    >
      {children}
    </ShellMetricsContext.Provider>
  );
}

export function useShellMetrics() {
  const value = useContext(ShellMetricsContext);
  if (!value) {
    throw new Error("useShellMetrics must be used within ShellMetricsProvider");
  }
  return value;
}
