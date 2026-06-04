<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-06-04 | Updated: 2026-06-04 -->

# design/archive/ - 已完成设计方案历史归档

## 用途

已**完成实现并验证**的设计/阶段计划文档。保留用于历史追溯与迁移对照，不再作为当前权威。
当前权威的主题设计请看上级目录的 `../theme-schema-v3.md`（设计语义）与 `../theme-v3-freeze-report.md`（冻结契约）。

## 归档文件

| 文件 | 描述 |
|------|------|
| [theme-view-architecture.md](theme-view-architecture.md) | P0 架构演进叙事：从固定化渲染到统一 View 盒模型的设计与迁移路线（含已删除的 adapter/density/LayoutSchema/ResolvedTheme 等 v2.5/v2.6 机制）。**已被 `../theme-schema-v3.md` 取代** |
| [theme-view-p7-schema-completion.md](theme-view-p7-schema-completion.md) | P7 完成度审计：候选窗 schema 能力清单与验收（点时态快照，引用已删除类型）。**已被 `../theme-v3-freeze-report.md` 取代** |
| [theme-view-p8-other-windows.md](theme-view-p8-other-windows.md) | P8 其它窗口几何 View 化的阶段记录（菜单/Tooltip/状态泡/Toast，toolbar 几何延后） |
| [theme-v26-freeze-report.md](theme-v26-freeze-report.md) | 主题 v2.6 候选窗盒模型 schema 冻结契约。**已解冻，被 v3 取代**（见 `../theme-v3-freeze-report.md`）；保留作 v2.6→v3 迁移对照 |
| [theme-view-p2-slice0-plan.md](theme-view-p2-slice0-plan.md) | P2 切片0：theme 解析 views + default 主题 YAML 基线（已实现） |
| [theme-view-p2-slice0-views.md](theme-view-p2-slice0-views.md) | P2 切片0：View 盒模型 schema 上位 spec（已实现） |
| [theme-view-p2-slice1-colors.md](theme-view-p2-slice1-colors.md) | P2 切片1：颜色/token 上位 spec（已实现） |
| [theme-view-p2-slice1-plan.md](theme-view-p2-slice1-plan.md) | P2 切片1：颜色落地计划（已实现） |
| [theme-view-p6-bridge-retirement.md](theme-view-p6-bridge-retirement.md) | P6：schema 三权分立 + 合成桥退役总设计（已实现） |
| [theme-view-p6-stage1-schema-plan.md](theme-view-p6-stage1-schema-plan.md) | P6 阶段1：Behavior schema 定义与接入（已实现） |
| [theme-view-p6-stage2a-plan.md](theme-view-p6-stage2a-plan.md) | P6 阶段2a：mergeViews 指针透传补全（已实现） |
| [theme-view-p6-stage2b-plan.md](theme-view-p6-stage2b-plan.md) | P6 阶段2b：ResolveCandidateViews 几何/颜色求值（已实现） |

## 说明

- 这些文档描述的功能均已落地，其中涉及的 `v2.5`/`v2.6` 版本号为历史阶段标记——主题 schema 已于 2026-06-04 重构并冻结为 **v3**。
- P4/P5 阶段计划归档在另一棵文档树：`../../../wind_input/docs/design/archive/`。
- 归档文档之间的互相引用按同目录兄弟文件名解析，仍然有效。

<!-- MANUAL: -->
