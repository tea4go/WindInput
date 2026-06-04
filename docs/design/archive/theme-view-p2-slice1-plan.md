# P2 切片-1：候选窗颜色 YAML 化 实现计划

> **For agentic workers:** 用 superpowers:executing-plans 逐任务实现。步骤用 `- [ ]` 跟踪。
> 上位 spec：`theme-view-p2-slice1-colors.md`；切片-0：`theme-view-p2-slice0-views.md`。

**Goal:** 候选窗静态颜色由 `ResolvedViews` 驱动，build 不再散落读 `cfg.XxxColor`/`getCommentColor`/`getShadowColor`；YAML views 可用颜色 token 配色；几何 + 颜色双零回归。

**Architecture:** 沿用切片-0 模式（合成桥 + applyThemeViews）扩展到颜色。合成桥从 cfg 填 RVNode 颜色；`applyThemeViews` 在 ui 侧解析 views 颜色 token（`${name}`→`ResolvedTheme.CandidateWindow.*` / hex）覆盖；build 静态色从 `r.resolvedViews` 读；运行时动态色（ModeAccent blend/glow）留 build。

**Tech Stack:** Go、image/color、现有 viewbox 引擎、几何+颜色回归网。

**正确性网:** Task 0 把几何指纹扩展为「几何+颜色」指纹；每个改色 task 后跑它，位置+颜色均不变才过。

---

## File Structure

- `wind_input/internal/ui/viewbox_geometry_test.go`（改）：`flattenRects`→`flattenNodes`（Rect+颜色），重捕 golden。
- `wind_input/pkg/theme/views.go`（改）：`ResolvedViews` 加 `ShadowColor color.Color`。
- `wind_input/internal/ui/viewbox_views_bridge.go`（改）：合成桥填 RVNode 颜色 + ShadowColor；`applyThemeViews` 加颜色 token 解析参数。
- `wind_input/internal/ui/viewbox_build.go` / `viewbox_build_vertical.go`（改）：静态色从 `rv` 读。
- `wind_input/internal/ui/renderer_layout.go`（改）：`applyThemeViews` 调用传 `CandidateWindow` 色。
- `wind_input/themes/default/theme.yaml`（改）：views 块加颜色 token。
- 文档：`internal/ui/AGENTS.md`、`pkg/theme/AGENTS.md`。

---

## Task 0：几何指纹扩展为「几何+颜色」

**Files:** Modify `wind_input/internal/ui/viewbox_geometry_test.go`

- [ ] **Step 1: 扩展 flatten 加颜色**

把 `flattenRects` 改名 `flattenNodes`，每节点记录 `Rect + Background.Color + Border.Color + TextStyle.Color`：

```go
func colorHex(c color.Color) string {
	if c == nil {
		return "-"
	}
	r, g, b, a := c.RGBA()
	return fmt.Sprintf("%02x%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8))
}

func flattenNodes(v *View) []string {
	if v == nil {
		return nil
	}
	r := v.Rect()
	out := []string{fmt.Sprintf("%d,%d,%d,%d|bg=%s|bd=%s|tx=%s",
		r.Min.X, r.Min.Y, r.Dx(), r.Dy(),
		colorHex(v.Background.Color), colorHex(v.Border.Color), colorHex(v.TextStyle.Color))}
	for _, c := range v.Children {
		out = append(out, flattenNodes(c)...)
	}
	return out
}
```

（`geometryFingerprint` 改调 `flattenNodes`；import 加 `image/color`。先核对 `View` 字段名 `Background.Color`/`Border.Color`/`TextStyle.Color` 与 viewbox.go 一致。）

- [ ] **Step 2: 临时打印新指纹**

把 `wantHGeometry`/`wantVGeometry` 断言暂时改 `t.Logf` 打印。
Run: `cd wind_input && go test ./internal/ui/ -run TestGeometryFingerprint -v`
Expected: PASS，打印含颜色的新指纹。

- [ ] **Step 3: 固化新 golden**

把打印的几何+颜色指纹替换 `wantHGeometry`/`wantVGeometry`，恢复 `reflect.DeepEqual` 断言。
Run: 同上 → PASS。

- [ ] **Step 4: Commit**

```bash
git add wind_input/internal/ui/viewbox_geometry_test.go
git commit -m "test(ui): 几何回归网扩展为几何+颜色指纹（P2 切片-1 基准）"
```

---

## Task 1：ResolvedViews.ShadowColor + 合成桥填颜色

**Files:** Modify `wind_input/pkg/theme/views.go`, `wind_input/internal/ui/viewbox_views_bridge.go`

- [ ] **Step 1: ResolvedViews 加 ShadowColor**

`views.go` 的 `ResolvedViews` 顶层加 `ShadowColor color.Color`（紧邻 VerticalMaxWidth）。

- [ ] **Step 2: 合成桥填 RVNode 颜色**

`renderConfigToViews` 各 RVNode 补颜色（按 spec 第二节映射）：

```go
Window:     theme.RVNode{ ...几何..., BgColor: cfg.BackgroundColor, BorderColor: cfg.BorderColor },
PreeditBar: theme.RVNode{ ...几何..., BgColor: cfg.InputBgColor, TextColor: cfg.InputTextColor },
Item:       theme.RVNode{ ...几何..., SelectedBg: cfg.SelectedBgColor, HoverBg: cfg.HoverBgColor },
Index:      theme.RVNode{ FontSize: cfg.IndexFontSize, FontWeight: cfg.IndexFontWeight, BgColor: cfg.IndexBgColor, TextColor: cfg.IndexColor },
Text:       theme.RVNode{ ...几何..., TextColor: cfg.TextColor },
Comment:    theme.RVNode{ ...几何..., TextColor: <见下> },
AccentBar:  theme.RVNode{ BgColor: cfg.AccentBarColor },
```

Comment/Shadow 来自运行时方法，合成桥无 cfg 字段直接对应：`renderConfigToViews` 改为 `(*Renderer) renderConfigToViews()` 方法（或额外传参），用 `r.getCommentColor()`/`r.getShadowColor()` 填 `Comment.TextColor`/`ShadowColor`。
> 决策：把 `renderConfigToViews(cfg)` 改为 `r.buildResolvedViews()`（Renderer 方法），内部读 `r.config` + `r.getCommentColor()`/`r.getShadowColor()`。renderer_layout.go 调用点同步改。

- [ ] **Step 3: 合成桥颜色单测**

```go
func TestBridgeColors(t *testing.T) {
	r := NewRenderer(parityConfig())
	rv := r.buildResolvedViews()
	if rv.Window.BgColor != parityConfig().BackgroundColor {
		t.Error("window bg 应=cfg.BackgroundColor")
	}
	if rv.Item.SelectedBg != parityConfig().SelectedBgColor {
		t.Error("item selected bg 应=cfg.SelectedBgColor")
	}
}
```

- [ ] **Step 4: Run & Commit** → `feat(ui): 合成桥填 RVNode 颜色 + ResolvedViews.ShadowColor`

---

## Task 2：build 静态色从 rv 读

**Files:** Modify `wind_input/internal/ui/viewbox_build.go`, `viewbox_build_vertical.go`

- [ ] **Step 1: 替换横排静态色引用**

`buildHorizontalCandidateTree` + 共用 `buildPreeditBand`/`buildPager` 内：
- `cfg.BackgroundColor`→`rv.Window.BgColor`（window Background.Color；注意 BackgroundImage 等仍 cfg）
- `cfg.SelectedBgColor`→`rv.Item.SelectedBg`，`cfg.HoverBgColor`→`rv.Item.HoverBg`
- `cfg.IndexBgColor`→`rv.Index.BgColor`，`cfg.IndexColor`→`rv.Index.TextColor`
- `cfg.TextColor`→`rv.Text.TextColor`
- `commentColor`（`r.getCommentColor()`）→`rv.Comment.TextColor`
- `cfg.AccentBarColor`→`rv.AccentBar.BgColor`（accent layer；`cfg.HasAccentBar` 判定保留）
- `cfg.InputBgColor`→`rv.PreeditBar.BgColor`，`cfg.InputTextColor`→`rv.PreeditBar.TextColor`
- `r.getShadowColor()`→`rv.ShadowColor`
- `windowBorder` 非 accent 分支 `cfg.BorderColor`→`rv.Window.BorderColor`
- **运行时动态色保留**：`cfg.ModeAccentColor` blend/glow 不变（blend 的 base 改用 `rv.PreeditBar.BgColor`）

- [ ] **Step 2: 跑几何+颜色网（横）**

Run: `cd wind_input && go test ./internal/ui/ -run TestGeometryFingerprint_Horizontal -v` → PASS（颜色与基准一致）。

- [ ] **Step 3: 竖排同样替换 + 跑竖网**

`viewbox_build_vertical.go` 同 Step 1；Run: `-run TestGeometryFingerprint_Vertical` → PASS。

- [ ] **Step 4: Commit** → `refactor(ui): 候选窗静态色改从 ResolvedViews 读`

---

## Task 3：applyThemeViews 颜色 token 解析 + 移除字号覆盖

**Files:** Modify `wind_input/internal/ui/viewbox_views_bridge.go`, `renderer_layout.go`

- [ ] **Step 1: 颜色 token 解析 helper**

```go
// resolveViewColor 解析 views 颜色字段：hex 直值或 ${name} 映射 candColors（CandidateWindow）。
func resolveViewColor(s string, cand theme.ResolvedCandidateWindowColors) color.Color {
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		switch s[2 : len(s)-1] {
		case "background": return cand.BackgroundColor
		case "border": return cand.BorderColor
		case "text": return cand.TextColor
		case "index_bg": return cand.IndexBgColor
		case "index_text": return cand.IndexColor
		case "hover_bg": return cand.HoverBgColor
		case "selected_bg": return cand.SelectedBgColor
		case "preedit_bg": return cand.InputBgColor
		case "preedit_text": return cand.InputTextColor
		case "comment": return cand.CommentColor
		case "accent": return cand.AccentBarColor
		case "shadow": return cand.ShadowColor
		}
		return nil // 未知 token：不覆盖
	}
	if c, err := theme.ParseHexColor(s); err == nil {
		return c
	}
	return nil
}
```

（核对 `ResolvedCandidateWindowColors` 字段名与 theme.go 一致。）

- [ ] **Step 2: applyThemeViews 加颜色覆盖 + 移除字号覆盖**

`applyThemeViews` 签名加 `cand theme.ResolvedCandidateWindowColors`；`apply` 闭包内：
- **删除** `if src.FontSize != nil { dst.FontSize = ... }` 和 FontWeight 两段（字号用户优先）。
- 加颜色：`if c := resolveViewColor(src.Background.Color, cand); c != nil { dst.BgColor = c }`；同理 Border.Color→dst.BorderColor、src.Color→dst.TextColor。
- Selected/Hover patch 的 background.color → dst.SelectedBg/HoverBg（item 专用，单独处理：`if src.Selected != nil { if c := resolveViewColor(src.Selected.Background.Color, cand); c != nil { dst.SelectedBg = c } }`）。

- [ ] **Step 3: renderer_layout.go 调用点传 candColors**

`applyThemeViews(&r.resolvedViews, r.themeViews, r.resolvedTheme.CandidateWindow)`（`r.resolvedTheme` 非 nil 时；nil 用零值）。

- [ ] **Step 4: 单测 token 解析 + 覆盖**

```go
func TestResolveViewColor(t *testing.T) {
	cand := theme.ResolvedCandidateWindowColors{TextColor: color.RGBA{1, 2, 3, 255}}
	if resolveViewColor("${text}", cand) != cand.TextColor {
		t.Error("${text} 应映射 TextColor")
	}
	if resolveViewColor("#FF0000", cand) == nil {
		t.Error("hex 应解析")
	}
	if resolveViewColor("${unknown}", cand) != nil {
		t.Error("未知 token 应返回 nil（不覆盖）")
	}
}
```

- [ ] **Step 5: Run & Commit** → `feat(ui): applyThemeViews 颜色 token 解析 + 移除字号覆盖`

---

## Task 4：default theme.yaml views 写颜色 token

**Files:** Modify `wind_input/themes/default/theme.yaml`

- [ ] **Step 1: views 块补颜色 token**

在切片-0 已有几何 views 块基础上补颜色（spec 第七节示例）：window background/border、item selected/hover、index bg/text、text、comment、accent_bar 的 `${语义}` token。

- [ ] **Step 2: 验证零回归**

因 token 解析后 = `CandidateWindow.*`（与 adapter 同源），运行时颜色不变。
单测层：`go test ./internal/ui/ ./pkg/theme/` 全绿（几何+颜色网用 parityConfig 不加载该文件，故文件改动靠逻辑保证；运行时验证需构建后手动观察绿点+配色）。

- [ ] **Step 3: Commit** → `feat(theme): default 主题 views 写颜色 token（P2 切片-1）`

---

## Task 5：文档 + 收尾

- [ ] **Step 1:** `internal/ui/AGENTS.md`：build 静态色从 rv 读；合成桥/applyThemeViews 含颜色；运行时动态色留 build。
- [ ] **Step 2:** `pkg/theme/AGENTS.md`：views.go ResolvedViews 加 ShadowColor + 颜色字段。
- [ ] **Step 3:** `go fmt` 改动；`go vet ./internal/ui/ ./pkg/theme/`；`go test ./internal/ui/ ./pkg/theme/ ./pkg/config/`（builtin 主题测试 pre-existing fail 除外）。
- [ ] **Step 4: Commit** → `docs(theme): 同步 P2 切片-1 颜色 YAML 化接口说明`

---

## Self-Review 记录

- **Spec 覆盖**：颜色映射(T1/T2)、ShadowColor(T1)、三来源(T1合成桥/T3 token)、build 读 rv(T2)、运行时动态色留 build(T2 保留)、字号移除覆盖(T3)、comment/shadow 迁入(T1/T2)、default token(T4)、几何+颜色验收(T0+各 task)。✓
- **运行时动态色**：T2 Step1 明确 ModeAccent blend/glow 保留，blend base 改 rv.PreeditBar.BgColor。
- **占位**：T1 Step2 / T2 Step1 的颜色映射须对照现 build 实码替换（已给 spec 映射表 + 锚点），非占位；几何+颜色网保证正确性。
- **类型一致**：RVNode 颜色字段 BgColor/BorderColor/TextColor/SelectedBg/HoverBg + ResolvedViews.ShadowColor；`resolveViewColor` 返回 color.Color；`buildResolvedViews()` 方法贯穿 T1/T2/T3。
