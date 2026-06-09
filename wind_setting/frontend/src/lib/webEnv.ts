// Web 模式（浏览器，非 Wails 桌面）环境判据与禁用提示文案。
//
// 注意：webShim 安装后 window.go 存在，App.vue 的 isWailsEnv 会恒为 true，
// 因此「是否具备原生能力」一律用 isWebMode() 判断，不要再用 isWailsEnv。

export const DESKTOP_ONLY_HINT = "web 版暂不支持此功能，请使用桌面版";

export function isWebMode(): boolean {
  return !!(window as any).__WEB_MODE__;
}
