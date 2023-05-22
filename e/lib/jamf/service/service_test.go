package service_test

import (
	"context"
	"errors"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	jamfservice "github.com/gravitational/teleport/e/lib/jamf/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

func TestS_Run_stopsOnCancel(t *testing.T) {
	env := testenv.MustNew()
	defer env.Close()

	logger := log.New()
	logger.SetLevel(log.PanicLevel) // mostly silent logger

	s, err := jamfservice.New(jamfservice.Opts{
		Logger: logger,
		Config: &servicecfg.JamfConfig{
			Spec: &types.JamfSpecV1{
				Enabled:     true,
				ApiEndpoint: "https://yourtenant.jamfcloud.com",
				Username:    "llama",
				Password:    "secret!!1!",
			},
		},
		DevicesClient: env.DevicesClient,
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	started := make(chan struct{})
	exited := make(chan error)
	go func() {
		started <- struct{}{}
		exited <- s.Run(ctx)
	}()

	// Assert, to some extent, that the service started running.
	<-started
	select {
	case <-exited:
		t.Fatalf("Service exited before context cancellation")
	default:
		// OK, expected
	}

	// Service should stop on cancel
	cancel()
	if err := <-exited; !errors.Is(err, context.Canceled) {
		t.Errorf("Service exited with err=%q, wanted context.Canceled", err)
	}
}
