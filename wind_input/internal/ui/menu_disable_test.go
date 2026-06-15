//go:build windows

package ui

import (
	"testing"
)

// computeMenuDisableState mirrors the logic in handleRightClick for testability.
// Returns (isGlobalFirst, isGlobalLast).
func computeMenuDisableState(pageStartIndex, hitIndex, totalCandidateCount int) (bool, bool) {
	globalIndex := pageStartIndex + hitIndex
	isGlobalFirst := globalIndex == 0
	isGlobalLast := totalCandidateCount <= 0 || globalIndex >= totalCandidateCount-1
	return isGlobalFirst, isGlobalLast
}

func TestMenuDisable_SinglePage(t *testing.T) {
	// 7 candidates on a single page (page 1, candidatesPerPage=7)
	total := 7
	pageStart := 0

	tests := []struct {
		name      string
		hitIndex  int
		wantFirst bool
		wantLast  bool
	}{
		{"first item", 0, true, false},
		{"middle item", 3, false, false},
		{"last item", 6, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isFirst, isLast := computeMenuDisableState(pageStart, tt.hitIndex, total)
			if isFirst != tt.wantFirst {
				t.Errorf("isGlobalFirst = %v, want %v", isFirst, tt.wantFirst)
			}
			if isLast != tt.wantLast {
				t.Errorf("isGlobalLast = %v, want %v", isLast, tt.wantLast)
			}
		})
	}
}

func TestMenuDisable_MultiPage(t *testing.T) {
	// 20 candidates, 7 per page → 3 pages
	total := 20
	perPage := 7

	tests := []struct {
		name      string
		page      int // 1-based
		hitIndex  int // 0-based within page
		wantFirst bool
		wantLast  bool
	}{
		// Page 1: globalIndex 0-6
		{"page1 first", 1, 0, true, false},
		{"page1 middle", 1, 3, false, false},
		{"page1 last", 1, 6, false, false}, // NOT global last

		// Page 2: globalIndex 7-13
		{"page2 first", 2, 0, false, false}, // NOT global first
		{"page2 middle", 2, 3, false, false},
		{"page2 last", 2, 6, false, false}, // NOT global last

		// Page 3: globalIndex 14-19 (6 candidates on last page)
		{"page3 first", 3, 0, false, false},
		{"page3 last", 3, 5, false, true}, // global last (index 19)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pageStart := (tt.page - 1) * perPage
			isFirst, isLast := computeMenuDisableState(pageStart, tt.hitIndex, total)
			if isFirst != tt.wantFirst {
				t.Errorf("isGlobalFirst = %v, want %v (globalIndex=%d)", isFirst, tt.wantFirst, pageStart+tt.hitIndex)
			}
			if isLast != tt.wantLast {
				t.Errorf("isGlobalLast = %v, want %v (globalIndex=%d, total=%d)", isLast, tt.wantLast, pageStart+tt.hitIndex, total)
			}
		})
	}
}

func TestMenuDisable_ZeroTotal(t *testing.T) {
	// Edge case: totalCandidateCount == 0 (uninitialized)
	// All items should have both move-up and move-down disabled
	isFirst, isLast := computeMenuDisableState(0, 0, 0)
	if !isFirst {
		t.Error("expected isGlobalFirst=true when total=0")
	}
	if !isLast {
		t.Error("expected isGlobalLast=true when total=0")
	}
}

func TestMenuDisable_SingleCandidate(t *testing.T) {
	// Only 1 candidate total
	isFirst, isLast := computeMenuDisableState(0, 0, 1)
	if !isFirst {
		t.Error("expected isGlobalFirst=true for single candidate")
	}
	if !isLast {
		t.Error("expected isGlobalLast=true for single candidate")
	}
}

// computeMenuDisableForGroupMember 复制 handleRightClick 中针对 IsGroupMember
// 的 disable 规则, 验证 $AA/$SS 字符组/字符串组子项所有可编辑菜单都被屏蔽。
// 入参分别对应 window_mouse.go 内的 isGlobalFirst / isGlobalLast / 单候选 /
// pinyin / command / quickInput / hasShadow / isGroupMember。
func computeMenuDisableForGroupMember(
	isGlobalFirst, isGlobalLast, isSingleCandidate,
	isPinyin, isCommand, isQuickInput, hasShadow, isGroupMember bool,
) (disableMoveUp, disableMoveDown, disableTop, disableDelete, disableReset bool) {
	disableMoveUp = isGlobalFirst || isSingleCandidate || (isPinyin && !isCommand) || isQuickInput || isGroupMember
	disableMoveDown = isGlobalLast || isSingleCandidate || (isPinyin && !isCommand) || isQuickInput || isGroupMember
	disableTop = isGlobalFirst || isQuickInput || isGroupMember
	disableDelete = isQuickInput || isGroupMember
	disableReset = !hasShadow || isGroupMember
	return
}

// TestMenuDisable_GroupMemberAllDisabled 验证 $AA/$SS 字符组/字符串组的子项候选
// 右键菜单 pin/delete/前移/后移/置顶/恢复默认 全 disable, 即使其它条件都允许。
//
// 引入: 2026-05-17 R2 follow-up (字符组顺序在源短语中已完整定义,
// 走"编辑短语"路径而非 Shadow 双轨漂移)。
func TestMenuDisable_GroupMemberAllDisabled(t *testing.T) {
	// 中段、命令候选、有 Shadow、非快捷输入 — 所有 ungated 条件都允许操作,
	// 但 isGroupMember=true 应让所有可编辑项 disable。
	up, down, top, del, reset := computeMenuDisableForGroupMember(
		false, // isGlobalFirst
		false, // isGlobalLast
		false, // isSingleCandidate
		false, // isPinyin
		true,  // isCommand
		false, // isQuickInput
		true,  // hasShadow
		true,  // isGroupMember
	)
	if !up || !down || !top || !del || !reset {
		t.Errorf("$AA/$SS group member: all menu actions must be disabled, got up=%v down=%v top=%v del=%v reset=%v",
			up, down, top, del, reset)
	}
}

// TestMenuDisable_NonGroupMemberHonorsOtherRules 回归测试: IsGroupMember=false
// 的命令候选仍按现有规则启用 (不被新逻辑无意中卡住)。
func TestMenuDisable_NonGroupMemberHonorsOtherRules(t *testing.T) {
	// 中段、命令候选、有 Shadow — 期望全部启用
	up, down, top, del, reset := computeMenuDisableForGroupMember(
		false, false, false, false, true, false, true, false,
	)
	if up || down || top || del || reset {
		t.Errorf("non-group-member: middle command with shadow should enable all, got up=%v down=%v top=%v del=%v reset=%v",
			up, down, top, del, reset)
	}
}

// TestMenuDisable_SingleCharDeletable 验证普通单字候选 (非命令 / 非组成员 /
// 非快捷输入) 的"删除/隐藏候选"菜单项**启用**。
//
// 引入: 2026-06-15 取消"单字不可删除/隐藏"限制后的回归守护。历史上 disableDelete
// 含 `isSingleChar && !isCommand` 条件, 会禁用单字删除; 现已移除。若未来有人重新
// 给 disableDelete 加回单字门控, 同步更新本镜像函数会让此用例失败, 提示行为回退。
func TestMenuDisable_SingleCharDeletable(t *testing.T) {
	// 中段、普通单字 (isCommand=false)、无 Shadow、非拼音、非快捷输入、非组成员。
	// 唯一会影响 delete 的剩余门控 isQuickInput / isGroupMember 均为 false。
	_, _, _, del, _ := computeMenuDisableForGroupMember(
		false, // isGlobalFirst
		false, // isGlobalLast
		false, // isSingleCandidate
		false, // isPinyin
		false, // isCommand — 普通单字, 非命令候选
		false, // isQuickInput
		false, // hasShadow
		false, // isGroupMember
	)
	if del {
		t.Errorf("普通单字候选的删除/隐藏菜单应启用 (单字限制已取消), got disableDelete=%v", del)
	}
}

// TestComputeDeleteMenuLabel 验证右键"删除"菜单文案按候选类型动态化。
// 详见 docs/design/candidate-actions.md §2 操作权能矩阵。
func TestComputeDeleteMenuLabel(t *testing.T) {
	cases := []struct {
		name        string
		phrase      bool
		userDict    bool
		tempDict    bool
		groupMember bool
		wantLabel   string
	}{
		{"短语 → 禁用短语", true, false, false, false, "禁用短语(X)"},
		{"短语 + UserDict 标记 (短语优先)", true, true, false, false, "禁用短语(X)"},
		{"nav (IsPhrase=true, IsGroupMember=false)", true, false, false, false, "禁用短语(X)"},
		{"用户词 → 删除用户词", false, true, false, false, "删除用户词(X)"},
		{"临时词 → 删除临时词", false, false, true, false, "删除临时词(X)"},
		{"用户词 + 临时词 (用户词优先)", false, true, true, false, "删除用户词(X)"},
		{"系统码表/拼音默认 → 隐藏候选", false, false, false, false, "隐藏候选(X)"},
		{"字符组成员 → 兜底文案 (disabled 不影响 UX)", false, false, false, true, "删除词条(X)"},
		{"字符组成员 + 短语标记 (groupMember 优先)", true, false, false, true, "删除词条(X)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeDeleteMenuLabel(tc.phrase, tc.userDict, tc.tempDict, tc.groupMember)
			if got != tc.wantLabel {
				t.Errorf("computeDeleteMenuLabel(phrase=%v, userDict=%v, tempDict=%v, groupMember=%v) = %q, want %q",
					tc.phrase, tc.userDict, tc.tempDict, tc.groupMember, got, tc.wantLabel)
			}
		})
	}
}
