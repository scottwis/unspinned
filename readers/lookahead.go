package readers

import (
	"bytes"
	"io"
	"math/bits"
	"unicode/utf8"
)

type LookaheadReader interface {
	Peek(i int) (rune, error)
	Read() (rune, error)
	Buffered() io.Reader
}

type lookahead struct {
	reader io.Reader
	buf []rune
	start int
	length int
}

func NewLookahead(reader io.Reader) LookaheadReader {
	return &lookahead{
		reader: reader,
		buf:    nil,
		start:  0,
		length: 0,
	}
}

func (l * lookahead) resize() {
	if len(l.buf) == 0 {
		l.buf = make([]rune, 1, 1)
	} else {
		newLen := len(l.buf)*2
		newBuf := make([]rune, newLen, newLen)
		copy(newBuf, l.buf[l.start:])
		if l.start != 0 {
			//say length is 5, start is 2, then this will copy into
			//newBuf[3:] from l.buf[:2]
			copy(newBuf[len(l.buf) - l.start:], l.buf[:l.start])
		}
		l.buf = newBuf
		l.start = 0
	}
}

func (l *lookahead) consumeRune() error {
	if l.length == len(l.buf) {
		l.resize()
	}

	var readBuf [4]byte
	n, err := l.reader.Read(readBuf[:1])
	if n == 1 {
		numBytes := bits.LeadingZeros8(^readBuf[0])
		if numBytes == 0 {
			l.buf[(l.length + l.start) % len(l.buf)] = rune(readBuf[0])
			l.length++
		} else {
			readBytes := 1
			for readBytes < numBytes && err != io.EOF {
				n, err = l.reader.Read(readBuf[readBytes:])
				readBytes += n
			}

			if readBytes < numBytes {
				return err
			}

			r, _ := utf8.DecodeRune(readBuf[:])
			l.buf[(l.length + l.start) % len(l.buf)] = r
			l.length++
		}
	}
	return err
}

func (l *lookahead) Peek(i int) (rune, error) {
	var err error
	for i >= l.length && err != io.EOF {
		err = l.consumeRune()
	}

	if i < l.length {
		return l.buf[(l.start + i) % len(l.buf)], err
	}
	return -1, io.EOF
}

func (l *lookahead) Read() (rune, error) {
	ret, err := l.Peek(0)
	if ret != -1 {
		l.start = (l.start + 1) % len(l.buf)
		l.length--
	}
	return ret, err
}

func (l * lookahead) Buffered() io.Reader {
	startPos := l.start
	endPos := (l.start + l.length) % len(l.buf)

	if endPos > startPos {
		return bytes.NewBuffer([]byte(string(l.buf[startPos:endPos])))
	}

	return Union(
		bytes.NewBuffer([]byte(string(l.buf[startPos:]))),
		bytes.NewBuffer([]byte(string(l.buf[:endPos]))),
	)
}