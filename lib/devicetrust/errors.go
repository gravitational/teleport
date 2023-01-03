// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package devicetrust

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HandleUnimplemented turns remote unimplemented errors to a more user-friendly
// error.
func HandleUnimplemented(err error) error {
	for e := err; e != nil; {
		switch s, ok := status.FromError(e); {
		case ok && s.Code() == codes.Unimplemented:
			log.WithError(err).Debug("Device Trust: interpreting error as OSS or older Enterprise cluster")
			return errors.New("device trust not supported by remote cluster")
		case ok:
			return err // Unexpected status error.
		default:
			e = errors.Unwrap(e)
		}
	}
	return err
}
