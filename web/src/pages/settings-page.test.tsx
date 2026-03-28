import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SettingsPage } from "./settings-page";
import { api } from "@/lib/api";

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

vi.mock("@/hooks/use-authorized-request", () => ({
  useAuthorizedRequest: () => ({
    run: (request: (token: string) => Promise<unknown>) => request("test-token"),
  }),
}));

vi.mock("@/components/layout/app-shell", () => ({
  AppShell: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  PanelTitle: ({ title }: { title: string }) => <h1>{title}</h1>,
}));

describe("SettingsPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  it("选择预设时输入框只读并显示对应 URL", async () => {
    vi.spyOn(api.settings, "probe").mockResolvedValue({
      test_url: "http://cp.cloudflare.com/generate_204",
      default_test_url: "http://cp.cloudflare.com/generate_204",
      preset_urls: [
        "http://cp.cloudflare.com/generate_204",
        "https://www.gstatic.com/generate_204",
      ],
      updated_at: null,
    });

    render(<SettingsPage />);

    const input = await screen.findByLabelText("URL");
    expect(input).toHaveValue("http://cp.cloudflare.com/generate_204");
    expect(input).toBeDisabled();

    fireEvent.click(screen.getByLabelText("Gstatic"));

    await waitFor(() => {
      expect(input).toHaveValue("https://www.gstatic.com/generate_204");
    });
    expect(input).toBeDisabled();
  });

  it("选择 custom 时允许编辑并保存", async () => {
    vi.spyOn(api.settings, "probe").mockResolvedValue({
      test_url: "http://cp.cloudflare.com/generate_204",
      default_test_url: "http://cp.cloudflare.com/generate_204",
      preset_urls: [
        "http://cp.cloudflare.com/generate_204",
        "https://www.gstatic.com/generate_204",
      ],
      updated_at: null,
    });
    const updateSpy = vi.spyOn(api.settings, "updateProbe").mockResolvedValue({
      test_url: "https://probe.example.com/204",
      default_test_url: "http://cp.cloudflare.com/generate_204",
      preset_urls: [
        "http://cp.cloudflare.com/generate_204",
        "https://www.gstatic.com/generate_204",
      ],
      updated_at: "2026-03-27T12:00:00Z",
    });

    render(<SettingsPage />);

    const input = await screen.findByLabelText("URL");
    fireEvent.click(screen.getByLabelText("Custom"));
    expect(input).toBeEnabled();

    fireEvent.change(input, { target: { value: "https://probe.example.com/204" } });
    fireEvent.click(screen.getByRole("button", { name: "保存设置" }));

    await waitFor(() => {
      expect(updateSpy).toHaveBeenCalledWith("test-token", {
        test_url: "https://probe.example.com/204",
      });
    });
  });
});
