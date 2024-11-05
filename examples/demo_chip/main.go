package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"log"

	"github.com/arl/blip"
	"github.com/arl/blip/wave"
)

const sampleRate = 44100      /* 44.1 kHz sample rate*/
const clockRate = 1789772.727 /* 1.78 MHz clock rate */

var bl *blip.Buffer
var wv *wave.Writer

// indices into regs
const (
	period = iota
	volume
	timbre
)

type channel struct {
	run   func(c *channel, endTime int)
	gain  int    // overall volume of channel
	regs  [3]int // period (clocks between deltas), volume, timbre
	time  int    // clock time of next delta
	phase int    // position within waveform
	amp   int    // current amplitude in delta buffer
}

// updateAmp updates amplitude of waveform in delta buffer.
func (m *channel) updateAmp(new_amp int) {
	delta := new_amp*m.gain - m.amp
	m.amp += delta
	bl.AddDelta(uint64(m.time), int32(delta))
}

// runSquare runs square wave to endTime
func (m *channel) runSquare(endTime int) {
	for ; m.time < endTime; m.time += m.regs[period] {
		m.phase = (m.phase + 1) % 8
		amp := 0
		if m.phase >= m.regs[timbre] {
			amp = m.regs[volume]
		}
		m.updateAmp(amp)
	}
}

// Runs triangle wave to endTime
func (m *channel) runTriangle(endTime int) {
	for ; m.time < endTime; m.time += m.regs[period] {
		// phase only increments when volume is non-zero
		// (volume is otherwise ignored)
		if m.regs[volume] != 0 {
			m.phase = (m.phase + 1) % 32
			amp := m.phase
			if m.phase >= 16 {
				amp = 31 - m.phase
			}
			m.updateAmp(amp)
		}
	}
}

// runNoise runs noise to endTime
func (m *channel) runNoise(endTime int) {
	// phase is noise LFSR, which must never be zero
	if m.phase == 0 {
		m.phase = 1
	}

	for ; m.time < endTime; m.time += m.regs[period] {
		m.phase = ((m.phase & 1) * m.regs[timbre]) ^ (m.phase >> 1)
		m.updateAmp((m.phase & 1) * m.regs[volume])
	}
}

const masterVol = 65536 / 15
const chanCount = 4

var chans = [chanCount]channel{
	{(*channel).runSquare, masterVol * 26 / 100, [3]int{10, 0, 0}, 0, 0, 0},
	{(*channel).runSquare, masterVol * 26 / 100, [3]int{10, 0, 0}, 0, 0, 0},
	{(*channel).runTriangle, masterVol * 30 / 100, [3]int{10, 0, 0}, 0, 0, 0},
	{(*channel).runNoise, masterVol * 18 / 100, [3]int{10, 0, 0}, 0, 0, 0},
}

// writeChannel runs channel to specified time,
// then writes data to channel's register
func writeChannel(time, ch, addr, data int) {
	c := &chans[ch]
	c.run(c, time)
	c.regs[addr] = data
}

// endFrame runs time frame and flushes samples
func endFrame(endTime int) {
	for i := range chanCount {
		chans[i].run(&chans[i], endTime)
		chans[i].time -= endTime
	}

	bl.EndFrame(endTime)

	const tempSize = 1024
	var temp [tempSize]int16

	for bl.SamplesAvailable() > 0 {
		// count is number of samples actually read (in case there
		// were fewer than tempSize samples actually available)
		count := bl.ReadSamples(temp[:], tempSize, blip.Mono)
		wv.Write(temp[:count])
	}
}

//go:embed chipLog.txt
var chipLog []byte

func main() {
	bl, _ = blip.NewBuffer(sampleRate / 10)
	bl.SetRates(clockRate, sampleRate)

	// Play back logged writes and record to wave sound file
	in := bytes.NewReader(chipLog)

	var err error
	wv, err = wave.NewFile("out.wav", sampleRate)
	if err != nil {
		log.Fatal(err)
	}

	for wv.SampleCount() < 120*sampleRate {
		// In an emulator these writes would be generated by the emulated CPU
		var time, ch, addr, data int
		if n, _ := fmt.Fscanf(in, "%d %d %d %d\n", &time, &ch, &addr, &data); n < 4 {
			break
		}

		if ch < chanCount {
			writeChannel(time, ch, addr, data)
		} else {
			endFrame(time)
		}
	}

	wv.Close()
}