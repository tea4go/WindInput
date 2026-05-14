<script setup lang="ts">
// 普通短语编辑器: 单 textarea, 内容可以是字面量或带 $ 占位符的模板。

export interface NormalBuffer {
  text: string;
}

const props = defineProps<{ modelValue: NormalBuffer }>();
const emit = defineEmits<{
  (e: "update:modelValue", value: NormalBuffer): void;
}>();

function updateText(text: string) {
  emit("update:modelValue", { ...props.modelValue, text });
}
</script>

<template>
  <textarea
    :value="modelValue.text"
    @input="updateText(($event.target as HTMLTextAreaElement).value)"
    rows="2"
    placeholder="纯文本 / 含 $Y-$MM-$DD 等占位符的模板"
    class="flex w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring resize-y"
  />
</template>
