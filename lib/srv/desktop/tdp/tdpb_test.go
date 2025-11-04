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
	_, err := encodeTDPB(&fso)
	require.Error(t, err)
}

//func TestAs(t *testing.T) {
//	msg := Message(&TDPBMessage{Message: &tdpb.ClientUsername{Username: "name"}})
//
//	//var usernameMsg *tdpb.ClientUsername
//	usernameMsg, ok := To[*tdpb.ClientUsername](msg)
//	require.True(t, ok)
//	//require.True(t, As(msg, &usernameMsg))
//	require.Equal(t, "name", usernameMsg.Username)
//
//	kbLayout := &tdpb.ClientKeyboardLayout{}
//	require.False(t, AsProto(msg, kbLayout))
//}

func TestStreamProtos(t *testing.T) {
	inMessages := []proto.Message{
		&tdpb.Alert{
			Message: "oh no!",
		},
		&tdpb.ClientScreenSpec{
			Width:  1459,
			Height: 720,
		},
		&tdpb.ClientUsername{
			Username: "bob",
		},
	}

	dec := DecoderFunc(DecodeTDPB)

	rdr, writer := net.Pipe()
	// Need 'ReadByte()'
	reader := bufio.NewReader(rdr)

	var writerError error
	go func() {
		defer writer.Close()
		for _, msg := range inMessages {
			tmsg := NewTDPBMessage(msg)
			data, err := tmsg.Encode()
			if err != nil {
				return
			}
			_, writerError = writer.Write(data)
			if writerError != nil {
				return
			}

			//var encodable io.Reader
			//encodable, writerError = WireCapable(msg)
			//if writerError != nil {
			//	return
			//}
			//
			//_, writerError = io.Copy(writer, encodable)
			//if writerError != nil {
			//	return
			//}
		}

	}()

	var readerError error
	outMessages := []Message{}

	for readerError == nil {
		var msg Message
		msg, readerError = dec.Decode(reader)
		if readerError == nil {
			outMessages = append(outMessages, msg)
		}
	}
	rdr.Close()

	require.ErrorIs(t, readerError, io.EOF)
	assert.Len(t, outMessages, 3)

	alertMsg := tdpb.Alert{}
	require.NoError(t, As(outMessages[0], &alertMsg))
	screenSpecMsg := tdpb.ClientScreenSpec{}
	require.NoError(t, As(outMessages[1], &screenSpecMsg))
	assert.Equal(t, (inMessages[1].(*tdpb.ClientScreenSpec)).Height, screenSpecMsg.Height)
	assert.Equal(t, (inMessages[1].(*tdpb.ClientScreenSpec)).Width, screenSpecMsg.Width)
	usernameMsg := tdpb.ClientUsername{}
	require.NoError(t, As(outMessages[2], &usernameMsg))
	//assert.Equal(t, outMessages[0].Type, tdpb.MessageType_MESSAGE_ALERT)
	//assert.Equal(t, outMessages[1].Type, tdpb.MessageType_MESSAGE_CLIENT_SCREEN_SPEC)
	//assert.Equal(t, outMessages[2].Type, tdpb.MessageType_MESSAGE_CLIENT_USERNAME)
}
