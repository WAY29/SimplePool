import { describe, expect, it } from "vitest";
import { countAvailableNodes, formatRegionFlag, inferRegion } from "@/lib/format";

describe("countAvailableNodes", () => {
  it("将已启用且非不可达节点计入 available", () => {
    expect(
      countAvailableNodes([
        { enabled: true, last_status: "healthy" },
        { enabled: true, last_status: "unknown" },
        { enabled: true, last_status: "unreachable" },
        { enabled: false, last_status: "unknown" },
      ]),
    ).toBe(2);
  });
});

describe("formatRegionFlag", () => {
  it("为已识别地区返回对应旗帜", () => {
    expect(formatRegionFlag("HK")).toBe("🇭🇰");
    expect(formatRegionFlag("JP")).toBe("🇯🇵");
    expect(formatRegionFlag("US")).toBe("🇺🇸");
    expect(formatRegionFlag("FR")).toBe("🇫🇷");
    expect(formatRegionFlag("CA")).toBe("🇨🇦");
  });

  it("为未知地区返回通用图标", () => {
    expect(formatRegionFlag("—")).toBe("🌐");
    expect(formatRegionFlag("AUTO")).toBe("🌐");
  });
});

describe("inferRegion", () => {
  it("识别新增的常见国家地区", () => {
    expect(inferRegion("法国-Paris-01")).toBe("FR");
    expect(inferRegion("Canada Toronto")).toBe("CA");
    expect(inferRegion("London Edge")).toBe("GB");
    expect(inferRegion("Frankfurt Core")).toBe("DE");
    expect(inferRegion("Seoul Premium")).toBe("KR");
  });
});
