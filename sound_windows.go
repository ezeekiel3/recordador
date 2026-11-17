//go:build windows

package main

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	winmm          = syscall.NewLazyDLL("winmm.dll")
	procPlaySoundW = winmm.NewProc("PlaySoundW")
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	procBeep       = kernel32.NewProc("Beep")
	user32         = syscall.NewLazyDLL("user32.dll")
	procMessageBeep = user32.NewProc("MessageBeep")
)

const (
	SND_ASYNC = 0x0001
	SND_ALIAS = 0x00010000
)

// PlaySystemSound plays a Windows system sound or a sequence of beeps
func PlaySystemSound(soundType string) {
	go func() {
		switch soundType {
		case "digital", "despertador":
			// Sequence of rhythmic beeps
			for i := 0; i < 3; i++ {
				procBeep.Call(1200, 120)
				time.Sleep(80 * time.Millisecond)
				procBeep.Call(1200, 120)
				time.Sleep(80 * time.Millisecond)
				procBeep.Call(1500, 200)
				time.Sleep(300 * time.Millisecond)
			}
		case "melodia", "suave":
			// Gentle chime sequence
			notes := []uint32{523, 659, 784, 1046} // C5, E5, G5, C6
			for _, note := range notes {
				procBeep.Call(uintptr(note), 150)
				time.Sleep(60 * time.Millisecond)
			}
		case "ding":
			procMessageBeep.Call(0x40) // MB_ICONASTERISK
		case "campana":
			fallthrough
		default:
			// System notification sound + chime
			namePtr, err := syscall.UTF16PtrFromString("SystemNotification")
			if err == nil {
				procPlaySoundW.Call(uintptr(unsafe.Pointer(namePtr)), 0, uintptr(SND_ASYNC|SND_ALIAS))
			}
			time.Sleep(100 * time.Millisecond)
			procBeep.Call(880, 250)
			time.Sleep(50 * time.Millisecond)
			procBeep.Call(1175, 400)
		}
	}()
}
