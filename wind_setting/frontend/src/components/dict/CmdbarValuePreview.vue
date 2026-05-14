<script setup lang="ts">
import { ref, watch, onUnmounted, computed } from "vue";
import {
  validatePhraseValue,
  type PhraseValidateValueReply,
} from "@/api/wails";
import { Badge } from "@/components/ui/badge";

const props = withDefaults(
  defineProps<{
    value: string;
    debounceMs?: number;
  }>(),
  { debounceMs: 300 },
);

const emit = defineEmits<{
  (e: "validation-error", hasError: boolean): void;
}>();

const result = ref<PhraseValidateValueReply | null>(null);

let timer: ReturnType<typeof setTimeout> | null = null;
let reqSeq = 0;

function clearTimer() {
  if (timer) {
    clearTimeout(timer);
    timer = null;
  }
}

async function runValidate(value: string) {
  const seq = ++reqSeq;
  try {
    const reply = await validatePhraseValue(value);
    if (seq !== reqSeq) return;
    result.value = reply;
    emit("validation-error", reply.kind === "error");
  } catch {
    if (seq !== reqSeq) return;
    result.value = null;
    emit("validation-error", false);
  }
}

watch(
  () => props.value,
  (val) => {
    clearTimer();
    const v = (val ?? "").trim();
    if (!v) {
      result.value = null;
      emit("validation-error", false);
      return;
    }
    timer = setTimeout(() => {
      runValidate(val);
    }, props.debounceMs);
  },
  { immediate: true },
);

onUnmounted(() => {
  clearTimer();
});

// 错误信息: 完整内容 (hover title 显示), 单行 truncate
const errorMsg = computed(() => result.value?.error_msg || "请检查语法");
</script>

<template>
  <!-- 始终渲染容器以预留固定空间, 避免对话框因 kind 切换而跳动。
       单行高度: 24px (badge 行高约 18~20px + 上下间距)。 -->
  <div class="mt-1.5 text-xs leading-relaxed min-h-[24px] flex items-center">
    <!-- literal / 空值: 渲染空容器, 保持 min-height -->
    <template v-if="!result || result.kind === 'literal'">
      <span class="sr-only">无附加信息</span>
    </template>

    <!-- 命令 (仅精确匹配) -->
    <template v-else-if="result.kind === 'command'">
      <div class="flex items-center gap-1.5 min-w-0 w-full">
        <Badge
          variant="secondary"
          class="text-[10px] px-1.5 py-0 bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-300 border-0 shrink-0"
        >
          命令
        </Badge>
        <span
          class="text-muted-foreground truncate"
          :title="result.display || ''"
        >
          {{ result.display || "(空)" }}
        </span>
      </div>
    </template>

    <!-- 命令 (含前缀展开) -->
    <template v-else-if="result.kind === 'command-prefix'">
      <div class="flex items-center gap-1.5 min-w-0 w-full">
        <Badge
          variant="secondary"
          class="text-[10px] px-1.5 py-0 bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-300 border-0 shrink-0"
        >
          命令·前缀
        </Badge>
        <span
          class="text-muted-foreground truncate"
          :title="result.display || ''"
        >
          {{ result.display || "(空)" }}
        </span>
      </div>
    </template>

    <!-- 字符组 -->
    <template v-else-if="result.kind === 'array'">
      <div class="flex items-center gap-1.5 min-w-0 w-full">
        <Badge
          variant="secondary"
          class="text-[10px] px-1.5 py-0 bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300 border-0 shrink-0"
        >
          字符组
        </Badge>
        <span
          class="text-muted-foreground truncate"
          :title="result.display || ''"
        >
          {{ result.display || "(未命名)" }}
        </span>
      </div>
    </template>

    <!-- 模板 -->
    <template v-else-if="result.kind === 'template'">
      <div class="flex items-center gap-1.5 min-w-0 w-full">
        <Badge
          variant="secondary"
          class="text-[10px] px-1.5 py-0 bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300 border-0 shrink-0"
        >
          模板
        </Badge>
        <span
          class="text-muted-foreground truncate"
          :title="result.display || ''"
        >
          {{ result.display || "(空)" }}
        </span>
      </div>
    </template>

    <!-- 错误: 单行截断 + 完整 title -->
    <template v-else-if="result.kind === 'error'">
      <div class="flex items-center gap-1.5 min-w-0 w-full" :title="errorMsg">
        <Badge
          variant="secondary"
          class="text-[10px] px-1.5 py-0 bg-destructive/15 text-destructive border-0 shrink-0"
        >
          错误
        </Badge>
        <span class="text-destructive truncate">
          {{ errorMsg }}
        </span>
      </div>
    </template>
  </div>
</template>
