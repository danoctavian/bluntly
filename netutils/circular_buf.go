package netutils

import (
	"errors"
)

/* Circular byte buffer */

// a a circular byte buffer 
type CircularBuf struct {
	buf []byte
	start, end int
}

func NewCircularBuf(capacity int) *CircularBuf {
	buf := make([]byte, capacity)
	return &CircularBuf{buf, 0, 0}
}

func (b *CircularBuf) GimmeBuf() []byte { return b.buf}

// the read consumes the buffer
func (b *CircularBuf) Read(p []byte) (bytesCopied int, err error) {
	err = nil
	if b.start == b.end {
		return 0, nil
	} else if b.start < b.end {
		bytesCopied = copy(p, b.buf[b.start:b.end])
		b.start = b.start + bytesCopied
	} else if (b.end < b.start) {
		bytesCopied = copy(p, b.buf[b.start:])
		bytesCopied += copy(p[bytesCopied:], b.buf[0:b.end])
		b.start = (b.start + bytesCopied) % b.Capacity()
	}
	return
}

func (b *CircularBuf) Write(buf []byte) (bytesCopied int, err error) {
	if b.Capacity() - b.Size() < len(buf) {
		err = errors.New("Buffer doesn't have capacity to hold the entire input.")
	}

	if b.start > b.end {
		bytesCopied = copy(b.buf[b.end:b.start], buf)
		b.end = b.end + bytesCopied
	} else if (b.start <= b.end) {
		bytesCopied = copy(b.buf[b.end:], buf)
		bytesCopied += copy(b.buf[0:b.start], buf[bytesCopied:])
		b.end = (b.end + bytesCopied) % b.Capacity()
	}
	return
}

func (b *CircularBuf) Size() int {
	if b.start <= b.end {
		return b.end - b.start
	} else {return b.Capacity() - (b.start - b.end)}
}

func (b *CircularBuf) Capacity() int {
	return len(b.buf)
}

