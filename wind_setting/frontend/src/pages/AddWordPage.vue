<template>
  <Dialog :open="open" @update:open="onOpenUpdate">
    <DialogContent
      class="max-w-xl max-h-[85vh] flex flex-col p-0 gap-0 overflow-hidden"
    >
      <DialogHeader class="px-6 pt-6 pb-4 border-b shrink-0">
        <DialogTitle>
          {{ isEditing ? "编辑用户词库" : "添加用户词库"
          }}{{ schemaName ? ` (${schemaName})` : "" }}
        </DialogTitle>
      </DialogHeader>

      <div class="flex-1 overflow-y-auto px-6 py-4 min-h-0">
        <PhraseFormBody
          v-model="formState"
          :show-code-gen="hasAutoEncode"
          :code-generating="generatingCode"
          :code-label="codeLabel"
          :code-placeholder="codePlaceholder"
          :code-hint="codeHint"
          :on-generate-code="onClickGenerate"
          @composed-text="onComposedTextChanged"
          @validation-error="hasValidationError = $event"
          @code-input="onCodeManualInput"
        />
      </div>

      <DialogFooter
        class="px-6 py-4 border-t shrink-0 bg-background flex flex-row flex-nowrap items-center justify-end gap-2"
      >
        <label
          v-if="!isEditing"
          class="mr-auto flex items-center gap-2 text-sm text-muted-foreground select-none whitespace-nowrap"
        >
          <Checkbox
            :checked="continuousAdd"
            @update:checked="(v: boolean) => (continuousAdd = v)"
          />
          <span>连续添加</span>
        </label>
        <Button
          variant="outline"
          size="sm"
          class="shrink-0"
          @click="handleCancel"
          >取消</Button
        >
        <Button
          size="sm"
          class="shrink-0"
          @click="handleAdd"
          :disabled="!canAdd || adding"
        >
          {{ adding ? "保存中..." : "保存" }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch } from "vue";
import * as wailsApi from "../api/wails";
import { useToast } from "../composables/useToast";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import PhraseFormBody from "@/components/dict/PhraseFormBody.vue";
import {
  createEmptyPhraseFormState,
  type PhraseFormState,
} from "@/components/dict/phraseForm";

interface SchemaItem {
  id: string;
  name: string;
  engineType: string;
  aliasIds: string[];
}

const props = defineProps<{
  initialText?: string;
  initialCode?: string;
  initialSchema?: string;
  standalone?: boolean;
  // 编辑模式: 传入 editingItem 后, 标题/按钮文案切到"编辑"且隐藏连续添加 checkbox
  editingItem?: { text: string; code: string; weight?: number } | null;
}>();

const isEditing = computed(() => !!props.editingItem);
const continuousAdd = ref(false);
// 用户是否已手动编辑过编码字段; 为 true 时 watch(composedText) 不再自动重生成
const codeManuallyEdited = ref(false);

const emit = defineEmits<{
  close: [];
}>();

// shadcn Dialog 的开关由本组件内部 ref 控制, 关闭事件再外抛 close。
// 首帧渲染优化: open 初始为 false, 等 onMounted 把 schemas / 自动生成的编码都
// 准备好后再置 true, 避免"空白对话框 → 数据填充"的二次渲染。
const open = ref(false);

function onOpenUpdate(val: boolean) {
  open.value = val;
  if (!val) emit("close");
}

// 编辑模式: 用 editingItem.weight 作为初始权重 (>0 时); 否则默认 1200。
const initialWeight =
  props.editingItem &&
  typeof props.editingItem.weight === "number" &&
  props.editingItem.weight > 0
    ? props.editingItem.weight
    : 1200;
const formState = ref<PhraseFormState>(
  createEmptyPhraseFormState(initialWeight),
);
// 初始 text/code 注入
formState.value.buffers.normal.text = props.initialText ?? "";
formState.value.code = props.initialCode ?? "";

const composedText = ref(formState.value.buffers.normal.text);
const hasValidationError = ref(false);

const schemaID = ref(props.initialSchema ?? "");
const schemas = ref<SchemaItem[]>([]);
const { toast } = useToast();
const adding = ref(false);
const generatingCode = ref(false);

const currentSchema = computed(() =>
  schemas.value.find((s) => s.id === schemaID.value),
);
const schemaName = computed(() => currentSchema.value?.name ?? "");
const isPinyin = computed(() => currentSchema.value?.engineType === "pinyin");
const isCodetable = computed(
  () => currentSchema.value?.engineType === "codetable",
);

const hasAutoEncode = computed(() => isPinyin.value || isCodetable.value);

const codeLabel = computed(() => (isPinyin.value ? "拼音" : "编码"));
const codePlaceholder = computed(() =>
  isPinyin.value ? "全拼（如 nihao）" : "编码（如 abcd）",
);
const codeHint = computed(() =>
  hasAutoEncode.value ? "(自动生成, 可手动修改)" : "",
);

const wordText = computed(() => composedText.value);

const canAdd = computed(() => {
  if (hasValidationError.value) return false;
  const hasText = wordText.value.trim().length >= 1;
  const w = formState.value.weight;
  const hasWeight = w >= 0 && w <= 10000;
  if (isPinyin.value) {
    return hasText && hasWeight;
  }
  return hasText && formState.value.code.trim().length > 0 && hasWeight;
});

async function autoGenerateCode() {
  const text = wordText.value.trim();
  if (!text) return;
  generatingCode.value = true;
  try {
    let code = "";
    if (isPinyin.value) {
      code = await wailsApi.generatePinyinCode(text);
    } else if (isCodetable.value) {
      code = await wailsApi.encodeWordForSchema(schemaID.value, text);
    }
    // 容错: 生成结果为空时保留现有编码, 不清空用户已输入或之前生成的内容
    if (code) {
      formState.value.code = code;
    }
  } catch {
    // 生成失败时保留用户输入，不强制清空
  } finally {
    generatingCode.value = false;
  }
}

// 点击 ↺ 按钮: 视为用户主动请求生成, 重置手改标记
function onClickGenerate() {
  codeManuallyEdited.value = false;
  autoGenerateCode();
}

// 用户在编码字段输入 (来自 PhraseFormBody 的 code-input 事件)
function onCodeManualInput() {
  codeManuallyEdited.value = true;
}

let autoGenTimer: ReturnType<typeof setTimeout> | null = null;

function onComposedTextChanged(text: string) {
  composedText.value = text;
}

// 当内容文本变化, 且方案支持自动编码时, 防抖触发;
// 用户手改过编码后不再自动覆盖。
watch(composedText, () => {
  if (!hasAutoEncode.value) return;
  if (codeManuallyEdited.value) return;
  if (autoGenTimer) clearTimeout(autoGenTimer);
  autoGenTimer = setTimeout(() => {
    autoGenerateCode();
  }, 300);
});

// 切换方案时重新生成编码
watch(schemaID, () => {
  if (hasAutoEncode.value && wordText.value.trim()) {
    autoGenerateCode();
  } else if (!hasAutoEncode.value) {
    if (formState.value.code && !props.initialCode) {
      formState.value.code = "";
    }
  }
});

async function handleAdd() {
  if (!canAdd.value || adding.value) return;

  const text = wordText.value.trim();
  const code = formState.value.code.trim();
  const weight = formState.value.weight;

  adding.value = true;
  try {
    if (isEditing.value && props.editingItem) {
      // 编辑模式: code/text 没变 → 仅更新权重; 否则先删旧条目再加新条目。
      const oldCode = props.editingItem.code;
      const oldText = props.editingItem.text;
      const sameKey = oldCode === code && oldText === text;
      if (sameKey) {
        if (schemaID.value) {
          await wailsApi.updateUserWordForSchema(
            schemaID.value,
            code,
            text,
            weight,
          );
        } else {
          await wailsApi.updateUserWord(code, text, weight);
        }
      } else {
        if (schemaID.value) {
          await wailsApi.removeUserWordForSchema(
            schemaID.value,
            oldCode,
            oldText,
          );
          await wailsApi.addUserWordForSchema(
            schemaID.value,
            code,
            text,
            weight,
          );
        } else {
          await wailsApi.removeUserWord(oldCode, oldText);
          await wailsApi.addUserWord(code, text, weight);
        }
      }
    } else if (schemaID.value) {
      const existing = await wailsApi.getUserDictBySchema(schemaID.value);
      const found = existing.find((w) => w.code === code && w.text === text);
      if (found) {
        toast(`该词已存在 (${text}: ${code})，已更新权重`);
        await wailsApi.addUserWordForSchema(schemaID.value, code, text, weight);
        await wailsApi.notifyReload("userdict");
        adding.value = false;
        return;
      }
      await wailsApi.addUserWordForSchema(schemaID.value, code, text, weight);
    } else {
      await wailsApi.addUserWord(code, text, weight);
    }
    await wailsApi.notifyReload("userdict");
    const displayCode = code || "(自动生成)";
    toast(
      isEditing.value
        ? `已更新: ${text} (${displayCode})`
        : `已保存: ${text} (${displayCode})`,
    );

    // 编辑模式 / 非连续添加: 保存后关闭对话框
    if (isEditing.value || !continuousAdd.value) {
      adding.value = false;
      open.value = false;
      emit("close");
      return;
    }

    // 连续添加: 清空 (保留方案 / 权重), 继续加词;
    // 同时重置编码手改标记, 下次输入内容会自动重生成
    formState.value.buffers.normal.text = "";
    formState.value.code = "";
    composedText.value = "";
    codeManuallyEdited.value = false;
  } catch (e: any) {
    toast(`添加失败: ${e.message || e}`, "error");
  } finally {
    adding.value = false;
  }
}

function handleCancel() {
  open.value = false;
  emit("close");
}

onMounted(async () => {
  try {
    const list = await wailsApi.getEnabledSchemasWithDictStats();
    schemas.value = list.map((s) => ({
      id: s.schema_id,
      name: s.schema_name,
      engineType: s.engine_type,
      aliasIds: s.alias_ids || [],
    }));
  } catch {
    schemas.value = [];
  }

  // 用 schemas 列表做别名匹配，修正 schemaID（双拼方案合并后 id 可能变为 "pinyin"）
  if (props.initialSchema) {
    const matched =
      schemas.value.find((s) => s.id === props.initialSchema) ||
      schemas.value.find((s) => s.aliasIds.includes(props.initialSchema!));
    schemaID.value = matched ? matched.id : schemas.value[0]?.id || "";
  } else if (schemas.value.length > 0 && !schemaID.value) {
    schemaID.value = schemas.value[0].id;
  }

  // 初始化时若有词语但无编码，自动生成
  if (hasAutoEncode.value && wordText.value.trim() && !formState.value.code) {
    await autoGenerateCode();
  }

  await nextTick();
  // 所有依赖数据 (schemas / 编码) 已就绪, 此时再展示对话框, 避免首帧空白。
  open.value = true;
});
</script>
