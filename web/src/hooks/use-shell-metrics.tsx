import {
  createContext,
  type ReactNode,
  useContext,
  useEffect,
  useState,
} from "react";
import type { GroupMemberView, NodeView, TunnelView } from "@/lib/api";
import { api } from "@/lib/api";
import { countAvailableNodes, countRunningTunnels, isAvailableNode } from "@/lib/format";
import { useSession } from "@/hooks/use-session";

type AvailableNodeLike = Pick<NodeView, "enabled" | "last_status"> | Pick<GroupMemberView, "enabled" | "last_status">;
type ActiveTunnelLike = Pick<TunnelView, "status">;

type ShellMetrics = {
  readyStatus: string;
  groupCount: number;
  activeTunnelCount: number;
  availableNodeCount: number;
  refresh: () => Promise<void>;
  reconcileAvailableNode: (previous: AvailableNodeLike, next: AvailableNodeLike) => void;
  reconcileActiveTunnel: (previous: ActiveTunnelLike, next: ActiveTunnelLike) => void;
};

type ShellMetricsSnapshot = Omit<ShellMetrics, "refresh" | "reconcileAvailableNode" | "reconcileActiveTunnel">;

const ShellMetricsContext = createContext<ShellMetrics | null>(null);

export function ShellMetricsProvider({ children }: { children: ReactNode }) {
  const session = useSession();
  const [metrics, setMetrics] = useState<ShellMetricsSnapshot>({
    readyStatus: "unknown",
    groupCount: 0,
    activeTunnelCount: 0,
    availableNodeCount: 0,
  });

  async function refresh() {
    if (session.status !== "authenticated") {
      setMetrics({
        readyStatus: "unknown",
        groupCount: 0,
        activeTunnelCount: 0,
        availableNodeCount: 0,
      });
      return;
    }

    try {
      const [ready, groups, nodes, tunnels] = await Promise.all([
        api.ready(),
        api.groups.list(session.token),
        api.nodes.list(session.token),
        api.tunnels.list(session.token),
      ]);
      setMetrics({
        readyStatus: ready.status,
        groupCount: groups.length,
        activeTunnelCount: countRunningTunnels(tunnels),
        availableNodeCount: countAvailableNodes(nodes),
      });
    } catch {
      setMetrics((current) => ({
        ...current,
        readyStatus: "degraded",
      }));
    }
  }

  function reconcileAvailableNode(previous: AvailableNodeLike, next: AvailableNodeLike) {
    setMetrics((current) => {
      const previousAvailable = isAvailableNode(previous);
      const nextAvailable = isAvailableNode(next);
      if (previousAvailable === nextAvailable) {
        return current;
      }
      return {
        ...current,
        availableNodeCount: Math.max(0, current.availableNodeCount + (nextAvailable ? 1 : -1)),
      };
    });
  }

  function reconcileActiveTunnel(previous: ActiveTunnelLike, next: ActiveTunnelLike) {
    setMetrics((current) => {
      const previousRunning = previous.status === "running";
      const nextRunning = next.status === "running";
      if (previousRunning === nextRunning) {
        return current;
      }
      return {
        ...current,
        activeTunnelCount: Math.max(0, current.activeTunnelCount + (nextRunning ? 1 : -1)),
      };
    });
  }

  useEffect(() => {
    if (session.status === "authenticated") {
      void refresh();
      return;
    }
    setMetrics({
      readyStatus: "unknown",
      groupCount: 0,
      activeTunnelCount: 0,
      availableNodeCount: 0,
    });
  }, [session.status]);

  return (
    <ShellMetricsContext.Provider
      value={{
        ...metrics,
        refresh,
        reconcileAvailableNode,
        reconcileActiveTunnel,
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
