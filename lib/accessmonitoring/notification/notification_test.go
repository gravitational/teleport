package notification

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeEmpty(t *testing.T) {
	notification := Notification{}

	encodeFn := newEncoder(encodeMockMessage)
	encoded, err := encodeFn(notification)
	require.NoError(t, err)
	assert.Equal(t, "", encoded["id"])
	assert.Equal(t, "", encoded["recipients"])
	assert.Equal(t, "", encoded["messages"])
	assert.Equal(t, "", encoded["messagesV2"])
}

func TestEncode(t *testing.T) {
	notification := Notification{
		ID:         "notification-id",
		Recipients: []string{"recipient"},
		Messages: map[string]Message{
			"id": &MockMessage{
				MessageID: "message-id",
				ChannelID: "channel-id",
				Content:   "content",
			},
		},
	}

	encodeFn := newEncoder(encodeMockMessage)
	encoded, err := encodeFn(notification)
	require.NoError(t, err)
	assert.Equal(t, "notification-id", encoded["id"])
	assert.Equal(t, `["recipient"]`, encoded["recipients"])
	assert.Equal(t, `{"id":{"channel_id":"channel-id","content":"content","message_id":"message-id"}}`, encoded["messages"])
}

func TestDecodeEmpty(t *testing.T) {
	data := map[string]string{}
	expected := Notification{}

	decodeFn := newDecoder(decodeMockMessage)
	decoded, err := decodeFn(data)
	require.NoError(t, err)
	require.Equal(t, expected, decoded)
}

func TestDecode(t *testing.T) {
	data := map[string]string{
		"id":         "notification-id",
		"recipients": `["recipient"]`,
		"messages":   `{"id":{"channel_id":"channel-id","content":"content","message_id":"message-id"}}`,
	}

	expected := Notification{
		ID:         "notification-id",
		Recipients: []string{"recipient"},
		Messages: map[string]Message{
			"id": &MockMessage{
				MessageID: "message-id",
				ChannelID: "channel-id",
				Content:   "content",
			},
		},
	}

	decodeFn := newDecoder(decodeMockMessage)
	decoded, err := decodeFn(data)
	require.NoError(t, err)
	require.Equal(t, expected, decoded)
}

type MockMessage struct {
	MessageID string
	ChannelID string
	Content   string
}

func (m *MockMessage) ID() string {
	return m.ChannelID + m.MessageID
}

const (
	keyChannelID = "channel_id"
	keyMessageID = "message_id"
	keyContent   = "content"
)

func encodeMockMessage(message Message) (map[string]string, error) {
	mock, ok := message.(*MockMessage)
	if !ok {
		return nil, trace.BadParameter("unexpected type")
	}

	result := make(map[string]string)
	result[keyChannelID] = mock.ChannelID
	result[keyMessageID] = mock.MessageID
	result[keyContent] = mock.Content
	return result, nil
}

func decodeMockMessage(data map[string]string) (Message, error) {
	var message MockMessage
	var exists bool

	if message.ChannelID, exists = data[keyChannelID]; !exists {
		return nil, trace.BadParameter("missing key %v", keyChannelID)
	}
	if message.MessageID, exists = data[keyMessageID]; !exists {
		return nil, trace.BadParameter("missing key %v", keyMessageID)
	}
	if message.Content, exists = data[keyContent]; !exists {
		return nil, trace.BadParameter("missing key %v", keyContent)
	}
	return &message, nil
}
