/*
Copyright 2026 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
