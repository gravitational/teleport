/*
Copyright 2022 Gravitational, Inc.

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

import (
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

func main() {
	err := run()

	if err != nil {
		logrus.Fatal(err.Error())
	}
}

func run() error {
	args, err := parseArgs()
	if err != nil {
		return trace.Wrap(err, "failed to parse args")
	}

	err = NewGonWrapper(args.GonConfig).SignAndNotarizeBinaries()
	if err != nil {
		return trace.Wrap(err, "failed to sign and notarize binaries")
	}

	return nil
}
