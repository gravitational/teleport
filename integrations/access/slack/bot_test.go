package slack

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	"net/url"
	"testing"
	"time"
)

func TestSlackNotificationMessage(t *testing.T) {
	webUrl, err := url.Parse("https://localhost:3080")
	require.NoError(t, err)
	b := Bot{webProxyURL: webUrl}
	name := "NAME"
	description := "DESCRIPTION"
	created := time.Now()
	labels := map[string]string{
		"severity":              "warning",
		"tag.teleport.dev/diff": "diff",
	}
	notification := &notificationsv1.Notification{
		Metadata: &headerv1.Metadata{
			Name:        name,
			Description: description,
			Labels:      labels,
		},
		Spec: &notificationsv1.NotificationSpec{
			Id:       uuid.New().String(),
			Created:  timestamppb.New(created),
			Unscoped: false,
		},
	}

	sections := b.slackNotificationMsgSections(notification)
	message := Message{BaseMessage: BaseMessage{Channel: "#channel"}, BlockItems: sections}
	foo, err := json.Marshal(message)
	require.NoError(t, err)

	fmt.Println(string(foo))
}
