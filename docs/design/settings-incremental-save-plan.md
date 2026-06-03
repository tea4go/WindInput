# 设置端配置保存重构实现计划（快照 diff + 按 key 最小化提交）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 wind_setting 全局「保存」从整份覆盖（`ConfigSetAll`）改为「快照 diff 出改动项 → 按 key 提交（`Config.Set`）」，根治独立段（stats）被 null 覆盖的 bug。

**Architecture:** 前端新增纯函数 `diffConfigToItems(base, current)` 对 `config`(加载快照) 与 `formData`(编辑中) 做 deep-diff，产出 `{key, value}` 列表，经新增的 `App.SetConfigItems` 走现成的后端 `Config.Set`。独立段（stats/dict）不在 formData，永不被 diff 出，天然隔离。后端零改动（保留 `SetAll` 兜底）。

**Tech Stack:** Vue 3 + TypeScript + Vite 8 + vitest（本计划首次引入）；Wails v2.12 绑定；Go RPC。

设计文档：`docs/design/settings-incremental-save.md`

---

## 提交策略（重要，覆盖 superpowers 默认的 frequent-commit）

项目约定「功能未经实测前不主动提交、不主动 push」。因此：
- Task 1、Task 2 产出**自动化单测**，单测通过即满足「测试过」，可以提交。
- Task 3–5 是接口/UI 改动，无自动化测试覆盖，**不单独提交**，统一等 Task 6 手动实测通过后提交（或由用户决定粒度）。
- 全程不执行 `git push`。

---

## 文件结构

| 文件 | 责任 | 动作 |
|------|------|------|
| `wind_setting/frontend/package.json` | 加 vitest devDep + `test` 脚本 | 修改 |
| `wind_setting/frontend/vite.config.ts` | vitest 配置（node 环境、include） | 修改 |
| `wind_setting/frontend/src/lib/configDiff.ts` | deep-diff 纯函数 + `ConfigSetItem` 类型 | 新建 |
| `wind_setting/frontend/src/lib/configDiff.test.ts` | deep-diff 单元测试 | 新建 |
| `wind_setting/frontend/src/lib/AGENTS.md` | 登记 `configDiff.ts` | 修改 |
| `wind_setting/app_config.go` | 加 `SetConfigItems`、删 `SaveConfig` | 修改 |
| `wind_setting/AGENTS.md` | 更新 app_config.go 方法列表 | 修改 |
| `wind_setting/frontend/wailsjs/go/main/App.{js,d.ts}`、`models.ts` | wails 重新生成绑定 | 生成 |
| `wind_setting/frontend/src/api/wails.ts` | 加 `setConfigItems`、删整份 `saveConfig` | 修改 |
| `wind_setting/frontend/src/App.vue` | `saveConfig` 改走 diff + setConfigItems | 修改 |

---

## Task 1: 引入 vitest 测试基础设施

**Files:**
- Modify: `wind_setting/frontend/package.json`
- Modify: `wind_setting/frontend/vite.config.ts`
- Test: `wind_setting/frontend/src/lib/smoke.test.ts`（临时 smoke，本任务末尾删除）

- [ ] **Step 1: 安装 vitest**

Run（在 `wind_setting/frontend` 目录）：
```bash
pnpm add -D vitest
```
Expected: `package.json` 的 `devDependencies` 出现 `vitest`，pnpm-lock 更新。若因 vite 8 兼容性报错，改用 `pnpm add -D vitest@latest` 并记录实际版本。

- [ ] **Step 2: 配置 vitest（复用 vite.config）**

修改 `wind_setting/frontend/vite.config.ts`，在文件首行加三斜线引用，并在 `defineConfig` 对象里加 `test` 字段：
```ts
/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
})
```

- [ ] **Step 3: 加 test 脚本**

修改 `wind_setting/frontend/package.json` 的 `scripts`：
```json
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc --noEmit && vite build",
    "preview": "vite preview",
    "test": "vitest run"
  },
```

- [ ] **Step 4: 写 smoke 测试验证框架可用**

创建 `wind_setting/frontend/src/lib/smoke.test.ts`：
```ts
import { describe, it, expect } from "vitest";

describe("vitest 基础设施", () => {
  it("能运行测试", () => {
    expect(1 + 1).toBe(2);
  });
});
```

- [ ] **Step 5: 运行测试确认框架就绪**

Run（在 `wind_setting/frontend`）：
```bash
pnpm test
```
Expected: PASS，1 个测试通过。

- [ ] **Step 6: 删除 smoke 测试**

删除 `wind_setting/frontend/src/lib/smoke.test.ts`（它只为验证框架）。

- [ ] **Step 7: 提交**

```bash
git add wind_setting/frontend/package.json wind_setting/frontend/pnpm-lock.yaml wind_setting/frontend/vite.config.ts
git commit -m "test(setting): 引入 vitest 前端测试基础设施"
```

---

## Task 2: deep-diff 纯函数（TDD）

**Files:**
- Create: `wind_setting/frontend/src/lib/configDiff.ts`
- Create: `wind_setting/frontend/src/lib/configDiff.test.ts`
- Modify: `wind_setting/frontend/src/lib/AGENTS.md`

- [ ] **Step 1: 写失败测试**

创建 `wind_setting/frontend/src/lib/configDiff.test.ts`：
```ts
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
```

- [ ] **Step 2: 运行测试确认失败**

Run：
```bash
pnpm test
```
Expected: FAIL，报 `diffConfigToItems` 无法从 `./configDiff` 导入（模块不存在）。

- [ ] **Step 3: 实现 deep-diff**

创建 `wind_setting/frontend/src/lib/configDiff.ts`：
```ts
// 配置增量 diff：对 base(加载快照) 与 current(编辑中) 做 deep-diff，
// 只产出 current 相对 base 变化的叶子项，供按-key 提交（Config.Set）。
// 点路径约定与后端 resolveKeyPath/setNestedKey 对齐（如 input.auto_pair.chinese）。
//
// 规则：
// - 对象 → 递归进入；
// - 数组 / 标量 → 视为叶子，JSON.stringify 比较，不等则整体提交 current 值；
// - 只遍历 current 实际拥有的字段，因此 base 有而 current 没有的段（如 formData
//   不管理的 stats）永不被产出 —— 这是独立段隔离、根治覆盖 bug 的关键。

export interface ConfigSetItem {
  key: string;
  value: any;
}

function isPlainObject(v: any): boolean {
  return v !== null && typeof v === "object" && !Array.isArray(v);
}

export function diffConfigToItems(
  base: any,
  current: any,
  prefix = "",
): ConfigSetItem[] {
  const items: ConfigSetItem[] = [];
  if (current == null || typeof current !== "object") return items;

  for (const k of Object.keys(current)) {
    const path = prefix ? `${prefix}.${k}` : k;
    const cv = current[k];
    const bv = base == null ? undefined : base[k];

    if (isPlainObject(cv) && isPlainObject(bv)) {
      items.push(...diffConfigToItems(bv, cv, path));
    } else if (JSON.stringify(cv) !== JSON.stringify(bv)) {
      items.push({ key: path, value: cv });
    }
  }
  return items;
}
```

- [ ] **Step 4: 运行测试确认通过**

Run：
```bash
pnpm test
```
Expected: PASS，7 个测试全部通过。

- [ ] **Step 5: 登记到 lib/AGENTS.md**

修改 `wind_setting/frontend/src/lib/AGENTS.md` 的「Key Files」表格，新增一行：
```markdown
| `configDiff.ts` | 配置增量 diff 纯函数 `diffConfigToItems(base, current)`：对加载快照与编辑中 formData 做 deep-diff，产出 `{key, value}` 列表供按-key 提交；点路径与后端 `resolveKeyPath` 对齐 |
```

- [ ] **Step 6: 提交**

```bash
git add wind_setting/frontend/src/lib/configDiff.ts wind_setting/frontend/src/lib/configDiff.test.ts wind_setting/frontend/src/lib/AGENTS.md
git commit -m "feat(setting): 新增配置增量 diff 纯函数 diffConfigToItems"
```

---

## Task 3: 后端 App.SetConfigItems + 删除 App.SaveConfig + 重新生成绑定

**Files:**
- Modify: `wind_setting/app_config.go`
- Modify: `wind_setting/AGENTS.md`
- 生成: `wind_setting/frontend/wailsjs/go/main/App.{js,d.ts}`、`models.ts`

- [ ] **Step 1: 加 SetConfigItems、删 SaveConfig**

修改 `wind_setting/app_config.go`：删除 `SaveConfig`（约 31–45 行整个函数），新增 `SetConfigItems`。结果中相关部分应为：
```go
// SetConfigItems 按 key 增量保存配置项，返回是否需要重启
func (a *App) SetConfigItems(items []rpcapi.ConfigSetItem) (*SaveConfigResult, error) {
	if a.rpcClient == nil {
		return nil, fmt.Errorf("RPC client not initialized")
	}
	reply, err := a.rpcClient.ConfigSet(items)
	if err != nil {
		return nil, err
	}
	return &SaveConfigResult{RequiresRestart: reply.RequiresRestart}, nil
}
```
（`SaveConfigResult` 结构体保留不动；`rpcapi` 已 import。）

- [ ] **Step 2: go fmt + 编译**

Run（在 `wind_setting`）：
```bash
gofmt -w app_config.go
go build ./...
```
Expected: 退出码 0，无编译错误。

- [ ] **Step 3: 重新生成 Wails 绑定**

Run（在 `wind_setting`）：
```bash
wails generate module
```
Expected: `frontend/wailsjs/go/main/App.js` 与 `App.d.ts` 中 `SaveConfig` 消失、出现 `SetConfigItems`；`frontend/wailsjs/go/models.ts` 出现 `rpcapi.ConfigSetItem`。
（若环境无 `wails` CLI，回退方案见本任务末「附：手动绑定」。）

- [ ] **Step 4: 验证绑定生成正确**

Run（在 `wind_setting`）：
```bash
grep -n "SetConfigItems\|SaveConfig" frontend/wailsjs/go/main/App.js
```
Expected: 出现 `SetConfigItems`，不再出现 `SaveConfig`。

- [ ] **Step 5: 更新 wind_setting/AGENTS.md**

修改 `wind_setting/AGENTS.md` 「Key Files」表中 app_config.go 一行，把 `SaveConfig` 改为 `SetConfigItems`：
```markdown
| `app_config.go` | 配置读写 API：`GetConfig`、`SetConfigItems`（按 key 增量保存）、`ReloadConfig`、`CheckConfigModified` |
```

> **附：手动绑定回退**（仅当 `wails generate module` 不可用时）
> 在 `frontend/wailsjs/go/main/App.js` 删除 `SaveConfig` 函数、新增：
> ```js
> export function SetConfigItems(arg1) {
>   return window['go']['main']['App']['SetConfigItems'](arg1);
> }
> ```
> 在 `App.d.ts` 同步删 `SaveConfig` 声明、加 `export function SetConfigItems(arg1: Array<rpcapi.ConfigSetItem>): Promise<main.SaveConfigResult>;`；并在 `models.ts` 的 `rpcapi` 命名空间补 `ConfigSetItem` 类（字段 `key: string; value: any`）。

---

## Task 4: 前端 wails.ts 新增 setConfigItems、删除整份 saveConfig

**Files:**
- Modify: `wind_setting/frontend/src/api/wails.ts`

- [ ] **Step 1: 删除整份 saveConfig 封装**

在 `wind_setting/frontend/src/api/wails.ts` 删除（约 275–277 行）：
```ts
export async function saveConfig(cfg: Config): Promise<SaveConfigResult> {
  return (await App.SaveConfig(cfg as any)) as any;
}
```

- [ ] **Step 2: 新增 setConfigItems**

在原 `saveConfig` 位置（`SaveConfigResult` 接口之后）加入。`ConfigSetItem` 直接从 `configDiff.ts` 复用，避免重复定义：
```ts
import type { ConfigSetItem } from "../lib/configDiff";

export async function setConfigItems(
  items: ConfigSetItem[],
): Promise<SaveConfigResult> {
  return (await App.SetConfigItems(items as any)) as any;
}
```
（`import type` 放到文件顶部既有 import 区；此处展示就近，落地时归并到顶部。）

- [ ] **Step 3: 类型检查**

Run（在 `wind_setting/frontend`）：
```bash
pnpm vue-tsc --noEmit
```
Expected: 无类型错误（`App.SetConfigItems` 已由 Task 3 生成的绑定提供）。

---

## Task 5: 改写 App.vue saveConfig（wails 分支）

**Files:**
- Modify: `wind_setting/frontend/src/App.vue`

- [ ] **Step 1: 改写 wails 分支为 diff + setConfigItems**

在 `wind_setting/frontend/src/App.vue` 的 `saveConfig` 函数中，把 `if (isWailsEnv.value) { ... }` 块替换为：
```ts
    if (isWailsEnv.value) {
      const items = diffConfigToItems(config.value, formData.value);
      if (items.length === 0) {
        toast("当前无改动");
        return;
      }
      const reply = await wailsApi.setConfigItems(items);
      await wailsApi.saveTSFLogConfig(tsfLogConfig.value);
      toast(
        reply.requires_restart
          ? "保存成功（部分设置需重启生效）"
          : "保存成功",
      );
      config.value = JSON.parse(JSON.stringify(formData.value));
      savedTSFLogConfig.value = JSON.parse(JSON.stringify(tsfLogConfig.value));
      rebuildEngines(formData.value);
    } else {
```
（`return` 在 `try` 内，`finally` 的 `saving.value = false` 仍会执行；非 wails 分支与 `catch/finally` 保持不变。）

- [ ] **Step 2: 加 import**

在 `App.vue` `<script setup>` 顶部 import 区加入：
```ts
import { diffConfigToItems } from "./lib/configDiff";
```

- [ ] **Step 3: 类型检查 + 构建**

Run（在 `wind_setting/frontend`）：
```bash
pnpm build
```
Expected: `vue-tsc --noEmit` 通过、`vite build` 成功产出 dist。

---

## Task 6: 集成构建、手动验证、统一提交

**Files:** 无新增改动；构建与验证。

- [ ] **Step 1: 构建 debug 版（wind_input + wind_setting）**

按项目既有方式构建 debug 产物（含重新打包前端 dist 与 wind_setting）。确认 `build_debug` 下二进制更新。

- [ ] **Step 2: 重启服务并手动复现验证**

重启 wind_input 服务后，依次验证：
1. 在「外观/输入」页改一个字段（如字体大小），点全局保存 → 该字段生效；
2. 在统计页关闭「统计英文模式」→ 切回设置页，点全局保存 → 触发重载；
3. 确认 `D:\UserData\输入法数据\config.yaml` 出现 `stats:\n    track_english: false`，且统计页开关保持关闭；
4. 未改任何东西时点全局保存 → toast「当前无改动」，配置文件 mtime 不变；
5. 改一个 advanced 段字段保存 → toast 含「部分设置需重启生效」。

- [ ] **Step 3: 跑一遍自动化测试回归**

Run：
```bash
cd wind_setting/frontend && pnpm test
cd wind_input && go test ./internal/rpc/ ./pkg/config/
```
Expected: 前端 7 个 diff 测试通过；后端 `TestConfigSetAll_PreservesStats` 等通过。

- [ ] **Step 4: 实测通过后统一提交剩余改动**

```bash
git add wind_setting/app_config.go wind_setting/AGENTS.md \
  wind_setting/frontend/wailsjs/ wind_setting/frontend/src/api/wails.ts \
  wind_setting/frontend/src/App.vue
git commit -m "refactor(setting): 全局保存改为按 key 增量提交，根治 stats 被覆盖"
```
（不执行 `git push`。）

---

## 自检（Self-Review）

- **Spec 覆盖**：设计文档 6 个详细设计点 + 边界 → Task 2(deep-diff/数组/隔离)、Task 3(SetConfigItems/保留兜底)、Task 4(setConfigItems)、Task 5(saveConfig/空diff/requires_restart/基线)、Task 6(测试&手动验证) 全部覆盖；`SetAll` 兜底与 `TestConfigSetAll_PreservesStats` 明确保留。
- **类型一致**：`diffConfigToItems` 产出 `ConfigSetItem{key,value}`（configDiff.ts 定义）→ wails.ts `setConfigItems` 复用同一类型 → Go `App.SetConfigItems([]rpcapi.ConfigSetItem)` → `rpcClient.ConfigSet`；返回 `SaveConfigResult{requires_restart}` 全链一致。
- **无占位符**：各步含实代码、实路径、实命令与预期输出；wails 生成有手动回退方案。
- **约定**：Go 改动含 gofmt + build；前端含 vue-tsc/build；两处 AGENTS.md 同步；提交策略遵循「实测后提交、不 push」。
