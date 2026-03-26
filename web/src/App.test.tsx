import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { BrowserRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";

function jsonResponse(status: number, payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: {
      "Content-Type": "application/json",
    },
  });
}

function renderApp() {
  return render(
    <BrowserRouter>
      <App />
    </BrowserRouter>,
  );
}

function installAuthenticatedFetchMock() {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    if (url.endsWith("/api/auth/me")) {
      return jsonResponse(200, {
        user: {
          id: "user-1",
          username: "admin",
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
        session: {
          id: "session-1",
          expires_at: "2026-04-02T10:00:00Z",
          created_at: "2026-03-26T10:00:00Z",
          last_seen_at: "2026-03-26T10:05:00Z",
        },
      });
    }
    if (url.endsWith("/readyz")) {
      return jsonResponse(200, { status: "ready" });
    }
    if (url.endsWith("/api/nodes")) {
      return jsonResponse(200, [
        {
          id: "node-1",
          name: "香港-A1",
          source_kind: "manual",
          protocol: "trojan",
          server: "192.168.1.101",
          server_port: 443,
          transport_json: "{}",
          tls_json: "{}",
          raw_payload_json: "{}",
          enabled: true,
          last_status: "healthy",
          last_checked_at: "2026-03-26T10:04:00Z",
          has_credential: true,
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
      ]);
    }
    if (url.endsWith("/api/groups")) {
      return jsonResponse(200, [
        {
          id: "group-1",
          name: "测试分组",
          filter_regex: "HK",
          description: "香港节点",
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
      ]);
    }
    if (url.endsWith("/api/groups/group-1/members")) {
      return jsonResponse(200, [
        {
          id: "node-1",
          name: "香港-A1",
          source_kind: "manual",
          protocol: "trojan",
          server: "192.168.1.101",
          server_port: 443,
          enabled: true,
          last_latency_ms: 42,
          last_status: "healthy",
          last_checked_at: "2026-03-26T10:04:00Z",
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
      ]);
    }
    if (url.endsWith("/api/tunnels")) {
      return jsonResponse(200, [
        {
          id: "tunnel-1",
          name: "代理-A",
          group_id: "group-1",
          listen_host: "127.0.0.1",
          listen_port: 18080,
          status: "running",
          current_node_id: "node-1",
          controller_port: 19090,
          runtime_dir: "/tmp/runtime-1",
          last_refresh_at: "2026-03-26T10:05:00Z",
          last_refresh_error: "",
          has_auth: true,
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
      ]);
    }
    if (url.endsWith("/api/subscriptions")) {
      return jsonResponse(200, [
        {
          id: "subscription-1",
          name: "赔钱",
          fetch_fingerprint: "589dc2f9b74338fa9b7e",
          enabled: true,
          last_refresh_at: null,
          last_error: "",
          has_url: true,
          created_at: "2026-03-26T09:07:00Z",
          updated_at: "2026-03-26T09:07:00Z",
        },
      ]);
    }
    if (url.includes("/api/tunnels/") && url.includes("/events")) {
      return jsonResponse(200, []);
    }
    throw new Error(`unexpected request: ${url}`);
  });

  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

describe("App", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    window.localStorage.clear();
    window.history.pushState({}, "", "/");
  });

  afterEach(() => {
    cleanup();
  });

  it("未登录访问受保护页面时跳转到登录页", async () => {
    window.history.pushState({}, "", "/nodes");
    vi.stubGlobal("fetch", vi.fn(async () => jsonResponse(200, { status: "ready" })));

    renderApp();

    expect(await screen.findByRole("heading", { name: "登录 SimplePool" })).toBeInTheDocument();
  });

  it("存在会话令牌时恢复会话并渲染控制台壳层", async () => {
    window.history.pushState({}, "", "/");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    const fetchMock = installAuthenticatedFetchMock();

    renderApp();

    expect(await screen.findByRole("heading", { name: "动态分组管理" })).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "工作区" }).length).toBeGreaterThan(0);
    expect(screen.getAllByRole("link", { name: "节点" }).length).toBeGreaterThan(0);
    expect(screen.getByText("SimplePool")).toBeInTheDocument();
    expect(screen.getByText("动态分组")).toBeInTheDocument();
    expect(screen.getByText("隧道列表")).toBeInTheDocument();
    expect(screen.getByText("组成员节点")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "网格视图" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "表格视图" })).toBeInTheDocument();
    expect((await screen.findAllByText(/测试分组/)).length).toBeGreaterThan(0);
    expect(await screen.findByText("代理-A")).toBeInTheDocument();
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalled();
    });
  });

  it("旧分组路由失效后显示不存在页面", async () => {
    window.history.pushState({}, "", "/groups");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    installAuthenticatedFetchMock();

    renderApp();

    expect(await screen.findByRole("heading", { name: "页面不存在" })).toBeInTheDocument();
  });

  it("节点页与工作区共用统一导航骨架", async () => {
    window.history.pushState({}, "", "/nodes");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    installAuthenticatedFetchMock();

    renderApp();

    expect(await screen.findByRole("heading", { name: "节点编排" })).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "工作区" }).length).toBeGreaterThan(0);
    expect(screen.getAllByRole("link", { name: "订阅" }).length).toBeGreaterThan(0);
    expect(screen.getByText("SimplePool")).toBeInTheDocument();
    expect(screen.queryByText("Transport JSON")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "查看详情" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "网格视图" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "表格视图" })).toBeInTheDocument();
  });

  it("订阅页隐藏刷新指纹并重排操作按钮", async () => {
    window.history.pushState({}, "", "/subscriptions");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    installAuthenticatedFetchMock();

    renderApp();

    expect(await screen.findByRole("heading", { name: "订阅源列表" })).toBeInTheDocument();
    expect(screen.queryByText("刷新指纹")).not.toBeInTheDocument();
    expect(screen.queryByText("URL 存储")).not.toBeInTheDocument();
    expect(screen.queryByText("字段")).not.toBeInTheDocument();
    expect(screen.queryByText("最近状态正常")).not.toBeInTheDocument();
    expect(screen.queryByText("最近一次刷新结果")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "立即刷新" })).toBeInTheDocument();
    expect(screen.getByText("创建时间")).toBeInTheDocument();
    expect(screen.getByText("更新时间")).toBeInTheDocument();
  });
});
