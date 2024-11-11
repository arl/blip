package main

// Implements a simple square wave generator as would be in a sound chip emulator.

import (
	"os"

	"github.com/arl/blip"
	"github.com/arl/blip/wave"
)

const sampleRate = 44100             // 44.1 kHz sample rate
const clockRate float64 = 3579545.45 // 3.58 MHz clock rate

// Square wave state
var time int       // clock time of next delta
var period int = 1 // clocks between deltas
var phase = +1     // +1 or -1
var volume int
var amp int // current amplitude in delta buffer

func runWave(bl *blip.Buffer, clocks int) {
	// Add deltas that fall before end time
	for ; time < clocks; time += period {
		delta := phase*volume - amp
		amp += delta
		bl.AddDelta(uint64(time), int32(delta))
		phase = -phase
	}
}

func flushSamples(bl *blip.Buffer, wv *wave.Writer) {
	// If we only wanted 512-sample chunks, never smaller, we would
	// do >= 512 instead of > 0. Any remaining samples would be left
	// in buffer for next time.
	for bl.SamplesAvailable() > 0 {
		temp := make([]int16, 512)

		// count is number of samples actually read (in case there
		// were fewer than temp_size samples actually available)
		count := bl.ReadSamples(temp, len(temp), blip.Mono)
		wv.Write(temp[:count])
	}
}

func main() {
	bl := blip.NewBuffer(sampleRate / 10)
	bl.SetRates(clockRate, sampleRate)

	// Record output to a wave file
	f, err := os.Create("out.wav")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	wv := wave.NewWriter(f, sampleRate)
	defer wv.Close()

	for wv.SampleCount() < 2*sampleRate {
		// Generate 1/60 second each time through loop
		fclocks := clockRate / 60.0
		clocks := int(fclocks)

		runWave(bl, clocks)
		bl.EndFrame(clocks)
		time -= clocks // adjust for new time frame

		flushSamples(bl, wv)

		// Slowly increase volume and lower
		volume += 100
		period += period/28 + 3
	}
}
