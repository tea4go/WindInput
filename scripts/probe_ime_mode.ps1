# 实时探测当前前台窗口的 IME 转换模式（IME_CMODE_NATIVE 位 = 中文）
# 用法: pwsh scripts/probe_ime_mode.ps1
#   然后用鼠标点击想观察的应用（记事本、浏览器、WPS 等）让其获得焦点，
#   再用快捷键切中英文，看本脚本输出的 NATIVE 标志是否实时翻转。
#
# 与 KBLSwitch 同套路：用 ImmGetDefaultIMEWnd + WM_IME_CONTROL/IMC_GETCONVERSIONMODE
# 跨线程查询，能在 Win11 新版 Notepad / TSF-only 应用里也读出来。
Add-Type -Namespace W -Name Ime -MemberDefinition @'
[System.Runtime.InteropServices.DllImport("user32.dll")]
public static extern System.IntPtr GetForegroundWindow();
[System.Runtime.InteropServices.DllImport("imm32.dll")]
public static extern System.IntPtr ImmGetDefaultIMEWnd(System.IntPtr hWnd);
[System.Runtime.InteropServices.DllImport("user32.dll", CharSet=System.Runtime.InteropServices.CharSet.Auto)]
public static extern int GetWindowText(System.IntPtr hWnd, System.Text.StringBuilder s, int n);
[System.Runtime.InteropServices.DllImport("user32.dll", CharSet=System.Runtime.InteropServices.CharSet.Auto)]
public static extern System.IntPtr SendMessage(System.IntPtr hWnd, uint msg, System.IntPtr wp, System.IntPtr lp);
'@

$WM_IME_CONTROL       = 0x0283
$IMC_GETCONVERSIONMODE = 0x0001
$IMC_GETOPENSTATUS     = 0x0005
$IME_CMODE_NATIVE      = 0x0001
$IME_CMODE_FULLSHAPE   = 0x0008
$IME_CMODE_SYMBOL      = 0x0400

while ($true) {
    $hwnd = [W.Ime]::GetForegroundWindow()
    $sb = New-Object System.Text.StringBuilder 256
    [W.Ime]::GetWindowText($hwnd, $sb, 256) | Out-Null

    $imeWnd = [W.Ime]::ImmGetDefaultIMEWnd($hwnd)
    $open = 0; $conv = 0
    if ($imeWnd -ne [IntPtr]::Zero) {
        $open = [W.Ime]::SendMessage($imeWnd, $WM_IME_CONTROL, [IntPtr]$IMC_GETOPENSTATUS, [IntPtr]::Zero).ToInt32()
        $conv = [W.Ime]::SendMessage($imeWnd, $WM_IME_CONTROL, [IntPtr]$IMC_GETCONVERSIONMODE, [IntPtr]::Zero).ToInt32()
    }

    $mode = if (-not $open) {
        'OFF'
    } elseif ($conv -band $IME_CMODE_NATIVE) {
        'CN'
    } else {
        'EN'
    }
    $stamp = (Get-Date).ToString('HH:mm:ss.fff')
    $title = $sb.ToString()
    if ($title.Length -gt 40) { $title = $title.Substring(0, 40) }
    "$stamp  $mode  open=$open conv=0x{0:X4}  imeWnd=0x{1:X}  win=[{2}]" -f $conv, $imeWnd.ToInt64(), $title
    Start-Sleep -Milliseconds 200
}
