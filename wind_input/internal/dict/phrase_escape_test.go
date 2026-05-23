package dict

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecodePhraseEscapes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain newline", `第一行\n第二行`, "第一行\n第二行"},
		{"plain tab", `a\tb`, "a\tb"},
		{"no escape", "你好", "你好"},
		{"cmdbar marker untouched", `$CC("x\n", open("y"))`, `$CC("x\n", open("y"))`},
		{"aa marker untouched", `$AA("g", "ab\n")`, `$AA("g", "ab\n")`},
		{"ss marker untouched", `$SS("g", "a\nb")`, `$SS("g", "a\nb")`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decodePhraseEscapes(tc.in)
			if got != tc.want {
				t.Fatalf("decodePhraseEscapes(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSearch_DecodesLiteralEscapes(t *testing.T) {
	tmpDir := t.TempDir()
	userFile := filepath.Join(tmpDir, "user.phrases.yaml")
	content := "phrases:\n  - code: \"ml\"\n    text: \"第一行\\\\n第二行\"\n    position: 1\n"
	if err := os.WriteFile(userFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	pl := loadPhraseLayerFromYAML(t, "", userFile)
	got := pl.Search("ml", 10)
	if len(got) != 1 {
		t.Fatalf("Search returned %d candidates, want 1", len(got))
	}
	if got[0].Text != "第一行\n第二行" {
		t.Fatalf("candidate Text = %q, want decoded newline", got[0].Text)
	}
	if got[0].PhraseTemplate != `第一行\n第二行` {
		t.Fatalf("PhraseTemplate = %q, want raw escaped form", got[0].PhraseTemplate)
	}
}

// TestSearchPrefix_DecodesLiteralEscapes 覆盖「非全码」短语候选路径:
// 用户输入短语 code 的前缀时, SearchPrefix 出口同样需对静态短语 Text
// 应用 decodePhraseEscapes, 否则空格上屏会输出字面 `\n` 两字符,
// 候选标签也无法被 ui.candidateNewlineReplacer 折叠为 ↵。
func TestSearchPrefix_DecodesLiteralEscapes(t *testing.T) {
	tmpDir := t.TempDir()
	userFile := filepath.Join(tmpDir, "user.phrases.yaml")
	// min_prefix_length 默认 1, code "ml" 用前缀 "m" 命中。
	content := "phrases:\n  - code: \"ml\"\n    text: \"第一行\\\\n第二行\"\n    position: 1\n"
	if err := os.WriteFile(userFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	pl := loadPhraseLayerFromYAML(t, "", userFile)
	got := pl.SearchPrefix("m", 10)
	if len(got) != 1 {
		t.Fatalf("SearchPrefix returned %d candidates, want 1", len(got))
	}
	if got[0].Text != "第一行\n第二行" {
		t.Fatalf("candidate Text = %q, want decoded newline", got[0].Text)
	}
	if got[0].PhraseTemplate != `第一行\n第二行` {
		t.Fatalf("PhraseTemplate = %q, want raw escaped form", got[0].PhraseTemplate)
	}
}
