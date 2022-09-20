package roundtrip

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/gravitational/trace"
)

const (
	stateOK = iota
	stateEOF
	stateError
	stateClosed
)

// Seeker is a file download object
// that implements io.ReadSeeker interface over HTTP
type seeker struct {
	client        *Client
	endpoint      string
	fileSize      int64
	currentReader io.ReadCloser
	currentOffset int64
	lastError     error
	state         int
}

func (s *seeker) updateState(err error) error {
	switch s.state {
	case stateOK, stateEOF:
		if err == nil {
			return nil
		}
		if err == io.EOF {
			s.state = stateEOF
			s.lastError = err
			return err
		}
		s.state = stateError
		s.lastError = err
		return err
	}
	return err
}

func (s *seeker) canRead() error {
	if s.state == stateOK {
		return nil
	}
	return s.lastError
}

func (s *seeker) canSeek() error {
	if s.state == stateOK || s.state == stateEOF {
		return nil
	}
	return s.lastError
}

func newSeeker(c *Client, ctx context.Context, endpoint string) (ReadSeekCloser, error) {
	response, err := c.RoundTrip(func() (*http.Response, error) {
		req, err := http.NewRequest("HEAD", endpoint, nil)
		if err != nil {
			return nil, err
		}
		req = req.WithContext(ctx)
		c.addAuth(req)
		return c.client.Do(req)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sizeHeader := response.headers.Get("Content-Length")
	size, err := strconv.ParseInt(sizeHeader, 10, 0)
	if err != nil {
		return nil, trace.BadParameter("failed to determine size, Content-Length:%v", sizeHeader)
	}
	return &seeker{client: c, endpoint: endpoint, fileSize: size}, nil
}

func (s *seeker) Read(p []byte) (int, error) {
	if err := s.canRead(); err != nil {
		return 0, err
	}

	err := s.initReader()
	if err != nil {
		return 0, trace.Wrap(s.updateState(err))
	}

	bytesRead, err := s.currentReader.Read(p)
	s.currentOffset += int64(bytesRead)

	return bytesRead, s.updateState(err)
}

func (s *seeker) initReader() error {
	if s.currentReader != nil {
		return nil
	}
	// If the offset is greater than or equal to size, return error
	if s.currentOffset >= s.fileSize {
		return trace.BadParameter("offset(%v) is > size(%v)", s.currentOffset, s.fileSize)
	}

	req, err := http.NewRequest("GET", s.endpoint, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	s.client.addAuth(req)

	if s.currentOffset > 0 {
		// If we are at different offset, issue a range request from there.
		req.Header.Add("Range", fmt.Sprintf("bytes=%v-", s.currentOffset))
	}

	response, err := s.client.client.Do(req)
	if err != nil {
		return trace.Wrap(err)
	}

	if response.StatusCode >= 200 && response.StatusCode <= 399 {
		s.currentReader = response.Body
		return nil
	}
	defer response.Body.Close()
	bytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return trace.Wrap(err, "failed reading response body")
	}
	return trace.ReadError(response.StatusCode, bytes)
}

func (s *seeker) Seek(offset int64, whence int) (int64, error) {
	if err := s.canSeek(); err != nil {
		return s.currentOffset, err
	}

	newOffset := s.currentOffset

	switch whence {
	case os.SEEK_CUR:
		newOffset += int64(offset)
	case os.SEEK_END:
		newOffset = s.fileSize + int64(offset)
	case os.SEEK_SET:
		newOffset = int64(offset)
	}
	if newOffset < 0 {
		return 0, trace.BadParameter("can not seek to negative offset(%v)", newOffset)
	}
	if s.currentOffset == newOffset {
		return s.currentOffset, nil
	}
	err := s.resetCurrentReader()
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		return 0, trace.Wrap(err)
	}
	s.currentOffset = newOffset
	return s.currentOffset, nil
}

func (s *seeker) resetCurrentReader() error {
	if s.currentReader == nil {
		return nil
	}
	err := s.currentReader.Close()
	s.currentReader = nil
	s.state = stateOK
	s.lastError = nil
	return err
}

func (s *seeker) Close() error {
	if s.currentReader == nil {
		return nil
	}
	err := s.currentReader.Close()
	s.currentReader = nil
	s.state = stateClosed
	s.lastError = io.EOF
	return err
}
