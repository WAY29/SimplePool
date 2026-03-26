import { useDeferredValue, useEffect, useState } from "react";
import { Cable, Download, LoaderCircle, Plus, Radar, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { AppShell, EmptyState, PanelTitle } from "@/components/layout/app-shell";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  NodeCollectionView,
  NodeViewModeSwitch,
  formatNodeSource,
  nodeMetricValue,
} from "@/components/nodes/node-collection-view";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
import { formatDateTime, formatLatency, formatNodeStatus, nodeStatusTone } from "@/lib/format";
import { hasErrors, type NodeFormValues, validateNodeForm } from "@/lib/forms";
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
  const [viewMode, setViewMode] = usePersistedViewMode("simplepool.nodes.view_mode", "grid");

  async function load() {
    setLoading(true);
    try {
      const data = await run((token) => api.nodes.list(token));
      setItems(data);
      setSelected((current) => data.find((item) => item.id === current?.id) ?? data[0] ?? null);
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
  const healthyCount = items.filter((item) => item.enabled && item.last_status === "healthy").length;
  const unreachableCount = items.filter((item) => item.last_status === "unreachable").length;
  const enabledCount = items.filter((item) => item.enabled).length;

  function openCreate() {
    setEditing(null);
    setForm(defaultNodeForm);
    setErrors({});
    setShowForm(true);
  }

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
    try {
      const result = await run((token) => api.nodes.probe(token, item.id, true));
      setProbeResults((current) => ({
        ...current,
        [item.id]: {
          node_id: item.id,
          ...result,
        },
      }));
      toast.success(result.success ? `${item.name} 延迟 ${result.latency_ms} ms` : `${item.name} 探测失败`);
      await load();
      await metrics.refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "节点探测失败");
    }
  }

  async function probeBatch() {
    if (items.length === 0) {
      return;
    }
    try {
      const result = await run((token) => api.nodes.probeBatch(token, items.map((item) => item.id), true));
      setProbeResults(Object.fromEntries(result.map((item) => [item.node_id, item])));
      toast.success(`批量探测完成，共 ${result.length} 个节点`);
      await load();
      await metrics.refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "批量探测失败");
    }
  }

  return (
    <AppShell>
      <div className="grid gap-4">
        <Card className="overflow-hidden">
          <CardHeader className="gap-5">
            <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
              <div className="space-y-4">
                <PanelTitle eyebrow="Nodes" title="节点池"/>
                <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                  <MetaCard label="节点总数" value={`${items.length}`} />
                  <MetaCard label="可用节点" value={`${healthyCount}`} />
                  <MetaCard label="不可达" value={`${unreachableCount}`} />
                  <MetaCard label="已启用" value={`${enabledCount}`} />
                </div>
              </div>

              <div className="grid gap-3 xl:min-w-[340px]">
                <Input onChange={(event) => setSearch(event.target.value)} placeholder="搜索名称、协议、来源..." value={search} />
                <div className="grid gap-2 sm:grid-cols-3">
                  <Button onClick={openCreate}>
                    <Plus className="h-4 w-4" />
                    新建节点
                  </Button>
                  <Button onClick={() => setShowImport(true)} variant="secondary">
                    <Download className="h-4 w-4" />
                    导入节点
                  </Button>
                  <Button onClick={() => void probeBatch()} variant="secondary">
                    <Radar className="h-4 w-4" />
                    批量探测
                  </Button>
                </div>
              </div>
            </div>
          </CardHeader>
        </Card>

        {selected ? (
          <Card className="overflow-hidden">
            <CardHeader className="gap-5">
              <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_auto] xl:items-start">
                <div className="space-y-4">
                  <div className="flex flex-wrap items-center gap-3">
                    <CardTitle className="text-3xl">{selected.name}</CardTitle>
                    <Badge tone={nodeStatusTone(selected.last_status)}>{formatNodeStatus(selected.last_status)}</Badge>
                    {!selected.enabled ? <Badge tone="muted">已禁用</Badge> : null}
                  </div>
                  <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                    <MetaCard label="协议" value={selected.protocol.toUpperCase()} />
                    <MetaCard label="来源" value={formatNodeSource(selected.source_kind)} />
                    <MetaCard label="最近延迟" value={nodeMetricValue(selected)} />
                    <MetaCard label="最近探测" value={formatDateTime(selected.last_checked_at)} />
                  </div>
                </div>

                <div className="grid gap-3 xl:justify-items-end">
                  <Button onClick={() => void probeSingle(selected)} size="lg">
                    <Radar className="h-4 w-4" />
                    立即探测
                  </Button>
                  <div className="flex flex-wrap gap-2 xl:justify-end">
                    <Button onClick={() => openEdit(selected)} variant="secondary">
                      <Cable className="h-4 w-4" />
                      编辑节点
                    </Button>
                    <Button onClick={() => void remove(selected)} variant="danger">
                      <Trash2 className="h-4 w-4" />
                      删除
                    </Button>
                  </div>
                </div>
              </div>
            </CardHeader>
            {probeResults[selected.id] ? (
              <CardContent className="pt-0">
                <div className="rounded-[24px] border border-emerald-400/20 bg-emerald-400/10 p-4">
                  <p className="text-sm font-medium text-white">最近一次前端触发探测</p>
                  <p className="mt-2 text-sm text-[var(--muted-foreground)]">
                    {probeResults[selected.id].success
                      ? `成功，延迟 ${probeResults[selected.id].latency_ms} ms`
                      : probeResults[selected.id].error_message || "探测失败"}
                  </p>
                </div>
              </CardContent>
            ) : null}
          </Card>
        ) : null}

        <Card className="overflow-hidden">
          <CardHeader className="pb-0">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-3">
                <CardTitle className="text-2xl">节点列表</CardTitle>
                <span className="inline-flex h-8 min-w-8 items-center justify-center rounded-full border border-white/10 bg-white/5 px-3 text-sm text-white">
                  {filtered.length}
                </span>
              </div>
              <NodeViewModeSwitch mode={viewMode} onChange={setViewMode} />
            </div>
          </CardHeader>
          <CardContent className="pt-4">
            {loading ? (
              <div className="flex items-center justify-center py-16 text-[var(--muted-foreground)]">
                <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                加载节点中...
              </div>
            ) : filtered.length === 0 ? (
              <EmptyState
                title="没有匹配节点"
                description="先手动新增节点，或通过导入/订阅刷新让列表变得有意义。"
                action={<Button onClick={openCreate}>立即新建</Button>}
              />
            ) : (
              <NodeCollectionView
                emptyMessage="暂无节点"
                items={filtered}
                mode={viewMode}
                onSelect={(item) => setSelected(item as NodeView)}
                selectedId={selected?.id}
              />
            )}
          </CardContent>
        </Card>
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
            <Field error={errors.credential} hint="例如 trojan/password、vmess uuid 等 JSON 字符串。" label="认证信息">
              <Textarea onChange={(event) => setForm((current) => ({ ...current, credential: event.target.value }))} value={form.credential} />
            </Field>
            {editing ? (
              <div className="rounded-2xl border border-amber-400/20 bg-amber-400/10 px-4 py-3 text-sm text-amber-100">
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
              <label className="flex items-center gap-3 rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-white">
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
            <Button onClick={() => setShowForm(false)} variant="ghost">
              取消
            </Button>
            <Button disabled={submitting} onClick={() => void submitForm()}>
              {submitting ? "提交中..." : editing ? "保存修改" : "创建节点"}
            </Button>
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
            <Button onClick={() => setShowImport(false)} variant="ghost">
              取消
            </Button>
            <Button disabled={submitting} onClick={() => void submitImport()}>
              {submitting ? "导入中..." : "开始导入"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </AppShell>
  );
}

function MetaCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[24px] border border-white/10 bg-white/5 px-4 py-4">
      <p className="text-xs uppercase tracking-[0.16em] text-[var(--muted-foreground)]">{label}</p>
      <p className="mt-3 text-lg font-semibold text-white">{value}</p>
    </div>
  );
}
