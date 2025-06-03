package main

import (
	"context"

	"github.com/gravitational/teleport/lib/kube/proxy"
)

func test(ctx context.Context) error {
	proxy.NewForwarder(proxy.ForwarderConfig{})
	return nil
}

func main() {
	if err := test(context.Background()); err != nil {
		println("Fai2l:", err.Error())
		return
	}
	println("success")
}
