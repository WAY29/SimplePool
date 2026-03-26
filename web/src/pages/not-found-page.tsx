import { GitBranch, Link2Off, Radio } from "lucide-react";
import { Link } from "react-router-dom";
import { AppShell } from "@/components/layout/app-shell";
import { IconButton } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";

export function NotFoundPage() {
  return (
    <AppShell>
      <Card className="flex min-h-[420px] items-center justify-center">
        <CardContent className="flex max-w-xl flex-col items-center justify-center gap-4 p-8 text-center">
          <div className="rounded-full border border-amber-400/25 bg-amber-400/12 p-4 text-amber-100">
            <Link2Off className="h-6 w-6" />
          </div>
          <div className="space-y-3">
            <h1 className="text-3xl font-semibold text-white">页面不存在</h1>
            <p className="text-sm leading-6 text-[var(--muted-foreground)]">
              旧的分组页和隧道页入口已经移除。请使用新的节点组入口继续操作。
            </p>
          </div>
          <div className="flex flex-wrap justify-center gap-3 pt-2">
            <IconButton asChild label="进入节点组">
              <Link to="/node-groups">
                <GitBranch className="h-4 w-4" />
              </Link>
            </IconButton>
            <IconButton asChild label="查看节点" variant="secondary">
              <Link to="/nodes">
                <Radio className="h-4 w-4" />
              </Link>
            </IconButton>
          </div>
        </CardContent>
      </Card>
    </AppShell>
  );
}
