// PhraseFormBody 的状态类型与工厂函数 (拆到独立 .ts 文件,
// 避免在 <script setup> 中使用 ES module exports 的限制)。
import type { CmdOpenBuffer } from "./editors/CmdOpenEditor.vue";

export type EditorType = "normal" | "cmd-open" | "cmd-raw" | "array";

export interface PhraseFormState {
  code: string;
  weight: number;
  editorType: EditorType;
  buffers: {
    normal: { text: string };
    cmdOpen: CmdOpenBuffer;
    cmdRaw: { text: string };
    array: { name: string; chars: string };
  };
}

export function createEmptyPhraseFormState(weight = 1000): PhraseFormState {
  return {
    code: "",
    weight,
    editorType: "normal",
    buffers: {
      normal: { text: "" },
      cmdOpen: {
        display: "",
        subKind: "url",
        target: "",
        args: "",
        prefixVisible: false,
      },
      cmdRaw: { text: "" },
      array: { name: "", chars: "" },
    },
  };
}
