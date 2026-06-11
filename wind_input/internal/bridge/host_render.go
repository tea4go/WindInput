//go:build windows

package bridge

import (
	"fmt"
	"image"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"github.com/huanfeng/wind_input/internal/ipc"
	"github.com/huanfeng/wind_input/pkg/buildvariant"
	"golang.org/x/sys/windows"
)

var (
	procOpenProcess                = modkernel32.NewProc("OpenProcess")
	procQueryFullProcessImageNameW = modkernel32.NewProc("QueryFullProcessImageNameW")
)

const processQueryLimitedInformation = 0x1000

// hostRenderKinds 列出当前接入 host render 的窗口种类。每类有独立的 SHM 段 + per-PID
// 唤醒 event + DLL band 窗口——因为候选/tooltip/状态窗可能同时可见，不能共用单 bitmap
// 通道。DLL 侧 _EnsureHostRenderSetup 通用遍历条目建窗，新增种类只需加入此列表。
var hostRenderKinds = []ipc.HostWindowKind{
	ipc.HostWindowCandidate,
	ipc.HostWindowTooltip,
	ipc.HostWindowStatus,
}

// winSHMNameFor 返回某窗口种类的全局 SHM 段名。候选保持原名（向后兼容），其余加后缀。
// 变体后缀隔离 release/debug，避免两变体服务互相打开对方的 section 导致渲染串扰。
// Windows 命名 file-mapping 的物理页在所有映射进程间共享，故每类物理内存恒为一份，
// 与同时注入的进程数无关。
func winSHMNameFor(kind ipc.HostWindowKind) string {
	base := "Local\\WindInput_SHM" + buildvariant.Suffix()
	switch kind {
	case ipc.HostWindowTooltip:
		return base + "_TIP"
	case ipc.HostWindowStatus:
		return base + "_STS"
	default:
		return base
	}
}

// winEvtNameFor 返回某「实例（bridge clientID）」某窗口种类的私有唤醒 event 名。
// 按 (clientID, kind) 隔离，而非按 PID：同一宿主进程内可能有多个 TextService 实例
// （如两个记事本窗口 = 同 PID），它们各自的 render 线程绝不能等在同一个 auto-reset
// event 上——单次 SetEvent 只放行一个等待线程，会造成「两层候选、只隐一个」。clientID
// 是服务进程内单调递增、全局唯一的连接号，天然给每个实例一个独立 event。
func winEvtNameFor(clientID int, kind ipc.HostWindowKind) string {
	base := fmt.Sprintf("Local\\WindInput_EVT_C%d", clientID)
	switch kind {
	case ipc.HostWindowTooltip:
		return base + "_TIP"
	case ipc.HostWindowStatus:
		return base + "_STS"
	default:
		return base
	}
}

// hostRenderChannel 是单个窗口种类的发送通道：全局共享段 + 本实例私有唤醒 event。
type hostRenderChannel struct {
	SHM   *SharedMemory // 该 kind 的全局共享段（懒建常驻，所有 state 共享同一个）
	Event *NamedEvent   // 本实例该 kind 私有唤醒 event
}

// HostRenderState tracks host rendering state for a single bridge connection (one
// TextService instance), keyed by window kind. Each channel's SHM points at the global
// per-kind section; its Event is this INSTANCE's private wake event for that kind.
// InstanceID == the bridge clientID; Go stamps it into SharedRenderHeader.TargetInstanceID
// so the DLL render thread can tell whether a frame on the shared SHM targets it.
type HostRenderState struct {
	InstanceID int    // bridge clientID (也是 SHM 帧的 TargetInstanceID)
	ProcessID  uint32 // 宿主进程 PID（同 PID 可有多个实例）
	channels   map[ipc.HostWindowKind]*hostRenderChannel
	Active     bool   // Whether host render is currently active
	SetupSeq   uint64 // Monotonic counter to distinguish old vs new state
}

// HostRenderManager manages host rendering for whitelisted processes.
//
// 每窗口种类一块全局 SHM + per-(实例,kind) event 模型：每类窗口（候选/tooltip/状态）
// 各有一份全局共享 SHM（内存恒一份），但每个 TextService 实例（bridge 连接）对每类有
// 独立唤醒 event。同一 PID 可有多个实例（如两个记事本窗口）：写帧时 Go 把活动实例 ID
// 盖进 SHM 头的 TargetInstanceID，并 signal 该 PID 下所有实例的 event——目标实例渲染、
// 其余实例据 TargetInstanceID 自行隐藏，从而"恰好一个 band 窗口显示、其余清空"。
type HostRenderManager struct {
	mu       sync.Mutex
	logger   *slog.Logger
	patterns []string                             // 小写进程名模式，支持 filepath.Match 通配符（"*" 短路匹配全部）
	shms     map[ipc.HostWindowKind]*SharedMemory // 每窗口种类的全局共享段（懒建常驻）
	clients  map[int]*HostRenderState             // clientID -> state（持 per-(实例,kind) event）
	setupSeq uint64                               // Monotonic counter for setup generation
}

// NewHostRenderManager creates a new host render manager with the given whitelist.
func NewHostRenderManager(logger *slog.Logger, processNames []string) *HostRenderManager {
	return &HostRenderManager{
		logger:   logger,
		patterns: normalizePatterns(processNames),
		shms:     make(map[ipc.HostWindowKind]*SharedMemory),
		clients:  make(map[int]*HostRenderState),
	}
}

// normalizePatterns 小写化白名单模式，保持顺序。
func normalizePatterns(processNames []string) []string {
	patterns := make([]string, 0, len(processNames))
	for _, name := range processNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		patterns = append(patterns, strings.ToLower(name))
	}
	return patterns
}

// UpdateWhitelist updates the process whitelist (e.g. after config reload).
func (m *HostRenderManager) UpdateWhitelist(processNames []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patterns = normalizePatterns(processNames)
}

// matchLocked 在已持有 m.mu 的前提下，按通配符模式匹配进程名（已小写）。
// 支持 filepath.Match 语法（* ? [..]）；模式 "*" 单独短路为"匹配全部进程"（全局模式）。
func (m *HostRenderManager) matchLocked(lowerName string) bool {
	for _, p := range m.patterns {
		if p == "*" {
			return true
		}
		if ok, err := filepath.Match(p, lowerName); err == nil && ok {
			return true
		}
	}
	return false
}

// IsProcessWhitelisted checks if a process should use host rendering.
func (m *HostRenderManager) IsProcessWhitelisted(processID uint32) bool {
	if processID == 0 {
		return false
	}

	name := GetProcessName(processID) // syscall 放在锁外
	if name == "" {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	return m.matchLocked(strings.ToLower(name))
}

// SetupHostRender lazily creates one global shared-memory section per host window kind
// and one per-INSTANCE wake event per kind for this bridge connection, returning one setup
// entry per kind for the DLL (which creates one band window per entry). Keyed by clientID
// so multiple TextService instances in one process each get their own events.
func (m *HostRenderManager) SetupHostRender(clientID int, processID uint32) ([]ipc.HostRenderSetupEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 重建该实例的私有 events（若已存在先关旧的所有 kind）
	if old, ok := m.clients[clientID]; ok {
		for _, ch := range old.channels {
			if ch.Event != nil {
				ch.Event.Close()
			}
		}
		delete(m.clients, clientID)
	}

	state := &HostRenderState{
		InstanceID: clientID,
		ProcessID:  processID,
		channels:   make(map[ipc.HostWindowKind]*hostRenderChannel, len(hostRenderKinds)),
		Active:     true,
	}
	entries := make([]ipc.HostRenderSetupEntry, 0, len(hostRenderKinds))

	for _, kind := range hostRenderKinds {
		// 懒建该 kind 的全局共享段（一次，常驻）
		shm := m.shms[kind]
		if shm == nil {
			s, err := NewSharedMemory(winSHMNameFor(kind), ipc.MaxSharedRenderSize)
			if err != nil {
				return nil, fmt.Errorf("create host render SHM kind %d: %w", kind, err)
			}
			m.shms[kind] = s
			shm = s
			m.logger.Info("Host render SHM created", "kind", kind, "shmName", s.Name())
		}

		evtName := winEvtNameFor(clientID, kind)
		evt, err := newNamedEvent(evtName)
		if err != nil {
			return nil, fmt.Errorf("create wake event kind %d for client %d: %w", kind, clientID, err)
		}
		state.channels[kind] = &hostRenderChannel{SHM: shm, Event: evt}
		entries = append(entries, ipc.HostRenderSetupEntry{
			WindowKind:    kind,
			MaxBufferSize: shm.Size(),
			ShmName:       shm.Name(),
			EventName:     evtName,
		})
	}

	m.setupSeq++
	state.SetupSeq = m.setupSeq
	m.clients[clientID] = state

	m.logger.Info("Host render setup created", "clientID", clientID, "processID", processID, "kinds", len(entries))
	return entries, nil
}

// GetSetupSeq returns the current setup sequence for a client, or 0 if not found.
// Used by disconnect handlers to pass to CleanupClient for race-safe cleanup.
func (m *HostRenderManager) GetSetupSeq(clientID int) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state, ok := m.clients[clientID]; ok {
		return state.SetupSeq
	}
	return 0
}

// HasChannel reports whether the given client has an active host-render channel for the
// kind. server.GetActiveHostRenderFor uses it to decide if the active instance can host.
func (m *HostRenderManager) HasChannel(clientID int, kind ipc.HostWindowKind) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.clients[clientID]
	return state != nil && state.Active && state.channels[kind] != nil
}

// eventsForLocked collects every instance's wake event for the given (PID, kind). Caller
// must hold m.mu. Writing a frame signals ALL of them so non-target instances see the
// frame's TargetInstanceID and hide; the target renders.
func (m *HostRenderManager) eventsForLocked(pid uint32, kind ipc.HostWindowKind) []*NamedEvent {
	var events []*NamedEvent
	for _, state := range m.clients {
		if state.ProcessID != pid || !state.Active {
			continue
		}
		if ch := state.channels[kind]; ch != nil && ch.Event != nil {
			events = append(events, ch.Event)
		}
	}
	return events
}

// WriteFrameForKind writes a frame to the kind's global SHM stamped with targetInstanceID,
// then wakes EVERY instance of pid for that kind. The target instance renders it; siblings
// (same PID, different instance) see the mismatch and hide their band window.
func (m *HostRenderManager) WriteFrameForKind(kind ipc.HostWindowKind, pid uint32, targetInstanceID uint32, img *image.RGBA, x, y int, rects []ipc.CandidateHitRect, renderedHover int) error {
	m.mu.Lock()
	shm := m.shms[kind]
	events := m.eventsForLocked(pid, kind)
	m.mu.Unlock()

	if shm == nil {
		return fmt.Errorf("host render SHM kind %d unavailable", kind)
	}
	if err := shm.WriteFrame(img, x, y, rects, renderedHover, targetInstanceID); err != nil {
		return err
	}
	for _, e := range events {
		e.Signal()
	}
	return nil
}

// WriteHideForKind writes a hide frame (not visible) to the kind's global SHM, then wakes
// every instance of pid for that kind so they all clear their band window.
func (m *HostRenderManager) WriteHideForKind(kind ipc.HostWindowKind, pid uint32) {
	m.mu.Lock()
	shm := m.shms[kind]
	events := m.eventsForLocked(pid, kind)
	m.mu.Unlock()

	if shm == nil {
		return
	}
	shm.WriteHide()
	for _, e := range events {
		e.Signal()
	}
}

// CleanupClient removes host render state for a disconnected client. Only this instance's
// per-kind wake events are closed; the global per-kind SHM sections persist (other clients
// share them). The expectedSeq guard prevents an old connection's cleanup goroutine from
// closing a newer connection's events.
func (m *HostRenderManager) CleanupClient(clientID int, expectedSeq uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.clients[clientID]
	if !ok {
		return
	}

	if expectedSeq != 0 && state.SetupSeq != expectedSeq {
		m.logger.Info("Host render cleanup skipped: stale generation",
			"clientID", clientID, "expected", expectedSeq, "current", state.SetupSeq)
		return
	}

	for _, ch := range state.channels {
		if ch.Event != nil {
			ch.Event.Close()
		}
	}
	delete(m.clients, clientID)
	m.logger.Info("Host render cleanup", "clientID", clientID, "seq", expectedSeq)
}

// CleanupAll closes all per-instance events and all per-kind global shared memory sections.
// Called on service shutdown.
func (m *HostRenderManager) CleanupAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for clientID, state := range m.clients {
		for _, ch := range state.channels {
			if ch.Event != nil {
				ch.Event.Close()
			}
		}
		delete(m.clients, clientID)
	}
	for kind, shm := range m.shms {
		if shm != nil {
			shm.Close()
		}
		delete(m.shms, kind)
	}
}

// GetProcessName returns the executable name (e.g. "SearchHost.exe") for a process ID.
func GetProcessName(pid uint32) string {
	hProcess, _, _ := procOpenProcess.Call(
		processQueryLimitedInformation,
		0,
		uintptr(pid),
	)
	if hProcess == 0 {
		return ""
	}
	defer windows.CloseHandle(windows.Handle(hProcess))

	var buf [windows.MAX_PATH]uint16
	size := uint32(windows.MAX_PATH)
	ret, _, _ := procQueryFullProcessImageNameW.Call(
		hProcess,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		return ""
	}

	fullPath := windows.UTF16ToString(buf[:size])
	// Extract just the filename
	for i := len(fullPath) - 1; i >= 0; i-- {
		if fullPath[i] == '\\' || fullPath[i] == '/' {
			return fullPath[i+1:]
		}
	}
	return fullPath
}
