package prehog

import (
	"errors"

	"github.com/bufbuild/connect-go"

	"github.com/gravitational/prehog/lib/posthog"
)

func NewHandler(client *posthog.Client) *Handler {
	return &Handler{
		client: client,
	}
}

type Handler struct {
	client *posthog.Client
}

func invalidArgument(msg string) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New(msg))
}
