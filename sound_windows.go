//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"sync"
	"syscall"
	"unsafe"
)

var (
	winmm          = syscall.NewLazyDLL("winmm.dll")
	procPlaySoundW = winmm.NewProc("PlaySoundW")

	volMu         sync.RWMutex
	currentVolume = 0.8 // 80% por defecto (0.0 a 1.0)
)

const (
	SND_ASYNC  = 0x0001
	SND_MEMORY = 0x0004
)

func SetGlobalVolume(percent int) {
	volMu.Lock()
	defer volMu.Unlock()
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	currentVolume = float64(percent) / 100.0
}

func GetGlobalVolume() int {
	volMu.RLock()
	defer volMu.RUnlock()
	return int(math.Round(currentVolume * 100))
}

// makeWAV genera en memoria un archivo de audio WAV PCM de 16 bits y 44.1kHz
func makeWAV(durationSec float64, generator func(t float64) float64) []byte {
	volMu.RLock()
	vol := currentVolume
	volMu.RUnlock()

	if vol <= 0.001 {
		return nil
	}

	sampleRate := 44100
	numSamples := int(float64(sampleRate) * durationSec)
	dataSize := numSamples * 2

	buf := new(bytes.Buffer)

	// Encabezado RIFF
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")

	// Chunk "fmt "
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1))      // AudioFormat PCM
	binary.Write(buf, binary.LittleEndian, uint16(1))      // Mono
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate*2))
	binary.Write(buf, binary.LittleEndian, uint16(2))
	binary.Write(buf, binary.LittleEndian, uint16(16))     // 16 bits

	// Chunk "data"
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(dataSize))

	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		raw := generator(t) * vol
		if raw > 1.0 {
			raw = 1.0
		} else if raw < -1.0 {
			raw = -1.0
		}
		val := int16(raw * 32767.0)
		binary.Write(buf, binary.LittleEndian, val)
	}

	return buf.Bytes()
}

// playWAVAsync reproduce un buffer WAV en memoria mediante PlaySoundW
func playWAVAsync(wav []byte) {
	if len(wav) == 0 {
		return
	}
	procPlaySoundW.Call(
		uintptr(unsafe.Pointer(&wav[0])),
		0,
		uintptr(SND_ASYNC|SND_MEMORY),
	)
}

// PlaySystemSound reproduce el sonido seleccionado con el volumen actual
func PlaySystemSound(soundType string) {
	go func() {
		var wav []byte

		switch soundType {
		case "ding":
			// Ding rápido: campana cristalina E6 (1318.5 Hz) con decaimiento rápido
			wav = makeWAV(0.65, func(t float64) float64 {
				decay := math.Exp(-t * 6.5)
				f1 := math.Sin(2 * math.Pi * 1318.5 * t)
				f2 := math.Sin(2 * math.Pi * 2637.0 * t) * 0.35
				return (f1 + f2) * decay
			})

		case "digital":
			// Despertador digital: 3 pares de bips electrónicos rítmicos
			wav = makeWAV(1.15, func(t float64) float64 {
				// Pulsos cada 0.35s
				pattern := math.Mod(t, 0.38)
				if pattern < 0.09 {
					return math.Sin(2 * math.Pi * 1250 * t) * 0.9
				} else if pattern >= 0.12 && pattern < 0.21 {
					return math.Sin(2 * math.Pi * 1550 * t) * 0.9
				}
				return 0
			})

		case "melodia":
			// Melodía armónica ascendente: C5 (523Hz), E5 (659Hz), G5 (784Hz), C6 (1046Hz)
			wav = makeWAV(1.2, func(t float64) float64 {
				notes := []float64{523.25, 659.25, 783.99, 1046.5}
				idx := int(t / 0.25)
				if idx >= len(notes) {
					idx = len(notes) - 1
				}
				freq := notes[idx]
				subT := math.Mod(t, 0.25)
				decay := math.Exp(-subT * 4.0)
				return math.Sin(2*math.Pi*freq*t) * decay
			})

		case "campana":
			fallthrough
		default:
			// Campana clásica: acordes armónicos ricos A5 (880Hz) + E6 (1320Hz)
			wav = makeWAV(1.2, func(t float64) float64 {
				decay := math.Exp(-t * 3.5)
				s1 := math.Sin(2 * math.Pi * 880.0 * t) * 0.6
				s2 := math.Sin(2 * math.Pi * 1320.0 * t) * 0.3
				s3 := math.Sin(2 * math.Pi * 1760.0 * t) * 0.2
				return (s1 + s2 + s3) * decay
			})
		}

		if len(wav) > 0 {
			playWAVAsync(wav)
		}
	}()
}
