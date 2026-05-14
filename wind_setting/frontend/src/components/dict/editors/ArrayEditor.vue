<script setup lang="ts">
// 字符组编辑器: $AA("名称", "字符列表") 的结构化输入。

import { Input } from "@/components/ui/input";

export interface ArrayBuffer {
  name: string;
  chars: string;
}

const props = defineProps<{ modelValue: ArrayBuffer }>();
const emit = defineEmits<{
  (e: "update:modelValue", value: ArrayBuffer): void;
}>();

function updateName(name: string) {
  emit("update:modelValue", { ...props.modelValue, name });
}
function updateChars(chars: string) {
  emit("update:modelValue", { ...props.modelValue, chars });
}
</script>

<template>
  <div class="space-y-2">
    <div class="grid grid-cols-[80px_1fr] items-center gap-2">
      <label class="text-sm font-medium text-right">名称</label>
      <Input
        :model-value="modelValue.name"
        @update:model-value="updateName(String($event ?? ''))"
        placeholder="如: 标点"
      />
    </div>
    <div class="grid grid-cols-[80px_1fr] items-start gap-2">
      <label class="text-sm font-medium text-right pt-2">字符列表</label>
      <textarea
        :value="modelValue.chars"
        @input="updateChars(($event.target as HTMLTextAreaElement).value)"
        rows="2"
        placeholder="如: 、。·"
        class="flex w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring resize-y"
      />
    </div>
  </div>
</template>
