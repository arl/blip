package main

// Plays two square waves with mouse control of frequency,
// using SDL multimedia library.

// typedef unsigned char Uint8;
// void FillBuffer(void *userdata, Uint8 *stream, int len);
import "C"
import (
	"log"
	"unsafe"

	"github.com/arl/blip"
	"github.com/veandco/go-sdl2/sdl"
)

const sampleRate = 44100 // 44.1 kHz sample rate
const clockRate = sampleRate * blip.MaxRatio

var bl *blip.Buffer

type wave struct {
	frequency float64
	volume    float64
	phase     int
	time      int
	amp       int
}

var waves = [2]wave{
	{
		frequency: 1000,
		volume:    0.2,
		phase:     1,
	},
	{
		frequency: 1000,
		volume:    0.2,
		phase:     1,
	},
}

func runWave(w *wave, clocks int) {
	period := int((clockRate/w.frequency/2 + 0.5))
	volume := int((w.volume*65536/2 + 0.5))
	for ; w.time < clocks; w.time += period {
		delta := w.phase*volume - w.amp
		w.amp += delta
		bl.AddDelta(uint64(w.time), int32(delta))
		w.phase = -w.phase
	}
	w.time -= clocks
}

func genSamples(out []int16) {
	clocks := bl.ClocksNeeded(len(out))
	runWave(&waves[0], clocks)
	runWave(&waves[1], clocks)
	bl.EndFrame(clocks)

	bl.ReadSamples(out, len(out), blip.Mono)
}

func main() {
	waves[0].frequency = 1000
	waves[0].volume = 0.2
	waves[0].phase = 1

	waves[1].frequency = 1000
	waves[1].volume = 0.2
	waves[1].phase = 1

	bl = blip.NewBuffer(sampleRate / 10)
	bl.SetRates(clockRate, sampleRate)

	// setup SDL
	as := sdl.AudioSpec{
		Format:   sdl.AUDIO_S16SYS,
		Freq:     sampleRate,
		Channels: 1,
	}

	// initialize SDL
	if err := sdl.Init(sdl.INIT_AUDIO | sdl.INIT_VIDEO); err != nil {
		log.Fatal(err)
	}
	defer sdl.Quit()

	w, err := sdl.CreateWindow("blip/sdl example",
		sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		512, 512, sdl.WINDOW_SHOWN)
	if err != nil {
		log.Fatal(err)
	}
	defer w.Destroy()

	w.Show()

	// start audio
	as.Callback = sdl.AudioCallback(C.FillBuffer)
	as.Samples = 1024
	if err := sdl.OpenAudio(&as, nil); err != nil {
		log.Fatal(err)
	}
	sdl.PauseAudio(false)

pollLoop:
	for {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			// run until mouse or keyboard is pressed
			switch t := event.(type) {
			case *sdl.QuitEvent:
				break pollLoop
			case *sdl.KeyboardEvent:
				if t.Keysym.Sym == sdl.K_ESCAPE {
					break pollLoop
				}
			case *sdl.MouseButtonEvent:
				// mouse controls frequency and volume
				ix, iy, _ := sdl.GetMouseState()
				waves[0].frequency = float64(ix)/511.0*2000 + 100
				waves[1].frequency = float64(iy)/511.0*2000 + 100
			}
		}
	}

	sdl.PauseAudio(true)
}

//export FillBuffer
func FillBuffer(_ unsafe.Pointer, out *C.Uint8, nbytes C.int) {
	genSamples(unsafe.Slice((*int16)(unsafe.Pointer(out)), nbytes/2))
}
