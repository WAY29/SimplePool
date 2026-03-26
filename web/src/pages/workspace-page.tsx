import { useDeferredValue, useEffect, useMemo, useState, type ReactNode } from "react";
import {
  Boxes,
  LoaderCircle,
  Play,
  Plus,
  RefreshCw,
  Square,
  SquarePen,
  Trash2,
} from "lucide-react";
import { toast } from "sonner";
import { AppShell } from "@/components/layout/app-shell";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { NodeCollectionView, NodeViewModeSwitch } from "@/components/nodes/node-collection-view";
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
import { useAuthorizedRequest } from "@/hooks/use-authorized-request";
import { usePersistedViewMode } from "@/hooks/use-persisted-view-mode";
import { useShellMetrics } from "@/hooks/use-shell-metrics";
import { api, type GroupMemberView, type GroupView, type TunnelView } from "@/lib/api";
import { cn } from "@/lib/utils";
import {
  countHealthyNodes,
  formatDateTime,
  inferRegion,
  formatTunnelStatus,
  tunnelStatusTone,
} from "@/lib/format";
import {
  hasErrors,
  type GroupFormValues,
  type TunnelFormValues,
  validateGroupForm,
  validateTunnelForm,
} from "@/lib/forms";

const defaultGroupForm: GroupFormValues = {
  name: "",
  filterRegex: "",
  description: "",
};

const defaultTunnelForm: TunnelFormValues = {
  name: "",
  groupID: "",
  listenHost: "127.0.0.1",
  username: "",
  password: "",
};

export function WorkspacePage() {
  const { run } = useAuthorizedRequest();
  const metrics = useShellMetrics();
  const [groups, setGroups] = useState<GroupView[]>([]);
  const [tunnels, setTunnels] = useState<TunnelView[]>([]);
  const [selectedGroupID, setSelectedGroupID] = useState<string | null>(null);
  const [selectedTunnelID, setSelectedTunnelID] = useState<string | null>(null);
  const [members, setMembers] = useState<GroupMemberView[]>([]);
  const [loading, setLoading] = useState(true);
  const [memberLoading, setMemberLoading] = useState(false);
  const [search, setSearch] = useState("");
  const deferredSearch = useDeferredValue(search);
  const [showGroupForm, setShowGroupForm] = useState(false);
  const [editingGroup, setEditingGroup] = useState<GroupView | null>(null);
  const [groupForm, setGroupForm] = useState<GroupFormValues>(defaultGroupForm);
  const [groupErrors, setGroupErrors] = useState<Partial<Record<keyof GroupFormValues, string>>>({});
  const [groupSubmitting, setGroupSubmitting] = useState(false);
  const [showTunnelForm, setShowTunnelForm] = useState(false);
  const [editingTunnel, setEditingTunnel] = useState<TunnelView | null>(null);
  const [tunnelForm, setTunnelForm] = useState<TunnelFormValues>(defaultTunnelForm);
  const [tunnelErrors, setTunnelErrors] = useState<Partial<Record<keyof TunnelFormValues, string>>>({});
  const [tunnelSubmitting, setTunnelSubmitting] = useState(false);
  const [memberViewMode, setMemberViewMode] = usePersistedViewMode("simplepool.workspace.members.view_mode", "grid");

  const selectedGroup = groups.find((item) => item.id === selectedGroupID) ?? null;
  const groupTunnels = useMemo(
    () => (selectedGroup ? tunnels.filter((item) => item.group_id === selectedGroup.id) : []),
    [selectedGroup, tunnels],
  );
  const selectedTunnel = groupTunnels.find((item) => item.id === selectedTunnelID) ?? groupTunnels[0] ?? null;
  const tunnelKeyword = deferredSearch.trim().toLowerCase();
  const filteredTunnels = !tunnelKeyword
    ? groupTunnels
    : groupTunnels.filter((item) =>
        [item.name, item.listen_host, formatTunnelStatus(item.status)].join(" ").toLowerCase().includes(tunnelKeyword),
      );

  const activeTunnel = selectedTunnel
    ?? groupTunnels.find((item) => item.status === "running" || item.status === "starting" || item.status === "degraded")
    ?? tunnels.find((item) => item.status === "running" || item.status === "starting" || item.status === "degraded")
    ?? null;

  const healthyMembers = countHealthyNodes(members);
  const degradedTunnelCount = groupTunnels.filter((item) => item.status === "degraded" || item.status === "error").length;
  const activeTunnelCount = groupTunnels.filter((item) => item.status === "running" || item.status === "starting").length;
  const availableRuntimeCount = groupTunnels.filter((item) => item.runtime_dir?.trim()).length;

  async function loadWorkspace(preferredGroupID?: string | null) {
    setLoading(true);
    try {
      const [groupItems, tunnelItems] = await Promise.all([
        run((token) => api.groups.list(token)),
        run((token) => api.tunnels.list(token)),
      ]);
      setGroups(groupItems);
      setTunnels(tunnelItems);

      const nextGroupID =
        groupItems.find((item) => item.id === preferredGroupID)?.id
        ?? groupItems.find((item) => item.id === selectedGroupID)?.id
        ?? groupItems[0]?.id
        ?? null;
      setSelectedGroupID(nextGroupID);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "工作区数据加载失败");
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
      setMembers([]);
    } finally {
      setMemberLoading(false);
    }
  }

  useEffect(() => {
    void loadWorkspace();
  }, []);

  useEffect(() => {
    if (!selectedGroupID) {
      setMembers([]);
      return;
    }
    void loadMembers(selectedGroupID);
  }, [selectedGroupID]);

  useEffect(() => {
    const nextTunnelID = groupTunnels.find((item) => item.id === selectedTunnelID)?.id ?? groupTunnels[0]?.id ?? null;
    if (nextTunnelID !== selectedTunnelID) {
      setSelectedTunnelID(nextTunnelID);
    }
  }, [groupTunnels, selectedTunnelID]);

  function openCreateGroup() {
    setEditingGroup(null);
    setGroupForm(defaultGroupForm);
    setGroupErrors({});
    setShowGroupForm(true);
  }

  function openEditGroup(item: GroupView) {
    setEditingGroup(item);
    setGroupForm({
      name: item.name,
      filterRegex: item.filter_regex,
      description: item.description,
    });
    setGroupErrors({});
    setShowGroupForm(true);
  }

  async function submitGroup() {
    const nextErrors = validateGroupForm(groupForm);
    setGroupErrors(nextErrors);
    if (hasErrors(nextErrors)) {
      return;
    }

    setGroupSubmitting(true);
    try {
      if (editingGroup) {
        const updated = await run((token) =>
          api.groups.update(token, editingGroup.id, {
            name: groupForm.name.trim(),
            filter_regex: groupForm.filterRegex,
            description: groupForm.description.trim(),
          }),
        );
        toast.success("分组已更新");
        setShowGroupForm(false);
        await loadWorkspace(updated.id);
      } else {
        const created = await run((token) =>
          api.groups.create(token, {
            name: groupForm.name.trim(),
            filter_regex: groupForm.filterRegex,
            description: groupForm.description.trim(),
          }),
        );
        toast.success("分组已创建");
        setShowGroupForm(false);
        await loadWorkspace(created.id);
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "分组保存失败");
    } finally {
      setGroupSubmitting(false);
    }
  }

  async function removeGroup(item: GroupView) {
    if (!window.confirm(`确认删除分组 ${item.name}？`)) {
      return;
    }
    try {
      await run((token) => api.groups.remove(token, item.id));
      toast.success("分组已删除");
      await loadWorkspace(selectedGroupID === item.id ? null : selectedGroupID);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除分组失败");
    }
  }

  function openCreateTunnel() {
    setEditingTunnel(null);
    setTunnelForm({
      ...defaultTunnelForm,
      groupID: selectedGroup?.id ?? groups[0]?.id ?? "",
    });
    setTunnelErrors({});
    setShowTunnelForm(true);
  }

  function openEditTunnel(item: TunnelView) {
    setEditingTunnel(item);
    setTunnelForm({
      name: item.name,
      groupID: item.group_id,
      listenHost: item.listen_host,
      username: "",
      password: "",
    });
    setTunnelErrors({});
    setShowTunnelForm(true);
  }

  async function submitTunnel() {
    const nextErrors = validateTunnelForm(tunnelForm);
    if (editingTunnel?.has_auth && (!tunnelForm.username.trim() || !tunnelForm.password.trim())) {
      nextErrors.password = "当前隧道已启用认证，编辑时必须重新填写用户名和密码";
    }
    setTunnelErrors(nextErrors);
    if (hasErrors(nextErrors)) {
      return;
    }

    setTunnelSubmitting(true);
    try {
      if (editingTunnel) {
        const updated = await run((token) =>
          api.tunnels.update(token, editingTunnel.id, {
            name: tunnelForm.name.trim(),
            group_id: tunnelForm.groupID,
            listen_host: tunnelForm.listenHost.trim(),
            username: tunnelForm.username.trim(),
            password: tunnelForm.password.trim(),
          }),
        );
        toast.success("隧道已更新");
        setShowTunnelForm(false);
        await loadWorkspace(updated.group_id);
        await metrics.refresh();
        setSelectedTunnelID(updated.id);
      } else {
        const created = await run((token) =>
          api.tunnels.create(token, {
            name: tunnelForm.name.trim(),
            group_id: tunnelForm.groupID,
            listen_host: tunnelForm.listenHost.trim(),
            username: tunnelForm.username.trim(),
            password: tunnelForm.password.trim(),
          }),
        );
        toast.success("隧道已创建");
        setShowTunnelForm(false);
        await loadWorkspace(created.group_id);
        await metrics.refresh();
        setSelectedTunnelID(created.id);
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "隧道保存失败");
    } finally {
      setTunnelSubmitting(false);
    }
  }

  async function runTunnelAction(actionName: "refresh" | "start" | "stop", item: TunnelView) {
    try {
      await run((token) => {
        switch (actionName) {
          case "refresh":
            return api.tunnels.refresh(token, item.id);
          case "start":
            return api.tunnels.start(token, item.id);
          case "stop":
            return api.tunnels.stop(token, item.id);
        }
      });
      toast.success(
        actionName === "refresh"
          ? "隧道已刷新"
          : actionName === "start"
            ? "隧道已启动"
            : "隧道已停止",
      );
      await loadWorkspace(item.group_id);
      await metrics.refresh();
      setSelectedTunnelID(item.id);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "隧道操作失败");
    }
  }

  async function removeTunnel(item: TunnelView) {
    if (!window.confirm(`确认删除隧道 ${item.name}？运行时目录也会一起清理。`)) {
      return;
    }
    try {
      await run((token) => api.tunnels.remove(token, item.id));
      toast.success("隧道已删除");
      await loadWorkspace(item.group_id);
      await metrics.refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除隧道失败");
    }
  }

  async function refreshAll() {
    await loadWorkspace(selectedGroupID);
    await metrics.refresh();
  }

  const proxySummary = activeTunnel
    ? `HTTP 代理已启用。活动地址 ${activeTunnel.listen_host}:${activeTunnel.listen_port}。认证${activeTunnel.has_auth ? "已启用" : "未启用"}。`
    : "HTTP 代理未启用。当前没有运行中的隧道。";

  return (
    <AppShell hideHeader>
      <div className="rounded-[30px] border border-white/10 bg-[linear-gradient(180deg,rgba(12,18,28,0.96),rgba(8,12,20,0.98))] shadow-[0_30px_120px_rgba(2,8,20,0.48)]">
        <div className="border-b border-white/8 px-5 py-5 sm:px-6">
          <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
            <div className="space-y-3">
              <div className="flex flex-wrap items-center gap-3">
                <div className="flex items-center gap-3">
                  <div className="rounded-2xl border border-white/10 bg-white/5 p-2 text-violet-200">
                    <Boxes className="h-5 w-5" />
                  </div>
                  <div>
                    <h1 className="text-3xl font-semibold text-white">动态分组管理</h1>
                  </div>
                </div>
                <div className="rounded-full border border-violet-400/25 bg-violet-500/10 px-4 py-2 text-sm text-violet-100">
                  {proxySummary}
                </div>
              </div>
              <div className="flex flex-wrap items-center gap-x-3 gap-y-2 text-sm text-[var(--muted-foreground)]">
                <span>左侧导航已恢复，工作区内部布局继续保持参考图的分组/隧道双栏结构。</span>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-3">
              <Button onClick={openCreateGroup}>
                <Plus className="h-4 w-4" />
                新建分组
              </Button>
              <Button onClick={() => void refreshAll()} variant="secondary">
                <RefreshCw className="h-4 w-4" />
                刷新数据
              </Button>
            </div>
          </div>
        </div>

        <div className="grid gap-4 p-4 sm:p-5 xl:grid-cols-[320px_minmax(0,1fr)]">
          <div className="grid gap-4">
            <WorkspaceSection count={groups.length} title="动态分组">
              {loading ? (
                <SectionLoading label="正在加载分组..." />
              ) : groups.length === 0 ? (
                <SectionEmpty message="暂无分组，请先创建一个动态分组。" />
              ) : (
                <div className="grid gap-3">
                  {groups.map((item) => (
                    <button
                      className={cn(
                        "grid gap-2 rounded-[18px] border px-4 py-4 text-left transition-colors",
                        selectedGroup?.id === item.id
                          ? "border-violet-400/40 bg-violet-500/12"
                          : "border-white/10 bg-white/4 hover:border-white/20 hover:bg-white/7",
                      )}
                      key={item.id}
                      onClick={() => setSelectedGroupID(item.id)}
                      type="button"
                    >
                      <div className="flex items-center justify-between gap-3">
                        <div>
                          <p className="text-base font-medium text-white">
                            {item.name}
                            {selectedGroup?.id === item.id ? "（当前）" : ""}
                          </p>
                          <p className="mt-1 text-sm text-[var(--muted-foreground)]">
                            {summarizeGroupRegion(item.name, item.filter_regex)}
                          </p>
                        </div>
                        <div className="rounded-xl bg-violet-500/18 p-2 text-violet-100">
                          <Boxes className="h-3.5 w-3.5" />
                        </div>
                      </div>
                      <p className="text-sm text-[var(--muted-foreground)]">最近更新 {formatDateTime(item.updated_at)}</p>
                    </button>
                  ))}
                </div>
              )}
            </WorkspaceSection>

            <WorkspaceSection count={groupTunnels.length} title="隧道列表">
              <div className="flex items-center gap-2">
                <Input
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder="搜索隧道名称"
                  value={search}
                />
                <Button disabled={!selectedGroup} onClick={openCreateTunnel}>
                  创建隧道
                </Button>
              </div>

              {!selectedGroup ? (
                <SectionEmpty message="请先选择一个分组，再为该分组创建隧道。" />
              ) : filteredTunnels.length === 0 ? (
                <SectionEmpty message={groupTunnels.length === 0 ? "当前分组还没有隧道。" : "没有匹配的隧道。"} />
              ) : (
                <div className="grid gap-3">
                  {filteredTunnels.map((item) => (
                    <button
                      className={cn(
                        "grid gap-3 rounded-[18px] border px-4 py-4 text-left transition-colors",
                        selectedTunnel?.id === item.id
                          ? "border-violet-400/40 bg-violet-500/12"
                          : "border-white/10 bg-white/4 hover:border-white/20 hover:bg-white/7",
                      )}
                      key={item.id}
                      onClick={() => setSelectedTunnelID(item.id)}
                      type="button"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <p className="text-sm font-medium text-white">{item.name}</p>
                          <p className="mt-1 text-xs text-[var(--muted-foreground)]">
                            {item.listen_host}:{item.listen_port}
                          </p>
                        </div>
                        <Badge tone={tunnelStatusTone(item.status)}>{formatTunnelStatus(item.status)}</Badge>
                      </div>
                      <div className="flex items-center justify-between gap-3 text-xs text-[var(--muted-foreground)]">
                        <span>{item.has_auth ? "已启用认证" : "未启用认证"}</span>
                        <span>最近刷新 {formatDateTime(item.last_refresh_at)}</span>
                      </div>
                    </button>
                  ))}
                </div>
              )}

              {selectedTunnel ? (
                <div className="grid grid-cols-4 gap-2">
                  <SmallActionButton onClick={() => void runTunnelAction("refresh", selectedTunnel)}>
                    <RefreshCw className="h-3.5 w-3.5" />
                    刷新
                  </SmallActionButton>
                  {selectedTunnel.status === "stopped" ? (
                    <SmallActionButton onClick={() => void runTunnelAction("start", selectedTunnel)}>
                      <Play className="h-3.5 w-3.5" />
                      启动
                    </SmallActionButton>
                  ) : (
                    <SmallActionButton onClick={() => void runTunnelAction("stop", selectedTunnel)}>
                      <Square className="h-3.5 w-3.5" />
                      停止
                    </SmallActionButton>
                  )}
                  <SmallActionButton onClick={() => openEditTunnel(selectedTunnel)}>
                    <SquarePen className="h-3.5 w-3.5" />
                    编辑
                  </SmallActionButton>
                  <SmallActionButton danger onClick={() => void removeTunnel(selectedTunnel)}>
                    <Trash2 className="h-3.5 w-3.5" />
                    删除
                  </SmallActionButton>
                </div>
              ) : null}
            </WorkspaceSection>
          </div>

          <Card className="overflow-hidden rounded-[24px] border-white/10 bg-[rgba(18,24,38,0.92)]">
            <CardHeader className="gap-5 border-b border-white/8 p-5">
              <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
                <div className="space-y-4">
                  <div className="flex flex-wrap items-center gap-3">
                    <CardTitle className="text-3xl">{selectedGroup?.name ?? "未选择分组"}</CardTitle>
                    <span className="rounded-md bg-violet-500/80 px-2 py-1 text-xs font-medium text-white">
                      {selectedGroup ? "已选中" : "等待选择"}
                    </span>
                  </div>
                  <p className="max-w-3xl text-sm text-[var(--muted-foreground)]">
                    {selectedGroup?.description || "选择左侧分组后，这里会显示匹配成员、隧道状态和节点清单。"}
                  </p>
                </div>
                {selectedGroup ? (
                  <div className="flex flex-wrap gap-2">
                    <Button onClick={() => openEditGroup(selectedGroup)} variant="secondary">
                      <SquarePen className="h-4 w-4" />
                      编辑分组
                    </Button>
                    <Button onClick={() => void removeGroup(selectedGroup)} variant="danger">
                      <Trash2 className="h-4 w-4" />
                      删除分组
                    </Button>
                  </div>
                ) : null}
              </div>

              <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                <WorkspaceStatCard label="匹配节点" tone="neutral" value={`${members.length}`} />
                <WorkspaceStatCard label="活跃成员" tone="neutral" value={`${healthyMembers}`} />
                <WorkspaceStatCard label="活动/降级隧道" tone="warn" value={`${activeTunnelCount}/${degradedTunnelCount}`} />
                <WorkspaceStatCard label="可用运行时" tone="danger" value={`${availableRuntimeCount}/${groupTunnels.length}`} />
              </div>
            </CardHeader>

            <CardContent className="grid gap-4 p-5">
              <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
                <div>
                  <h2 className="text-2xl font-semibold text-white">组成员节点</h2>
                  <p className="mt-1 text-sm text-[var(--muted-foreground)]">
                    当前分组命中的节点支持网格视图与表格视图切换，两种展示方式与节点页保持统一。
                  </p>
                </div>
                <div className="flex flex-wrap items-center gap-3">
                  {selectedGroup ? (
                    <div className="rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-sm text-[var(--muted-foreground)]">
                      过滤正则 <span className="ml-2 font-medium text-white">{selectedGroup.filter_regex}</span>
                    </div>
                  ) : null}
                  <NodeViewModeSwitch mode={memberViewMode} onChange={setMemberViewMode} />
                </div>
              </div>

              {loading || memberLoading ? (
                <SectionLoading label="正在加载组成员..." />
              ) : (
                <NodeCollectionView emptyMessage="当前分组没有匹配节点。" items={members} mode={memberViewMode} />
              )}
            </CardContent>
          </Card>
        </div>
      </div>

      <Dialog onOpenChange={setShowGroupForm} open={showGroupForm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editingGroup ? "编辑分组" : "新建分组"}</DialogTitle>
            <DialogDescription>分组成员由 `filter_regex` 实时匹配节点名称生成。</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4">
            <Field error={groupErrors.name} label="分组名称">
              <Input onChange={(event) => setGroupForm((current) => ({ ...current, name: event.target.value }))} value={groupForm.name} />
            </Field>
            <Field error={groupErrors.filterRegex} label="过滤正则">
              <Input onChange={(event) => setGroupForm((current) => ({ ...current, filterRegex: event.target.value }))} value={groupForm.filterRegex} />
            </Field>
            <Field label="描述">
              <Textarea onChange={(event) => setGroupForm((current) => ({ ...current, description: event.target.value }))} value={groupForm.description} />
            </Field>
          </div>
          <DialogFooter>
            <Button onClick={() => setShowGroupForm(false)} variant="ghost">
              取消
            </Button>
            <Button disabled={groupSubmitting} onClick={() => void submitGroup()}>
              {groupSubmitting ? "提交中..." : editingGroup ? "保存修改" : "创建分组"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog onOpenChange={setShowTunnelForm} open={showTunnelForm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{editingTunnel ? "编辑隧道" : "创建隧道"}</DialogTitle>
            <DialogDescription>创建后会立即出现在当前分组的隧道列表中。</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4">
            <Field error={tunnelErrors.name} label="隧道名称">
              <Input onChange={(event) => setTunnelForm((current) => ({ ...current, name: event.target.value }))} value={tunnelForm.name} />
            </Field>
            <Field error={tunnelErrors.groupID} label="分组 ID">
              <Input list="group-options" onChange={(event) => setTunnelForm((current) => ({ ...current, groupID: event.target.value }))} value={tunnelForm.groupID} />
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
                <Input onChange={(event) => setTunnelForm((current) => ({ ...current, listenHost: event.target.value }))} value={tunnelForm.listenHost} />
              </Field>
              <div className="rounded-[20px] border border-white/10 bg-white/5 px-4 py-4 text-sm text-[var(--muted-foreground)]">
                监听端口由后端自动分配，创建后显示在隧道列表。
              </div>
            </InlineFields>
            <InlineFields>
              <Field label="代理用户名">
                <Input onChange={(event) => setTunnelForm((current) => ({ ...current, username: event.target.value }))} value={tunnelForm.username} />
              </Field>
              <Field error={tunnelErrors.password} label="代理密码">
                <Input onChange={(event) => setTunnelForm((current) => ({ ...current, password: event.target.value }))} type="password" value={tunnelForm.password} />
              </Field>
            </InlineFields>
            {editingTunnel?.has_auth ? (
              <div className="rounded-2xl border border-amber-400/20 bg-amber-400/10 px-4 py-3 text-sm text-amber-100">
                当前隧道已启用认证。后端不会返回原始用户名/密码，编辑时需要重新输入才能保留认证。
              </div>
            ) : null}
          </div>
          <DialogFooter>
            <Button onClick={() => setShowTunnelForm(false)} variant="ghost">
              取消
            </Button>
            <Button disabled={tunnelSubmitting} onClick={() => void submitTunnel()}>
              {tunnelSubmitting ? "提交中..." : editingTunnel ? "保存修改" : "创建隧道"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </AppShell>
  );
}

function WorkspaceSection({
  title,
  count,
  children,
}: {
  title: string;
  count: number;
  children: ReactNode;
}) {
  return (
    <Card className="rounded-[24px] border-white/10 bg-[rgba(18,24,38,0.92)]">
      <CardHeader className="gap-4 border-b border-white/8 p-4">
        <div className="flex items-center justify-between">
          <CardTitle className="text-[1.35rem]">{title}</CardTitle>
          <span className="inline-flex h-8 min-w-8 items-center justify-center rounded-full bg-white/6 px-2 text-sm text-white">
            {count}
          </span>
        </div>
      </CardHeader>
      <CardContent className="grid gap-4 p-4">{children}</CardContent>
    </Card>
  );
}

function WorkspaceStatCard({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone: "neutral" | "warn" | "danger";
}) {
  const tones = {
    neutral: "border-white/10 bg-white/4",
    warn: "border-amber-400/30 bg-amber-500/14",
    danger: "border-rose-400/25 bg-rose-500/14",
  };

  return (
    <div className={cn("rounded-[20px] border px-4 py-4", tones[tone])}>
      <p className="text-sm text-[var(--muted-foreground)]">{label}</p>
      <p className="mt-3 text-4xl font-semibold text-white">{value}</p>
    </div>
  );
}

function SectionLoading({ label }: { label: string }) {
  return (
    <div className="flex items-center justify-center rounded-[18px] border border-white/10 bg-white/4 px-4 py-8 text-sm text-[var(--muted-foreground)]">
      <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
      {label}
    </div>
  );
}

function SectionEmpty({ message }: { message: string }) {
  return (
    <div className="rounded-[18px] border border-white/10 bg-white/4 px-4 py-6 text-sm text-[var(--muted-foreground)]">
      {message}
    </div>
  );
}

function SmallActionButton({
  children,
  danger,
  onClick,
}: {
  children: ReactNode;
  danger?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      className={cn(
        "inline-flex h-10 items-center justify-center gap-1.5 rounded-2xl border px-3 text-sm transition-colors",
        danger
          ? "border-rose-400/25 bg-rose-500/12 text-rose-100 hover:bg-rose-500/18"
          : "border-white/10 bg-white/5 text-white hover:bg-white/10",
      )}
      onClick={onClick}
      type="button"
    >
      {children}
    </button>
  );
}

function summarizeGroupRegion(name: string, filterRegex: string) {
  const region = inferRegion(`${name} ${filterRegex}`);
  return region === "—" ? filterRegex : region;
}
