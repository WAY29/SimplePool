import { useCallback, useDeferredValue, useEffect, useState } from "react";
import {
  Check,
  Download,
  Link2,
  LoaderCircle,
  Plus,
  Radar,
  RefreshCw,
  SquarePen,
  Trash2,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { AppShell, EmptyState } from "@/components/layout/app-shell";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";
import { Badge } from "@/components/ui/badge";
import { IconButton } from "@/components/ui/button";
import {
  NodeCollectionView,
  NodeViewModeSwitch,
  formatNodeSource,
  nodeMetricValue,
} from "@/components/nodes/node-collection-view";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Field, InlineFields } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { usePersistedViewMode } from "@/hooks/use-persisted-view-mode";
import { useAuthorizedRequest } from "@/hooks/use-authorized-request";
import { useGroupMemberStream } from "@/hooks/use-group-member-stream";
import { useShellMetrics } from "@/hooks/use-shell-metrics";
import {
  api,
  type GroupMemberView,
  type NodeView,
  type ProbeBatchResult,
  type SubscriptionView,
} from "@/lib/api";
import {
  countAvailableNodes,
  formatDateTime,
  formatNodeStatus,
  nodeStatusTone,
} from "@/lib/format";
import {
  hasErrors,
  type NodeFormValues,
  type SubscriptionFormValues,
  validateNodeForm,
  validateSubscriptionForm,
} from "@/lib/forms";
import { cn } from "@/lib/utils";

const ALL_SUBSCRIPTIONS = "ALL";

const defaultNodeForm: NodeFormValues = {
  name: "",
  protocol: "trojan",
  server: "",
  serverPort: "443",
  enabled: true,
  transportJSON: "{}",
  tlsJSON: "{}",
  rawPayloadJSON: "{}",
  credential: "{}",
};

const defaultSubscriptionForm: SubscriptionFormValues = {
  name: "",
  url: "",
  enabled: true,
};

type DeleteTarget =
  | { kind: "node"; item: NodeView }
  | { kind: "subscription"; item: SubscriptionView };

export function NodesPage() {
  const { run } = useAuthorizedRequest();
  const metrics = useShellMetrics();
  const [items, setItems] = useState<NodeView[]>([]);
  const [subscriptions, setSubscriptions] = useState<SubscriptionView[]>([]);
  const [groupIDs, setGroupIDs] = useState<string[]>([]);
  const [selected, setSelected] = useState<NodeView | null>(null);
  const [loading, setLoading] = useState(true);
  const [submittingNode, setSubmittingNode] = useState(false);
  const [submittingSubscription, setSubmittingSubscription] = useState(false);
  const [importing, setImporting] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [showImport, setShowImport] = useState(false);
  const [showSubscriptionForm, setShowSubscriptionForm] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);
  const [editing, setEditing] = useState<NodeView | null>(null);
  const [editingSubscription, setEditingSubscription] = useState<SubscriptionView | null>(null);
  const [search, setSearch] = useState("");
  const deferredSearch = useDeferredValue(search);
  const [form, setForm] = useState<NodeFormValues>(defaultNodeForm);
  const [errors, setErrors] = useState<Partial<Record<keyof NodeFormValues, string>>>({});
  const [subscriptionForm, setSubscriptionForm] = useState<SubscriptionFormValues>(defaultSubscriptionForm);
  const [subscriptionErrors, setSubscriptionErrors] = useState<Partial<Record<keyof SubscriptionFormValues, string>>>({});
  const [importPayload, setImportPayload] = useState("");
  const [probeResults, setProbeResults] = useState<Record<string, ProbeBatchResult>>({});
  const [probingNodeIDs, setProbingNodeIDs] = useState<Record<string, boolean>>({});
  const [refreshingSubscriptionIDs, setRefreshingSubscriptionIDs] = useState<Record<string, boolean>>({});
  const [deleting, setDeleting] = useState(false);
  const [viewMode, setViewMode] = usePersistedViewMode("simplepool.nodes.view_mode", "grid");
  const [subscriptionFilter, setSubscriptionFilter] = useState<string>(ALL_SUBSCRIPTIONS);
  const submitLabel = submittingNode ? "提交中..." : editing ? "保存修改" : "创建节点";
  const importLabel = importing ? "导入中..." : "开始导入";
  const subscriptionSubmitLabel = submittingSubscription ? "提交中..." : editingSubscription ? "保存订阅" : "创建订阅";

  function applyProbeResult(
    nodeID: string,
    result: { success: boolean; latency_ms?: number | null; checked_at?: string | null },
  ) {
    setItems((current) =>
      current.map((item) => {
        if (item.id !== nodeID) {
          return item;
        }
        const nextStatus = result.success ? "healthy" : "unreachable";
        const nextItem = {
          ...item,
          last_status: nextStatus,
          last_latency_ms: result.success ? result.latency_ms ?? null : null,
          last_checked_at: result.checked_at ?? new Date().toISOString(),
        };
        metrics.reconcileAvailableNode(item, nextItem);
        return nextItem;
      }),
    );
    setSelected((current) => {
      if (!current || current.id !== nodeID) {
        return current;
      }
      const nextStatus = result.success ? "healthy" : "unreachable";
      return {
        ...current,
        last_status: nextStatus,
        last_latency_ms: result.success ? result.latency_ms ?? null : null,
        last_checked_at: result.checked_at ?? new Date().toISOString(),
      };
    });
  }

  async function load() {
    setLoading(true);
    try {
      const [nodeItems, subscriptionItems, groupItems] = await Promise.all([
        run((token) => api.nodes.list(token)),
        run((token) => api.subscriptions.list(token)),
        run((token) => api.groups.list(token)),
      ]);
      setItems(nodeItems);
      setSubscriptions(subscriptionItems);
      setGroupIDs(groupItems.map((item) => item.id));
      setSelected((current) => (current ? nodeItems.find((item) => item.id === current.id) ?? null : null));
      setSubscriptionFilter((current) =>
        current === ALL_SUBSCRIPTIONS || subscriptionItems.some((item) => item.id === current) ? current : ALL_SUBSCRIPTIONS,
      );
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "节点列表加载失败");
    } finally {
      setLoading(false);
    }
  }

  const applyStreamNodeUpdate = useCallback((_groupID: string, member: GroupMemberView) => {
    setProbeResults((current) => {
      if (!current[member.id]) {
        return current;
      }
      const next = { ...current };
      delete next[member.id];
      return next;
    });
    setItems((current) => {
      const index = current.findIndex((item) => item.id === member.id);
      if (index < 0) {
        return current;
      }
      const previous = current[index];
      const nextItem = { ...previous, ...member };
      const next = [...current];
      next[index] = nextItem;
      metrics.reconcileAvailableNode(previous, nextItem);
      return next;
    });
    setSelected((current) => {
      if (!current || current.id !== member.id) {
        return current;
      }
      return { ...current, ...member };
    });
  }, [metrics]);

  useGroupMemberStream({
    groupIDs,
    onMemberUpdate: applyStreamNodeUpdate,
  });

  useEffect(() => {
    void load();
  }, []);

  const selectedSubscription =
    subscriptionFilter === ALL_SUBSCRIPTIONS
      ? null
      : subscriptions.find((item) => item.id === subscriptionFilter) ?? null;
  const scopedItems = selectedSubscription
    ? items.filter((item) => item.subscription_source_id === selectedSubscription.id)
    : items;
  const keyword = deferredSearch.trim().toLowerCase();
  const filtered = !keyword
    ? scopedItems
    : scopedItems.filter((item) =>
        [item.name, item.protocol, item.server, item.source_kind].join(" ").toLowerCase().includes(keyword),
      );
  const availableCount = countAvailableNodes(scopedItems);
  const unavailableCount = scopedItems.filter((item) => !item.enabled || item.last_status === "unreachable").length;
  const enabledCount = scopedItems.filter((item) => item.enabled).length;
  const manualCount = items.filter((item) => item.source_kind === "manual").length;
  const subscriptionNodeCount = items.length - manualCount;

  const probeStates = Object.fromEntries(
    items.map((item) => [
      item.id,
      {
        probing: Boolean(probingNodeIDs[item.id]),
        latencyMS: probeResults[item.id]?.latency_ms ?? item.last_latency_ms,
        success: probeResults[item.id]?.success,
      },
    ]),
  );

  useEffect(() => {
    if (!selectedSubscription) {
      return;
    }
    setSelected((current) => {
      if (!current) {
        return current;
      }
      return current.subscription_source_id === selectedSubscription.id ? current : null;
    });
  }, [selectedSubscription]);

  function openEdit(item: NodeView) {
    setEditing(item);
    setForm({
      name: item.name,
      protocol: item.protocol,
      server: item.server,
      serverPort: `${item.server_port}`,
      enabled: item.enabled,
      transportJSON: item.transport_json || "{}",
      tlsJSON: item.tls_json || "{}",
      rawPayloadJSON: item.raw_payload_json || "{}",
      credential: "",
    });
    setErrors({});
    setShowForm(true);
  }

  function openCreateSubscription() {
    setEditingSubscription(null);
    setSubscriptionForm(defaultSubscriptionForm);
    setSubscriptionErrors({});
    setShowSubscriptionForm(true);
  }

  function openEditSubscription(item: SubscriptionView) {
    setEditingSubscription(item);
    setSubscriptionForm({
      name: item.name,
      url: "",
      enabled: item.enabled,
    });
    setSubscriptionErrors({});
    setShowSubscriptionForm(true);
  }

  async function submitForm() {
    const nextErrors = validateNodeForm(form);
    setErrors(nextErrors);
    if (hasErrors(nextErrors)) {
      return;
    }

    setSubmittingNode(true);
    try {
      if (editing) {
        const updated = await run((token) =>
          api.nodes.update(token, editing.id, {
            name: form.name.trim(),
            protocol: form.protocol.trim(),
            server: form.server.trim(),
            server_port: Number(form.serverPort),
            enabled: form.enabled,
            transport_json: form.transportJSON,
            tls_json: form.tlsJSON,
            raw_payload_json: form.rawPayloadJSON,
            credential: form.credential,
          }),
        );
        setItems((current) => current.map((item) => (item.id === updated.id ? updated : item)));
        setSelected(updated);
        toast.success("节点已更新");
      } else {
        const created = await run((token) =>
          api.nodes.create(token, {
            name: form.name.trim(),
            protocol: form.protocol.trim(),
            server: form.server.trim(),
            server_port: Number(form.serverPort),
            transport_json: form.transportJSON,
            tls_json: form.tlsJSON,
            raw_payload_json: form.rawPayloadJSON,
            credential: form.credential,
          }),
        );
        setItems((current) => [created, ...current]);
        setSelected(created);
        toast.success("节点已创建");
      }
      setShowForm(false);
      await metrics.refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "节点保存失败");
    } finally {
      setSubmittingNode(false);
    }
  }

  async function submitSubscriptionForm() {
    const nextErrors = validateSubscriptionForm(subscriptionForm);
    setSubscriptionErrors(nextErrors);
    if (hasErrors(nextErrors)) {
      return;
    }

    setSubmittingSubscription(true);
    try {
      if (editingSubscription) {
        const updated = await run((token) =>
          api.subscriptions.update(token, editingSubscription.id, {
            name: subscriptionForm.name.trim(),
            url: subscriptionForm.url.trim(),
            enabled: subscriptionForm.enabled,
          }),
        );
        setSubscriptions((current) => current.map((item) => (item.id === updated.id ? updated : item)));
        toast.success("订阅已更新");
      } else {
        const created = await run((token) =>
          api.subscriptions.create(token, {
            name: subscriptionForm.name.trim(),
            url: subscriptionForm.url.trim(),
          }),
        );
        setSubscriptions((current) => [created, ...current]);
        setSubscriptionFilter(created.id);
        setShowSubscriptionForm(false);
        await load();
        await metrics.refresh();
        if (created.last_error) {
          toast.success("订阅已创建");
          toast.error(`首次刷新失败：${created.last_error}`);
          return;
        }
        toast.success("订阅已创建并已自动刷新");
        return;
      }
      setShowSubscriptionForm(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "订阅保存失败");
    } finally {
      setSubmittingSubscription(false);
    }
  }

  function requestRemoveNode(item: NodeView) {
    setDeleteTarget({ kind: "node", item });
  }

  function requestRemoveSubscription(item: SubscriptionView) {
    setDeleteTarget({ kind: "subscription", item });
  }

  async function confirmDelete() {
    const target = deleteTarget;
    if (!target) {
      return;
    }
    setDeleting(true);
    try {
      if (target.kind === "node") {
        await run((token) => api.nodes.remove(token, target.item.id));
        setItems((current) => current.filter((currentItem) => currentItem.id !== target.item.id));
        setSelected((current) => (current?.id === target.item.id ? null : current));
        toast.success("节点已删除");
        setDeleteTarget(null);
        await metrics.refresh();
        return;
      }

      await run((token) => api.subscriptions.remove(token, target.item.id));
      toast.success("订阅已删除");
      setDeleteTarget(null);
      await load();
      await metrics.refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除失败");
    } finally {
      setDeleting(false);
    }
  }

  async function submitImport() {
    if (!importPayload.trim()) {
      toast.error("导入内容不能为空");
      return;
    }
    setImporting(true);
    try {
      const imported = await run((token) => api.nodes.import(token, importPayload));
      setItems((current) => [...imported, ...current]);
      setSelected(imported[0] ?? null);
      setImportPayload("");
      setShowImport(false);
      toast.success(`导入完成，新增/覆盖 ${imported.length} 个节点`);
      await metrics.refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "节点导入失败");
    } finally {
      setImporting(false);
    }
  }

  async function refreshSubscription(item: SubscriptionView) {
    setRefreshingSubscriptionIDs((current) => ({ ...current, [item.id]: true }));
    try {
      const result = await run((token) => api.subscriptions.refresh(token, item.id, true));
      toast.success(`刷新完成，新增/覆盖 ${result.upserted_nodes.length} 个节点`);
      await load();
      await metrics.refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "订阅刷新失败");
      await load();
    } finally {
      setRefreshingSubscriptionIDs((current) => {
        const next = { ...current };
        delete next[item.id];
        return next;
      });
    }
  }

  async function probeSingle(item: NodeView) {
    setProbingNodeIDs((current) => ({ ...current, [item.id]: true }));
    try {
      const result = await run((token) => api.nodes.probe(token, item.id, true));
      setProbeResults((current) => ({
        ...current,
        [item.id]: {
          node_id: item.id,
          ...result,
        },
      }));
      applyProbeResult(item.id, result);
      toast.success(result.success ? `${item.name} 延迟 ${result.latency_ms} ms` : `${item.name} 探测失败`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "节点探测失败");
    } finally {
      setProbingNodeIDs((current) => {
        const next = { ...current };
        delete next[item.id];
        return next;
      });
    }
  }

  async function probeBatch() {
    const candidates = filtered.filter((item) => item.enabled);
    if (candidates.length === 0) {
      return;
    }
    setProbingNodeIDs((current) => ({
      ...current,
      ...Object.fromEntries(candidates.map((item) => [item.id, true])),
    }));
    await new Promise((resolve) => window.setTimeout(resolve, 0));
    const results: ProbeBatchResult[] = [];
    await Promise.all(
      candidates.map(async (item) => {
        try {
          const result = await run((token) => api.nodes.probe(token, item.id, true));
          const batchResult = {
            node_id: item.id,
            ...result,
          };
          results.push(batchResult);
          setProbeResults((current) => ({
            ...current,
            [item.id]: batchResult,
          }));
          applyProbeResult(item.id, batchResult);
        } catch (error) {
          const failedResult = {
            node_id: item.id,
            success: false,
            test_url: "",
            error_message: error instanceof Error ? error.message : "节点探测失败",
            cached: false,
          };
          setProbeResults((current) => ({
            ...current,
            [item.id]: failedResult,
          }));
          applyProbeResult(item.id, failedResult);
        } finally {
          setProbingNodeIDs((current) => {
            const next = { ...current };
            delete next[item.id];
            return next;
          });
        }
      }),
    );
    toast.success(`批量探测完成，共 ${results.length} 个节点`);
  }

  const emptyAction = selectedSubscription ? undefined : (
    <div className="flex items-center justify-center gap-2">
      <IconButton label="添加订阅" onClick={openCreateSubscription} variant="secondary">
        <Link2 className="h-4 w-4" />
      </IconButton>
      <IconButton label="导入节点" onClick={() => setShowImport(true)} variant="secondary">
        <Download className="h-4 w-4" />
      </IconButton>
    </div>
  );

  return (
    <AppShell>
      <div className="grid gap-4">
        <section className="overflow-hidden border border-white/10 bg-[linear-gradient(180deg,rgba(12,18,31,0.96),rgba(9,14,24,0.98))] px-5 py-5 shadow-[0_24px_70px_rgba(2,8,20,0.28)] sm:px-6 lg:px-7 lg:py-6">
          <div className="flex flex-col gap-5 xl:flex-row xl:items-start xl:justify-between">
            <div className="space-y-4">
              <div className="space-y-2">
                <p className="text-xs uppercase tracking-[0.3em] text-white/35">Node Pool</p>
                <h1 className="text-4xl font-semibold tracking-tight text-white">节点池</h1>
              </div>
              <div className="flex flex-wrap items-center gap-x-8 gap-y-2 text-sm font-medium">
                <InlineStat label="可用节点" tone="text-emerald-300" value={`${availableCount}`} />
                <InlineStat label="不可用节点" tone="text-rose-300" value={`${unavailableCount}`} />
                <InlineStat label="已启用节点" tone="text-amber-200" value={`${enabledCount}`} />
              </div>
            </div>

            <div className="flex w-full max-w-[760px] flex-col gap-3 xl:items-end">
              <div className="flex flex-wrap items-center gap-2 xl:justify-end">
                <NodeViewModeSwitch mode={viewMode} onChange={setViewMode} />
                <IconButton
                  className="border-white/10 bg-white/5"
                  label="添加订阅"
                  onClick={openCreateSubscription}
                  variant="secondary"
                >
                  <Link2 className="h-4 w-4" />
                </IconButton>
                <IconButton
                  className="border-white/10 bg-white/5"
                  label="导入节点"
                  onClick={() => setShowImport(true)}
                  variant="secondary"
                >
                  <Download className="h-4 w-4" />
                </IconButton>
                <IconButton
                  className="border-white/10 bg-white/5"
                  label="探测"
                  onClick={() => void probeBatch()}
                  variant="secondary"
                >
                  <Radar className="h-4 w-4" />
                </IconButton>
              </div>

              <div className="w-full max-w-[360px]">
                <Input
                  className="h-12 bg-[rgba(6,11,21,0.82)]"
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder="筛选名称、协议、来源或地址..."
                  value={search}
                />
              </div>
            </div>
          </div>

          <div className="mt-6 grid gap-4">
            <div className="flex flex-col gap-3">
              <div className="flex flex-wrap items-center gap-3">
                <SubscriptionFilterTag
                  active={subscriptionFilter === ALL_SUBSCRIPTIONS}
                  label="ALL"
                  onSelect={() => setSubscriptionFilter(ALL_SUBSCRIPTIONS)}
                  title="显示所有节点"
                />
                {subscriptions.map((item) => (
                  <SubscriptionFilterTag
                    active={subscriptionFilter === item.id}
                    key={item.id}
                    label={item.name}
                    onSelect={() => setSubscriptionFilter(item.id)}
                    title={subscriptionTooltip(item)}
                  />
                ))}
              </div>

              <div className="flex flex-col gap-3 border border-white/10 bg-white/4 px-4 py-4 sm:flex-row sm:items-center sm:justify-between">
                <div className="space-y-1">
                  <div className="flex flex-wrap items-center gap-3">
                    <p className="text-sm font-semibold text-white">
                      {selectedSubscription ? `当前订阅: ${selectedSubscription.name}` : "当前筛选: ALL"}
                    </p>
                    {selectedSubscription ? (
                      <Badge tone={subscriptionTone(selectedSubscription)}>{subscriptionState(selectedSubscription)}</Badge>
                    ) : (
                      <Badge tone="info">ALL</Badge>
                    )}
                  </div>
                  <p className="text-sm text-[var(--muted-foreground)]">
                    {selectedSubscription
                      ? selectedSubscription.last_error
                        ? `${selectedSubscription.last_error} · 节点 ${scopedItems.length} 个`
                        : `最近刷新 ${formatDateTime(selectedSubscription.last_refresh_at)} · 节点 ${scopedItems.length} 个`
                      : `显示全部节点，包含手动节点 ${manualCount} 个，订阅节点 ${subscriptionNodeCount} 个`}
                  </p>
                </div>

                {selectedSubscription ? (
                  <div className="flex flex-wrap gap-2 sm:justify-end">
                    <IconButton
                      label="刷新订阅"
                      onClick={() => void refreshSubscription(selectedSubscription)}
                      variant="secondary"
                    >
                      {refreshingSubscriptionIDs[selectedSubscription.id] ? (
                        <LoaderCircle className="h-4 w-4 animate-spin" />
                      ) : (
                        <RefreshCw className="h-4 w-4" />
                      )}
                    </IconButton>
                    <IconButton
                      label="编辑订阅"
                      onClick={() => openEditSubscription(selectedSubscription)}
                      variant="secondary"
                    >
                      <SquarePen className="h-4 w-4" />
                    </IconButton>
                    <IconButton
                      label="删除订阅"
                      onClick={() => requestRemoveSubscription(selectedSubscription)}
                      variant="danger"
                    >
                      <Trash2 className="h-4 w-4" />
                    </IconButton>
                  </div>
                ) : null}
              </div>
            </div>
          </div>

          {selected ? (
            <div className="mt-5 border border-violet-400/18 bg-[linear-gradient(180deg,rgba(35,33,67,0.9),rgba(24,29,54,0.92))] px-5 py-4">
              <div className="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
                <div className="space-y-3">
                  <div className="flex flex-wrap items-center gap-3">
                    <h2 className="text-2xl font-semibold text-white">{selected.name}</h2>
                    <Badge tone={nodeStatusTone(selected.last_status)}>{formatNodeStatus(selected.last_status)}</Badge>
                    {!selected.enabled ? <Badge tone="muted">已禁用</Badge> : null}
                  </div>

                  <div className="flex flex-wrap gap-3 text-sm">
                    <DetailChip label="协议" value={selected.protocol.toUpperCase()} />
                    <DetailChip label="来源" value={formatNodeSource(selected.source_kind)} />
                    <DetailChip label="延迟" value={nodeMetricValue(selected)} />
                    <DetailChip label="最近探测" value={formatDateTime(selected.last_checked_at)} />
                  </div>

                  {probeResults[selected.id] ? (
                    <p className="text-sm text-[var(--muted-foreground)]">
                      最近一次前端触发探测：
                      {probeResults[selected.id].success
                        ? `成功，延迟 ${probeResults[selected.id].latency_ms} ms`
                        : probeResults[selected.id].error_message || "探测失败"}
                    </p>
                  ) : null}
                </div>

                <div className="flex flex-wrap gap-2 xl:justify-end">
                  <IconButton label="立即探测" onClick={() => void probeSingle(selected)}>
                    <Radar className="h-4 w-4" />
                  </IconButton>
                  <IconButton label="编辑节点" onClick={() => openEdit(selected)} variant="secondary">
                    <SquarePen className="h-4 w-4" />
                  </IconButton>
                  <IconButton label="删除" onClick={() => requestRemoveNode(selected)} variant="danger">
                    <Trash2 className="h-4 w-4" />
                  </IconButton>
                </div>
              </div>
            </div>
          ) : null}

          <div className="mt-6">
            {loading ? (
              <div className="flex items-center justify-center py-16 text-[var(--muted-foreground)]">
                <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                加载节点中...
              </div>
            ) : filtered.length === 0 ? (
              <EmptyState
                action={emptyAction}
                description={
                  selectedSubscription
                    ? "当前订阅下没有节点，刷新订阅或切回 ALL 查看全部节点"
                    : "没有匹配节点，请导入节点、刷新订阅，或调整当前筛选"
                }
                title={selectedSubscription ? "该订阅下暂无节点" : "节点池为空"}
              />
            ) : (
              <NodeCollectionView
                emptyMessage={selectedSubscription ? "该订阅下暂无节点" : "暂无节点"}
                items={filtered}
                mode={viewMode}
                onProbe={(item) => void probeSingle(item as NodeView)}
                onSelect={(item) => setSelected(item as NodeView)}
                probeStates={probeStates}
                selectedId={selected?.id}
              />
            )}
          </div>
        </section>
      </div>

      <DeleteConfirmDialog
        busy={deleting}
        description={
          deleteTarget?.kind === "subscription"
            ? "关联订阅节点会一起删除。"
            : deleteTarget
              ? `节点 ${deleteTarget.item.name} 删除后无法恢复。`
              : ""
        }
        onConfirm={() => void confirmDelete()}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteTarget(null);
          }
        }}
        open={Boolean(deleteTarget)}
        title={deleteTarget?.kind === "subscription" ? "确认删除订阅" : "确认删除节点"}
      />

      <Dialog onOpenChange={setShowForm} open={showForm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editing ? "编辑节点" : "新建节点"}</DialogTitle>
            <DialogDescription>字段与后端 API 一致，敏感认证信息只在提交时下发。</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4">
            <InlineFields>
              <Field error={errors.name} label="节点名称">
                <Input onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} value={form.name} />
              </Field>
              <Field error={errors.protocol} label="协议">
                <Input onChange={(event) => setForm((current) => ({ ...current, protocol: event.target.value }))} value={form.protocol} />
              </Field>
            </InlineFields>
            <InlineFields>
              <Field error={errors.server} label="服务器地址">
                <Input onChange={(event) => setForm((current) => ({ ...current, server: event.target.value }))} value={form.server} />
              </Field>
              <Field error={errors.serverPort} label="端口">
                <Input onChange={(event) => setForm((current) => ({ ...current, serverPort: event.target.value }))} value={form.serverPort} />
              </Field>
            </InlineFields>
            <Field error={errors.credential} hint="例如 trojan/password、vmess uuid 等 JSON 字符串" label="认证信息">
              <Textarea onChange={(event) => setForm((current) => ({ ...current, credential: event.target.value }))} value={form.credential} />
            </Field>
            {editing ? (
              <div className="border border-amber-400/20 bg-amber-400/10 px-4 py-3 text-sm text-amber-100">
                编辑节点时后端不会回传原始认证信息。保存前必须重新填写 credential。
              </div>
            ) : null}
            <Field error={errors.transportJSON} label="Transport JSON">
              <Textarea onChange={(event) => setForm((current) => ({ ...current, transportJSON: event.target.value }))} value={form.transportJSON} />
            </Field>
            <Field error={errors.tlsJSON} label="TLS JSON">
              <Textarea onChange={(event) => setForm((current) => ({ ...current, tlsJSON: event.target.value }))} value={form.tlsJSON} />
            </Field>
            <Field error={errors.rawPayloadJSON} label="Raw Payload JSON">
              <Textarea onChange={(event) => setForm((current) => ({ ...current, rawPayloadJSON: event.target.value }))} value={form.rawPayloadJSON} />
            </Field>
            {editing ? (
              <label className="flex items-center gap-3 border border-white/10 bg-white/5 px-4 py-3 text-sm text-white">
                <input
                  checked={form.enabled}
                  onChange={(event) => setForm((current) => ({ ...current, enabled: event.target.checked }))}
                  type="checkbox"
                />
                启用节点
              </label>
            ) : null}
          </div>
          <DialogFooter>
            <IconButton label="取消" onClick={() => setShowForm(false)} variant="ghost">
              <X className="h-4 w-4" />
            </IconButton>
            <IconButton disabled={submittingNode} label={submitLabel} onClick={() => void submitForm()}>
              {submittingNode ? <LoaderCircle className="h-4 w-4 animate-spin" /> : editing ? <Check className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
            </IconButton>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog onOpenChange={setShowSubscriptionForm} open={showSubscriptionForm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editingSubscription ? "编辑订阅" : "添加订阅"}</DialogTitle>
            <DialogDescription>更新订阅时需要重新提供 URL，后端仅返回加密存储状态。</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4">
            <Field error={subscriptionErrors.name} label="名称">
              <Input
                onChange={(event) =>
                  setSubscriptionForm((current) => ({ ...current, name: event.target.value }))
                }
                value={subscriptionForm.name}
              />
            </Field>
            <Field error={subscriptionErrors.url} label="订阅 URL">
              <Input
                onChange={(event) =>
                  setSubscriptionForm((current) => ({ ...current, url: event.target.value }))
                }
                placeholder="https://example.com/sub.txt"
                value={subscriptionForm.url}
              />
            </Field>
          </div>
          <DialogFooter>
            <IconButton label="取消" onClick={() => setShowSubscriptionForm(false)} variant="ghost">
              <X className="h-4 w-4" />
            </IconButton>
            <IconButton
              disabled={submittingSubscription}
              label={subscriptionSubmitLabel}
              onClick={() => void submitSubscriptionForm()}
            >
              {submittingSubscription ? (
                <LoaderCircle className="h-4 w-4 animate-spin" />
              ) : editingSubscription ? (
                <Check className="h-4 w-4" />
              ) : (
                <Plus className="h-4 w-4" />
              )}
            </IconButton>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog onOpenChange={setShowImport} open={showImport}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>导入节点</DialogTitle>
            <DialogDescription>支持单条 URI、多行 URI 或常见订阅解码文本。</DialogDescription>
          </DialogHeader>
          <Field label="导入内容">
            <Textarea onChange={(event) => setImportPayload(event.target.value)} value={importPayload} />
          </Field>
          <DialogFooter>
            <IconButton label="取消" onClick={() => setShowImport(false)} variant="ghost">
              <X className="h-4 w-4" />
            </IconButton>
            <IconButton disabled={importing} label={importLabel} onClick={() => void submitImport()}>
              {importing ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
            </IconButton>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </AppShell>
  );
}

function InlineStat({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone: string;
}) {
  return (
    <div className="flex items-center gap-2">
      <span className={cn("text-base font-semibold", tone)}>{label}:</span>
      <span className={cn("text-lg font-semibold", tone)}>{value}</span>
    </div>
  );
}

function DetailChip({ label, value }: { label: string; value: string }) {
  return (
    <div className="border border-white/10 bg-white/6 px-3 py-2 text-sm text-[var(--muted-foreground)]">
      <span className="text-white/55">{label}:</span> <span className="font-medium text-white">{value}</span>
    </div>
  );
}

function SubscriptionFilterTag({
  active,
  label,
  onSelect,
  title,
}: {
  active: boolean;
  label: string;
  onSelect: () => void;
  title: string;
}) {
  const toneClass =
    active
      ? "border-violet-400/35 bg-violet-500/12 text-white"
      : "border-white/10 bg-white/6 text-[var(--muted-foreground)] hover:text-white";

  return (
    <button
      aria-label={`筛选 ${label}`}
      className={cn("inline-flex min-w-0 items-center rounded-full border px-4 py-2 text-sm font-medium", toneClass)}
      onClick={onSelect}
      title={title}
      type="button"
    >
      <span className="block max-w-[180px] truncate">{label}</span>
    </button>
  );
}

function subscriptionTone(item: SubscriptionView): "success" | "danger" | "muted" {
  if (item.last_error) {
    return "danger";
  }
  if (!item.enabled) {
    return "muted";
  }
  return "success";
}

function subscriptionState(item: SubscriptionView) {
  if (item.last_error) {
    return "异常";
  }
  if (!item.enabled) {
    return "已禁用";
  }
  return "可刷新";
}

function subscriptionTooltip(item: SubscriptionView) {
  return `创建时间: ${formatDateTime(item.created_at)}\n更新时间: ${formatDateTime(item.updated_at)}`;
}
