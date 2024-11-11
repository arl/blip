package main

import (
	"log"

	"github.com/arl/blip"
	"github.com/arl/blip/wave"
)

// Implements a simple square wave generator as might be used in an analog
// waveform synthesizer. Runs two square waves together.

const sampleRate = 44100

// Use the maximum supported clock rate, which is about one million times the
// sample rate, 46 GHz in this case. This gives all the accuracy needed, even
// for extremely fine frequency control.
const clockRate = sampleRate * blip.MaxRatio

var bl *blip.Buffer

type wavebuf struct {
	frequency float64 // cycles per second
	volume    float64 // 0.0 to 1.0
	phase     int     // +1 or -1
	time      int     // clock time of next delta
	amp       int     // current amplitude in delta buffer
}

var waves = [2]wavebuf{
	{
		phase:     1,
		volume:    0.0,
		frequency: 16000,
	}, {
		phase:     1,
		volume:    0.5,
		frequency: 1000,
	},
}

func (w *wavebuf) run(clocks int) {
	// Clocks for each half of square wave cycle
	period := int(clockRate/w.frequency/2 + 0.5)

	// Convert volume to 16-bit sample range (divided by 2 because it's bipolar)
	volume := int(w.volume*65536/2 + 0.5)

	// Add deltas that fall before end time
	for ; w.time < clocks; w.time += period {
		delta := w.phase*volume - w.amp
		w.amp += delta
		bl.AddDelta(uint64(w.time), int32(delta))
		w.phase = -w.phase
	}

	w.time -= clocks // adjust for next time frame
}

// Generates enough samples to exactly fill out
func genSamples(out []int16) {
	clocks := bl.ClocksNeeded(len(out))
	waves[0].run(clocks)
	waves[1].run(clocks)

	bl.EndFrame(clocks)
	bl.ReadSamples(out, len(out), blip.Mono)
}

func main() {
	bl = blip.NewBuffer(sampleRate / 10)
	bl.SetRates(clockRate, sampleRate)

	w, err := wave.NewFile("out.wav", sampleRate)
	if err != nil {
		log.Fatal(err)
	}

	const samples = 1024
	var temp [samples]int16

	for w.SampleCount() < 2*sampleRate*2 {
		genSamples(temp[:])
		w.Write(temp[:samples])

		// Slowly increase volume and lower pitch
		waves[0].volume += 0.005
		waves[0].frequency *= 0.950

		// Slowly decrease volume and raise pitch
		waves[1].volume -= 0.002
		waves[1].frequency *= 1.010
	}
	w.Close()
}
