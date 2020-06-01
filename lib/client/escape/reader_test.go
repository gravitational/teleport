package escape

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"gopkg.in/check.v1"
)

func Test(t *testing.T) { check.TestingT(t) }

type ReaderSuite struct {
}

var _ = check.Suite(&ReaderSuite{})

type readerTestCase struct {
	inChunks [][]byte
	inErr    error

	wantReadErr       error
	wantDisconnectErr error
	wantOut           string
	wantHelp          string
}

func (*ReaderSuite) runCase(c *check.C, t readerTestCase) {
	in := &mockReader{chunks: t.inChunks, finalErr: t.inErr}
	helpOut := new(bytes.Buffer)
	out := new(bytes.Buffer)
	var disconnectErr error

	r := NewReader(in, helpOut, func(err error) {
		disconnectErr = err
	})

	_, err := io.Copy(out, r)
	c.Assert(err, check.Equals, t.wantReadErr)
	c.Assert(disconnectErr, check.Equals, t.wantDisconnectErr)
	c.Assert(out.String(), check.Equals, t.wantOut)
	c.Assert(helpOut.String(), check.Equals, t.wantHelp)
}

func (s *ReaderSuite) TestNormalReads(c *check.C) {
	c.Log("normal read")
	s.runCase(c, readerTestCase{
		inChunks: [][]byte{[]byte("hello world")},
		wantOut:  "hello world",
	})

	c.Log("incomplete sequence")
	s.runCase(c, readerTestCase{
		inChunks: [][]byte{[]byte("hello\r~world")},
		wantOut:  "hello\r~world",
	})

	c.Log("escaped tilde character")
	s.runCase(c, readerTestCase{
		inChunks: [][]byte{[]byte("hello\r~~world")},
		wantOut:  "hello\r~world",
	})

	c.Log("other character between newline and tilde")
	s.runCase(c, readerTestCase{
		inChunks: [][]byte{[]byte("hello\rw~orld")},
		wantOut:  "hello\rw~orld",
	})

	c.Log("other character between newline and disconnect sequence")
	s.runCase(c, readerTestCase{
		inChunks: [][]byte{[]byte("hello\rw~.orld")},
		wantOut:  "hello\rw~.orld",
	})
}

func (s *ReaderSuite) TestReadError(c *check.C) {
	customErr := errors.New("oh no")

	s.runCase(c, readerTestCase{
		inChunks:          [][]byte{[]byte("hello world")},
		inErr:             customErr,
		wantOut:           "hello world",
		wantReadErr:       customErr,
		wantDisconnectErr: customErr,
	})
}

func (s *ReaderSuite) TestEscapeHelp(c *check.C) {
	c.Log("single help sequence between reads")
	s.runCase(c, readerTestCase{
		inChunks: [][]byte{[]byte("hello\r~?world")},
		wantOut:  "hello\rworld",
		wantHelp: helpText,
	})

	c.Log("single help sequence before any data")
	s.runCase(c, readerTestCase{
		inChunks: [][]byte{[]byte("~?hello world")},
		wantOut:  "hello world",
		wantHelp: helpText,
	})

	c.Log("repeated help sequences")
	s.runCase(c, readerTestCase{
		inChunks: [][]byte{[]byte("hello\r~?world\n~?")},
		wantOut:  "hello\rworld\n",
		wantHelp: helpText + helpText,
	})

	c.Log("help sequence split across reads")
	s.runCase(c, readerTestCase{
		inChunks: [][]byte{
			[]byte("hello\r"),
			[]byte("~"),
			[]byte("?"),
			[]byte("world"),
		},
		wantOut:  "hello\rworld",
		wantHelp: helpText,
	})
}

func (s *ReaderSuite) TestEscapeDisconnect(c *check.C) {
	c.Log("single disconnect sequence between reads")
	s.runCase(c, readerTestCase{
		inChunks: [][]byte{
			[]byte("hello"),
			[]byte("\r~."),
			[]byte("world"),
		},
		wantOut:           "hello",
		wantReadErr:       ErrDisconnect,
		wantDisconnectErr: ErrDisconnect,
	})

	c.Log("disconnect sequence before any data")
	s.runCase(c, readerTestCase{
		inChunks:          [][]byte{[]byte("~.hello world")},
		wantReadErr:       ErrDisconnect,
		wantDisconnectErr: ErrDisconnect,
	})

	c.Log("disconnect sequence split across reads")
	s.runCase(c, readerTestCase{
		inChunks: [][]byte{
			[]byte("hello\r"),
			[]byte("~"),
			[]byte("."),
			[]byte("world"),
		},
		wantOut:           "hello\r",
		wantReadErr:       ErrDisconnect,
		wantDisconnectErr: ErrDisconnect,
	})
}

func (*ReaderSuite) TestBufferOverflow(c *check.C) {
	in := &mockReader{chunks: [][]byte{make([]byte, 100)}}
	helpOut := new(bytes.Buffer)
	out := new(bytes.Buffer)
	var disconnectErr error

	r := newUnstartedReader(in, helpOut, func(err error) {
		disconnectErr = err
	})
	r.bufferLimit = 10
	go r.runReads()

	_, err := io.Copy(out, r)
	c.Assert(err, check.Equals, ErrTooMuchBufferedData)
	c.Assert(disconnectErr, check.Equals, ErrTooMuchBufferedData)
}

type mockReader struct {
	chunks   [][]byte
	finalErr error
}

func (r *mockReader) Read(buf []byte) (int, error) {
	if len(r.chunks) == 0 {
		if r.finalErr != nil {
			return 0, r.finalErr
		}
		return 0, io.EOF
	}

	chunk := r.chunks[0]
	r.chunks = r.chunks[1:]
	copy(buf, chunk)
	return len(chunk), nil
}
