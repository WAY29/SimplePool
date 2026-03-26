import { LoaderCircle } from "lucide-react";
import { startTransition } from "react";
import { Navigate, Outlet, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { Toaster } from "sonner";
import { LoginPage } from "@/pages/login-page";
import { NodesPage } from "@/pages/nodes-page";
import { NotFoundPage } from "@/pages/not-found-page";
import { SubscriptionsPage } from "@/pages/subscriptions-page";
import { WorkspacePage } from "@/pages/workspace-page";
import { ShellMetricsProvider } from "@/hooks/use-shell-metrics";
import { SessionProvider, useSession } from "@/hooks/use-session";

function AppRoutes() {
  const session = useSession();

  if (session.status === "booting") {
    return (
      <div className="flex min-h-screen items-center justify-center bg-[var(--background)] text-[var(--muted-foreground)]">
        <LoaderCircle className="mr-3 h-5 w-5 animate-spin" />
        正在恢复会话...
      </div>
    );
  }

  return (
    <Routes>
      <Route element={<RequireGuest />} path="/login">
        <Route index element={<LoginPage />} />
      </Route>
      <Route element={<RequireAuth />} path="/">
        <Route element={<Navigate replace to="/workspace" />} index />
        <Route element={<WorkspacePage />} path="workspace" />
        <Route element={<NodesPage />} path="nodes" />
        <Route element={<SubscriptionsPage />} path="subscriptions" />
        <Route element={<NotFoundPage />} path="*" />
      </Route>
    </Routes>
  );
}

function RequireAuth() {
  const session = useSession();
  const location = useLocation();

  if (session.status !== "authenticated") {
    return <Navigate replace state={{ from: location.pathname }} to="/login" />;
  }

  return (
    <ShellMetricsProvider>
      <Outlet />
    </ShellMetricsProvider>
  );
}

function RequireGuest() {
  const session = useSession();
  if (session.status === "authenticated") {
    return <Navigate replace to="/workspace" />;
  }
  return <Outlet />;
}

export default function App() {
  return (
    <SessionProvider>
      <AppRoutes />
      <Toaster
        position="top-right"
        richColors
        toastOptions={{
          classNames: {
            toast: "sonner-toast",
            title: "sonner-title",
            description: "sonner-description",
          },
        }}
      />
    </SessionProvider>
  );
}

export function useSafeNavigate() {
  const navigate = useNavigate();
  return (path: string, replace = false) => {
    startTransition(() => {
      navigate(path, { replace });
    });
  };
}
