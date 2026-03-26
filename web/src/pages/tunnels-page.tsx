import { useEffect, useState } from "react";
import { Cable, LoaderCircle, Play, Plus, RefreshCw, Square, Trash2 } from "lucide-react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { toast } from "sonner";
import { AppShell, EmptyState, PanelTitle } from "@/components/layout/app-shell";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Field, InlineFields } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableElement, TableHead, TableHeaderCell, TableRow } from "@/components/ui/table";
import { api, type GroupView, type NodeView, type TunnelEventView, type TunnelView } from "@/lib/api";
import { formatDateTime, formatTunnelStatus, parseEventDetail, tunnelStatusTone } from "@/lib/format";
import { hasErrors, type TunnelFormValues, validateTunnelForm } from "@/lib/forms";
import { useAuthorizedRequest } from "@/hooks/use-authorized-request";
import { useShellMetrics } from "@/hooks/use-shell-metrics";

const defaultForm: TunnelFormValues = {
  name: "",
  groupID: "",
  listenHost: "127.0.0.1",
  username: "",
  password: "",
};

export function TunnelsPage() {
  const { run } = useAuthorizedRequest();
  const metrics = useShellMetrics();
  const navigate = useNavigate();
  const params = useParams();
  const [items, setItems] = useState<TunnelView[]>([]);
  const [groups, setGroups] = useState<GroupView[]>([]);
  const [nodes, setNodes] = useState<NodeView[]>([]);
  const [events, setEvents] = useState<TunnelEventView[]>([]);
  const [selected, setSelected] = useState<TunnelView | null>(null);
  const [loading, setLoading] = useState(true);
  const [eventsLoading, setEventsLoading] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<TunnelView | null>(null);
  const [form, setForm] = useState<TunnelFormValues>(defaultForm);
  const [errors, setErrors] = useState<Partial<Record<keyof TunnelFormValues, string>>>({});
  const [submitting, setSubmitting] = useState(false);

  async function load() {
    setLoading(true);
    try {
      const [tunnels, groupItems, nodeItems] = await Promise.all([
        run((token) => api.tunnels.list(token)),
        run((token) => api.groups.list(token)),
        run((token) => api.nodes.list(token)),
      ]);
      setItems(tunnels);
      setGroups(groupItems);
      setNodes(nodeItems);

      const selectedID = params.tunnelId;
      const next = tunnels.find((item) => item.id === selectedID) ?? tunnels[0] ?? null;
      setSelected(next);
      if (next) {
        await loadEvents(next.id);
      } else {
        setEvents([]);
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "隧道列表加载失败");
    } finally {
      setLoading(false);
    }
  }

  async function loadEvents(tunnelID: string) {
    setEventsLoading(true);
    try {
      const data = await run((token) => api.tunnels.events(token, tunnelID, 20));
      setEvents(data);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "隧道事件加载失败");
    } finally {
      setEventsLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [params.tunnelId]);

  function select(item: TunnelView) {
    setSelected(item);
    navigate(`/tunnels/${item.id}`);
  }

  function openCreate() {
    setEditing(null);
    setForm({
      ...defaultForm,
      groupID: groups[0]?.id ?? "",
    });
    setErrors({});
    setShowForm(true);
  }

  function openEdit(item: TunnelView) {
    setEditing(item);
    setForm({
      name: item.name,
      groupID: item.group_id,
      listenHost: item.listen_host,
      username: "",
      password: "",
    });
    setErrors({});
    setShowForm(true);
  }

  async function submit() {
    const nextErrors = validateTunnelForm(form);
    if (editing?.has_auth && (!form.username.trim() || !form.password.trim())) {
      nextErrors.password = "当前隧道已启用认证，编辑时必须重新填写用户名和密码";
    }
    setErrors(nextErrors);
    if (hasErrors(nextErrors)) {
      return;
    }

    setSubmitting(true);
    try {
      if (editing) {
        const updated = await run((token) =>
          api.tunnels.update(token, editing.id, {
            name: form.name.trim(),
            group_id: form.groupID,
            listen_host: form.listenHost.trim(),
            username: form.username.trim(),
            password: form.password.trim(),
          }),
        );
        setItems((current) => current.map((item) => (item.id === updated.id ? updated : item)));
        setSelected(updated);
        toast.success("隧道已更新");
      } else {
        const created = await run((token) =>
          api.tunnels.create(token, {
            name: form.name.trim(),
            group_id: form.groupID,
            listen_host: form.listenHost.trim(),
            username: form.username.trim(),
            password: form.password.trim(),
          }),
        );
        setItems((current) => [created, ...current]);
        setSelected(created);
        navigate(`/tunnels/${created.id}`);
        toast.success("隧道已创建");
      }
      setShowForm(false);
      await load();
      await metrics.refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "隧道保存失败");
    } finally {
      setSubmitting(false);
    }
  }

  async function action(actionName: "start" | "stop" | "refresh", item: TunnelView) {
    try {
      const next = await run((token) => {
        switch (actionName) {
          case "start":
            return api.tunnels.start(token, item.id);
          case "stop":
            return api.tunnels.stop(token, item.id);
          case "refresh":
            return api.tunnels.refresh(token, item.id);
        }
      });
      setItems((current) => current.map((currentItem) => (currentItem.id === next.id ? next : currentItem)));
      setSelected(next);
      await loadEvents(next.id);
      await metrics.refresh();
      toast.success(
        actionName === "refresh"
          ? "隧道刷新完成"
          : actionName === "start"
            ? "隧道已启动"
            : "隧道已停止",
      );
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "隧道操作失败");
      await load();
    }
  }

  async function remove(item: TunnelView) {
    if (!window.confirm(`确认删除隧道 ${item.name}？运行时目录也会一起清理。`)) {
      return;
    }
    try {
      await run((token) => api.tunnels.remove(token, item.id));
      setItems((current) => current.filter((currentItem) => currentItem.id !== item.id));
      if (selected?.id === item.id) {
        setSelected(null);
        setEvents([]);
        navigate("/tunnels", { replace: true });
      }
      await metrics.refresh();
      toast.success("隧道已删除");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除失败");
    }
  }

  const currentNode = selected?.current_node_id
    ? nodes.find((item) => item.id === selected.current_node_id)
    : null;
  const currentGroup = selected ? groups.find((item) => item.id === selected.group_id) : null;

  return (
    <AppShell>
      <div className="grid gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
        <Card className="overflow-hidden">
          <CardHeader className="space-y-4">
            <PanelTitle eyebrow="Tunnels" title="隧道列表" description="创建隧道时按组快照挑选节点，刷新支持 selector 热切换或重建。" />
            <Button onClick={openCreate}>
              <Plus className="h-4 w-4" />
              创建隧道
            </Button>
          </CardHeader>
          <CardContent className="space-y-3">
            {loading ? (
              <div className="flex items-center justify-center py-16 text-[var(--muted-foreground)]">
                <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                加载隧道中...
              </div>
            ) : items.length === 0 ? (
              <EmptyState title="没有隧道" description="从分组中创建 HTTP 代理隧道后，这里会展示运行时状态。" action={<Button onClick={openCreate}>创建隧道</Button>} />
            ) : (
              items.map((item) => (
                <button
                  className={`grid w-full cursor-pointer gap-3 rounded-[24px] border px-4 py-4 text-left transition-colors ${
                    selected?.id === item.id
                      ? "border-sky-400/30 bg-sky-400/10"
                      : "border-white/10 bg-white/5 hover:border-white/20 hover:bg-white/7"
                  }`}
                  key={item.id}
                  onClick={() => select(item)}
                  type="button"
                >
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <p className="text-base font-medium text-white">{item.name}</p>
                      <p className="mt-1 text-xs uppercase tracking-[0.16em] text-[var(--muted-foreground)]">
                        {item.listen_host}:{item.listen_port}
                      </p>
                    </div>
                    <Badge tone={tunnelStatusTone(item.status)}>{formatTunnelStatus(item.status)}</Badge>
                  </div>
                  <p className="text-sm text-[var(--muted-foreground)]">
                    当前节点 {item.current_node_id ?? "未选择"} / 控制端口 {item.controller_port}
                  </p>
                </button>
              ))
            )}
          </CardContent>
        </Card>

        {selected ? (
          <Card>
            <CardHeader className="flex flex-col gap-5">
              <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
                <div className="space-y-4">
                  <div className="flex flex-wrap items-center gap-3">
                    <CardTitle className="text-2xl">{selected.name}</CardTitle>
                    <Badge tone={tunnelStatusTone(selected.status)}>{formatTunnelStatus(selected.status)}</Badge>
                    {selected.has_auth ? <Badge tone="warn">启用代理认证</Badge> : null}
                  </div>
                  <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                    <TunnelMeta label="监听地址" value={`${selected.listen_host}:${selected.listen_port}`} />
                    <TunnelMeta label="当前节点" value={currentNode?.name ?? selected.current_node_id ?? "未锁定"} />
                    <TunnelMeta label="所属分组" value={currentGroup?.name ?? selected.group_id} />
                    <TunnelMeta label="控制端口" value={`${selected.controller_port}`} />
                  </div>
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button onClick={() => void action("refresh", selected)} variant="secondary">
                    <RefreshCw className="h-4 w-4" />
                    刷新
                  </Button>
                  {selected.status === "stopped" ? (
                    <Button onClick={() => void action("start", selected)} variant="secondary">
                      <Play className="h-4 w-4" />
                      启动
                    </Button>
                  ) : (
                    <Button onClick={() => void action("stop", selected)} variant="secondary">
                      <Square className="h-4 w-4" />
                      停止
                    </Button>
                  )}
                  <Button onClick={() => openEdit(selected)} variant="secondary">
                    <Cable className="h-4 w-4" />
                    编辑
                  </Button>
                  <Button onClick={() => void remove(selected)} variant="danger">
                    <Trash2 className="h-4 w-4" />
                    删除
                  </Button>
                </div>
              </div>

              <div className="grid gap-4 xl:grid-cols-[1.2fr_0.8fr]">
                <Card className="border-white/8 bg-white/4">
                  <CardHeader>
                    <CardTitle className="text-lg">隧道详情</CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-3 text-sm text-[var(--muted-foreground)]">
                    <div className="rounded-2xl border border-white/10 bg-[rgba(5,10,18,0.82)] p-4">
                      <p>运行时目录</p>
                      <p className="mt-2 break-all text-white">{selected.runtime_dir}</p>
                    </div>
                    <div className="rounded-2xl border border-white/10 bg-[rgba(5,10,18,0.82)] p-4">
                      <p>最近刷新时间</p>
                      <p className="mt-2 text-white">{formatDateTime(selected.last_refresh_at)}</p>
                    </div>
                    <div className="rounded-2xl border border-white/10 bg-[rgba(5,10,18,0.82)] p-4">
                      <p>最近错误</p>
                      <p className="mt-2 whitespace-pre-wrap text-white">{selected.last_refresh_error || "无"}</p>
                    </div>
                    <Link className="text-sky-300 underline-offset-4 transition hover:text-sky-100 hover:underline" to={`/tunnels/${selected.id}`}>
                      进入详情路由
                    </Link>
                  </CardContent>
                </Card>

                <Card className="border-white/8 bg-white/4">
                  <CardHeader>
                    <CardTitle className="text-lg">最近事件</CardTitle>
                  </CardHeader>
                  <CardContent>
                    {eventsLoading ? (
                      <div className="flex items-center justify-center py-14 text-[var(--muted-foreground)]">
                        <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                        加载事件中...
                      </div>
                    ) : events.length === 0 ? (
                      <EmptyState title="暂无事件" description="创建、刷新、启停后的事件会显示在这里。" />
                    ) : (
                      <div className="grid gap-3">
                        {events.map((event) => {
                          const detail = parseEventDetail(event.detail_json);
                          return (
                            <div className="rounded-2xl border border-white/10 bg-[rgba(5,10,18,0.82)] p-4" key={event.id}>
                              <div className="flex items-center justify-between gap-3">
                                <p className="text-sm font-medium text-white">{event.event_type}</p>
                                <span className="text-xs text-[var(--muted-foreground)]">{formatDateTime(event.created_at)}</span>
                              </div>
                              <pre className="mt-3 overflow-x-auto whitespace-pre-wrap text-xs text-sky-100">
                                {JSON.stringify(detail, null, 2)}
                              </pre>
                            </div>
                          );
                        })}
                      </div>
                    )}
                  </CardContent>
                </Card>
              </div>
            </CardHeader>
          </Card>
        ) : (
          <EmptyState title="还没有选中隧道" description="左侧选择一个隧道后，这里会展示端口、状态、最近错误和事件列表。" />
        )}
      </div>

      <Dialog onOpenChange={setShowForm} open={showForm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editing ? "编辑隧道" : "创建隧道"}</DialogTitle>
            <DialogDescription>创建时会对组成员做探测并锁定当前节点；认证信息为可选项。</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4">
            <Field error={errors.name} label="隧道名称">
              <Input onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} value={form.name} />
            </Field>
            <Field error={errors.groupID} label="分组 ID">
              <Input list="group-options" onChange={(event) => setForm((current) => ({ ...current, groupID: event.target.value }))} value={form.groupID} />
              <datalist id="group-options">
                {groups.map((group) => (
                  <option key={group.id} value={group.id}>
                    {group.name}
                  </option>
                ))}
              </datalist>
            </Field>
            <InlineFields>
              <Field label="监听地址">
                <Input onChange={(event) => setForm((current) => ({ ...current, listenHost: event.target.value }))} value={form.listenHost} />
              </Field>
              <div className="rounded-[24px] border border-white/10 bg-white/5 px-4 py-4 text-sm text-[var(--muted-foreground)]">
                监听端口由后端自动分配，创建后可在详情页查看。
              </div>
            </InlineFields>
            <InlineFields>
              <Field label="代理用户名">
                <Input onChange={(event) => setForm((current) => ({ ...current, username: event.target.value }))} value={form.username} />
              </Field>
              <Field error={errors.password} label="代理密码">
                <Input onChange={(event) => setForm((current) => ({ ...current, password: event.target.value }))} type="password" value={form.password} />
              </Field>
            </InlineFields>
            {editing?.has_auth ? (
              <div className="rounded-2xl border border-amber-400/20 bg-amber-400/10 px-4 py-3 text-sm text-amber-100">
                当前隧道启用了代理认证。后端不会返回原始用户名/密码，编辑时需要重新输入才能保留认证。
              </div>
            ) : null}
          </div>
          <DialogFooter>
            <Button onClick={() => setShowForm(false)} variant="ghost">
              取消
            </Button>
            <Button disabled={submitting} onClick={() => void submit()}>
              {submitting ? "提交中..." : editing ? "保存修改" : "创建隧道"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </AppShell>
  );
}

function TunnelMeta({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[24px] border border-white/10 bg-white/5 px-4 py-4">
      <p className="text-xs uppercase tracking-[0.16em] text-[var(--muted-foreground)]">{label}</p>
      <p className="mt-3 text-lg font-semibold text-white">{value}</p>
    </div>
  );
}
