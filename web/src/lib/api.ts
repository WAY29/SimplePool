export type APIErrorPayload = {
  code?: string;
  message?: string;
};

export class APIError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.code = code;
  }
}

export type ReadyStatus = {
  status: string;
};

export type AdminUser = {
  id: string;
  username: string;
  created_at: string;
  updated_at: string;
};

export type SessionView = {
  id: string;
  expires_at: string;
  created_at: string;
  last_seen_at: string;
};

export type AuthSession = {
  token: string;
  expires_at: string;
  user: AdminUser;
  session: SessionView;
};

export type AuthSnapshot = {
  user: AdminUser;
  session: SessionView;
};

export type NodeView = {
  id: string;
  name: string;
  source_kind: string;
  subscription_source_id?: string | null;
  protocol: string;
  server: string;
  server_port: number;
  transport_json: string;
  tls_json: string;
  raw_payload_json: string;
  enabled: boolean;
  last_latency_ms?: number | null;
  last_status: string;
  last_checked_at?: string | null;
  has_credential: boolean;
  created_at: string;
  updated_at: string;
};

export type ProbeResult = {
  success: boolean;
  latency_ms?: number;
  test_url: string;
  error_message?: string;
  cached: boolean;
  checked_at?: string | null;
};

export type ProbeBatchResult = ProbeResult & {
  node_id: string;
};

export type SubscriptionView = {
  id: string;
  name: string;
  fetch_fingerprint: string;
  enabled: boolean;
  last_refresh_at?: string | null;
  last_error: string;
  has_url: boolean;
  created_at: string;
  updated_at: string;
};

export type SubscriptionRefreshResult = {
  source_id: string;
  upserted_nodes: NodeView[];
  deleted_count: number;
};

export type GroupView = {
  id: string;
  name: string;
  filter_regex: string;
  description: string;
  created_at: string;
  updated_at: string;
};

export type GroupMemberView = {
  id: string;
  name: string;
  source_kind: string;
  subscription_source_id?: string | null;
  protocol: string;
  server: string;
  server_port: number;
  enabled: boolean;
  last_latency_ms?: number | null;
  last_status: string;
  last_checked_at?: string | null;
  created_at: string;
  updated_at: string;
};

export type TunnelView = {
  id: string;
  name: string;
  group_id: string;
  listen_host: string;
  listen_port: number;
  status: string;
  current_node_id?: string | null;
  controller_port: number;
  runtime_dir: string;
  last_refresh_at?: string | null;
  last_refresh_error: string;
  has_auth: boolean;
  created_at: string;
  updated_at: string;
};

export type TunnelEventView = {
  id: string;
  tunnel_id: string;
  event_type: string;
  detail_json: string;
  created_at: string;
};

type RequestOptions = {
  method?: string;
  token?: string;
  body?: unknown;
  headers?: HeadersInit;
};

async function parseResponse<T>(response: Response): Promise<T> {
  if (response.status === 204) {
    return undefined as T;
  }

  const raw = await response.text();
  const payload = raw ? safeJSONParse(raw) : null;

  if (!response.ok) {
    const message =
      typeof payload === "object" &&
      payload !== null &&
      "message" in payload &&
      typeof payload.message === "string"
        ? payload.message
        : `请求失败 (${response.status})`;
    const code =
      typeof payload === "object" &&
      payload !== null &&
      "code" in payload &&
      typeof payload.code === "string"
        ? payload.code
        : "request_failed";
    throw new APIError(response.status, code, message);
  }

  return payload as T;
}

function safeJSONParse(raw: string): unknown {
  try {
    return JSON.parse(raw);
  } catch {
    return raw;
  }
}

async function request<T>(path: string, options: RequestOptions = {}) {
  const headers = new Headers(options.headers);
  let body: BodyInit | undefined;

  if (options.body !== undefined) {
    headers.set("Content-Type", "application/json");
    body = JSON.stringify(options.body);
  }

  if (options.token) {
    headers.set("Authorization", `Bearer ${options.token}`);
  }

  const response = await fetch(path, {
    method: options.method ?? "GET",
    headers,
    body,
  });

  return parseResponse<T>(response);
}

export const api = {
  ready() {
    return request<ReadyStatus>("/readyz");
  },
  auth: {
    login(input: { username: string; password: string }) {
      return request<AuthSession>("/api/auth/login", {
        method: "POST",
        body: input,
      });
    },
    me(token: string) {
      return request<AuthSnapshot>("/api/auth/me", { token });
    },
    logout(token: string) {
      return request<void>("/api/auth/logout", {
        method: "POST",
        token,
      });
    },
  },
  nodes: {
    list(token: string) {
      return request<NodeView[]>("/api/nodes", { token });
    },
    create(
      token: string,
      input: {
        name: string;
        protocol: string;
        server: string;
        server_port: number;
        transport_json: string;
        tls_json: string;
        raw_payload_json: string;
        credential: string;
      },
    ) {
      return request<NodeView>("/api/nodes", {
        method: "POST",
        token,
        body: input,
      });
    },
    update(
      token: string,
      id: string,
      input: {
        name: string;
        protocol: string;
        server: string;
        server_port: number;
        enabled: boolean;
        transport_json: string;
        tls_json: string;
        raw_payload_json: string;
        credential: string;
      },
    ) {
      return request<NodeView>(`/api/nodes/${id}`, {
        method: "PUT",
        token,
        body: input,
      });
    },
    remove(token: string, id: string) {
      return request<void>(`/api/nodes/${id}`, {
        method: "DELETE",
        token,
      });
    },
    import(token: string, payload: string) {
      return request<NodeView[]>("/api/nodes/import", {
        method: "POST",
        token,
        body: { payload },
      });
    },
    probe(token: string, id: string, force = true) {
      return request<ProbeResult>(`/api/nodes/${id}/probe`, {
        method: "POST",
        token,
        body: { force },
      });
    },
    probeBatch(token: string, ids: string[], force = true) {
      return request<ProbeBatchResult[]>("/api/nodes/probe", {
        method: "POST",
        token,
        body: { ids, force },
      });
    },
  },
  subscriptions: {
    list(token: string) {
      return request<SubscriptionView[]>("/api/subscriptions", { token });
    },
    create(token: string, input: { name: string; url: string }) {
      return request<SubscriptionView>("/api/subscriptions", {
        method: "POST",
        token,
        body: input,
      });
    },
    update(
      token: string,
      id: string,
      input: { name: string; url: string; enabled: boolean },
    ) {
      return request<SubscriptionView>(`/api/subscriptions/${id}`, {
        method: "PUT",
        token,
        body: input,
      });
    },
    remove(token: string, id: string) {
      return request<void>(`/api/subscriptions/${id}`, {
        method: "DELETE",
        token,
      });
    },
    refresh(token: string, id: string, force = true) {
      return request<SubscriptionRefreshResult>(`/api/subscriptions/${id}/refresh`, {
        method: "POST",
        token,
        body: { force },
      });
    },
  },
  groups: {
    list(token: string) {
      return request<GroupView[]>("/api/groups", { token });
    },
    create(
      token: string,
      input: { name: string; filter_regex: string; description: string },
    ) {
      return request<GroupView>("/api/groups", {
        method: "POST",
        token,
        body: input,
      });
    },
    update(
      token: string,
      id: string,
      input: { name: string; filter_regex: string; description: string },
    ) {
      return request<GroupView>(`/api/groups/${id}`, {
        method: "PUT",
        token,
        body: input,
      });
    },
    remove(token: string, id: string) {
      return request<void>(`/api/groups/${id}`, {
        method: "DELETE",
        token,
      });
    },
    members(token: string, id: string) {
      return request<GroupMemberView[]>(`/api/groups/${id}/members`, {
        token,
      });
    },
  },
  tunnels: {
    list(token: string) {
      return request<TunnelView[]>("/api/tunnels", { token });
    },
    get(token: string, id: string) {
      return request<TunnelView>(`/api/tunnels/${id}`, { token });
    },
    create(
      token: string,
      input: {
        name: string;
        group_id: string;
        listen_host: string;
        username: string;
        password: string;
      },
    ) {
      return request<TunnelView>("/api/tunnels", {
        method: "POST",
        token,
        body: input,
      });
    },
    update(
      token: string,
      id: string,
      input: {
        name: string;
        group_id: string;
        listen_host: string;
        username: string;
        password: string;
      },
    ) {
      return request<TunnelView>(`/api/tunnels/${id}`, {
        method: "PUT",
        token,
        body: input,
      });
    },
    remove(token: string, id: string) {
      return request<void>(`/api/tunnels/${id}`, {
        method: "DELETE",
        token,
      });
    },
    start(token: string, id: string) {
      return request<TunnelView>(`/api/tunnels/${id}/start`, {
        method: "POST",
        token,
      });
    },
    stop(token: string, id: string) {
      return request<TunnelView>(`/api/tunnels/${id}/stop`, {
        method: "POST",
        token,
      });
    },
    refresh(token: string, id: string) {
      return request<TunnelView>(`/api/tunnels/${id}/refresh`, {
        method: "POST",
        token,
      });
    },
    events(token: string, id: string, limit = 20) {
      return request<TunnelEventView[]>(`/api/tunnels/${id}/events?limit=${limit}`, {
        token,
      });
    },
  },
};
