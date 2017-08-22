package oidc

import (
	"net/http"
	"testing"

	"golang.org/x/net/context"
)

func TestClientContext(t *testing.T) {
	myClient := &http.Client{}

	ctx := ClientContext(context.Background(), myClient)

	gotClient := clientFromContext(ctx)

	// Compare pointer values.
	if gotClient != myClient {
		t.Fatal("clientFromContext did not return the value set by ClientContext")
	}
}
