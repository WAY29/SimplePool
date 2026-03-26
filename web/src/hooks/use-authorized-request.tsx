import { startTransition } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { APIError } from "@/lib/api";
import { useSession } from "@/hooks/use-session";

export function useAuthorizedRequest() {
  const navigate = useNavigate();
  const location = useLocation();
  const session = useSession();

  async function run<T>(request: (token: string) => Promise<T>) {
    if (session.status !== "authenticated") {
      session.clearSession();
      startTransition(() => {
        navigate("/login", {
          replace: true,
          state: { from: location.pathname },
        });
      });
      throw new APIError(401, "unauthorized", "会话未登录");
    }

    try {
      return await request(session.token);
    } catch (error) {
      if (error instanceof APIError && error.status === 401) {
        session.clearSession();
        toast.error("会话已失效，请重新登录");
        startTransition(() => {
          navigate("/login", {
            replace: true,
            state: { from: location.pathname },
          });
        });
      }
      throw error;
    }
  }

  return { run };
}

