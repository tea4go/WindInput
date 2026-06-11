// 按键 token 一致性测试：断言前端 enums.ts 的 Key / Modifier 值都是 Go pkg/keys
// 导出的规范名（generated/keys.json），杜绝前端用别名（如 "open_bracket"）造成的
// 前后端漂移。keys.json 由 `go test ./pkg/keys -run TestExportKeys -update` 生成。
import { describe, it, expect } from "vitest";
import keysData from "./generated/keys.json";
import { Key, Modifier } from "./lib/enums";

const canonicalKeys = new Set<string>(keysData.keys);
const canonicalMods = new Set<string>(keysData.modifiers);

describe("按键 token 与 Go keys.go 一致性", () => {
  it("keys.json 非空", () => {
    expect(canonicalKeys.size).toBeGreaterThan(0);
    expect(canonicalMods.size).toBeGreaterThan(0);
  });

  for (const [name, val] of Object.entries(Key)) {
    it(`Key.${name} = "${val}" 是 Go 规范按键名`, () => {
      expect(
        canonicalKeys.has(val),
        `Key.${name} 值 "${val}" 不在 keys.json 规范清单（疑似别名/拼写漂移）`,
      ).toBe(true);
    });
  }

  for (const [name, val] of Object.entries(Modifier)) {
    it(`Modifier.${name} = "${val}" 是 Go 规范修饰键`, () => {
      expect(
        canonicalMods.has(val),
        `Modifier.${name} 值 "${val}" 不在 keys.json 修饰键清单`,
      ).toBe(true);
    });
  }
});
