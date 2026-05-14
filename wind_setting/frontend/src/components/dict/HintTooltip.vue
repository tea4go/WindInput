<script setup lang="ts">
// 通用提示 tooltip:
//   ? 图标 + hover/focus 触发, 气泡通过 <Teleport to="body"> 渲染到 body,
//   脱离对话框 overflow:auto 容器, 避免气泡撑出滚动条。
//   位置: 通过 button.getBoundingClientRect() 实时计算 viewport 坐标, fixed 定位。
//
// 用法:
//   - 简单单段: <HintTooltip text="切换不兼容的类型会修改内容" />
//   - 多行档位: <HintTooltip head="范围: 0~10000" :rows="['1000 默认中间值', ...]" />
import { ref, nextTick, computed } from "vue";

const props = withDefaults(
  defineProps<{
    text?: string;
    head?: string;
    rows?: ReadonlyArray<string>;
    ariaLabel?: string;
  }>(),
  {
    text: "",
    head: "",
    rows: () => [] as ReadonlyArray<string>,
    ariaLabel: "提示",
  },
);

const btnRef = ref<HTMLButtonElement | null>(null);
const visible = ref(false);
const bubblePos = ref({ top: 0, left: 0 });

// 单段文本模式: 只有 text 时, 渲染单行气泡; 否则 head + rows 多行模式。
const isSingleText = computed(
  () => !!props.text && !props.head && props.rows.length === 0,
);

async function showBubble() {
  visible.value = true;
  await nextTick();
  updatePos();
}

function hideBubble() {
  visible.value = false;
}

function updatePos() {
  if (!btnRef.value) return;
  const rect = btnRef.value.getBoundingClientRect();
  bubblePos.value = {
    top: rect.top + rect.height / 2,
    left: rect.right + 8,
  };
}
</script>

<template>
  <span class="hint-tt">
    <button
      ref="btnRef"
      type="button"
      class="hint-tt-btn"
      :aria-label="ariaLabel"
      tabindex="-1"
      @mouseenter="showBubble"
      @mouseleave="hideBubble"
    >
      ?
    </button>
    <Teleport to="body">
      <div
        v-if="visible"
        class="hint-tt-bubble"
        role="tooltip"
        :style="{
          top: bubblePos.top + 'px',
          left: bubblePos.left + 'px',
        }"
      >
        <template v-if="isSingleText">
          <div class="hint-tt-text">{{ text }}</div>
        </template>
        <template v-else>
          <div v-if="head" class="hint-tt-head">{{ head }}</div>
          <div v-for="row in rows" :key="row" class="hint-tt-row">
            {{ row }}
          </div>
        </template>
      </div>
    </Teleport>
  </span>
</template>

<style scoped>
.hint-tt {
  display: inline-flex;
  align-items: center;
  vertical-align: middle;
}

.hint-tt-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border-radius: 999px;
  border: 1px solid hsl(var(--border));
  background: hsl(var(--muted));
  color: hsl(var(--muted-foreground));
  font-size: 11px;
  line-height: 1;
  cursor: help;
  padding: 0;
}

.hint-tt-btn:hover,
.hint-tt-btn:focus-visible {
  background: hsl(var(--accent));
  color: hsl(var(--foreground));
  outline: none;
}
</style>

<style>
/* 全局样式: 因为 Teleport 后气泡脱离了 scoped 作用域 */
.hint-tt-bubble {
  position: fixed;
  transform: translateY(-50%);
  max-width: 240px;
  background: hsl(var(--popover, var(--card)));
  color: hsl(var(--popover-foreground, var(--foreground)));
  border: 1px solid hsl(var(--border));
  border-radius: 6px;
  padding: 8px 12px;
  font-size: 11px;
  line-height: 1.5;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
  z-index: 9999;
  pointer-events: none;
  white-space: normal;
}
.hint-tt-bubble .hint-tt-head {
  font-weight: 600;
  margin-bottom: 2px;
}
.hint-tt-bubble .hint-tt-row {
  padding-left: 8px;
  font-size: 10.5px;
  color: hsl(var(--muted-foreground));
  font-variant-numeric: tabular-nums;
}
.hint-tt-bubble .hint-tt-text {
  color: hsl(var(--popover-foreground, var(--foreground)));
}
</style>
