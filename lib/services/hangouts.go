/*
Copyright 2015 Gravitational, Inc.

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
package services

/*import (
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gokyle/hotp"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
)

// UpsertPasswordHash upserts user password hash
func (s *WebService) SetHangoutID(id string) error {
	err := s.backend.UpsertVal([]string{"hangout", "id", user},
		"pwd", hash, 0)
	if err != nil {
		log.Errorf(err.Error())
		return trace.Wrap(err)
	}
	return err
}

// GetPasswordHash returns the password hash for a given user
func (s *WebService) GetPasswordHash(user string) ([]byte, error) {
	hash, err := s.backend.GetVal([]string{"web", "users", user}, "pwd")
	if err != nil {
		return nil, err
	}
	return hash, err
}
*/
