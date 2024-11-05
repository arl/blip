package blip

import (
	"hash/crc32"
	"math"
	"testing"
	"unsafe"

	"github.com/google/go-cmp/cmp"
)

func shouldPanic(t *testing.T, f func()) {
	t.Helper()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	f()
}
func TestShouldPanic(t *testing.T) {
	shouldPanic(t, func() { panic("test") })
}

func assert[T comparable](t *testing.T, got, want T) {
	t.Helper()

	if got != want {
		t.Fatalf("assertion failed: got = %v want %v", got, want)
	}
}

func TestAssumptions(t *testing.T) {
	const blipSize = MaxFrame / 2

	if _, err := NewBuffer(blipSize); err != nil {
		t.Fatal(err)
	}
}

const oversample = MaxRatio

func TestEndFrame(t *testing.T) {
	const blipSize = MaxFrame / 2

	t.Run("SamplesAvailable", func(t *testing.T) {
		bl, _ := NewBuffer(blipSize)
		bl.EndFrame(oversample)
		assert(t, bl.SamplesAvailable(), 1)

		bl.EndFrame(oversample * 2)
		assert(t, bl.SamplesAvailable(), 3)
	})
	t.Run("SamplesAvailable fractional", func(t *testing.T) {
		bl, _ := NewBuffer(blipSize)
		bl.EndFrame(oversample*2 - 1)
		assert(t, bl.SamplesAvailable(), 1)

		bl.EndFrame(1)
		assert(t, bl.SamplesAvailable(), 2)
	})
	t.Run("limits", func(t *testing.T) {
		bl, _ := NewBuffer(blipSize)
		bl.EndFrame(0)
		assert(t, bl.SamplesAvailable(), 0)

		bl.EndFrame(blipSize*oversample + oversample - 1)
		shouldPanic(t, func() { bl.EndFrame(1) })
	})
}

func TestClocksNeeded(t *testing.T) {
	const blipSize = MaxFrame / 2

	t.Run(" ", func(t *testing.T) {
		bl, _ := NewBuffer(blipSize)
		assert(t, bl.ClocksNeeded(0), 0*oversample)
		assert(t, bl.ClocksNeeded(2), 2*oversample)

		bl.EndFrame(1)
		assert(t, bl.ClocksNeeded(0), 0)
		assert(t, bl.ClocksNeeded(2), 2*oversample-1)
	})

	t.Run("limits", func(t *testing.T) {
		bl, _ := NewBuffer(blipSize)

		shouldPanic(t, func() { bl.ClocksNeeded(-1) })

		bl.EndFrame(oversample*2 - 1)
		assert(t, bl.ClocksNeeded(blipSize-1), (blipSize-2)*oversample+1)

		bl.EndFrame(1)
		shouldPanic(t, func() { (bl.ClocksNeeded(blipSize - 1)) })
	})
}

func TestClearBasic(t *testing.T) {
	const blipSize = MaxFrame / 2

	bl, _ := NewBuffer(blipSize)
	bl.EndFrame(2*oversample - 1)

	bl.Clear()
	assert(t, bl.SamplesAvailable(), 0)
	assert(t, bl.ClocksNeeded(1), oversample)
}

func TestReadSamples(t *testing.T) {
	const blipSize = MaxFrame / 2

	t.Run("mono", func(t *testing.T) {
		bl, _ := NewBuffer(blipSize)
		buf := []int16{-1, -1}

		bl.EndFrame(3*oversample + oversample - 1)
		assert(t, bl.ReadSamples(buf, 2, Mono), 2)
		assert(t, buf[0], 0)
		assert(t, buf[1], 0)

		assert(t, bl.SamplesAvailable(), 1)
		assert(t, bl.ClocksNeeded(1), 1)
	})

	t.Run("stereo", func(t *testing.T) {
		bl, _ := NewBuffer(blipSize)
		buf := []int16{-1, -1, -1}

		bl.EndFrame(2 * oversample)
		assert(t, bl.ReadSamples(buf, 2, Stereo), 2)
		assert(t, buf[0], 0)
		assert(t, buf[1], -1)
		assert(t, buf[2], 0)
	})

	t.Run("limits to avail", func(t *testing.T) {
		bl, _ := NewBuffer(blipSize)
		bl.EndFrame(oversample * 2)

		buf := []int16{-1, -1}

		assert(t, bl.ReadSamples(buf, 3, Mono), 2)
		assert(t, bl.SamplesAvailable(), 0)
		assert(t, buf[0], 0)
		assert(t, buf[1], 0)
	})

	t.Run("limits", func(t *testing.T) {
		bl, _ := NewBuffer(blipSize)
		assert(t, bl.ReadSamples(nil, 1, Mono), 0)

		shouldPanic(t, func() { bl.ReadSamples(nil, -1, Mono) })
	})
}

func TestSetRates(t *testing.T) {
	const blipSize = MaxFrame / 2

	t.Run(" ", func(t *testing.T) {
		bl, _ := NewBuffer(blipSize)
		bl.SetRates(2, 2)
		assert(t, bl.ClocksNeeded(10), 10)

		bl.SetRates(2, 4)
		assert(t, bl.ClocksNeeded(10), 5)

		bl.SetRates(4, 2)
		assert(t, bl.ClocksNeeded(10), 20)
	})

	t.Run("rounds sample rate up", func(t *testing.T) {
		bl, _ := NewBuffer(blipSize)
		for r := 1; r < 10000; r++ {
			bl.SetRates(float64(r), 1)
			assert(t, bl.ClocksNeeded(1) <= r, true)
		}
	})

	t.Run("accuracy", func(t *testing.T) {
		maxError := 100 // 1%
		bl, _ := NewBuffer(blipSize)

		for r := blipSize / 2; r < blipSize; r++ {
			for c := r / 2; c < 8000000; c += c / 32 {
				bl.SetRates(float64(c), float64(r))
				error := bl.ClocksNeeded(r) - c
				if error < 0 {
					error = -error
				}
				assert(t, error < c/maxError, true)
			}
		}
	})

	t.Run("high accuracy", func(t *testing.T) {
		bl, _ := NewBuffer(blipSize)
		bl.SetRates(1000000, blipSize)
		if bl.ClocksNeeded(blipSize) != 1000000 {
			t.Skip("skipping because 64-bit int isn't available")
		}

		for r := blipSize / 2; r < blipSize; r++ {
			for c := r / 2; c < 200000000; c += c / 32 {
				bl.SetRates(float64(c), float64(r))
				assert(t, bl.ClocksNeeded(r), c)
			}
		}
	})

	t.Run("long-term accuracy", func(t *testing.T) {
		bl, _ := NewBuffer(blipSize)
		bl.SetRates(1000000, blipSize)
		if bl.ClocksNeeded(blipSize) != 1000000 {
			t.Skip("skipping because 64-bit int isn't available")
		}

		// Generates secs seconds and ensures that exactly
		// secs*sampleRate samples are generated.
		const clockRate = 1789773
		const sampleRate = 44100
		const secs = 1000

		bl.SetRates(clockRate, sampleRate)

		const bufSize = blipSize / 2

		clockSize := bl.ClocksNeeded(bufSize) - 1
		totalSamples := 0.0

		buf := make([]int16, bufSize)

		remain := float64(clockRate) * secs
		for remain > 0 {
			n := int(math.Min(remain, float64(clockSize)))

			bl.EndFrame(n)

			totalSamples += float64(bl.ReadSamples(buf, bufSize, Mono))
			remain -= float64(n)
		}

		assert(t, totalSamples, float64(sampleRate)*secs)
	})
}

func TestAddDelta(t *testing.T) {
	const blipSize = MaxFrame / 2

	t.Run("limits", func(t *testing.T) {
		bl, _ := NewBuffer(blipSize)
		bl.AddDelta(0, 1)
		bl.AddDelta((blipSize+3)*oversample-1, 1)

		shouldPanic(t, func() { bl.AddDelta((blipSize+3)*oversample, 1) })
		shouldPanic(t, func() { bl.AddDelta(math.MaxUint, 1) })
	})
}

func makefill[T any](size int, v T) []T {
	s := make([]T, size)
	for i := range s {
		s[i] = v
	}
	return s
}

func add_deltas(b *Buffer, offset uint64) {
	const frame_len = 20*oversample + oversample/4

	b.AddDelta(frame_len/2+offset, +1000)
	b.AddDelta(frame_len+offset+endFrameExtra*oversample, +1000)
}

func TestInvariance(t *testing.T) {
	const frame_len = 20*oversample + oversample/4
	const blipSize = (frame_len * 2) / oversample

	t.Run("EndFrame, AddDelta", func(t *testing.T) {
		want := []int16{
			0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 1, -3, 7, -5, 21,
			9, 119, 750, 1004, 963, 1001, 983, 993,
			985, 992, 982, 997, 999, 1050, 1649, 1982,
			1932, 1976, 1955, 1966, 1980, 1986, 2534, 2944,
		}

		{
			got := makefill[int16](blipSize, +1)
			bl, _ := NewBuffer(blipSize)
			add_deltas(bl, 0)
			add_deltas(bl, frame_len)
			bl.EndFrame(frame_len * 2)
			assert(t, bl.ReadSamples(got, blipSize, Mono), blipSize)

			if diff := cmp.Diff(got, want); diff != "" {
				t.Errorf("response mismatch (-got +want):\n%s", diff)
			}
		}
		{
			got := makefill[int16](blipSize, -1)
			bl, _ := NewBuffer(blipSize)
			add_deltas(bl, 0)
			bl.EndFrame(frame_len)
			add_deltas(bl, 0)
			bl.EndFrame(frame_len)
			assert(t, bl.ReadSamples(got, blipSize, Mono), blipSize)
			if diff := cmp.Diff(got, want); diff != "" {
				t.Errorf("response mismatch (-got +want):\n%s", diff)
			}
		}
	})

	want := []int16{0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 1, -3, 7, -5, 21,
		9, 119, 750, 1004, 963, 1001, 983, 993,
		985, 992, 982, 997, 999, 1050, 1649, 1982,
		1932, 1976, 1955, 1966, 1980, 1986, 2534, 2944,
		2900, 2933, 2918, 2919, 2912, 2909, 2905, 2898,
		2923, 2896, 3379, 3861, 3832, 3856, 3852, 3837,
		3871, 3824, 4232, 4775,
	}

	t.Run("ReadSamples/1", func(t *testing.T) {
		const blipSize = (frame_len * 3) / oversample
		out := makefill[int16](blipSize, +1)

		bl, _ := NewBuffer(blipSize)
		add_deltas(bl, 0*frame_len)
		add_deltas(bl, 1*frame_len)
		add_deltas(bl, 2*frame_len)
		bl.EndFrame(3 * frame_len)

		assert(t, bl.ReadSamples(out, blipSize, Mono), blipSize)

		if diff := cmp.Diff(out, want); diff != "" {
			t.Errorf("response mismatch (-one +want):\n%s", diff)
		}
	})

	t.Run("ReadSamples/2", func(t *testing.T) {
		const blipSize = (frame_len * 3) / oversample
		got := makefill[int16](blipSize, -1)

		bl, _ := NewBuffer(blipSize / 3)
		count := 0

		for range 3 {
			add_deltas(bl, 0)
			bl.EndFrame(frame_len)
			count += bl.ReadSamples(got[count:], blipSize-count, Mono)
		}

		assert(t, count, blipSize)

		if diff := cmp.Diff(got, want); diff != "" {
			t.Errorf("response mismatch (-two +want):\n%s", diff)
		}
	})

	t.Run("MaxFrame", func(t *testing.T) {
		const oversample = 32
		const frame_len = MaxFrame * oversample
		const blipSize = frame_len / oversample * 3

		one := makefill[int16](blipSize, +1)
		two := makefill[int16](blipSize, -1)

		{
			bl, _ := NewBuffer(blipSize)
			bl.SetRates(oversample, 1)

			count := 0
			for range 3 {
				bl.EndFrame(frame_len / 2)
				bl.AddDelta(frame_len/2+endFrameExtra*oversample, +1000)
				bl.EndFrame(frame_len / 2)
				count += bl.ReadSamples(one[count:], blipSize-count, Mono)
			}
			assert(t, count, blipSize)
		}

		{
			bl, _ := NewBuffer(blipSize)
			bl.SetRates(oversample, 1)

			count := 0
			for range 3 {
				bl.AddDelta(frame_len+endFrameExtra*oversample, +1000)
				bl.EndFrame(frame_len)
			}
			count += bl.ReadSamples(two[count:], blipSize-count, Mono)
			assert(t, count, blipSize)
		}

		if diff := cmp.Diff(one, two); diff != "" {
			t.Errorf("response mismatch (-one +two):\n%s", diff)
		}
	})

	t.Run("AddDeltaFast ReadSamples", func(t *testing.T) {
		const blipSize = 32
		bl, _ := NewBuffer(blipSize)

		bl.AddDeltaFast(2*oversample, +16384)
		endFrameAndCheckCRC(t, bl, blipSize, 0x7401D8E4)

		bl.AddDeltaFast(uint64(int(2.5*oversample)), +16384)
		endFrameAndCheckCRC(t, bl, blipSize, 0x17E3745D)
	})

	t.Run("tails", func(t *testing.T) {
		const blipSize = 32

		bl, _ := NewBuffer(blipSize)

		bl.AddDelta(0, +16384)
		endFrameAndCheckCRC(t, bl, blipSize, 0xCA2F85D1)

		bl.AddDelta(oversample/2, +16384)
		endFrameAndCheckCRC(t, bl, blipSize, 0x9A4F1B43)
	})

	t.Run("AddDelta interpolation", func(t *testing.T) {
		const blipSize = 32

		bl, _ := NewBuffer(blipSize)

		bl.AddDelta(oversample/2, +32768)
		endFrameAndCheckCRC(t, bl, blipSize, 0xFD326B1)

		// Values should be half-way between values for above and below
		bl.AddDelta(oversample/2+oversample/64, +32768)
		endFrameAndCheckCRC(t, bl, blipSize, 0x7CB83EFD)

		bl.AddDelta(oversample/2+oversample/32, +32768)
		endFrameAndCheckCRC(t, bl, blipSize, 0xD8864668)
	})
}

func TestSaturation(t *testing.T) {
	test := func(delta int32, want int16) {
		t.Helper()

		const blipSize = 32
		bl, _ := NewBuffer(blipSize)

		bl.AddDeltaFast(0, delta)
		bl.EndFrame(oversample * blipSize)
		var buf [32]int16
		bl.ReadSamples(buf[:], int(blipSize), Mono)
		assert(t, buf[20], want)
	}

	test(+35000, +32767)
	test(-35000, -32768)
}

func TestStereoInterleave(t *testing.T) {
	const blipSize = 32
	bl, _ := NewBuffer(blipSize)

	monobuf := make([]int16, blipSize)
	bl.AddDelta(0, +16384)
	bl.EndFrame(blipSize * oversample)
	bl.ReadSamples(monobuf, blipSize, Mono)

	bl.Clear()

	stereouf := make([]int16, blipSize*2)
	bl.AddDelta(0, +16384)
	bl.EndFrame(blipSize * oversample)
	bl.ReadSamples(stereouf, blipSize, Stereo)

	for i := range blipSize {
		assert(t, stereouf[i*2], monobuf[i])
	}
}

func TestClearSynthesis(t *testing.T) {
	const blipSize = 32
	bl, _ := NewBuffer(blipSize)

	// Make first and last internal samples non-zero
	bl.AddDelta(0, 32768)
	bl.AddDelta((blipSize+2)*oversample+oversample/2, 32768)

	bl.Clear()

	buf := make([]int16, blipSize)
	for range 2 {
		bl.EndFrame(blipSize * oversample)
		assert(t, bl.ReadSamples(buf, blipSize, Mono), blipSize)
		for i := range blipSize {
			assert(t, buf[i], 0)
		}
	}
}

func endFrameAndCheckCRC(t *testing.T, bl *Buffer, bsize int, wantcrc uint32) {
	t.Helper()

	bl.EndFrame(bsize * oversample)
	buf := make([]int16, bsize)
	bl.ReadSamples(buf, bsize, Mono)

	bbuf := unsafe.Slice((*byte)(unsafe.Pointer(&buf[0])), len(buf)*2)
	assert(t, wantcrc, crc32.ChecksumIEEE(bbuf))
	bl.Clear()
}
