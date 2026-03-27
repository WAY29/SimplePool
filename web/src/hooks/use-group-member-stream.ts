import { useEffect, useRef } from "react";
import type { GroupMemberView } from "@/lib/api";
import { api } from "@/lib/api";
import { useSession } from "@/hooks/use-session";

type UseGroupMemberStreamOptions = {
  groupIDs: string[];
  onMemberUpdate: (groupID: string, member: GroupMemberView) => void;
};

export function useGroupMemberStream({ groupIDs, onMemberUpdate }: UseGroupMemberStreamOptions) {
  const session = useSession();
  const onMemberUpdateRef = useRef(onMemberUpdate);

  useEffect(() => {
    onMemberUpdateRef.current = onMemberUpdate;
  }, [onMemberUpdate]);

  useEffect(() => {
    if (session.status !== "authenticated") {
      return;
    }

    const normalizedGroupIDs = Array.from(new Set(groupIDs.filter(Boolean))).sort();
    if (normalizedGroupIDs.length === 0) {
      return;
    }

    let disposed = false;
    const connections = new Map<string, { controller: AbortController | null; reconnectTimer: number | null }>();

    const connect = (groupID: string) => {
      if (disposed) {
        return;
      }
      const connection = connections.get(groupID) ?? { controller: null, reconnectTimer: null };
      connection.controller = new AbortController();
      connections.set(groupID, connection);
      void api.groups
        .streamMembers(session.token, groupID, connection.controller.signal, (member) => {
          onMemberUpdateRef.current(groupID, member);
        })
        .catch((error) => {
          if (disposed || (error instanceof Error && error.name === "AbortError")) {
            return;
          }
          connection.reconnectTimer = window.setTimeout(() => {
            connection.reconnectTimer = null;
            connect(groupID);
          }, 1000);
        });
    };

    normalizedGroupIDs.forEach((groupID) => {
      connect(groupID);
    });

    return () => {
      disposed = true;
      connections.forEach(({ controller, reconnectTimer }) => {
        controller?.abort();
        if (reconnectTimer !== null) {
          window.clearTimeout(reconnectTimer);
        }
      });
    };
  }, [groupIDs.join("|"), session.status, session.status === "authenticated" ? session.token : null]);
}
