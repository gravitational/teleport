package tdp

import (
	"bufio"
	"io"
	"net"
	"testing"

	tdpb "github.com/gravitational/teleport/desktop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestEncodeUnsupportedMessage(t *testing.T) {
	// FileSystemObject is intended to be composed into other
	// message types. It is not valid to send on the wire directly.
	fso := tdpb.FileSystemObject{
		LastModified: 1,
	}
	_, err := WireCapable(&fso)
	require.Error(t, err)
}

func TestStreamProtos(t *testing.T) {
	inMessages := []proto.Message{
		&tdpb.Alert{
			Message: "oh no!",
		},
		&tdpb.ClientScreenSpec{
			Width:  1,
			Height: 2,
		},
		&tdpb.ClientUsername{
			Username: "bob",
		},
	}

	rdr, writer := net.Pipe()
	// Need 'ReadByte()'
	reader := bufio.NewReader(rdr)

	var writerError error
	go func() {
		defer writer.Close()
		for _, msg := range inMessages {
			var encodable io.Reader
			encodable, writerError = WireCapable(msg)
			if writerError != nil {
				return
			}

			_, writerError = io.Copy(writer, encodable)
			if writerError != nil {
				return
			}
		}

	}()

	var readerError error
	type output struct {
		Type tdpb.MessageType
		Data []byte
	}
	outMessages := []output{}

	var mType tdpb.MessageType
	var data []byte
	for readerError == nil {
		mType, data, readerError = ReadTDPBMessage(reader)
		if readerError == nil {
			outMessages = append(outMessages, output{mType, data})
		}
	}
	rdr.Close()

	require.ErrorIs(t, readerError, io.EOF)
	assert.Len(t, outMessages, 3)
	assert.Equal(t, outMessages[0].Type, tdpb.MessageType_MESSAGE_ALERT)
	assert.Equal(t, outMessages[1].Type, tdpb.MessageType_MESSAGE_CLIENT_SCREEN_SPEC)
	assert.Equal(t, outMessages[2].Type, tdpb.MessageType_MESSAGE_CLIENT_USERNAME)
}
