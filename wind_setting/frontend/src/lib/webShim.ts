// 非 Wails（浏览器 Web 模式）下，注入与 Wails runtime 等价的 window.go / window.runtime，
// 使 wails.ts 与所有页面零改动透明走 HTTP（/api/call）与 SSE（/api/events）。
//
// 关键对齐点：
// 1. wailsjs/go/main/App.js 内部就是 `return window.go.main.App.XXX(args)`，且真实 Wails
//    返回的 Promise resolve 的是「已拆信封的 data」、reject 的是 error。因此 shim 的
//    appProxy 方法必须把后端 {data, error} 信封拆开：error 非空则 reject，否则 resolve(data)。
// 2. 事件名与后端 rpcapi.WailsEventXxx 常量一致（config-event/dict-event/stats-event/
//    system-event）；EventsOn(name, cb) 的 cb 收到的就是后端 payload（与桌面 EventsEmit 同形）。

import { DESKTOP_ONLY_HINT } from "./webEnv";

// showWebToast 注入一个轻量、自动消失的提示条，独立于各页 toast 系统，
// 仅用于 Web 模式下的兜底提示（如调用桌面专属方法且页面未自行处理时）。
function showWebToast(msg: string): void {
  const el = document.createElement("div");
  el.textContent = msg;
  el.style.cssText =
    "position:fixed;left:50%;bottom:32px;transform:translateX(-50%);" +
    "z-index:99999;max-width:80vw;padding:10px 16px;border-radius:8px;" +
    "background:rgba(0,0,0,0.82);color:#fff;font-size:14px;" +
    "box-shadow:0 4px 16px rgba(0,0,0,0.3);pointer-events:none;";
  document.body.appendChild(el);
  setTimeout(() => el.remove(), 2500);
}

// 需经 SSE 监听的事件名：
// - config/dict/stats/system-event：后端 rpcClient 转发（见 rpcapi/types.go WailsEventXxx）。
// - update:*：App 直接 emit、经 webServer.broadcast 投递（在线升级下载进度/完成/失败/有更新）。
// EventSource 必须按名 addEventListener，故需枚举；新增 App 广播事件时在此补充。
const BACKEND_EVENTS = [
  "config-event",
  "dict-event",
  "stats-event",
  "system-event",
  "update:progress",
  "update:done",
  "update:error",
  "update:available",
] as const;

// 桌面专属方法：后端实现依赖 wailsRuntime.*Dialog（文件选择/保存/选目录），需 Wails
// 窗口，Web 模式下无法工作。这里在唯一咽喉处（window.go.main.App 代理）直接拦截并
// reject 友好提示，覆盖所有调用点（无论经 wails.ts 封装还是直接 App.X）。
// 依据：后端 app_*.go 中所有 wailsRuntime.*Dialog 调用所属的导出方法（2026-06 核实）。
// 注意：openFolder / 打开外链类用的是 shellOpen（ShellExecuteW，不依赖 Wails），在
// 本机 127.0.0.1 下照常可用，故不在此列。
const DESKTOP_ONLY_METHODS = new Set<string>([
  // 备份 / 还原
  "BackupData",
  "GetRestorePreview",
  // 词库 / 短语 导入导出
  "ExportSchemaData",
  "ExportPhrasesFile",
  "ExportFullBackup",
  "SelectImportFile",
  "ImportPhrases",
  "ExportPhrases",
  "ImportUserDict",
  "ExportUserDict",
  "ImportUserDictForSchema",
  "ExportUserDictForSchema",
  // 方案 导入导出
  "ExportSchema",
  "ExportSchemas",
  "PreviewImportSchema",
  // 路径选择（命令短语引用 exe / 任意文件）
  "PickExePath",
  "PickAnyPath",
  // 数据目录 / 诊断 / 主题导入
  "SelectDataDir",
  "ExportPerfData",
  "ImportThemeFromFile",
]);

export function installWebShimIfNeeded(): void {
  const w = window as any;
  if (w.go?.main?.App) return; // 已在 Wails 桌面环境

  // ── HTTP 反射网关：window.go.main.App.XXX(...args) -> POST /api/call ──
  const call = (method: string, args: any[]) =>
    fetch("/api/call", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ method, args }),
    })
      .then((r) => r.json())
      .then((r) =>
        r.error ? Promise.reject(new Error(r.error)) : r.data,
      );

  w.go = {
    main: {
      App: new Proxy(
        {},
        {
          get: (_t, m: string) => (...a: any[]) => {
            // 桌面专属（文件对话框类）方法在 Web 模式无法工作。直接在调用点弹提示
            // （保证可见，不依赖各页 catch——有的页面只 console.error 不 toast），
            // 再 reject 终止流程。错误打 desktopOnly 标记，下方监听仅用于消除控制台噪声。
            if (DESKTOP_ONLY_METHODS.has(m)) {
              showWebToast(DESKTOP_ONLY_HINT);
              const err: any = new Error(DESKTOP_ONLY_HINT);
              err.desktopOnly = true;
              return Promise.reject(err);
            }
            // 安装会运行安装器并关闭本设置进程，浏览器页面随之失联。Web 模式下先确认告知。
            if (m === "InstallRelease") {
              const ok = window.confirm(
                "开始安装后，设置程序会关闭，本页面将无法访问。\n安装完成后请重新打开设置。\n\n是否继续安装？",
              );
              if (!ok) {
                const err: any = new Error("已取消安装");
                err.desktopOnly = true; // 复用静默标记，避免控制台未捕获噪声
                return Promise.reject(err);
              }
              showWebToast("正在安装，设置即将关闭，请稍候重新打开…");
            }
            return call(m, a);
          },
        },
      ),
    },
  };

  // ── 事件系统：忠实复刻 Wails runtime 的事件 API ──
  // 关键：wailsjs/runtime/runtime.js 的 EventsOn/EventsOnce 内部调的是
  // window.runtime.EventsOnMultiple(name, cb, maxCallbacks)（-1 不限 / 1 一次 / N 次），
  // EventsOff 调 window.runtime.EventsOff，并有 EventsOffAll。必须按这套实现，否则
  // 注册监听会抛 "EventsOnMultiple is not a function"（升级进度等事件全部失效）。
  interface Listener {
    cb: (...a: any[]) => void;
    remaining: number; // -1 表示不限次
  }
  const listeners: Record<string, Listener[]> = {};

  const register = (name: string, cb: (...a: any[]) => void, max: number) => {
    const l: Listener = { cb, remaining: max };
    (listeners[name] ||= []).push(l);
    return () => {
      const arr = listeners[name];
      if (arr) listeners[name] = arr.filter((x) => x !== l);
    };
  };
  const dispatch = (name: string, ...data: any[]) => {
    const arr = listeners[name];
    if (!arr || !arr.length) return;
    const keep: Listener[] = [];
    for (const l of arr.slice()) {
      try {
        l.cb(...data);
      } catch (e) {
        console.error("[webShim] 事件回调异常", name, e);
      }
      if (l.remaining < 0) keep.push(l);
      else if (l.remaining > 1) {
        l.remaining -= 1;
        keep.push(l);
      }
      // remaining === 1：已触发，丢弃（EventsOnce 语义）
    }
    listeners[name] = keep;
  };

  // ── SSE 事件桥：/api/events 帧 -> dispatch ──
  const es = new EventSource("/api/events");
  const onFrame = (name: string) => (e: MessageEvent) => {
    let payload: any;
    try {
      payload = JSON.parse(e.data);
    } catch {
      payload = e.data;
    }
    dispatch(name, payload);
  };
  BACKEND_EVENTS.forEach((name) => es.addEventListener(name, onFrame(name)));

  // window.runtime：事件方法忠实实现；其余 Wails runtime 方法（Window*/Log*/通知等）
  // 在 Web 无意义，用 Proxy 统一安全 no-op，避免任何 "X is not a function"。
  const noop = () => {};
  const runtimeImpl: Record<string, (...a: any[]) => any> = {
    EventsOnMultiple: (name: string, cb: (...a: any[]) => void, max: number) =>
      register(name, cb, max),
    EventsOn: (name: string, cb: (...a: any[]) => void) =>
      register(name, cb, -1),
    EventsOnce: (name: string, cb: (...a: any[]) => void) =>
      register(name, cb, 1),
    EventsOff: (name: string, ...rest: string[]) => {
      [name, ...rest].forEach((n) => delete listeners[n]);
    },
    EventsOffAll: () => {
      for (const k of Object.keys(listeners)) delete listeners[k];
    },
    EventsEmit: (name: string, ...data: any[]) => dispatch(name, ...data),
    BrowserOpenURL: (url: string) => {
      window.open(url, "_blank");
    },
  };
  w.runtime = new Proxy(runtimeImpl, {
    get: (target, prop: string | symbol) => {
      if (typeof prop === "string" && prop in target) return target[prop];
      // 未实现的 runtime 方法（窗口控制/日志/通知等）在 Web 模式安全 no-op
      return noop;
    },
  });

  // 桌面专属方法的提示已在调用点弹出；此处仅在未被页面 catch 时消除控制台未捕获噪声。
  window.addEventListener("unhandledrejection", (ev) => {
    if ((ev.reason as any)?.desktopOnly) {
      ev.preventDefault();
    }
  });

  // ── 存活心跳 ──
  // 与 SSE/后端管道状态无关，只表示「页面还开着」。后端 monitorIdle 超时无心跳即退出，
  // 避免 Web 进程常驻锁定 exe；同时心跳足够频繁、超时足够宽松，避免使用中误退出。
  const PING_MS = 10000;
  const ping = () => {
    fetch("/api/ping", { method: "POST" }).catch(() => {});
  };
  ping();
  window.setInterval(ping, PING_MS);
  // 标签从后台切回前台时立刻补一次心跳（后台定时器会被浏览器节流）。
  document.addEventListener("visibilitychange", () => {
    if (!document.hidden) ping();
  });
  // 关闭/刷新页面时发关闭信标，让后端尽快退出。刷新场景：pagehide 后新页面会立即
  // 重新 ping，后端在 byeGrace 内收到 ping 即取消退出。
  window.addEventListener("pagehide", () => {
    try {
      navigator.sendBeacon("/api/bye");
    } catch {
      // 忽略：sendBeacon 不可用时退化为 monitorIdle 超时兜底
    }
  });

  w.__WEB_MODE__ = true;
}
