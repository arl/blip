package wave

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
)

// A Writer writes samples to a wave file.
type Writer struct {
	w           io.WriteCloser
	sampleRate  int
	sampleCount int
	chanCount   uint8
	bb          bytes.Buffer
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

func newWriter(w io.WriteCloser, newSampleRate int) *Writer {
	ww := &Writer{
		w:          w,
		sampleRate: newSampleRate,
		chanCount:  1,
	}
	return ww
}

// NewWriter creates a new Writer with the given sample rate, onto which samples
// can be written with Write. Close must be called when done writing samples to
// finalize the wave file.
func NewWriter(w io.Writer, newSampleRate int) *Writer {
	ww := &Writer{
		w:          nopCloser{Writer: w},
		sampleRate: newSampleRate,
		chanCount:  1,
	}
	return ww
}

// NewFile creates a new wave file at the given path with the given sample rate.
// Close must be called when done writing samples to finalize the wave file.
func NewFile(path string, newSampleRate int) (*Writer, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return newWriter(f, newSampleRate), nil
}

const sampleSize = 2

func (w *Writer) header() [0x2C]byte {
	dataSize := sampleSize * w.sampleCount
	frameSize := sampleSize * w.chanCount
	h := [0x2C]byte{
		'R', 'I', 'F', 'F',
		0, 0, 0, 0, //        length of rest of file
		'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ',
		16, 0, 0, 0, //       size of fmt chunk
		1, 0, //              uncompressed format
		0, 0, //              channel count
		0, 0, 0, 0, //        sample rate
		0, 0, 0, 0, //        bytes per second
		0, 0, //              bytes per sample frame
		sampleSize * 8, 0, // bits per sample
		'd', 'a', 't', 'a',
		0, 0, 0, 0, //        size of sample data
		// ...                sample data
	}

	binary.LittleEndian.PutUint32(h[0x04:], uint32(len(h)-8+dataSize))

	h[0x16] = w.chanCount
	binary.LittleEndian.PutUint32(h[0x18:], uint32(w.sampleRate))
	binary.LittleEndian.PutUint32(h[0x1C:], uint32(w.sampleRate)*uint32(frameSize))
	h[0x20] = frameSize
	binary.LittleEndian.PutUint32(h[0x28:], uint32(dataSize))
	return h
}

func (w *Writer) EnableStereo() {
	w.chanCount = 2
}

func (w *Writer) SampleCount() int {
	return w.sampleCount
}

func (w *Writer) Write(p []int16) (n int, err error) {
	remain := len(p)
	w.sampleCount += remain

	for remain != 0 {
		var buf [4096]byte

		n := len(buf) / sampleSize
		if n > remain {
			n = remain
		}
		remain -= n

		for i := range p {
			binary.LittleEndian.PutUint16(buf[i*2:], uint16(p[i]))
		}
		w.bb.Write(buf[:len(p)*2])
	}
	return len(p), nil
}

// Close finalizes the wave file. It must be called when done writing samples.
func (w *Writer) Close() error {
	hdr := w.header()
	if _, err := w.w.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := w.w.Write(w.bb.Bytes()); err != nil {
		return err
	}

	w.bb.Reset()
	w.sampleCount = 0
	w.chanCount = 1

	return w.w.Close()
}
