package config

import (
	"reflect"
	"strings"
	"testing"
)

// config_contract_test.go — 三态规范防漂移守护（设计 §2.3，机器执行的规则）：
//   R1: Config 全树禁用指针类型（"未设置=继承默认"由磁盘键缺失表达，bool 永不为 null）；
//   R2: 标量字段（bool/数值/string/枚举）yaml/json tag 一律不加 omitempty
//       （omitempty 基于 Go 零值而非默认值，会把显式 false/0/"" 丢键）；
//       slice/map 豁免（nil 与空集语义一致）。
// 任何人新增字段违反规则，本测试自动失败，无需注释守护。

// walkConfigFields 深度遍历 Config 类型树，对每个 struct 字段调用 visit。
func walkConfigFields(t reflect.Type, path string, visit func(path string, f reflect.StructField)) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		p := path + "." + f.Name
		visit(p, f)
		ft := f.Type
		// 进入嵌套 struct（含 slice 元素中的 struct，如 SpecialModeConfig）
		for ft.Kind() == reflect.Slice || ft.Kind() == reflect.Map || ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct {
			walkConfigFields(ft, p, visit)
		}
	}
}

// TestContract_NoPointerFields R1：Config 全树无指针字段。
func TestContract_NoPointerFields(t *testing.T) {
	walkConfigFields(reflect.TypeOf(Config{}), "Config", func(path string, f reflect.StructField) {
		k := f.Type.Kind()
		if k == reflect.Pointer {
			t.Errorf("%s: 配置 struct 禁用指针类型（设计 §2.3 R1，bool 永不为 null；"+
				"需要'未设置'语义请用枚举哨兵值或双字段）", path)
		}
	})
}

// TestContract_NoOmitemptyOnScalars R2：标量字段 tag 禁 omitempty（slice/map 豁免）。
func TestContract_NoOmitemptyOnScalars(t *testing.T) {
	walkConfigFields(reflect.TypeOf(Config{}), "Config", func(path string, f reflect.StructField) {
		switch f.Type.Kind() {
		case reflect.Slice, reflect.Map:
			return // 豁免
		}
		for _, tagName := range []string{"yaml", "json"} {
			tag := f.Tag.Get(tagName)
			for _, opt := range strings.Split(tag, ",")[1:] {
				if opt == "omitempty" {
					t.Errorf("%s: 标量字段 %s tag 不得带 omitempty（设计 §2.3 R2，"+
						"会把显式 false/0/\"\" 丢键破坏 diff-save 闭环）", path, tagName)
				}
			}
		}
	})
}
