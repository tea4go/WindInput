#!/usr/bin/env swift
// list_input_sources.swift — 直接调 TextInputSources API 列出系统知道的所有输入源,
// 用于诊断 WindInput.app 是否真的被 macOS 注册到了 TIS 数据库.
//
// 用法:
//   swift scripts_mac/test/list_input_sources.swift            # 全量摘要 + 关键词高亮
//   swift scripts_mac/test/list_input_sources.swift wind       # 只显示 ID/Bundle 含 "wind" 的
import Carbon
import Foundation

func string(from src: TISInputSource, _ key: CFString) -> String? {
    guard let p = TISGetInputSourceProperty(src, key) else { return nil }
    return Unmanaged<CFString>.fromOpaque(p).takeUnretainedValue() as String
}

func bool(from src: TISInputSource, _ key: CFString) -> Bool {
    guard let p = TISGetInputSourceProperty(src, key) else { return false }
    return CFBooleanGetValue((Unmanaged<CFBoolean>.fromOpaque(p).takeUnretainedValue()))
}

// 用法:
//   swift scripts_mac/test/list_input_sources.swift            # 默认: 列出所有非 com.apple.* 输入源 (含第三方 IME)
//   swift scripts_mac/test/list_input_sources.swift wind       # 模糊搜 ID/bundle 含 "wind" 的
//   swift scripts_mac/test/list_input_sources.swift --all      # 全 320 项原样列出
let raw = CommandLine.arguments.dropFirst().first ?? ""
let filter = raw.lowercased()
let showAll = (raw == "--all")
let listRef = TISCreateInputSourceList(nil, true)
guard let arr = listRef?.takeRetainedValue() as? [TISInputSource] else {
    print("!! TISCreateInputSourceList 返回空")
    exit(1)
}
print("总数: \(arr.count) 输入源 (includeAllInstalled=true)")
print(String(repeating: "-", count: 70))

var matched = 0
let hi = ["wind", "huanfeng", "qingg", "aodaren"]
for src in arr {
    let id    = string(from: src, kTISPropertyInputSourceID) ?? "?"
    let bid   = string(from: src, kTISPropertyBundleID) ?? "?"
    // kTISPropertyInputSourceLanguages 实际返回 CFArray, 别按 String 强转 (会 NSException 崩).
    let lang: String = {
        guard let p = TISGetInputSourceProperty(src, kTISPropertyInputSourceLanguages) else { return "?" }
        let arr = Unmanaged<CFArray>.fromOpaque(p).takeUnretainedValue() as? [String] ?? []
        return arr.joined(separator: ",")
    }()
    let cat   = string(from: src, kTISPropertyInputSourceCategory) ?? "?"
    let kind  = string(from: src, kTISPropertyInputSourceType) ?? "?"
    let enab  = bool(from: src, kTISPropertyInputSourceIsEnabled)
    let sel   = bool(from: src, kTISPropertyInputSourceIsSelectCapable)

    let lower = (id + " " + bid).lowercased()
    let shouldShow: Bool
    if showAll {
        shouldShow = true
    } else if !filter.isEmpty {
        shouldShow = lower.contains(filter)
    } else {
        // 默认: 任何非 Apple 自带的输入源 (com.apple.*) 都列出 —
        // 三方 IME (Qingg / WindInput / Squirrel...) 全在这一桶.
        shouldShow = !id.hasPrefix("com.apple.")
    }
    if shouldShow {
        matched += 1
        print("ID     : \(id)")
        print("bundle : \(bid)")
        print("lang   : \(lang)")
        print("cat    : \(cat)")
        print("kind   : \(kind)")
        print("enabled: \(enab)  selectable: \(sel)")
        print("")
    }
}
print(String(repeating: "-", count: 70))
print("匹配 \(matched) 项 (过滤词: \(filter.isEmpty ? "wind/huanfeng/qingg/aodaren" : filter))")

// 若仍想看全表头, 跑下面这条:
//   swift scripts_mac/test/list_input_sources.swift '' | grep -E '^(ID|bundle)' | sort -u
