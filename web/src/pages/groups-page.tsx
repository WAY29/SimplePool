import { useEffect, useState } from "react";
import { Check, LoaderCircle, Plus, SquarePen, Trash2, UsersRound, X } from "lucide-react";
import { toast } from "sonner";
import { AppShell, EmptyState, PanelTitle } from "@/components/layout/app-shell";
import { Badge } from "@/components/ui/badge";
import { IconButton } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Field } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableElement, TableHead, TableHeaderCell, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { api, type GroupMemberView, type GroupView } from "@/lib/api";
import { countHealthyNodes, formatLatency, formatNodeStatus } from "@/lib/format";
import { hasErrors, type GroupFormValues, validateGroupForm } from "@/lib/forms";
import { useAuthorizedRequest } from "@/hooks/use-authorized-request";

const defaultForm: GroupFormValues = {
  name: "",
  filterRegex: "",
  description: "",
};

export function GroupsPage() {
  const { run } = useAuthorizedRequest();
  const [items, setItems] = useState<GroupView[]>([]);
  const [selected, setSelected] = useState<GroupView | null>(null);
  const [members, setMembers] = useState<GroupMemberView[]>([]);
  const [loading, setLoading] = useState(true);
  const [memberLoading, setMemberLoading] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<GroupView | null>(null);
  const [form, setForm] = useState<GroupFormValues>(defaultForm);
  const [errors, setErrors] = useState<Partial<Record<keyof GroupFormValues, string>>>({});
  const [submitting, setSubmitting] = useState(false);
  const submitLabel = submitting ? "提交中..." : editing ? "保存修改" : "创建分组";

  async function load() {
    setLoading(true);
    try {
      const data = await run((token) => api.groups.list(token));
      setItems(data);
      const next = data.find((item) => item.id === selected?.id) ?? data[0] ?? null;
      setSelected(next);
      if (next) {
        await loadMembers(next.id);
      } else {
        setMembers([]);
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "分组列表加载失败");
    } finally {
      setLoading(false);
    }
  }

  async function loadMembers(groupID: string) {
    setMemberLoading(true);
    try {
      const data = await run((token) => api.groups.members(token, groupID));
      setMembers(data);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "组成员加载失败");
    } finally {
      setMemberLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  const memberStats = {
    total: members.length,
    healthy: countHealthyNodes(members),
  };

  function openCreate() {
    setEditing(null);
    setForm(defaultForm);
    setErrors({});
    setShowForm(true);
  }

  function openEdit(item: GroupView) {
    setEditing(item);
    setForm({
      name: item.name,
      filterRegex: item.filter_regex,
      description: item.description,
    });
    setErrors({});
    setShowForm(true);
  }

  async function submit() {
    const nextErrors = validateGroupForm(form);
    setErrors(nextErrors);
    if (hasErrors(nextErrors)) {
      return;
    }

    setSubmitting(true);
    try {
      if (editing) {
        const updated = await run((token) =>
          api.groups.update(token, editing.id, {
            name: form.name.trim(),
            filter_regex: form.filterRegex,
            description: form.description.trim(),
          }),
        );
        setItems((current) => current.map((item) => (item.id === updated.id ? updated : item)));
        setSelected(updated);
        toast.success("分组已更新");
        await loadMembers(updated.id);
      } else {
        const created = await run((token) =>
          api.groups.create(token, {
            name: form.name.trim(),
            filter_regex: form.filterRegex,
            description: form.description.trim(),
          }),
        );
        setItems((current) => [created, ...current]);
        setSelected(created);
        toast.success("分组已创建");
        await loadMembers(created.id);
      }
      setShowForm(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "分组保存失败");
    } finally {
      setSubmitting(false);
    }
  }

  async function remove(item: GroupView) {
    if (!window.confirm(`确认删除分组 ${item.name}？`)) {
      return;
    }
    try {
      await run((token) => api.groups.remove(token, item.id));
      setItems((current) => current.filter((currentItem) => currentItem.id !== item.id));
      if (selected?.id === item.id) {
        setSelected(null);
        setMembers([]);
      }
      toast.success("分组已删除");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除失败");
    }
  }

  async function select(item: GroupView) {
    setSelected(item);
    await loadMembers(item.id);
  }

  return (
    <AppShell>
      <div className="grid gap-4 xl:grid-cols-[340px_minmax(0,1fr)]">
        <Card className="overflow-hidden">
          <CardHeader className="space-y-4">
            <PanelTitle eyebrow="Groups" title="动态组" description="通过正则规则实时计算成员，不做静态落表" />
            <IconButton label="新建分组" onClick={openCreate}>
              <Plus className="h-4 w-4" />
            </IconButton>
          </CardHeader>
          <CardContent className="space-y-3">
            {loading ? (
              <div className="flex items-center justify-center py-16 text-[var(--muted-foreground)]">
                <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                加载分组中...
              </div>
            ) : items.length === 0 ? (
              <EmptyState
                action={
                  <IconButton label="新建分组" onClick={openCreate}>
                    <Plus className="h-4 w-4" />
                  </IconButton>
                }
                description="先创建一个正则组，再从组中创建隧道"
                title="没有分组"
              />
            ) : (
              items.map((item) => (
                <button
                  className={`grid w-full cursor-pointer gap-3 rounded-[24px] border px-4 py-4 text-left transition-colors ${
                    selected?.id === item.id
                      ? "border-sky-400/30 bg-sky-400/10"
                      : "border-white/10 bg-white/5 hover:border-white/20 hover:bg-white/7"
                  }`}
                  key={item.id}
                  onClick={() => void select(item)}
                  type="button"
                >
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <p className="text-base font-medium text-white">{item.name}</p>
                      <p className="mt-1 text-xs uppercase tracking-[0.16em] text-[var(--muted-foreground)]">
                        {item.filter_regex}
                      </p>
                    </div>
                    <UsersRound className="h-4 w-4 text-sky-300" />
                  </div>
                  <p className="text-sm text-[var(--muted-foreground)]">{item.description || "未填写描述"}</p>
                </button>
              ))
            )}
          </CardContent>
        </Card>

        {selected ? (
          <Card>
            <CardHeader className="flex flex-col gap-5 xl:flex-row xl:items-start xl:justify-between">
              <div className="space-y-4">
                <div className="flex flex-wrap items-center gap-3">
                  <CardTitle className="text-2xl">{selected.name}</CardTitle>
                  <Badge tone="info">实时成员 {memberStats.total}</Badge>
                </div>
                <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                  <StatCard label="过滤正则" value={selected.filter_regex} />
                  <StatCard label="当前成员" value={`${memberStats.total}`} />
                  <StatCard label="健康节点" value={`${memberStats.healthy}`} />
                  <StatCard label="描述" value={selected.description || "未填写"} />
                </div>
              </div>
              <div className="flex flex-wrap gap-2">
                <IconButton label="编辑" onClick={() => openEdit(selected)} variant="secondary">
                  <SquarePen className="h-4 w-4" />
                </IconButton>
                <IconButton label="删除" onClick={() => void remove(selected)} variant="danger">
                  <Trash2 className="h-4 w-4" />
                </IconButton>
              </div>
            </CardHeader>
            <CardContent className="grid gap-4">
              {memberLoading ? (
                <div className="flex items-center justify-center py-16 text-[var(--muted-foreground)]">
                  <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                  加载成员中...
                </div>
              ) : members.length === 0 ? (
                <EmptyState title="当前组没有匹配节点" description="检查正则规则或先导入更多节点" />
              ) : (
                <Table>
                  <TableElement>
                    <TableHead>
                      <tr>
                        <TableHeaderCell>节点</TableHeaderCell>
                        <TableHeaderCell>协议</TableHeaderCell>
                        <TableHeaderCell>状态</TableHeaderCell>
                        <TableHeaderCell>延迟</TableHeaderCell>
                        <TableHeaderCell>地址</TableHeaderCell>
                      </tr>
                    </TableHead>
                    <TableBody>
                      {members.map((member) => (
                        <TableRow key={member.id}>
                          <TableCell>{member.name}</TableCell>
                          <TableCell>{member.protocol}</TableCell>
                          <TableCell>{formatNodeStatus(member.last_status)}</TableCell>
                          <TableCell>{formatLatency(member.last_latency_ms)}</TableCell>
                          <TableCell>
                            {member.server}:{member.server_port}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </TableElement>
                </Table>
              )}
            </CardContent>
          </Card>
        ) : (
          <EmptyState title="还没有选中分组" description="左侧选择一个分组后，这里会展示实时成员预览" />
        )}
      </div>

      <Dialog onOpenChange={setShowForm} open={showForm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editing ? "编辑分组" : "新建分组"}</DialogTitle>
            <DialogDescription>分组成员由 `filter_regex` 实时匹配节点名称生成。</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4">
            <Field error={errors.name} label="分组名称">
              <Input onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))} value={form.name} />
            </Field>
            <Field error={errors.filterRegex} label="过滤正则">
              <Input onChange={(event) => setForm((current) => ({ ...current, filterRegex: event.target.value }))} value={form.filterRegex} />
            </Field>
            <Field label="描述">
              <Textarea onChange={(event) => setForm((current) => ({ ...current, description: event.target.value }))} value={form.description} />
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

function StatCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[24px] border border-white/10 bg-white/5 px-4 py-4">
      <p className="text-xs uppercase tracking-[0.16em] text-[var(--muted-foreground)]">{label}</p>
      <p className="mt-3 text-lg font-semibold text-white">{value}</p>
    </div>
  );
}
