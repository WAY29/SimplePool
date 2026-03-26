import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { PanelTitle } from "./app-shell";

describe("PanelTitle", () => {
  it("description 为空时不渲染描述段落", () => {
    render(<PanelTitle eyebrow="Nodes" title="节点列表" />);

    expect(screen.getByRole("heading", { name: "节点列表" })).toBeInTheDocument();
    expect(screen.queryByText(/支持手动新增/)).not.toBeInTheDocument();
  });
});
