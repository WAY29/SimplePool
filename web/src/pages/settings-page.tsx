import { LoaderCircle, RefreshCw, Save } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { AppShell, PanelTitle } from "@/components/layout/app-shell";
import { Card, CardContent } from "@/components/ui/card";
import { Field } from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { IconButton } from "@/components/ui/button";
import { useAuthorizedRequest } from "@/hooks/use-authorized-request";
import { api, type ProbeConfigView } from "@/lib/api";
import { cn } from "@/lib/utils";

type PresetMode = "cloudflare" | "gstatic" | "custom";

type PresetOption = {
  label: string;
  mode: PresetMode;
};

const presetOptions: PresetOption[] = [
  { label: "Cloudflare", mode: "cloudflare" },
  { label: "Gstatic", mode: "gstatic" },
  { label: "Custom", mode: "custom" },
];

export function SettingsPage() {
  const { run } = useAuthorizedRequest();
  const [config, setConfig] = useState<ProbeConfigView | null>(null);
  const [selectedMode, setSelectedMode] = useState<PresetMode>("cloudflare");
  const [draftURL, setDraftURL] = useState("");
  const [loading, setLoading] = useState(true);
  const [reloading, setReloading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    void load("loading");
  }, []);

  async function load(mode: "loading" | "reloading") {
    if (mode === "loading") {
      setLoading(true);
    } else {
      setReloading(true);
    }

    try {
      const view = await run((token) => api.settings.probe(token));
      setConfig(view);
      setSelectedMode(inferPresetMode(view.test_url, view));
      setDraftURL(view.test_url);
      setError("");
    } catch (loadError) {
      toast.error(loadError instanceof Error ? loadError.message : "系统设置加载失败");
    } finally {
      setLoading(false);
      setReloading(false);
    }
  }

  async function save() {
    const nextURL = draftURL.trim();
    const nextError = validateProbeTestURL(nextURL);
    if (nextError) {
      setError(nextError);
      return;
    }

    setSaving(true);
    try {
      const view = await run((token) => api.settings.updateProbe(token, { test_url: nextURL }));
      setConfig(view);
      setSelectedMode(inferPresetMode(view.test_url, view));
      setDraftURL(view.test_url);
      setError("");
      toast.success("设置已保存");
    } catch (saveError) {
      const message = saveError instanceof Error ? saveError.message : "设置保存失败";
      setError(message);
      toast.error(message);
    } finally {
      setSaving(false);
    }
  }

  function handlePresetChange(mode: PresetMode) {
    setSelectedMode(mode);
    setError("");
    if (mode === "custom") {
      return;
    }
    setDraftURL(presetURL(mode, config));
  }

  return (
    <AppShell>
      <div className="w-full max-w-6xl space-y-6">
        <div className="flex items-start justify-between gap-4">
          <PanelTitle eyebrow="Settings" title="系统设置" />
          <div className="flex items-center gap-2">
            <IconButton
              className="h-10 w-10 rounded-xl border-white/8 bg-white/4 text-white/70 hover:bg-white/8 hover:text-white"
              disabled={loading || reloading || saving}
              label="重新加载系统设置"
              onClick={() => void load("reloading")}
              variant="secondary"
            >
              {reloading ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
            </IconButton>
            <IconButton
              className="h-10 w-10 rounded-xl border-white/8 bg-white/4 text-white/70 hover:bg-white/8 hover:text-white"
              disabled={loading || saving}
              label="保存设置"
              onClick={() => void save()}
              variant="secondary"
            >
              {saving ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
            </IconButton>
          </div>
        </div>

        <Card className="w-full border-white/8 bg-[linear-gradient(180deg,rgba(12,17,28,0.98),rgba(8,12,20,0.98))] shadow-[0_28px_90px_rgba(2,8,20,0.38)]">
          <CardContent className="p-6 sm:p-8">
            {loading ? (
              <div className="flex min-h-40 items-center justify-center text-sm text-[var(--muted-foreground)]">
                <LoaderCircle className="mr-3 h-4 w-4 animate-spin" />
                加载中...
              </div>
            ) : (
              <div className="grid gap-8 lg:grid-cols-[240px_minmax(0,1fr)]">
                <div className="space-y-2 border-b border-white/8 pb-4 lg:border-b-0 lg:border-r lg:pb-0 lg:pr-8">
                  <p className="text-[11px] uppercase tracking-[0.28em] text-white/38">Probe</p>
                  <h2 className="text-2xl font-semibold tracking-[-0.03em] text-white">测速地址</h2>
                </div>

                <div className="space-y-5">
                  <fieldset className="space-y-3">
                    <legend className="text-sm font-medium text-white">模式</legend>
                    <div className="grid gap-3 md:grid-cols-3">
                      {presetOptions.map((item) => (
                        <label
                          className={cn(
                            "flex cursor-pointer items-center gap-3 rounded-xl border px-4 py-3 text-sm transition-colors",
                            selectedMode === item.mode
                              ? "border-white/18 bg-white/8 text-white"
                              : "border-white/8 bg-transparent text-[var(--muted-foreground)] hover:border-white/14 hover:text-white",
                          )}
                          key={item.mode}
                        >
                          <input
                            checked={selectedMode === item.mode}
                            className="h-4 w-4 accent-white"
                            name="probe_url_mode"
                            onChange={() => handlePresetChange(item.mode)}
                            type="radio"
                          />
                          <span>{item.label}</span>
                        </label>
                      ))}
                    </div>
                  </fieldset>

                  <Field error={error} label="URL">
                    <Input
                      className={cn(
                        "h-12 rounded-xl",
                        selectedMode !== "custom" ? "border-white/8 bg-white/[0.04] text-white/78" : "",
                      )}
                      disabled={selectedMode !== "custom"}
                      onChange={(event) => {
                        setDraftURL(event.target.value);
                        if (error) {
                          setError("");
                        }
                      }}
                      placeholder="https://example.com/generate_204"
                      value={draftURL}
                    />
                  </Field>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </AppShell>
  );
}

function inferPresetMode(value: string, config: Pick<ProbeConfigView, "default_test_url" | "preset_urls">): PresetMode {
  if (value === config.default_test_url) {
    return "cloudflare";
  }
  const gstatic = config.preset_urls.find((item) => item.includes("gstatic"));
  if (value === gstatic) {
    return "gstatic";
  }
  return "custom";
}

function presetURL(mode: Exclude<PresetMode, "custom">, config: Pick<ProbeConfigView, "default_test_url" | "preset_urls"> | null) {
  if (!config) {
    return "";
  }
  if (mode === "cloudflare") {
    return config.default_test_url;
  }
  return config.preset_urls.find((item) => item.includes("gstatic")) ?? "";
}

function validateProbeTestURL(value: string) {
  if (!value) {
    return "URL 不能为空";
  }
  try {
    const parsed = new URL(value);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
      return "URL 必须使用 http 或 https";
    }
    if (!parsed.hostname) {
      return "URL 非法";
    }
  } catch {
    return "URL 非法";
  }
  return "";
}
