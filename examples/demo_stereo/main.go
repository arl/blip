package main

// Generates left and right channels and interleaves them together

import (
	"log"

	"github.com/arl/blip"
	"github.com/arl/blip/wave"
)

const sampleRate = 44100
const clockRate = sampleRate * blip.MaxRatio

// Delta buffers for left and right channels
var blips [2]*blip.Buffer

type wavebuf struct {
	bl        *blip.Buffer // delta buffer to output to
	frequency float64
	volume    float64
	phase     int
	time      int
	amp       int
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
	period := int((clockRate/w.frequency/2 + 0.5))
	volume := int((w.volume*65536/2 + 0.5))
	for ; w.time < clocks; w.time += period {
		delta := w.phase*volume - w.amp
		w.amp += delta
		w.bl.AddDelta(uint64(w.time), int32(delta))
		w.phase = -w.phase
	}
	w.time -= clocks
}

func genSamples(out []int16) {
	pairs := len(out) / 2 // number of stereo sample pairs
	clocks := blips[0].ClocksNeeded(pairs)

	waves[0].run(clocks)
	waves[1].run(clocks)

	// Generate left and right channels, interleaved into out
	for i := range 2 {
		blips[i].EndFrame(clocks)
		blips[i].ReadSamples(out[i:], pairs, blip.Stereo)
	}
}

func initSound() {
	// Create left and right delta buffers
	for i := range 2 {
		blips[i] = blip.NewBuffer(sampleRate / 10)
		waves[i].bl = blips[i]
		blips[i].SetRates(clockRate, sampleRate)
	}
}

func main() {
	initSound()

	w, err := wave.NewFile("out.wav", sampleRate)
	if err != nil {
		log.Fatal(err)
	}
	w.EnableStereo()

	const samples = 2048
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
