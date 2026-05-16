// PhraseFormBody 的状态类型与工厂函数 (拆到独立 .ts 文件,
// 避免在 <script setup> 中使用 ES module exports 的限制)。
import type { CmdOpenBuffer, CmdOpenSubKind } from "./editors/CmdOpenEditor.vue";

export type EditorType =
  | "normal"
  | "cmd-open"
  | "cmd-raw"
  | "array"
  | "array-ss";

// ArraySS 字符串数组元素: 字符串字面量或 $CC 命令 (CmdOpen 子集, 不含
// prefixVisible — 组级 prefix 由 $SS 外层 modifier 控制)。
export type ArraySSElement =
  | { kind: "string"; text: string }
  | {
      kind: "cmd";
      display: string;
      subKind: CmdOpenSubKind;
      target: string;
      args: string;
    };

export interface ArraySSBuffer {
  name: string;
  elements: ArraySSElement[];
}

export interface PhraseFormState {
  code: string;
  weight: number;
  editorType: EditorType;
  buffers: {
    normal: { text: string };
    cmdOpen: CmdOpenBuffer;
    cmdRaw: { text: string };
    array: { name: string; chars: string };
    arraySS: ArraySSBuffer;
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
      arraySS: { name: "", elements: [] },
    },
  };
}

export function newArraySSStringElement(text = ""): ArraySSElement {
  return { kind: "string", text };
}

export function newArraySSCmdElement(): ArraySSElement {
  return {
    kind: "cmd",
    display: "",
    subKind: "url",
    target: "",
    args: "",
  };
}
