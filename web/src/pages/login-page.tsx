import { ShieldCheck, Workflow } from "lucide-react";
import { type FormEvent, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, InlineFields } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { APIError } from "@/lib/api";
import { useSession } from "@/hooks/use-session";

export function LoginPage() {
  const session = useSession();
  const navigate = useNavigate();
  const location = useLocation();
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!username.trim() || !password.trim()) {
      setError("用户名和密码不能为空");
      return;
    }

    setSubmitting(true);
    setError("");
    try {
      await session.login({ username, password });
      toast.success("登录成功");
      const next = (location.state as { from?: string } | null)?.from ?? "/workspace";
      navigate(next, { replace: true });
    } catch (cause) {
      const message = cause instanceof APIError ? cause.message : "登录失败";
      setError(message);
      toast.error(message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-[var(--background)] px-4 py-10">
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top_left,rgba(56,189,248,0.16),transparent_28%),radial-gradient(circle_at_bottom_right,rgba(234,179,8,0.16),transparent_28%)]" />
      <div className="relative grid w-full max-w-5xl gap-6 lg:grid-cols-[1.1fr_0.9fr]">
        <Card className="p-8">
          <div className="flex h-full flex-col justify-between gap-10">
            <div className="space-y-5">
              <div className="inline-flex items-center gap-2 rounded-full border border-sky-400/20 bg-sky-400/10 px-3 py-1 text-xs uppercase tracking-[0.2em] text-sky-100">
                <Workflow className="h-3.5 w-3.5" />
                Runtime Console
              </div>
              <div className="space-y-4">
                <h1 className="font-display text-4xl font-semibold leading-tight text-white sm:text-5xl">
                  登录 SimplePool
                </h1>
                <p className="max-w-xl text-base leading-7 text-[var(--muted-foreground)]">
                  节点池、动态分组、HTTP 隧道与刷新事件在一个控制面板内完成闭环操作。
                </p>
              </div>
            </div>

            <InlineFields className="grid-cols-1 gap-4 md:grid-cols-3">
              <MiniStat title="单机控制面" value="V1" />
              <MiniStat title="隧道语义" value="手动刷新锁定" />
              <MiniStat title="运行方式" value="独立 runtime" />
            </InlineFields>
          </div>
        </Card>

        <Card className="border-white/14">
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle>管理员认证</CardTitle>
                <CardDescription>使用后端初始化管理员账号登录。</CardDescription>
              </div>
              <div className="rounded-full border border-emerald-400/25 bg-emerald-400/12 p-3 text-emerald-200">
                <ShieldCheck className="h-5 w-5" />
              </div>
            </div>
          </CardHeader>
          <CardContent>
            <form className="grid gap-4" onSubmit={handleSubmit}>
              <Field label="用户名" error={!username.trim() && error ? "用户名不能为空" : undefined}>
                <Input
                  autoComplete="username"
                  onChange={(event) => setUsername(event.target.value)}
                  placeholder="admin"
                  value={username}
                />
              </Field>
              <Field label="密码" error={!password.trim() && error ? "密码不能为空" : undefined}>
                <Input
                  autoComplete="current-password"
                  onChange={(event) => setPassword(event.target.value)}
                  placeholder="输入后台密码"
                  type="password"
                  value={password}
                />
              </Field>
              {error ? (
                <div className="rounded-2xl border border-rose-400/20 bg-rose-400/10 px-4 py-3 text-sm text-rose-100">
                  {error}
                </div>
              ) : null}
              <Button disabled={submitting} size="lg" type="submit">
                {submitting ? "登录中..." : "进入控制台"}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function MiniStat({ title, value }: { title: string; value: string }) {
  return (
    <div className="rounded-[24px] border border-white/10 bg-white/5 p-4">
      <p className="text-xs uppercase tracking-[0.16em] text-[var(--muted-foreground)]">{title}</p>
      <p className="mt-3 text-lg font-semibold text-white">{value}</p>
    </div>
  );
}
