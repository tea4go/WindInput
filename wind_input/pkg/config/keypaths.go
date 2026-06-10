package config

import (
	"reflect"
	"sort"
	"strings"
)

// AllKeyPaths 反射遍历 Config 的 yaml tag 树，返回全量 v1 配置键点路径
// （叶子字段；嵌套 struct 展开，slice/map 字段记为叶子节点）。
// 用途：导出给设置前端做 key 一致性校验（设计 §7.2——前端 schema/搜索索引
// 中出现的 key 必须 ∈ 本清单，拼写漂移在 CI 一次性抓出），
// 由 keypaths_test.go 的 TestExportKeyPaths -update 写出 JSON。
func AllKeyPaths() []string {
	var out []string
	collectKeyPaths(reflect.TypeOf(Config{}), "", &out)
	sort.Strings(out)
	return out
}

func collectKeyPaths(t reflect.Type, prefix string, out *[]string) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := strings.Split(f.Tag.Get("yaml"), ",")[0]
		if tag == "" || tag == "-" {
			continue
		}
		p := tag
		if prefix != "" {
			p = prefix + "." + tag
		}
		if f.Type.Kind() == reflect.Struct {
			collectKeyPaths(f.Type, p, out)
			continue
		}
		*out = append(*out, p)
	}
}
