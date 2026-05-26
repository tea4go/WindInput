//go:build windows

package ui

import (
	"sync"

	"golang.org/x/sys/windows"
)

// WindowRegistry is a type-safe HWND -> *T mapping for wndProc callbacks.
type WindowRegistry[T any] struct {
	mu sync.RWMutex
	m  map[windows.HWND]*T
}

func NewWindowRegistry[T any]() *WindowRegistry[T] {
	return &WindowRegistry[T]{
		m: make(map[windows.HWND]*T),
	}
}

func (r *WindowRegistry[T]) Register(hwnd windows.HWND, w *T) {
	r.mu.Lock()
	r.m[hwnd] = w
	r.mu.Unlock()
}

func (r *WindowRegistry[T]) Unregister(hwnd windows.HWND) {
	r.mu.Lock()
	delete(r.m, hwnd)
	r.mu.Unlock()
}

func (r *WindowRegistry[T]) Get(hwnd windows.HWND) *T {
	r.mu.RLock()
	w := r.m[hwnd]
	r.mu.RUnlock()
	return w
}
