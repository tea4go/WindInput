package dictcache

// dictsource.go —— 词库源格式抽象层。
//
// 词库磁盘格式为 rime `.dict.yaml`：YAML 头 + TSV 数据体，单文件（rime 生态交换格式）。
// 头解析为类型化 DictHeader；OpenDictSource 把"头来源"与"体来源"解耦为统一表示，
// 上层体解析逻辑（loadRimeCodetableFile / loadRimeFile）消费同一个 io.Reader。

import (
	"bufio"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// 词库文件后缀。
const (
	dictSuffixYAML = ".dict.yaml" // rime 单文件格式（头+体）
)

// DictHeader 词库头（rime YAML header 的类型化表示）。
// columns 缺省时按 rime 标准 text/code/weight 顺序解析。
type DictHeader struct {
	Name         string   `yaml:"name"`
	Version      string   `yaml:"version"`
	Sort         string   `yaml:"sort"`
	Columns      []string `yaml:"columns"`
	ImportTables []string `yaml:"import_tables"`
}

// dictStem 去掉词库后缀（.dict.yaml）返回 stem（含目录）。无已知后缀时原样返回。
func dictStem(path string) string {
	if s, ok := strings.CutSuffix(path, dictSuffixYAML); ok {
		return s
	}
	return path
}

// bodyReadCloser 组合数据体 Reader 与底层文件 Closer。
// Reader = 定位到 `...` 之后的 bufio.Reader，closer = 底层文件。
type bodyReadCloser struct {
	io.Reader
	closer io.Closer
}

func (b bodyReadCloser) Close() error {
	if b.closer != nil {
		return b.closer.Close()
	}
	return nil
}

// scanRimeYAMLHeader 从 br 读取 rime YAML header（读到 `...` 结束标记为止；
// 起始 `---` 可选），解析为 DictHeader。读取完成后 br 恰好定位到数据体首行。
// header 解析错误被容忍（降级为已部分填充/空的 DictHeader），与旧版逐行解析的
// 宽容语义一致——外部 rime 词库头偶有非常规内容不应中断转换。
func scanRimeYAMLHeader(br *bufio.Reader) DictHeader {
	var headerBuf strings.Builder
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			trimmed := strings.TrimSpace(line)
			if trimmed == "..." {
				break
			}
			// `---` 是 YAML 文档起始标记，不计入 header 内容（避免空文档干扰）。
			if trimmed != "---" {
				headerBuf.WriteString(line)
			}
		}
		if err != nil {
			break // EOF 或读取错误：无 `...` 分隔符，整段视为 header、数据体为空
		}
	}
	var hdr DictHeader
	if headerBuf.Len() > 0 {
		_ = yaml.Unmarshal([]byte(headerBuf.String()), &hdr)
	}
	return hdr
}

// OpenDictSource 打开 rime `.dict.yaml` 词库源，返回解析好的头与定位到数据体首行的流。
// 读到 `...` 为止解析头；body = 同一文件 `...` 之后的续流（单次 open、保持流式）。
// 调用方负责 Close 返回的 body。
func OpenDictSource(path string) (DictHeader, io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return DictHeader{}, nil, err
	}
	br := bufio.NewReaderSize(f, 64*1024)
	hdr := scanRimeYAMLHeader(br)
	return hdr, bodyReadCloser{Reader: br, closer: f}, nil
}

// ReadDictHeader 仅解析词库头（不打开数据体），用于 import_tables / 源清单发现。
// 解析错误被容忍（返回空/部分头 + nil error），不中断发现流程。
func ReadDictHeader(path string) (DictHeader, error) {
	f, err := os.Open(path)
	if err != nil {
		return DictHeader{}, err
	}
	defer f.Close()
	return scanRimeYAMLHeader(bufio.NewReaderSize(f, 64*1024)), nil
}
