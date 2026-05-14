package dict

import "testing"

func TestParseAAMarker(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantName  string
		wantChars string
		wantOK    bool
	}{
		{
			name:      "标准用法",
			input:     `$AA("标点", "、。·")`,
			wantName:  "标点",
			wantChars: "、。·",
			wantOK:    true,
		},
		{
			name:      "前后空白容忍",
			input:     `  $AA(  "标点"  ,  "、。"  )  `,
			wantName:  "标点",
			wantChars: "、。",
			wantOK:    true,
		},
		{
			name:      "含转义引号",
			input:     `$AA("引号 \"a\"", "ab")`,
			wantName:  `引号 "a"`,
			wantChars: "ab",
			wantOK:    true,
		},
		{
			name:      "含反斜杠转义",
			input:     `$AA("name", "\\\"")`,
			wantName:  "name",
			wantChars: `\"`,
			wantOK:    true,
		},
		{
			name:      "空字符列表合法",
			input:     `$AA("空", "")`,
			wantName:  "空",
			wantChars: "",
			wantOK:    true,
		},
		{
			name:   "非 $AA 形式",
			input:  `$CC("a", open("b"))`,
			wantOK: false,
		},
		{
			name:   "缺尾括号",
			input:  `$AA("a", "b"`,
			wantOK: false,
		},
		{
			name:   "缺逗号",
			input:  `$AA("a" "b")`,
			wantOK: false,
		},
		{
			name:   "多余参数",
			input:  `$AA("a", "b", "c")`,
			wantOK: false,
		},
		{
			name:   "单引号不支持",
			input:  `$AA('a', 'b')`,
			wantOK: false,
		},
		{
			name:   "首参非字符串",
			input:  `$AA(name, "b")`,
			wantOK: false,
		},
		{
			name:   "空字符串非 AA",
			input:  ``,
			wantOK: false,
		},
		{
			name:   "$AA 缺括号",
			input:  `$AA"a","b"`,
			wantOK: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotName, gotChars, gotOK := ParseAAMarker(c.input)
			if gotOK != c.wantOK {
				t.Fatalf("ok=%v, want %v (input=%q)", gotOK, c.wantOK, c.input)
			}
			if !c.wantOK {
				return
			}
			if gotName != c.wantName {
				t.Errorf("name=%q, want %q", gotName, c.wantName)
			}
			if gotChars != c.wantChars {
				t.Errorf("chars=%q, want %q", gotChars, c.wantChars)
			}
		})
	}
}

func TestHasAAMarker(t *testing.T) {
	cases := map[string]bool{
		`$AA("a", "b")`:   true,
		`  $AA("a", "b")`: true,
		`$CC("a", run())`: false,
		`hello`:           false,
		`$AA`:             false, // 无括号也算; 严格匹配前缀 "$AA("
		`$AA(`:            true,
	}
	for in, want := range cases {
		if got := HasAAMarker(in); got != want {
			t.Errorf("HasAAMarker(%q)=%v, want %v", in, got, want)
		}
	}
}
