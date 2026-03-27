import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
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
  subscriptionRefreshResults?: Record<
    string,
    {
      upserted_nodes: Array<Record<string, unknown>>;
      deleted_node_ids?: string[];
      updated_at?: string;
      last_refresh_at?: string;
      last_error?: string;
    }
  >;
  groups?: Array<Record<string, unknown>>;
  groupMembers?: Record<string, Array<Record<string, unknown>>>;
  tunnels?: Array<Record<string, unknown>>;
  probeDelays?: Record<string, number>;
  probeResults?: Record<string, Record<string, unknown>>;
  tunnelActionBehaviors?: Record<string, { delay_ms?: number; status?: number; message?: string }>;
  groupMemberStreamEvents?: Record<string, Array<{ delay_ms: number; member: Record<string, unknown> }>>;
  groupMemberStreamObserver?: { aborted_group_ids: string[] };
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

function upsertNodes(
  current: Array<Record<string, unknown>>,
  incoming: Array<Record<string, unknown>>,
) {
  const next = [...current];
  incoming.forEach((item) => {
    const index = next.findIndex((currentItem) => currentItem.id === item.id);
    if (index >= 0) {
      next[index] = item;
      return;
    }
    next.unshift(item);
  });
  return next;
}

function installAuthenticatedFetchMock(options: FetchMockOptions = {}) {
  let nodes = structuredClone(options.nodes ?? defaultNodes());
  let subscriptions = structuredClone(options.subscriptions ?? [
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
  let groupMembers = structuredClone(options.groupMembers ?? {
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
  });
  let tunnels = structuredClone(options.tunnels ?? [
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
  const probeDelays = options.probeDelays ?? {};
  const probeResults = options.probeResults ?? {};
  const subscriptionRefreshResults = options.subscriptionRefreshResults ?? {};
  const tunnelActionBehaviors = options.tunnelActionBehaviors ?? {};
  const groupMemberStreamEvents = options.groupMemberStreamEvents ?? {};
  const groupMemberStreamObserver = options.groupMemberStreamObserver;

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
      groupMembers = Object.fromEntries(
        Object.entries(groupMembers).map(([groupID, members]) => [
          groupID,
          members.map((item) =>
            item.id === nodeID
              ? {
                  ...item,
                  last_status: result.success ? "healthy" : "unreachable",
                  last_latency_ms: result.success ? result.latency_ms : null,
                  last_checked_at: checkedAt,
                }
              : item,
          ),
        ]),
      );
      return jsonResponse(200, result);
    }
    const setEnabledMatch = url.match(/\/api\/nodes\/([^/]+)\/enabled$/);
    if (setEnabledMatch && init?.method === "PUT") {
      const nodeID = setEnabledMatch[1];
      const payload = init?.body ? JSON.parse(String(init.body)) as { enabled?: boolean } : {};
      const nextEnabled = Boolean(payload.enabled);
      nodes = nodes.map((item) =>
        item.id === nodeID
          ? {
              ...item,
              enabled: nextEnabled,
            }
          : item,
      );
      groupMembers = Object.fromEntries(
        Object.entries(groupMembers).map(([groupID, members]) => [
          groupID,
          members.map((item) =>
            item.id === nodeID
              ? {
                  ...item,
                  enabled: nextEnabled,
                }
              : item,
          ),
        ]),
      );
      const updatedNode = nodes.find((item) => item.id === nodeID) ?? { id: nodeID, enabled: nextEnabled };
      return jsonResponse(200, updatedNode);
    }
    if (url.endsWith("/api/groups")) {
      return jsonResponse(200, groups);
    }
    const groupMembersMatch = url.match(/\/api\/groups\/([^/]+)\/members$/);
    if (groupMembersMatch) {
      return jsonResponse(200, groupMembers[groupMembersMatch[1]] ?? []);
    }
    const groupMembersStreamMatch = url.match(/\/api\/groups\/([^/]+)\/members\/stream$/);
    if (groupMembersStreamMatch) {
      const groupID = groupMembersStreamMatch[1];
      const encoder = new TextEncoder();
      const events = groupMemberStreamEvents[groupID] ?? [];
      return new Response(new ReadableStream({
        start(controller) {
          const timers = events.map(({ delay_ms, member }) =>
            setTimeout(() => {
              controller.enqueue(encoder.encode(`${JSON.stringify(member)}\n`));
            }, delay_ms),
          );
          const abortSignal = init?.signal;
          const handleAbort = () => {
            timers.forEach((timer) => clearTimeout(timer));
            groupMemberStreamObserver?.aborted_group_ids.push(groupID);
            try {
              controller.close();
            } catch {
              // ignore repeated abort/close in tests
            }
          };
          if (abortSignal) {
            if (abortSignal.aborted) {
              handleAbort();
              return;
            }
            abortSignal.addEventListener("abort", handleAbort, { once: true });
          }
        },
      }), {
        status: 200,
        headers: {
          "Content-Type": "application/x-ndjson",
        },
      });
    }
    if (url.endsWith("/api/tunnels") && (init?.method === undefined || init.method === "GET")) {
      return jsonResponse(200, tunnels);
    }
    if (url.endsWith("/api/tunnels") && init?.method === "POST") {
      const payload = init.body
        ? JSON.parse(String(init.body)) as {
            name: string;
            group_id: string;
            listen_host: string;
            username: string;
            password: string;
          }
        : { name: "", group_id: "", listen_host: "", username: "", password: "" };
      const groupID = payload.group_id;
      const currentNodeID = groupMembers[groupID]?.[0]?.id ?? null;
      const created = {
        id: `tunnel-${tunnels.length + 1}`,
        name: payload.name,
        group_id: groupID,
        listen_host: payload.listen_host || "0.0.0.0",
        listen_port: 18080 + tunnels.length,
        status: "running",
        current_node_id: currentNodeID,
        controller_port: 19090 + tunnels.length,
        runtime_dir: `/tmp/runtime-${tunnels.length + 1}`,
        last_refresh_at: "2026-03-26T10:07:00Z",
        last_refresh_error: "",
        has_auth: Boolean(payload.username || payload.password),
        created_at: "2026-03-26T10:07:00Z",
        updated_at: "2026-03-26T10:07:00Z",
      };
      tunnels = [created, ...tunnels];
      return jsonResponse(201, created);
    }
    const tunnelActionMatch = url.match(/\/api\/tunnels\/([^/]+)\/(refresh|start|stop)$/);
    if (tunnelActionMatch && init?.method === "POST") {
      const tunnelID = tunnelActionMatch[1];
      const action = tunnelActionMatch[2];
      const actionBehavior = tunnelActionBehaviors[`${tunnelID}:${action}`];
      if ((actionBehavior?.delay_ms ?? 0) > 0) {
        await new Promise((resolve) => setTimeout(resolve, actionBehavior?.delay_ms));
      }
      if ((actionBehavior?.status ?? 200) >= 400) {
        return jsonResponse(actionBehavior?.status ?? 500, {
          code: "request_failed",
          message: actionBehavior?.message ?? "隧道操作失败",
        });
      }
      const now =
        action === "refresh"
          ? "2026-03-26T10:08:00Z"
          : "2026-03-26T10:09:00Z";
      let updatedTunnel: Record<string, unknown> | undefined;
      tunnels = tunnels.map((item) => {
        if (item.id !== tunnelID) {
          return item;
        }
        updatedTunnel = {
          ...item,
          status: action === "stop" ? "stopped" : "running",
          last_refresh_at: action === "refresh" ? now : item.last_refresh_at,
          updated_at: now,
        };
        return updatedTunnel;
      });
      return jsonResponse(200, updatedTunnel ?? { id: tunnelID });
    }
    const tunnelMatch = url.match(/\/api\/tunnels\/([^/]+)$/);
    if (tunnelMatch && init?.method === "PUT") {
      const tunnelID = tunnelMatch[1];
      const payload = init.body
        ? JSON.parse(String(init.body)) as {
            name: string;
            group_id: string;
            listen_host: string;
            username: string;
            password: string;
          }
        : { name: "", group_id: "", listen_host: "", username: "", password: "" };
      let updatedTunnel: Record<string, unknown> | undefined;
      tunnels = tunnels.map((item) => {
        if (item.id !== tunnelID) {
          return item;
        }
        updatedTunnel = {
          ...item,
          name: payload.name,
          group_id: payload.group_id,
          listen_host: payload.listen_host || item.listen_host,
          has_auth: Boolean(payload.username || payload.password),
          updated_at: "2026-03-26T10:08:00Z",
        };
        return updatedTunnel;
      });
      return jsonResponse(200, updatedTunnel ?? { id: tunnelID });
    }
    if (tunnelMatch && init?.method === "DELETE") {
      const tunnelID = tunnelMatch[1];
      tunnels = tunnels.filter((item) => item.id !== tunnelID);
      return new Response(null, { status: 204 });
    }
    if (url.endsWith("/api/subscriptions") && (init?.method === undefined || init.method === "GET")) {
      return jsonResponse(200, subscriptions);
    }
    if (url.endsWith("/api/subscriptions") && init?.method === "POST") {
      const payload = init.body ? JSON.parse(String(init.body)) as { name: string; url: string } : { name: "", url: "" };
      const created = {
        id: `subscription-${subscriptions.length + 1}`,
        name: payload.name,
        fetch_fingerprint: `fingerprint-${subscriptions.length + 1}`,
        enabled: true,
        last_refresh_at: null,
        last_error: "",
        has_url: Boolean(payload.url),
        created_at: "2026-03-26T10:07:00Z",
        updated_at: "2026-03-26T10:07:00Z",
      };
      subscriptions = [created, ...subscriptions];
      return jsonResponse(200, created);
    }
    const subscriptionRefreshMatch = url.match(/\/api\/subscriptions\/([^/]+)\/refresh$/);
    if (subscriptionRefreshMatch && init?.method === "POST") {
      const subscriptionID = subscriptionRefreshMatch[1];
      const refreshPayload = subscriptionRefreshResults[subscriptionID] ?? {
        upserted_nodes: [],
      };
      const upsertedNodes = structuredClone(refreshPayload.upserted_nodes);
      const deletedNodeIDs = refreshPayload.deleted_node_ids ?? [];
      nodes = upsertNodes(
        nodes.filter((item) => !deletedNodeIDs.includes(String(item.id))),
        upsertedNodes,
      );
      subscriptions = subscriptions.map((item) =>
        item.id === subscriptionID
          ? {
              ...item,
              last_refresh_at: refreshPayload.last_refresh_at ?? "2026-03-26T10:08:00Z",
              updated_at: refreshPayload.updated_at ?? "2026-03-26T10:08:00Z",
              last_error: refreshPayload.last_error ?? "",
            }
          : item,
      );
      return jsonResponse(200, {
        source_id: subscriptionID,
        upserted_nodes: upsertedNodes,
        deleted_count: deletedNodeIDs.length,
      });
    }
    const subscriptionMatch = url.match(/\/api\/subscriptions\/([^/]+)$/);
    if (subscriptionMatch && init?.method === "PUT") {
      const subscriptionID = subscriptionMatch[1];
      const payload = init.body
        ? JSON.parse(String(init.body)) as { name: string; url: string; enabled: boolean }
        : { name: "", url: "", enabled: true };
      let updatedSubscription: Record<string, unknown> | undefined;
      subscriptions = subscriptions.map((item) => {
        if (item.id !== subscriptionID) {
          return item;
        }
        updatedSubscription = {
          ...item,
          name: payload.name,
          enabled: payload.enabled,
          has_url: Boolean(payload.url),
          updated_at: "2026-03-26T10:09:00Z",
        };
        return updatedSubscription;
      });
      return jsonResponse(200, updatedSubscription ?? { id: subscriptionID });
    }
    if (subscriptionMatch && init?.method === "DELETE") {
      const subscriptionID = subscriptionMatch[1];
      subscriptions = subscriptions.filter((item) => item.id !== subscriptionID);
      nodes = nodes.filter((item) => item.subscription_source_id !== subscriptionID);
      return new Response(null, { status: 204 });
    }
    if (url.includes("/api/tunnels/") && url.includes("/events")) {
      return jsonResponse(200, []);
    }
    throw new Error(`unexpected request: ${url}`);
  });

  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function countFetchCalls(fetchMock: ReturnType<typeof installAuthenticatedFetchMock>, matcher: RegExp, method = "GET") {
  return fetchMock.mock.calls.filter(([input, init]) => matcher.test(String(input)) && (init?.method ?? "GET") === method).length;
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
    expect(screen.queryByRole("link", { name: "订阅" })).not.toBeInTheDocument();
    expect(screen.getByText("SimplePool")).toBeInTheDocument();
    expect(screen.queryByText("Transport JSON")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "查看详情" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "新建节点" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "添加订阅" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "导入节点" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "探测" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "网格视图" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "列表视图" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "筛选 ALL" })).toBeInTheDocument();
  });

  it("旧工作区路由现在显示不存在页面", async () => {
    window.history.pushState({}, "", "/workspace");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    installAuthenticatedFetchMock();

    renderApp();

    expect(await screen.findByRole("heading", { name: "页面不存在" })).toBeInTheDocument();
  });

  it("旧订阅路由现在显示不存在页面", async () => {
    window.history.pushState({}, "", "/subscriptions");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    installAuthenticatedFetchMock();

    renderApp();

    expect(await screen.findByRole("heading", { name: "页面不存在" })).toBeInTheDocument();
  });

  it("节点页支持按订阅 Tag 筛选并显示时间 tooltip", async () => {
    window.history.pushState({}, "", "/nodes");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    installAuthenticatedFetchMock({
      subscriptions: [
        {
          id: "subscription-1",
          name: "赔钱",
          fetch_fingerprint: "fp-1",
          enabled: true,
          last_refresh_at: "2026-03-26T10:06:00Z",
          last_error: "",
          has_url: true,
          created_at: "2026-03-26T09:07:00Z",
          updated_at: "2026-03-26T09:08:00Z",
        },
        {
          id: "subscription-2",
          name: "稳健",
          fetch_fingerprint: "fp-2",
          enabled: true,
          last_refresh_at: "2026-03-26T10:06:00Z",
          last_error: "",
          has_url: true,
          created_at: "2026-03-26T09:09:00Z",
          updated_at: "2026-03-26T09:10:00Z",
        },
      ],
      nodes: [
        {
          id: "manual-1",
          name: "手动-A1",
          source_kind: "manual",
          protocol: "trojan",
          server: "192.168.1.100",
          server_port: 443,
          transport_json: "{}",
          tls_json: "{}",
          raw_payload_json: "{}",
          enabled: true,
          last_latency_ms: 30,
          last_status: "healthy",
          last_checked_at: "2026-03-26T10:04:00Z",
          has_credential: true,
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
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
          subscription_source_id: "subscription-2",
          protocol: "trojan",
          server: "192.168.1.103",
          server_port: 443,
          transport_json: "{}",
          tls_json: "{}",
          raw_payload_json: "{}",
          enabled: true,
          last_latency_ms: 52,
          last_status: "healthy",
          last_checked_at: "2026-03-26T10:01:00Z",
          has_credential: true,
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
      ],
    });

    renderApp();

    const user = userEvent.setup();
    expect(await screen.findByRole("heading", { name: "节点池" })).toBeInTheDocument();
    expect(screen.getByText("手动-A1")).toBeInTheDocument();
    expect(screen.getByText("香港-A1")).toBeInTheDocument();
    expect(screen.getByText("日本-B2")).toBeInTheDocument();
    expect(screen.getByText("美国-C3")).toBeInTheDocument();

    const subscriptionFilter = screen.getByRole("button", { name: "筛选 赔钱" });
    expect(subscriptionFilter).toHaveAttribute("title", expect.stringContaining("创建时间:"));
    expect(subscriptionFilter).toHaveAttribute("title", expect.stringContaining("更新时间:"));

    await user.click(subscriptionFilter);

    await waitFor(() => {
      expect(screen.queryByText("手动-A1")).not.toBeInTheDocument();
    });
    expect(screen.getByText("香港-A1")).toBeInTheDocument();
    expect(screen.getByText("日本-B2")).toBeInTheDocument();
    expect(screen.queryByText("美国-C3")).not.toBeInTheDocument();
  });

  it("节点页刷新当前订阅时保持当前筛选并更新节点数据", async () => {
    window.history.pushState({}, "", "/nodes");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    const fetchMock = installAuthenticatedFetchMock({
      subscriptions: [
        {
          id: "subscription-1",
          name: "赔钱",
          fetch_fingerprint: "fp-1",
          enabled: true,
          last_refresh_at: "2026-03-26T10:06:00Z",
          last_error: "",
          has_url: true,
          created_at: "2026-03-26T09:07:00Z",
          updated_at: "2026-03-26T09:08:00Z",
        },
        {
          id: "subscription-2",
          name: "稳健",
          fetch_fingerprint: "fp-2",
          enabled: true,
          last_refresh_at: "2026-03-26T10:06:00Z",
          last_error: "",
          has_url: true,
          created_at: "2026-03-26T09:09:00Z",
          updated_at: "2026-03-26T09:10:00Z",
        },
      ],
      nodes: [
        {
          id: "node-1",
          name: "香港-A1",
          source_kind: "subscription",
          subscription_source_id: "subscription-1",
          protocol: "trojan",
          server: "192.168.1.103",
          server_port: 443,
          transport_json: "{}",
          tls_json: "{}",
          raw_payload_json: "{}",
          enabled: true,
          last_latency_ms: 42,
          last_status: "healthy",
          last_checked_at: "2026-03-26T10:01:00Z",
          has_credential: true,
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
        {
          id: "node-2",
          name: "日本-B2",
          source_kind: "subscription",
          subscription_source_id: "subscription-2",
          protocol: "vmess",
          server: "192.168.1.102",
          server_port: 443,
          transport_json: "{}",
          tls_json: "{}",
          raw_payload_json: "{}",
          enabled: true,
          last_latency_ms: 55,
          last_status: "healthy",
          last_checked_at: "2026-03-26T10:04:00Z",
          has_credential: true,
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
        },
      ],
      subscriptionRefreshResults: {
        "subscription-1": {
          upserted_nodes: [
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
              last_latency_ms: 61,
              last_status: "healthy",
              last_checked_at: "2026-03-26T10:08:00Z",
              has_credential: true,
              created_at: "2026-03-26T10:08:00Z",
              updated_at: "2026-03-26T10:08:00Z",
            },
          ],
        },
      },
    });

    renderApp();

    const user = userEvent.setup();
    expect(await screen.findByRole("heading", { name: "节点池" })).toBeInTheDocument();

    expect(screen.queryByRole("button", { name: "刷新 赔钱" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "筛选 赔钱" }));

    await waitFor(() => {
      expect(screen.getByText("香港-A1")).toBeInTheDocument();
    });
    expect(screen.queryByText("日本-B2")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "刷新订阅" }));

    await waitFor(() => {
      expect(
        fetchMock.mock.calls.some(
          ([input, init]) => String(input).endsWith("/api/subscriptions/subscription-1/refresh") && init?.method === "POST",
        ),
      ).toBe(true);
    });
    expect(screen.getByText("香港-A1")).toBeInTheDocument();
    expect(screen.getByText("美国-C3")).toBeInTheDocument();
    expect(screen.queryByText("日本-B2")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "筛选 ALL" }));
    expect(await screen.findByText("美国-C3")).toBeInTheDocument();
  });

  it("节点页支持新增编辑删除订阅，并使用自定义确认弹窗", async () => {
    window.history.pushState({}, "", "/nodes");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    const fetchMock = installAuthenticatedFetchMock({
      subscriptions: [
        {
          id: "subscription-1",
          name: "原始订阅",
          fetch_fingerprint: "fp-1",
          enabled: true,
          last_refresh_at: null,
          last_error: "",
          has_url: true,
          created_at: "2026-03-26T09:07:00Z",
          updated_at: "2026-03-26T09:07:00Z",
        },
      ],
    });
    const confirmSpy = vi.spyOn(window, "confirm").mockImplementation(() => {
      throw new Error("不应调用浏览器原生确认弹窗");
    });

    renderApp();

    const user = userEvent.setup();
    expect(await screen.findByRole("heading", { name: "节点池" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "添加订阅" }));
    await user.type(screen.getByLabelText("名称"), "新增订阅");
    await user.type(screen.getByLabelText("订阅 URL"), "https://example.com/sub.txt");
    await user.click(screen.getByRole("button", { name: "创建订阅" }));

    expect(await screen.findByRole("button", { name: "筛选 新增订阅" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "筛选 新增订阅" }));
    await user.click(screen.getByRole("button", { name: "编辑订阅" }));

    const nameInput = screen.getByLabelText("名称");
    const urlInput = screen.getByLabelText("订阅 URL");
    await user.clear(nameInput);
    await user.type(nameInput, "新增订阅-改");
    await user.clear(urlInput);
    await user.type(urlInput, "https://example.com/sub-new.txt");
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "筛选 新增订阅-改" })).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: "删除订阅" }));

    expect(await screen.findByText("确认删除订阅")).toBeInTheDocument();
    expect(screen.getByText("关联订阅节点会一起删除。")).toBeInTheDocument();
    expect(countFetchCalls(fetchMock, /\/api\/subscriptions\/subscription-2$/, "DELETE")).toBe(0);

    await user.click(screen.getByRole("button", { name: "取消" }));

    await waitFor(() => {
      expect(screen.queryByText("确认删除订阅")).not.toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "筛选 新增订阅-改" })).toBeInTheDocument();
    expect(countFetchCalls(fetchMock, /\/api\/subscriptions\/subscription-2$/, "DELETE")).toBe(0);

    await user.click(screen.getByRole("button", { name: "删除订阅" }));
    await user.click(await screen.findByRole("button", { name: "确认删除" }));

    await waitFor(() => {
      expect(screen.queryByRole("button", { name: "筛选 新增订阅-改" })).not.toBeInTheDocument();
    });
    expect(countFetchCalls(fetchMock, /\/api\/subscriptions\/subscription-2$/, "DELETE")).toBe(1);
    expect(confirmSpy).not.toHaveBeenCalled();
  });

  it("节点页批量探测仅作用于当前订阅筛选并跳过禁用节点", async () => {
    window.history.pushState({}, "", "/nodes");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    const fetchMock = installAuthenticatedFetchMock({
      subscriptions: [
        {
          id: "subscription-1",
          name: "赔钱",
          fetch_fingerprint: "fp-1",
          enabled: true,
          last_refresh_at: null,
          last_error: "",
          has_url: true,
          created_at: "2026-03-26T09:07:00Z",
          updated_at: "2026-03-26T09:07:00Z",
        },
      ],
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
        {
          id: "node-4",
          name: "手动-D4",
          source_kind: "manual",
          protocol: "trojan",
          server: "192.168.1.104",
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
    expect(await screen.findByRole("heading", { name: "节点池" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "筛选 赔钱" }));
    await user.click(screen.getByRole("button", { name: "探测" }));

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
    expect(probeURLs.some((url) => url.includes("/api/nodes/node-4/probe"))).toBe(false);
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

  it("节点组页支持直接探测组成员", async () => {
    window.history.pushState({}, "", "/");
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
            last_latency_ms: null,
            last_status: "unknown",
            last_checked_at: null,
            created_at: "2026-03-26T10:00:00Z",
            updated_at: "2026-03-26T10:00:00Z",
          },
        ],
      },
      probeDelays: {
        "node-1": 20,
      },
      probeResults: {
        "node-1": {
          success: true,
          latency_ms: 91,
        },
      },
      tunnels: [],
    });

    renderApp();

    const user = userEvent.setup();
    expect(await screen.findByRole("heading", { name: "节点组" })).toBeInTheDocument();

    const probeButton = await screen.findByRole("button", { name: "测试 香港-A1 延迟" });
    await user.click(probeButton);
    expect(probeButton).toBeDisabled();

    await waitFor(() => {
      expect(screen.getByText("91 ms")).toBeInTheDocument();
    });
    expect(
      fetchMock.mock.calls.some(([input]) => String(input).endsWith("/api/nodes/node-1/probe")),
    ).toBe(true);
  });

  it("节点组页支持直接禁用和启用组成员", async () => {
    window.history.pushState({}, "", "/");
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
          last_latency_ms: 42,
          last_status: "healthy",
          last_checked_at: "2026-03-26T10:04:00Z",
          has_credential: true,
          created_at: "2026-03-26T10:00:00Z",
          updated_at: "2026-03-26T10:00:00Z",
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
      },
      tunnels: [],
    });

    renderApp();

    const user = userEvent.setup();
    expect(await screen.findByRole("heading", { name: "节点组" })).toBeInTheDocument();

    const disableButton = await screen.findByRole("button", { name: "禁用 香港-A1" });
    await user.click(disableButton);

    await waitFor(() => {
      expect(screen.getByText("已禁用")).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "启用 香港-A1" })).toBeInTheDocument();
    expect(
      fetchMock.mock.calls.some(
        ([input, init]) =>
          String(input).endsWith("/api/nodes/node-1/enabled")
          && init?.method === "PUT"
          && String(init.body).includes("\"enabled\":false"),
      ),
    ).toBe(true);

    await user.click(screen.getByRole("button", { name: "启用 香港-A1" }));

    await waitFor(() => {
      expect(screen.queryByText("已禁用")).not.toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "禁用 香港-A1" })).toBeInTheDocument();
    expect(
      fetchMock.mock.calls.some(
        ([input, init]) =>
          String(input).endsWith("/api/nodes/node-1/enabled")
          && init?.method === "PUT"
          && String(init.body).includes("\"enabled\":true"),
      ),
    ).toBe(true);
  });

  it("节点池页可实时接收组成员推送并更新节点延迟", async () => {
    window.history.pushState({}, "", "/nodes");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    installAuthenticatedFetchMock({
      groupMemberStreamEvents: {
        "group-1": [
          {
            delay_ms: 30,
            member: {
              id: "node-1",
              name: "香港-A1",
              source_kind: "manual",
              protocol: "trojan",
              server: "192.168.1.101",
              server_port: 443,
              enabled: true,
              last_latency_ms: 84,
              last_status: "healthy",
              last_checked_at: "2026-03-26T10:08:00Z",
              created_at: "2026-03-26T10:00:00Z",
              updated_at: "2026-03-26T10:08:00Z",
            },
          },
        ],
      },
    });

    renderApp();

    expect(await screen.findByRole("heading", { name: "节点池" })).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("84 ms")).toBeInTheDocument();
    });
  });

  it("动态隧道创建时自动带入当前分组并在卡片内显示节点与操作按钮", async () => {
    window.history.pushState({}, "", "/");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    vi.spyOn(Date, "now").mockReturnValue(new Date("2026-03-26T10:53:00Z").getTime());
    const fetchMock = installAuthenticatedFetchMock({
      tunnels: [],
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
      },
    });

    renderApp();

    const user = userEvent.setup();
    expect(await screen.findByRole("heading", { name: "节点组" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "创建隧道" }));
    expect(screen.queryByLabelText("分组 ID")).not.toBeInTheDocument();

    await user.type(screen.getByLabelText("隧道名称"), "代理-B");
    await user.click(screen.getByRole("button", { name: "创建隧道" }));

    await waitFor(() => {
      expect(
        fetchMock.mock.calls.some(
          ([input, init]) =>
            String(input).endsWith("/api/tunnels") &&
            init?.method === "POST" &&
            String(init.body).includes("\"group_id\":\"group-1\""),
        ),
      ).toBe(true);
    });

    const tunnelCard = await screen.findByRole("group", { name: "隧道 代理-B" });
    expect(within(tunnelCard).getByText("46m 前")).toBeInTheDocument();
    expect(within(tunnelCard).getByLabelText("认证未启用")).toBeInTheDocument();
    expect(within(tunnelCard).getByText("香港-A1")).toBeInTheDocument();
    expect(within(tunnelCard).getByText("监听地址")).toBeInTheDocument();
    expect(within(tunnelCard).getByText("0.0.0.0:18080")).toBeInTheDocument();
    expect(within(tunnelCard).queryByText(/当前节点/)).not.toBeInTheDocument();
    expect(within(tunnelCard).getByRole("button", { name: "刷新 代理-B" })).toBeInTheDocument();
    expect(within(tunnelCard).getByRole("button", { name: "停止 代理-B" })).toBeInTheDocument();
    expect(within(tunnelCard).getByRole("button", { name: "编辑 代理-B" })).toBeInTheDocument();
    expect(within(tunnelCard).getByRole("button", { name: "删除 代理-B" })).toBeInTheDocument();
  });

  it("动态隧道卡片显示认证图标与紧凑刷新时间，并在停止后灰化", async () => {
    window.history.pushState({}, "", "/");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    vi.spyOn(Date, "now").mockReturnValue(new Date("2026-03-26T10:53:00Z").getTime());
    installAuthenticatedFetchMock();

    renderApp();

    const user = userEvent.setup();
    const tunnelCard = await screen.findByRole("group", { name: "隧道 代理-A" });
    expect(within(tunnelCard).getByLabelText("认证已启用")).toBeInTheDocument();
    expect(within(tunnelCard).getByText("48m 前")).toBeInTheDocument();
    expect(within(tunnelCard).getByText("香港-A1")).toBeInTheDocument();
    expect(within(tunnelCard).getByText("127.0.0.1:18080")).toBeInTheDocument();
    expect(within(tunnelCard).getByRole("button", { name: "停止 代理-A" }).className).toContain("bg-rose-500/12");

    await user.click(within(tunnelCard).getByRole("button", { name: "停止 代理-A" }));

    await waitFor(() => {
      expect(within(screen.getByRole("group", { name: "隧道 代理-A" })).getByRole("button", { name: "启动 代理-A" })).toBeInTheDocument();
    });

    const stoppedTunnelCard = screen.getByRole("group", { name: "隧道 代理-A" });
    expect(stoppedTunnelCard.className).toContain("grayscale");
    expect(within(stoppedTunnelCard).getByRole("button", { name: "启动 代理-A" }).className).toContain("bg-emerald-500/12");
  });

  it("动态隧道启动停止只更新卡片且不重新拉取整个工作区", async () => {
    window.history.pushState({}, "", "/");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    const fetchMock = installAuthenticatedFetchMock();

    renderApp();

    const user = userEvent.setup();
    const tunnelCard = await screen.findByRole("group", { name: "隧道 代理-A" });

    const groupsBefore = countFetchCalls(fetchMock, /\/api\/groups$/);
    const tunnelsBefore = countFetchCalls(fetchMock, /\/api\/tunnels$/);
    const nodesBefore = countFetchCalls(fetchMock, /\/api\/nodes$/);
    const membersBefore = countFetchCalls(fetchMock, /\/api\/groups\/group-1\/members$/);

    await user.click(within(tunnelCard).getByRole("button", { name: "停止 代理-A" }));

    await waitFor(() => {
      expect(within(screen.getByRole("group", { name: "隧道 代理-A" })).getByRole("button", { name: "启动 代理-A" })).toBeInTheDocument();
    });

    expect(countFetchCalls(fetchMock, /\/api\/tunnels\/tunnel-1\/stop$/, "POST")).toBe(1);
    expect(countFetchCalls(fetchMock, /\/api\/groups$/)).toBe(groupsBefore);
    expect(countFetchCalls(fetchMock, /\/api\/tunnels$/)).toBe(tunnelsBefore);
    expect(countFetchCalls(fetchMock, /\/api\/nodes$/)).toBe(nodesBefore);
    expect(countFetchCalls(fetchMock, /\/api\/groups\/group-1\/members$/)).toBe(membersBefore);

    await user.click(within(screen.getByRole("group", { name: "隧道 代理-A" })).getByRole("button", { name: "启动 代理-A" }));

    await waitFor(() => {
      expect(within(screen.getByRole("group", { name: "隧道 代理-A" })).getByRole("button", { name: "停止 代理-A" })).toBeInTheDocument();
    });

    expect(countFetchCalls(fetchMock, /\/api\/tunnels\/tunnel-1\/start$/, "POST")).toBe(1);
    expect(countFetchCalls(fetchMock, /\/api\/groups$/)).toBe(groupsBefore);
    expect(countFetchCalls(fetchMock, /\/api\/tunnels$/)).toBe(tunnelsBefore);
    expect(countFetchCalls(fetchMock, /\/api\/nodes$/)).toBe(nodesBefore);
    expect(countFetchCalls(fetchMock, /\/api\/groups\/group-1\/members$/)).toBe(membersBefore);
    expect(within(screen.getByRole("group", { name: "隧道 代理-A" })).getByText("香港-A1")).toBeInTheDocument();
  });

  it("动态隧道刷新时显示淡化转圈并阻止重复点击，完成后恢复", async () => {
    window.history.pushState({}, "", "/");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    const fetchMock = installAuthenticatedFetchMock({
      tunnelActionBehaviors: {
        "tunnel-1:refresh": { delay_ms: 80 },
      },
    });

    renderApp();

    const user = userEvent.setup();
    const tunnelCard = await screen.findByRole("group", { name: "隧道 代理-A" });
    const refreshButton = within(tunnelCard).getByRole("button", { name: "刷新 代理-A" });

    await user.click(refreshButton);

    await waitFor(() => {
      expect(screen.getByRole("group", { name: "隧道 代理-A" })).toHaveAttribute("aria-busy", "true");
    });

    const busyCard = screen.getByRole("group", { name: "隧道 代理-A" });
    const busyRefreshButton = within(busyCard).getByRole("button", { name: "刷新 代理-A" });
    expect(busyRefreshButton).toBeDisabled();
    expect(busyCard.className).toContain("opacity-60");
    expect(busyRefreshButton.querySelector("svg")?.getAttribute("class") ?? "").toContain("animate-spin");

    await user.click(busyRefreshButton);
    expect(countFetchCalls(fetchMock, /\/api\/tunnels\/tunnel-1\/refresh$/, "POST")).toBe(1);

    await waitFor(() => {
      expect(screen.getByRole("group", { name: "隧道 代理-A" })).toHaveAttribute("aria-busy", "false");
    });

    const recoveredCard = screen.getByRole("group", { name: "隧道 代理-A" });
    const recoveredRefreshButton = within(recoveredCard).getByRole("button", { name: "刷新 代理-A" });
    expect(recoveredRefreshButton).not.toBeDisabled();
    expect(recoveredCard.className).not.toContain("opacity-60");
    expect(recoveredRefreshButton.querySelector("svg")?.getAttribute("class") ?? "").not.toContain("animate-spin");
  });

  it("动态隧道刷新失败后恢复动画状态", async () => {
    window.history.pushState({}, "", "/");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    const fetchMock = installAuthenticatedFetchMock({
      tunnelActionBehaviors: {
        "tunnel-1:refresh": { delay_ms: 50, status: 409, message: "没有可切换的新节点" },
      },
    });

    renderApp();

    const user = userEvent.setup();
    const tunnelCard = await screen.findByRole("group", { name: "隧道 代理-A" });

    await user.click(within(tunnelCard).getByRole("button", { name: "刷新 代理-A" }));

    await waitFor(() => {
      expect(screen.getByRole("group", { name: "隧道 代理-A" })).toHaveAttribute("aria-busy", "true");
    });

    await waitFor(() => {
      expect(within(screen.getByRole("group", { name: "隧道 代理-A" })).getByRole("button", { name: "刷新 代理-A" })).not.toBeDisabled();
    });

    const recoveredCard = screen.getByRole("group", { name: "隧道 代理-A" });
    expect(recoveredCard).toHaveAttribute("aria-busy", "false");
    expect(recoveredCard.className).not.toContain("opacity-60");
    expect(within(recoveredCard).getByRole("button", { name: "刷新 代理-A" }).querySelector("svg")?.getAttribute("class") ?? "").not.toContain("animate-spin");
    expect(countFetchCalls(fetchMock, /\/api\/tunnels\/tunnel-1\/refresh$/, "POST")).toBe(1);
  });

  it("组节点页可实时接收动态隧道后台探测推送并更新节点延迟", async () => {
    window.history.pushState({}, "", "/");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    installAuthenticatedFetchMock({
      groupMemberStreamEvents: {
        "group-1": [
          {
            delay_ms: 30,
            member: {
              id: "node-1",
              name: "香港-A1",
              source_kind: "manual",
              protocol: "trojan",
              server: "192.168.1.101",
              server_port: 443,
              enabled: true,
              last_latency_ms: 84,
              last_status: "healthy",
              last_checked_at: "2026-03-26T10:08:00Z",
              created_at: "2026-03-26T10:00:00Z",
              updated_at: "2026-03-26T10:08:00Z",
            },
          },
        ],
      },
    });

    renderApp();

    const user = userEvent.setup();
    const tunnelCard = await screen.findByRole("group", { name: "隧道 代理-A" });

    await user.click(within(tunnelCard).getByRole("button", { name: "刷新 代理-A" }));

    await waitFor(() => {
      expect(screen.getByText("84 ms")).toBeInTheDocument();
    });
  });

  it("切换分组时关闭旧的组节点推送流", async () => {
    window.history.pushState({}, "", "/");
    window.localStorage.setItem("simplepool.session_token", "token-1");
    const observer = { aborted_group_ids: [] as string[] };
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
            protocol: "trojan",
            server: "192.168.1.102",
            server_port: 443,
            enabled: true,
            last_latency_ms: 51,
            last_status: "healthy",
            last_checked_at: "2026-03-26T10:04:00Z",
            created_at: "2026-03-26T10:00:00Z",
            updated_at: "2026-03-26T10:00:00Z",
          },
        ],
      },
      tunnels: [],
      groupMemberStreamEvents: {
        "group-1": [
          {
            delay_ms: 80,
            member: {
              id: "node-1",
              name: "香港-A1",
              source_kind: "manual",
              protocol: "trojan",
              server: "192.168.1.101",
              server_port: 443,
              enabled: true,
              last_latency_ms: 84,
              last_status: "healthy",
              last_checked_at: "2026-03-26T10:08:00Z",
              created_at: "2026-03-26T10:00:00Z",
              updated_at: "2026-03-26T10:08:00Z",
            },
          },
        ],
        "group-2": [
          {
            delay_ms: 20,
            member: {
              id: "node-2",
              name: "日本-B2",
              source_kind: "manual",
              protocol: "trojan",
              server: "192.168.1.102",
              server_port: 443,
              enabled: true,
              last_latency_ms: 63,
              last_status: "healthy",
              last_checked_at: "2026-03-26T10:08:00Z",
              created_at: "2026-03-26T10:00:00Z",
              updated_at: "2026-03-26T10:08:00Z",
            },
          },
        ],
      },
      groupMemberStreamObserver: observer,
    });

    renderApp();

    const user = userEvent.setup();
    await screen.findByRole("heading", { name: "节点组" });
    await user.click(screen.getByRole("button", { name: /日本组/ }));

    await waitFor(() => {
      expect(observer.aborted_group_ids).toContain("group-1");
    });
    await waitFor(() => {
      expect(screen.getByText("63 ms")).toBeInTheDocument();
    });
    expect(screen.queryByText("84 ms")).not.toBeInTheDocument();
  });
});
