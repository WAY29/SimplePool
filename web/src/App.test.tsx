import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

type FetchMockOptions = {
  nodes?: Array<Record<string, unknown>>;
  subscriptions?: Array<Record<string, unknown>>;
  groups?: Array<Record<string, unknown>>;
  groupMembers?: Record<string, Array<Record<string, unknown>>>;
  tunnels?: Array<Record<string, unknown>>;
  probeDelays?: Record<string, number>;
  probeResults?: Record<string, Record<string, unknown>>;
};

function defaultNodes() {
  return [
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
  ];
}

function installAuthenticatedFetchMock(options: FetchMockOptions = {}) {
  let nodes = structuredClone(options.nodes ?? defaultNodes());
  const subscriptions = options.subscriptions ?? [
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
  ];
  const groups = options.groups ?? [
    {
      id: "group-1",
      name: "测试分组",
      filter_regex: "HK",
      description: "香港节点",
      created_at: "2026-03-26T10:00:00Z",
      updated_at: "2026-03-26T10:00:00Z",
    },
  ];
  const groupMembers = options.groupMembers ?? {
    "group-1": [
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
    ],
  };
  const tunnels = options.tunnels ?? [
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
  ];
  const probeDelays = options.probeDelays ?? {};
  const probeResults = options.probeResults ?? {};

  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
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
    if (url.endsWith("/api/nodes") && (init?.method === undefined || init.method === "GET")) {
      return jsonResponse(200, nodes);
    }
    const singleProbeMatch = url.match(/\/api\/nodes\/([^/]+)\/probe$/);
    if (singleProbeMatch && init?.method === "POST") {
      const nodeID = singleProbeMatch[1];
      const delay = probeDelays[nodeID] ?? 0;
      if (delay > 0) {
        await new Promise((resolve) => setTimeout(resolve, delay));
      }
      const checkedAt = "2026-03-26T10:06:00Z";
      const result = {
        success: true,
        latency_ms: 66,
        test_url: "https://cloudflare.com/cdn-cgi/trace",
        cached: false,
        checked_at: checkedAt,
        ...probeResults[nodeID],
      };
      nodes = nodes.map((item) =>
        item.id === nodeID
          ? {
              ...item,
              last_status: result.success ? "healthy" : "unreachable",
              last_latency_ms: result.success ? result.latency_ms : null,
              last_checked_at: checkedAt,
            }
          : item,
      );
      return jsonResponse(200, result);
    }
    if (url.endsWith("/api/groups")) {
      return jsonResponse(200, groups);
    }
    const groupMembersMatch = url.match(/\/api\/groups\/([^/]+)\/members$/);
    if (groupMembersMatch) {
      return jsonResponse(200, groupMembers[groupMembersMatch[1]] ?? []);
    }
    if (url.endsWith("/api/tunnels")) {
      return jsonResponse(200, tunnels);
    }
    if (url.endsWith("/api/subscriptions")) {
      return jsonResponse(200, subscriptions);
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

    expect(await screen.findByRole("heading", { name: "节点组" })).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "节点组" }).length).toBeGreaterThan(0);
    expect(screen.getAllByRole("link", { name: "节点" }).length).toBeGreaterThan(0);
    expect(screen.getByText("SimplePool")).toBeInTheDocument();
    expect(screen.getByText("动态分组")).toBeInTheDocument();
    expect(screen.getByText("动态隧道")).toBeInTheDocument();
    expect(screen.getByText("组节点")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("搜索分组名称")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "网格视图" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "列表视图" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "刷新数据" })).not.toBeInTheDocument();
    expect(screen.queryByText(/HTTP 代理已启用|HTTP 代理未启用/)).not.toBeInTheDocument();
    expect(screen.queryByText("活跃成员")).not.toBeInTheDocument();
    expect(screen.queryByText("活动/降级隧道")).not.toBeInTheDocument();
    expect(screen.queryByText("可用运行时")).not.toBeInTheDocument();
    expect((await screen.findAllByText(/测试分组/)).length).toBeGreaterThan(0);
    expect(await screen.findByText("代理-A")).toBeInTheDocument();
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalled();
    });
  });

  it("节点组页支持按名称搜索分组", async () => {
    window.history.pushState({}, "", "/");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    installAuthenticatedFetchMock({
      groups: [
        {
          id: "group-1",
          name: "测试分组",
          filter_regex: "HK",
          description: "香港节点",
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
        {
          id: "group-2",
          name: "日本组",
          filter_regex: "JP",
          description: "日本节点",
          created_at: "2026-03-26T10:01:00Z",
          updated_at: "2026-03-26T10:01:00Z",
        },
      ],
      groupMembers: {
        "group-1": [
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
        ],
        "group-2": [
          {
            id: "node-2",
            name: "日本-B2",
            source_kind: "manual",
            protocol: "vmess",
            server: "192.168.1.102",
            server_port: 443,
            enabled: true,
            last_latency_ms: 58,
            last_status: "healthy",
            last_checked_at: "2026-03-26T10:05:00Z",
            created_at: "2026-03-26T10:01:00Z",
            updated_at: "2026-03-26T10:01:00Z",
          },
        ],
      },
      tunnels: [],
    });

    renderApp();

    const user = userEvent.setup();
    const input = await screen.findByPlaceholderText("搜索分组名称");
    await user.type(input, "日本");

    await waitFor(() => {
      expect(screen.queryByRole("button", { name: /测试分组/ })).not.toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: /日本组/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "新建分组" })).toBeInTheDocument();
  });

  it("节点组页在没有隧道时也不显示 HTTP 代理摘要", async () => {
    window.history.pushState({}, "", "/");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    installAuthenticatedFetchMock({ tunnels: [] });

    renderApp();

    expect(await screen.findByRole("heading", { name: "节点组" })).toBeInTheDocument();
    expect(screen.queryByText("HTTP 代理未启用。当前没有运行中的隧道。")).not.toBeInTheDocument();
  });

  it("旧分组路由失效后显示不存在页面", async () => {
    window.history.pushState({}, "", "/groups");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    installAuthenticatedFetchMock();

    renderApp();

    expect(await screen.findByRole("heading", { name: "页面不存在" })).toBeInTheDocument();
  });

  it("节点页与节点组共用统一导航骨架", async () => {
    window.history.pushState({}, "", "/nodes");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    installAuthenticatedFetchMock();

    renderApp();

    expect(await screen.findByRole("heading", { name: "节点池" })).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "节点组" }).length).toBeGreaterThan(0);
    expect(screen.getAllByRole("link", { name: "订阅" }).length).toBeGreaterThan(0);
    expect(screen.getByText("SimplePool")).toBeInTheDocument();
    expect(screen.queryByText("Transport JSON")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "查看详情" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "新建节点" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "导入节点" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "批量探测" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "网格视图" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "列表视图" })).toBeInTheDocument();
  });

  it("旧工作区路由现在显示不存在页面", async () => {
    window.history.pushState({}, "", "/workspace");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    installAuthenticatedFetchMock();

    renderApp();

    expect(await screen.findByRole("heading", { name: "页面不存在" })).toBeInTheDocument();
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
    expect(await screen.findByRole("button", { name: "立即刷新" })).toBeInTheDocument();
    expect(screen.getByText("创建时间")).toBeInTheDocument();
    expect(screen.getByText("更新时间")).toBeInTheDocument();
  });

  it("订阅详情展示订阅节点并将未探测节点计入 Available Nodes", async () => {
    window.history.pushState({}, "", "/subscriptions");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    installAuthenticatedFetchMock({
      nodes: [
        {
          id: "node-1",
          name: "香港-A1",
          source_kind: "subscription",
          subscription_source_id: "subscription-1",
          protocol: "trojan",
          server: "192.168.1.101",
          server_port: 443,
          transport_json: "{}",
          tls_json: "{}",
          raw_payload_json: "{}",
          enabled: true,
          last_latency_ms: 42,
          last_status: "healthy",
          last_checked_at: "2026-03-26T10:04:00Z",
          has_credential: true,
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
        {
          id: "node-2",
          name: "日本-B2",
          source_kind: "subscription",
          subscription_source_id: "subscription-1",
          protocol: "vmess",
          server: "192.168.1.102",
          server_port: 443,
          transport_json: "{}",
          tls_json: "{}",
          raw_payload_json: "{}",
          enabled: true,
          last_latency_ms: null,
          last_status: "unknown",
          last_checked_at: null,
          has_credential: true,
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
        {
          id: "node-3",
          name: "美国-C3",
          source_kind: "subscription",
          subscription_source_id: "subscription-1",
          protocol: "trojan",
          server: "192.168.1.103",
          server_port: 443,
          transport_json: "{}",
          tls_json: "{}",
          raw_payload_json: "{}",
          enabled: true,
          last_latency_ms: null,
          last_status: "unreachable",
          last_checked_at: "2026-03-26T10:01:00Z",
          has_credential: true,
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
        {
          id: "node-4",
          name: "停用-D4",
          source_kind: "manual",
          protocol: "trojan",
          server: "192.168.1.104",
          server_port: 443,
          transport_json: "{}",
          tls_json: "{}",
          raw_payload_json: "{}",
          enabled: false,
          last_latency_ms: null,
          last_status: "unknown",
          last_checked_at: null,
          has_credential: true,
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
      ],
    });

    renderApp();

    expect(await screen.findByRole("heading", { name: "订阅源列表" })).toBeInTheDocument();
    expect(await screen.findByText("订阅节点")).toBeInTheDocument();
    expect(screen.getByText("香港-A1")).toBeInTheDocument();
    expect(screen.getByText("日本-B2")).toBeInTheDocument();
    expect(screen.getByText("美国-C3")).toBeInTheDocument();
    expect(screen.queryByText("停用-D4")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "批量探测订阅节点" })).toBeInTheDocument();
    expect(screen.getByText("Available Nodes:").parentElement).toHaveTextContent("2");
  });

  it("订阅详情批量探测会实时回填延迟并跳过禁用节点", async () => {
    window.history.pushState({}, "", "/subscriptions");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    const fetchMock = installAuthenticatedFetchMock({
      nodes: [
        {
          id: "node-1",
          name: "香港-A1",
          source_kind: "subscription",
          subscription_source_id: "subscription-1",
          protocol: "trojan",
          server: "192.168.1.101",
          server_port: 443,
          transport_json: "{}",
          tls_json: "{}",
          raw_payload_json: "{}",
          enabled: true,
          last_latency_ms: null,
          last_status: "unknown",
          last_checked_at: null,
          has_credential: true,
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
        {
          id: "node-2",
          name: "日本-B2",
          source_kind: "subscription",
          subscription_source_id: "subscription-1",
          protocol: "vmess",
          server: "192.168.1.102",
          server_port: 443,
          transport_json: "{}",
          tls_json: "{}",
          raw_payload_json: "{}",
          enabled: true,
          last_latency_ms: null,
          last_status: "unknown",
          last_checked_at: null,
          has_credential: true,
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
        {
          id: "node-3",
          name: "停用-C3",
          source_kind: "subscription",
          subscription_source_id: "subscription-1",
          protocol: "trojan",
          server: "192.168.1.103",
          server_port: 443,
          transport_json: "{}",
          tls_json: "{}",
          raw_payload_json: "{}",
          enabled: false,
          last_latency_ms: null,
          last_status: "unknown",
          last_checked_at: null,
          has_credential: true,
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
      ],
      probeDelays: {
        "node-1": 60,
        "node-2": 10,
      },
      probeResults: {
        "node-1": {
          success: true,
          latency_ms: 45,
        },
        "node-2": {
          success: true,
          latency_ms: 88,
        },
      },
    });

    renderApp();

    const user = userEvent.setup();
    expect(await screen.findByText("订阅节点")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "批量探测订阅节点" }));

    expect(screen.getAllByText("探测中")).toHaveLength(2);

    await waitFor(() => {
      expect(screen.getByText("88 ms")).toBeInTheDocument();
    });
    expect(screen.getByText("探测中")).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText("45 ms")).toBeInTheDocument();
    });
    await waitFor(() => {
      expect(screen.queryByText("探测中")).not.toBeInTheDocument();
    });

    const probeURLs = fetchMock.mock.calls
      .map(([input]) => String(input))
      .filter((url) => url.includes("/api/nodes/") && url.endsWith("/probe"));
    expect(probeURLs).toHaveLength(2);
    expect(probeURLs.some((url) => url.includes("/api/nodes/node-3/probe"))).toBe(false);
    const nodeListCalls = fetchMock.mock.calls.filter(
      ([input, init]) => String(input).endsWith("/api/nodes") && (!init?.method || init.method === "GET"),
    );
    expect(nodeListCalls).toHaveLength(2);
  });

  it("节点卡片支持单节点延迟测试按钮", async () => {
    window.history.pushState({}, "", "/nodes");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    const fetchMock = installAuthenticatedFetchMock({
      nodes: [
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
          last_latency_ms: null,
          last_status: "unknown",
          last_checked_at: null,
          has_credential: true,
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
      ],
      probeDelays: {
        "node-1": 20,
      },
      probeResults: {
        "node-1": {
          success: true,
          latency_ms: 77,
        },
      },
    });

    renderApp();

    const user = userEvent.setup();
    expect(await screen.findByRole("heading", { name: "节点池" })).toBeInTheDocument();

    const probeButton = await screen.findByRole("button", { name: "测试 香港-A1 延迟" });
    await user.click(probeButton);
    expect(probeButton).toBeDisabled();

    await waitFor(() => {
      expect(screen.getByText("77 ms")).toBeInTheDocument();
    });
    const nodeListCalls = fetchMock.mock.calls.filter(
      ([input, init]) => String(input).endsWith("/api/nodes") && (!init?.method || init.method === "GET"),
    );
    expect(nodeListCalls).toHaveLength(2);
  });
});
