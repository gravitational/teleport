/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
