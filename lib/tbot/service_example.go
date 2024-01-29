package tbot

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/teleport/lib/tbot/config"
)

// ExampleService is a temporary example service for testing purposes. It is
// not intended to be used and exists to demonstrate how a user configurable
// service integrates with the tbot service manager.
type ExampleService struct {
	cfg     *config.ExampleService
	Message string `yaml:"message"`
}

func (s *ExampleService) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second * 5):
			fmt.Println("Example Service prints message:", s.Message)
		}
	}
}

func (s *ExampleService) String() string {
	return fmt.Sprintf("%s:%s", config.ExampleServiceType, s.Message)
}
