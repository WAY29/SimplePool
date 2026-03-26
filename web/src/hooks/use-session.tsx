import {
  createContext,
  type ReactNode,
  useContext,
  useEffect,
  useState,
} from "react";
import { api, type AdminUser, type SessionView } from "@/lib/api";

const storageKey = "simplepool.session_token";

type SessionState =
  | {
      status: "booting";
      token: null;
      user: null;
      session: null;
    }
  | {
      status: "guest";
      token: null;
      user: null;
      session: null;
    }
  | {
      status: "authenticated";
      token: string;
      user: AdminUser;
      session: SessionView;
    };

type SessionContextValue = SessionState & {
  login: (input: { username: string; password: string }) => Promise<void>;
  logout: () => Promise<void>;
  clearSession: () => void;
};

const SessionContext = createContext<SessionContextValue | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<SessionState>({
    status: "booting",
    token: null,
    user: null,
    session: null,
  });

  useEffect(() => {
    let cancelled = false;
    const token = window.localStorage.getItem(storageKey);
    if (!token) {
      setState({
        status: "guest",
        token: null,
        user: null,
        session: null,
      });
      return () => {
        cancelled = true;
      };
    }

    void api.auth
      .me(token)
      .then((snapshot) => {
        if (cancelled) {
          return;
        }
        setState({
          status: "authenticated",
          token,
          user: snapshot.user,
          session: snapshot.session,
        });
      })
      .catch(() => {
        window.localStorage.removeItem(storageKey);
        if (cancelled) {
          return;
        }
        setState({
          status: "guest",
          token: null,
          user: null,
          session: null,
        });
      });

    return () => {
      cancelled = true;
    };
  }, []);

  async function login(input: { username: string; password: string }) {
    const result = await api.auth.login(input);
    window.localStorage.setItem(storageKey, result.token);
    setState({
      status: "authenticated",
      token: result.token,
      user: result.user,
      session: result.session,
    });
  }

  async function logout() {
    if (state.status === "authenticated") {
      try {
        await api.auth.logout(state.token);
      } catch {
        // ignore logout network errors and clear local state first
      }
    }
    window.localStorage.removeItem(storageKey);
    setState({
      status: "guest",
      token: null,
      user: null,
      session: null,
    });
  }

  function clearSession() {
    window.localStorage.removeItem(storageKey);
    setState({
      status: "guest",
      token: null,
      user: null,
      session: null,
    });
  }

  return (
    <SessionContext.Provider
      value={{
        ...state,
        login,
        logout,
        clearSession,
      }}
    >
      {children}
    </SessionContext.Provider>
  );
}

export function useSession() {
  const value = useContext(SessionContext);
  if (!value) {
    throw new Error("useSession must be used within SessionProvider");
  }
  return value;
}

