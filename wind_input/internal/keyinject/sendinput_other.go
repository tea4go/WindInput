//go:build !windows

package keyinject

import "errors"

// ErrUnsupportedPlatform is returned by Tap/Sequence on non-Windows
// builds. keyinject is Windows-specific because the underlying syscall
// (user32.SendInput) does not exist on other platforms; non-Windows
// builds keep Parse usable for unit tests.
var ErrUnsupportedPlatform = errors.New("keyinject: not supported on this platform")

// Tap is a no-op stub on non-Windows platforms.
func Tap(c Combo) error { return ErrUnsupportedPlatform }

// Sequence is a no-op stub on non-Windows platforms.
func Sequence(combos ...Combo) error { return ErrUnsupportedPlatform }
