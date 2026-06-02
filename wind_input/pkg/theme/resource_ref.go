package theme

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// ResourceRef 资源注册表条目（v2.6 P7-E）：单图（明暗通用）或 {light,dark} 双变体。
// YAML / JSON 均支持两种写法：
//   - 标量字符串："panel.png"（明暗共用同一字节）
//   - 映射对象：{light: "panel.png", dark: "panel-dark.png"}（缺一侧回退另一侧）
//
// ResolveV25 按 isDark 经 PathFor 选出单路径填入 ResolvedV25.Resources——下游（imageForRef）
// 只见解析后的单路径，与 palette light/dark 双变体对称。
type ResourceRef struct {
	Light string `yaml:"light,omitempty" json:"light,omitempty"`
	Dark  string `yaml:"dark,omitempty" json:"dark,omitempty"`
}

// PathFor 按是否暗色返回对应路径；缺失侧回退另一侧（保证单变体写法仍可用）。
func (r ResourceRef) PathFor(isDark bool) string {
	if isDark && r.Dark != "" {
		return r.Dark
	}
	if !isDark && r.Light != "" {
		return r.Light
	}
	if r.Light != "" {
		return r.Light
	}
	return r.Dark
}

// normalize 单侧为空时回退另一侧。
func (r *ResourceRef) normalize() {
	if r.Light == "" {
		r.Light = r.Dark
	}
	if r.Dark == "" {
		r.Dark = r.Light
	}
}

func (r *ResourceRef) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		r.Light, r.Dark = value.Value, value.Value
		return nil
	}
	var m struct {
		Light string `yaml:"light"`
		Dark  string `yaml:"dark"`
	}
	if err := value.Decode(&m); err != nil {
		return err
	}
	r.Light, r.Dark = m.Light, m.Dark
	r.normalize()
	return nil
}

// MarshalYAML：light==dark 输出标量（种子文件保持简洁），否则输出 {light,dark}。
func (r ResourceRef) MarshalYAML() (any, error) {
	if r.Light == r.Dark {
		return r.Light, nil
	}
	return map[string]string{"light": r.Light, "dark": r.Dark}, nil
}

func (r *ResourceRef) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		r.Light, r.Dark = s, s
		return nil
	}
	var m struct {
		Light string `json:"light"`
		Dark  string `json:"dark"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	r.Light, r.Dark = m.Light, m.Dark
	r.normalize()
	return nil
}

// MarshalJSON：light==dark 输出字符串，否则输出 {light,dark}。
func (r ResourceRef) MarshalJSON() ([]byte, error) {
	if r.Light == r.Dark {
		return json.Marshal(r.Light)
	}
	return json.Marshal(map[string]string{"light": r.Light, "dark": r.Dark})
}
