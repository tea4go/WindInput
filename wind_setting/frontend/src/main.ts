import { createApp } from "vue";
import App from "./App.vue";
import "./app.css";
import { installWebShimIfNeeded } from "./lib/webShim";

// 非 Wails（浏览器 Web 模式）下注入 window.go/window.runtime shim，必须早于 App 挂载。
// Wails 桌面环境已有 window.go，shim 内部自检后直接跳过。
installWebShimIfNeeded();

// Auto dark mode: follow system preference
const mq = window.matchMedia("(prefers-color-scheme: dark)");
function applyTheme(e: MediaQueryListEvent | MediaQueryList) {
  document.documentElement.classList.toggle("dark", e.matches);
}
applyTheme(mq);
mq.addEventListener("change", applyTheme);

createApp(App).mount("#app");
