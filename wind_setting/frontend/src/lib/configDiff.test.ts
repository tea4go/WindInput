import { describe, it, expect } from "vitest";
import { diffConfigToItems } from "./configDiff";

describe("diffConfigToItems", () => {
  it("无变化时返回空数组", () => {
    const base = { ui: { font_size: 18 }, toolbar: { visible: true } };
    const cur = { ui: { font_size: 18 }, toolbar: { visible: true } };
    expect(diffConfigToItems(base, cur)).toEqual([]);
  });

  it("标量改动产出点路径 key", () => {
    const base = { ui: { font_size: 18 } };
    const cur = { ui: { font_size: 22 } };
    expect(diffConfigToItems(base, cur)).toEqual([
      { key: "ui.font_size", value: 22 },
    ]);
  });

  it("嵌套对象只产出改动的叶子", () => {
    const base = { input: { auto_pair: { chinese: true, english: true } } };
    const cur = { input: { auto_pair: { chinese: false, english: true } } };
    expect(diffConfigToItems(base, cur)).toEqual([
      { key: "input.auto_pair.chinese", value: false },
    ]);
  });

  it("数组整体作为一个叶子提交", () => {
    const base = { schema: { available: ["wubi86"] } };
    const cur = { schema: { available: ["wubi86", "pinyin"] } };
    expect(diffConfigToItems(base, cur)).toEqual([
      { key: "schema.available", value: ["wubi86", "pinyin"] },
    ]);
  });

  it("current 比 base 多出的字段会被产出", () => {
    const base = { ui: {} as Record<string, unknown> };
    const cur = { ui: { theme: "msime" } };
    expect(diffConfigToItems(base, cur)).toEqual([
      { key: "ui.theme", value: "msime" },
    ]);
  });

  it("base 为 null 时整份 current 作为变化产出", () => {
    const cur = { ui: { font_size: 16 } };
    expect(diffConfigToItems(null, cur)).toEqual([
      { key: "ui.font_size", value: 16 },
    ]);
  });

  it("不会产出 base 有而 current 没有的字段（formData 不管理的段被忽略）", () => {
    const base = { ui: { font_size: 18 }, stats: { track_english: false } };
    const cur = { ui: { font_size: 18 } };
    expect(diffConfigToItems(base, cur)).toEqual([]);
  });
});
