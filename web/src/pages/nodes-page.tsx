import { useDeferredValue, useEffect, useState } from "react";
import { Check, Download, LoaderCircle, Plus, Radar, SquarePen, Trash2, X } from "lucide-react";
import { toast } from "sonner";
import { AppShell, EmptyState } from "@/components/layout/app-shell";
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
import { api, type NodeView, type ProbeBatchResult } from "@/lib/api";
import { countAvailableNodes, formatDateTime, formatNodeStatus, nodeStatusTone } from "@/lib/format";
import { hasErrors, type NodeFormValues, validateNodeForm } from "@/lib/forms";
import { cn } from "@/lib/utils";
import { useAuthorizedRequest } from "@/hooks/use-authorized-request";
import { useShellMetrics } from "@/hooks/use-shell-metrics";

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

export function NodesPage() {
  const { run } = useAuthorizedRequest();
  const metrics = useShellMetrics();
  const [items, setItems] = useState<NodeView[]>([]);
  const [selected, setSelected] = useState<NodeView | null>(null);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [showImport, setShowImport] = useState(false);
  const [editing, setEditing] = useState<NodeView | null>(null);
  const [search, setSearch] = useState("");
  const deferredSearch = useDeferredValue(search);
  const [form, setForm] = useState<NodeFormValues>(defaultNodeForm);
  const [errors, setErrors] = useState<Partial<Record<keyof NodeFormValues, string>>>({});
  const [importPayload, setImportPayload] = useState("");
  const [probeResults, setProbeResults] = useState<Record<string, ProbeBatchResult>>({});
  const [probingNodeIDs, setProbingNodeIDs] = useState<Record<string, boolean>>({});
  const [viewMode, setViewMode] = usePersistedViewMode("simplepool.nodes.view_mode", "grid");
  const submitLabel = submitting ? "提交中..." : editing ? "保存修改" : "创建节点";
  const importLabel = submitting ? "导入中..." : "开始导入";

  function applyProbeResult(nodeID: string, result: { success: boolean; latency_ms?: number | null; checked_at?: string | null }) {
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
      const nextSelected = {
        ...current,
        last_status: nextStatus,
        last_latency_ms: result.success ? result.latency_ms ?? null : null,
        last_checked_at: result.checked_at ?? new Date().toISOString(),
      };
      return nextSelected;
    });
  }

  async function load() {
    setLoading(true);
    try {
      const data = await run((token) => api.nodes.list(token));
      setItems(data);
      setSelected((current) => (current ? data.find((item) => item.id === current.id) ?? null : null));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "节点列表加载失败");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  const keyword = deferredSearch.trim().toLowerCase();
  const filtered = !keyword
    ? items
    : items.filter((item) =>
        [item.name, item.protocol, item.server, item.source_kind].join(" ").toLowerCase().includes(keyword),
      );
  const availableCount = countAvailableNodes(items);
  const unavailableCount = items.filter((item) => !item.enabled || item.last_status === "unreachable").length;
  const enabledCount = items.filter((item) => item.enabled).length;

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

  async function submitForm() {
    const nextErrors = validateNodeForm(form);
    setErrors(nextErrors);
    if (hasErrors(nextErrors)) {
      return;
    }

    setSubmitting(true);
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
      setSubmitting(false);
    }
  }

  async function remove(item: NodeView) {
    if (!window.confirm(`确认删除节点 ${item.name}？`)) {
      return;
    }
    try {
      await run((token) => api.nodes.remove(token, item.id));
      setItems((current) => current.filter((currentItem) => currentItem.id !== item.id));
      setSelected((current) => (current?.id === item.id ? null : current));
      toast.success("节点已删除");
      await metrics.refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除失败");
    }
  }

  async function submitImport() {
    if (!importPayload.trim()) {
      toast.error("导入内容不能为空");
      return;
    }
    setSubmitting(true);
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
      setSubmitting(false);
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
    const candidates = items.filter((item) => item.enabled);
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
                  <IconButton label="删除" onClick={() => void remove(selected)} variant="danger">
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
                action={
                  <IconButton label="导入节点" onClick={() => setShowImport(true)} variant="secondary">
                    <Download className="h-4 w-4" />
                  </IconButton>
                }
                description="没有匹配节点, 请先导入节点或刷新订阅，再回来查看节点池"
                title="节点池为空"
              />
            ) : (
              <NodeCollectionView
                emptyMessage="暂无节点"
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
            <IconButton disabled={submitting} label={submitLabel} onClick={() => void submitForm()}>
              {submitting ? <LoaderCircle className="h-4 w-4 animate-spin" /> : editing ? <Check className="h-4 w-4" /> : <Plus className="h-4 w-4" />}
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
            <IconButton disabled={submitting} label={importLabel} onClick={() => void submitImport()}>
              {submitting ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
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
