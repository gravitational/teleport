package main

import (
	"context"
	teleport "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/tbotv2"
	"github.com/gravitational/teleport/lib/utils"
	"time"
)

func main() {
	log := utils.NewLogger()
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()
	b := &tbotv2.IdentityStreamManager{}

	go func() {
		b.Run()
	}()

	ids, err := b.StreamIdentity(tbotv2.IdentityRequest{
		Roles:   []string{"access"},
		TTL:     time.Minute * 5,
		Refresh: time.Minute,
	})
	if err != nil {
		return err
	}
	defer ids.Close()

	teleport.New(ctx, teleport.Config{
		Credentials: []teleport.Credentials{},
	})

	return nil
}
