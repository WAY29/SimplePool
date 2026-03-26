import { useDeferredValue, useEffect, useState } from "react";
import { LoaderCircle, Plus, RefreshCw, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { AppShell, EmptyState, PanelTitle } from "@/components/layout/app-shell";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Field, InlineFields } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { api, type SubscriptionRefreshResult, type SubscriptionView } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { hasErrors, type SubscriptionFormValues, validateSubscriptionForm } from "@/lib/forms";
import { useAuthorizedRequest } from "@/hooks/use-authorized-request";

const defaultForm: SubscriptionFormValues = {
  name: "",
  url: "",
  enabled: true,
};

export function SubscriptionsPage() {
  const { run } = useAuthorizedRequest();
  const [items, setItems] = useState<SubscriptionView[]>([]);
  const [selected, setSelected] = useState<SubscriptionView | null>(null);
  const [search, setSearch] = useState("");
  const deferredSearch = useDeferredValue(search);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<SubscriptionView | null>(null);
  const [form, setForm] = useState<SubscriptionFormValues>(defaultForm);
  const [errors, setErrors] = useState<Partial<Record<keyof SubscriptionFormValues, string>>>({});
  const [submitting, setSubmitting] = useState(false);
  const [refreshResult, setRefreshResult] = useState<SubscriptionRefreshResult | null>(null);

  async function load() {
    setLoading(true);
    try {
      const data = await run((token) => api.subscriptions.list(token));
      setItems(data);
      setSelected((current) => data.find((item) => item.id === current?.id) ?? data[0] ?? null);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "订阅列表加载失败");
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
        [item.name, item.last_error, item.enabled ? "已启用" : "已禁用", item.has_url ? "已加密" : "缺失"]
          .join(" ")
          .toLowerCase()
          .includes(keyword),
      );

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

  return (
    <AppShell>
      <div className="grid gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
        <Card className="overflow-hidden">
          <CardHeader className="space-y-4">
            <PanelTitle eyebrow="Subscriptions" title="订阅源列表" />
            <Input onChange={(event) => setSearch(event.target.value)} placeholder="搜索名称或错误..." value={search} />
            <Button onClick={openCreate}>
              <Plus className="h-4 w-4" />
              新建订阅源
            </Button>
          </CardHeader>
          <CardContent className="space-y-3">
            {loading ? (
              <div className="flex items-center justify-center py-16 text-[var(--muted-foreground)]">
                <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                加载订阅中...
              </div>
            ) : filtered.length === 0 ? (
              <EmptyState title="没有订阅源" description="新增订阅链接后，这里会显示刷新状态和失败信息。" action={<Button onClick={openCreate}>添加订阅源</Button>} />
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
                  <p className="text-sm leading-6 text-[var(--muted-foreground)]">
                    已移除 URL 存储卡片、字段表和默认状态块。这里只保留时间信息与主操作。
                  </p>
                  <div className="grid max-w-[520px] gap-3 sm:grid-cols-2">
                    <InfoBlock label="创建时间" value={formatDateTime(selected.created_at)} />
                    <InfoBlock label="更新时间" value={formatDateTime(selected.updated_at)} />
                  </div>
                </div>

                <div className="grid gap-3 xl:justify-items-end">
                  <Button onClick={() => void refresh(selected)} size="lg">
                    <RefreshCw className="h-4 w-4" />
                    立即刷新
                  </Button>
                  <div className="flex flex-wrap gap-2 xl:justify-end">
                    <Button onClick={() => openEdit(selected)} variant="secondary">
                      编辑
                    </Button>
                    <Button onClick={() => void remove(selected)} variant="danger">
                      <Trash2 className="h-4 w-4" />
                      删除
                    </Button>
                  </div>
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
            </CardContent>
          </Card>
        ) : (
          <EmptyState title="还没有选中订阅源" description="左侧选择一个订阅源后，这里会展示刷新状态、错误信息和最近结果。" />
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
            <Button onClick={() => setShowForm(false)} variant="ghost">
              取消
            </Button>
            <Button disabled={submitting} onClick={() => void submit()}>
              {submitting ? "提交中..." : editing ? "保存修改" : "创建订阅源"}
            </Button>
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
