import { useDeferredValue, useEffect, useState } from "react";
import { Check, LoaderCircle, Plus, Radar, RefreshCw, SquarePen, Trash2, X } from "lucide-react";
import { toast } from "sonner";
import { AppShell, EmptyState, PanelTitle } from "@/components/layout/app-shell";
import { NodeCollectionView, NodeViewModeSwitch } from "@/components/nodes/node-collection-view";
import { Badge } from "@/components/ui/badge";
import { IconButton } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Field, InlineFields } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { usePersistedViewMode } from "@/hooks/use-persisted-view-mode";
import { api, type NodeView, type ProbeBatchResult, type SubscriptionRefreshResult, type SubscriptionView } from "@/lib/api";
import { countAvailableNodes, formatDateTime } from "@/lib/format";
import { hasErrors, type SubscriptionFormValues, validateSubscriptionForm } from "@/lib/forms";
import { useAuthorizedRequest } from "@/hooks/use-authorized-request";
import { useShellMetrics } from "@/hooks/use-shell-metrics";

const defaultForm: SubscriptionFormValues = {
  name: "",
  url: "",
  enabled: true,
};

export function SubscriptionsPage() {
  const { run } = useAuthorizedRequest();
  const metrics = useShellMetrics();
  const [items, setItems] = useState<SubscriptionView[]>([]);
  const [nodes, setNodes] = useState<NodeView[]>([]);
  const [selected, setSelected] = useState<SubscriptionView | null>(null);
  const [search, setSearch] = useState("");
  const deferredSearch = useDeferredValue(search);
  const [loading, setLoading] = useState(true);
  const [nodeLoading, setNodeLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<SubscriptionView | null>(null);
  const [form, setForm] = useState<SubscriptionFormValues>(defaultForm);
  const [errors, setErrors] = useState<Partial<Record<keyof SubscriptionFormValues, string>>>({});
  const [submitting, setSubmitting] = useState(false);
  const [refreshResult, setRefreshResult] = useState<SubscriptionRefreshResult | null>(null);
  const [probeResults, setProbeResults] = useState<Record<string, ProbeBatchResult>>({});
  const [probingNodeIDs, setProbingNodeIDs] = useState<Record<string, boolean>>({});
  const [viewMode, setViewMode] = usePersistedViewMode("simplepool.subscriptions.nodes.view_mode", "grid");

  function applyProbeResult(nodeID: string, result: { success: boolean; latency_ms?: number | null; checked_at?: string | null }) {
    setNodes((current) =>
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
  }

  async function load() {
    setLoading(true);
    setNodeLoading(true);
    try {
      const [subscriptionItems, nodeItems] = await Promise.all([
        run((token) => api.subscriptions.list(token)),
        run((token) => api.nodes.list(token)),
      ]);
      setItems(subscriptionItems);
      setNodes(nodeItems);
      setSelected((current) => subscriptionItems.find((item) => item.id === current?.id) ?? subscriptionItems[0] ?? null);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "订阅列表加载失败");
    } finally {
      setLoading(false);
      setNodeLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  const keyword = deferredSearch.trim().toLowerCase();
  const filtered = !keyword
    ? items
    : items.filter((item) =>
        [item.name, item.last_error, item.enabled ? "已启用" : "已禁用", item.has_url ? "已加密" : "缺失"]
          .join(" ")
          .toLowerCase()
          .includes(keyword),
      );
  const subscriptionNodes = nodes.filter((item) => item.subscription_source_id === selected?.id);
  const availableNodeCount = countAvailableNodes(subscriptionNodes);
  const probeStates = Object.fromEntries(
    subscriptionNodes.map((item) => [
      item.id,
      {
        probing: Boolean(probingNodeIDs[item.id]),
        latencyMS: probeResults[item.id]?.latency_ms ?? item.last_latency_ms,
        success: probeResults[item.id]?.success,
      },
    ]),
  );
  const submitLabel = submitting ? "提交中..." : editing ? "保存修改" : "创建订阅源";

  function openCreate() {
    setEditing(null);
    setForm(defaultForm);
    setErrors({});
    setShowForm(true);
  }

  function openEdit(item: SubscriptionView) {
    setEditing(item);
    setForm({
      name: item.name,
      url: "",
      enabled: item.enabled,
    });
    setErrors({});
    setShowForm(true);
  }

  async function submit() {
    const nextErrors = validateSubscriptionForm(form);
    setErrors(nextErrors);
    if (hasErrors(nextErrors)) {
      return;
    }

    setSubmitting(true);
    try {
      if (editing) {
        const updated = await run((token) =>
          api.subscriptions.update(token, editing.id, {
            name: form.name.trim(),
            url: form.url.trim(),
            enabled: form.enabled,
          }),
        );
        setItems((current) => current.map((item) => (item.id === updated.id ? updated : item)));
        setSelected(updated);
        toast.success("订阅源已更新");
      } else {
        const created = await run((token) =>
          api.subscriptions.create(token, {
            name: form.name.trim(),
            url: form.url.trim(),
          }),
        );
        setItems((current) => [created, ...current]);
        setSelected(created);
        toast.success("订阅源已创建");
      }
      setShowForm(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "订阅源保存失败");
    } finally {
      setSubmitting(false);
    }
  }

  async function refresh(item: SubscriptionView) {
    try {
      const result = await run((token) => api.subscriptions.refresh(token, item.id, true));
      setRefreshResult(result);
      toast.success(`刷新完成，覆盖/新增 ${result.upserted_nodes.length} 个节点`);
      await load();
      await metrics.refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "刷新失败");
      await load();
    }
  }

  async function remove(item: SubscriptionView) {
    if (!window.confirm(`确认删除订阅源 ${item.name}？关联订阅节点会一起删除。`)) {
      return;
    }
    try {
      await run((token) => api.subscriptions.remove(token, item.id));
      setItems((current) => current.filter((currentItem) => currentItem.id !== item.id));
      setSelected((current) => (current?.id === item.id ? null : current));
      toast.success("订阅源已删除");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除失败");
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

  async function probeSubscriptionNodes() {
    const candidates = subscriptionNodes.filter((item) => item.enabled);
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

  return (
    <AppShell>
      <div className="grid gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
        <Card className="overflow-hidden">
          <CardHeader className="space-y-4">
            <PanelTitle eyebrow="Subscriptions" title="订阅源列表" />
            <Input onChange={(event) => setSearch(event.target.value)} placeholder="搜索名称或错误..." value={search} />
            <IconButton label="新建订阅源" onClick={openCreate}>
              <Plus className="h-4 w-4" />
            </IconButton>
          </CardHeader>
          <CardContent className="space-y-3">
            {loading ? (
              <div className="flex items-center justify-center py-16 text-[var(--muted-foreground)]">
                <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                加载订阅中...
              </div>
            ) : filtered.length === 0 ? (
              <EmptyState
                action={
                  <IconButton label="添加订阅源" onClick={openCreate}>
                    <Plus className="h-4 w-4" />
                  </IconButton>
                }
                description="新增订阅链接后，这里会显示刷新状态和失败信息"
                title="没有订阅源"
              />
            ) : (
              filtered.map((item) => (
                <button
                  className={`grid w-full cursor-pointer gap-3 rounded-[24px] border px-4 py-4 text-left transition-colors ${
                    selected?.id === item.id
                      ? "border-violet-400/35 bg-violet-500/12"
                      : "border-white/10 bg-white/5 hover:border-white/20 hover:bg-white/7"
                  }`}
                  key={item.id}
                  onClick={() => setSelected(item)}
                  type="button"
                >
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <p className="text-base font-medium text-white">{item.name}</p>
                      <p className="mt-1 text-xs uppercase tracking-[0.16em] text-[var(--muted-foreground)]">订阅源</p>
                    </div>
                    <Badge tone={subscriptionTone(item)}>{subscriptionState(item)}</Badge>
                  </div>
                  <div className="grid gap-1 text-sm text-[var(--muted-foreground)]">
                    <p>最近刷新 {formatDateTime(item.last_refresh_at)}</p>
                    <p>{item.last_error ? "存在错误，需处理" : "当前状态正常"}</p>
                  </div>
                </button>
              ))
            )}
          </CardContent>
        </Card>

        {selected ? (
          <Card>
            <CardHeader className="gap-6">
              <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_240px] xl:items-start">
                <div className="space-y-4">
                  <div className="flex flex-wrap items-center gap-3">
                    <CardTitle className="text-3xl">{selected.name}</CardTitle>
                    <Badge tone={subscriptionTone(selected)}>{subscriptionState(selected)}</Badge>
                  </div>
                  <div className="grid max-w-[520px] gap-3 sm:grid-cols-2">
                    <InfoBlock label="创建时间" value={formatDateTime(selected.created_at)} />
                    <InfoBlock label="更新时间" value={formatDateTime(selected.updated_at)} />
                  </div>
                </div>

                <div className="flex items-center gap-2 self-start xl:justify-end">
                  <IconButton
                    className="h-10 w-10 rounded-2xl p-0"
                    label="立即刷新"
                    onClick={() => void refresh(selected)}
                    variant="secondary"
                  >
                    <RefreshCw className="h-4 w-4" />
                  </IconButton>
                  <IconButton
                    className="h-10 w-10 rounded-2xl p-0"
                    label="编辑"
                    onClick={() => openEdit(selected)}
                    variant="secondary"
                  >
                    <SquarePen className="h-4 w-4" />
                  </IconButton>
                  <IconButton
                    className="h-10 w-10 rounded-2xl p-0"
                    label="删除"
                    onClick={() => void remove(selected)}
                    variant="danger"
                  >
                    <Trash2 className="h-4 w-4" />
                  </IconButton>
                </div>
              </div>
            </CardHeader>
            <CardContent className="pt-0">
              {selected.last_error ? (
                <div className="rounded-[24px] border border-rose-400/20 bg-rose-400/10 p-4">
                  <p className="text-sm font-medium text-white">订阅源刷新异常</p>
                  <p className="mt-2 text-sm leading-6 text-rose-100">{selected.last_error}</p>
                  {refreshResult && refreshResult.source_id === selected.id ? (
                    <p className="mt-3 text-sm leading-6 text-rose-50/90">
                      最近一次刷新结果：upserted_nodes {refreshResult.upserted_nodes.length}，deleted_count {refreshResult.deleted_count}
                    </p>
                  ) : null}
                </div>
              ) : null}
              <div className="mt-6 grid gap-4">
                <div className="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
                  <div className="space-y-2">
                    <h3 className="text-2xl font-semibold text-white">订阅节点</h3>
                    <p className="text-sm text-[var(--muted-foreground)]">
                      共 {subscriptionNodes.length} 个节点  可用 {availableNodeCount} 个
                    </p>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <NodeViewModeSwitch mode={viewMode} onChange={setViewMode} />
                    <IconButton label="探测" onClick={() => void probeSubscriptionNodes()} variant="secondary">
                      <Radar className="h-4 w-4" />
                    </IconButton>
                  </div>
                </div>

                {nodeLoading ? (
                  <div className="flex items-center justify-center py-16 text-[var(--muted-foreground)]">
                    <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                    加载订阅节点中...
                  </div>
                ) : (
                  <NodeCollectionView
                    emptyMessage="该订阅下暂无节点"
                    items={subscriptionNodes}
                    mode={viewMode}
                    onProbe={(item) => void probeSingle(item as NodeView)}
                    probeStates={probeStates}
                  />
                )}
              </div>
            </CardContent>
          </Card>
        ) : (
          <EmptyState title="还没有选中订阅源" description="左侧选择一个订阅源后，这里会展示刷新状态、错误信息和最近结果" />
        )}
      </div>

      <Dialog onOpenChange={setShowForm} open={showForm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editing ? "编辑订阅源" : "新建订阅源"}</DialogTitle>
            <DialogDescription>更新时需要重新提供 URL，后端只返回已加密存储状态。</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4">
            <InlineFields>
              <Field error={errors.name} label="名称">
                <Input onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} value={form.name} />
              </Field>
              <label className="flex items-center gap-3 rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-white">
                <input
                  checked={form.enabled}
                  onChange={(event) => setForm((current) => ({ ...current, enabled: event.target.checked }))}
                  type="checkbox"
                />
                启用订阅源
              </label>
            </InlineFields>
            <Field error={errors.url} label="订阅 URL">
              <Input onChange={(event) => setForm((current) => ({ ...current, url: event.target.value }))} placeholder="https://example.com/sub.txt" value={form.url} />
            </Field>
          </div>
          <DialogFooter>
            <IconButton label="取消" onClick={() => setShowForm(false)} variant="ghost">
              <X className="h-4 w-4" />
            </IconButton>
            <IconButton disabled={submitting} label={submitLabel} onClick={() => void submit()}>
              {submitting ? <LoaderCircle className="h-4 w-4 animate-spin" /> : editing ? <Check className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
            </IconButton>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </AppShell>
  );
}

function InfoBlock({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[24px] border border-white/10 bg-white/5 px-4 py-4">
      <p className="text-xs uppercase tracking-[0.16em] text-[var(--muted-foreground)]">{label}</p>
      <p className="mt-3 text-lg font-semibold text-white">{value}</p>
    </div>
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
