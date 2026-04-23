package main

// Those imports are not used directly, but the enterprise tests depend on eauth, which depends on them.
// Without this file the go mod is not tidied the same way with and without the ent tests.
// This import makes sure the go mod looks the same, whether e tests are turned on or not.
// This is a temporary fix that should be revised once we merge everything into a monorepo.

import (
	_ "github.com/mailgun/errors"
	_ "github.com/mailgun/mailgun-go/v4"
	_ "gopkg.in/alexcesaro/quotedprintable.v3"
	_ "gopkg.in/mail.v2"
)
