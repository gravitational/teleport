/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package readyz

import (
	"encoding/json"
	"net/http"
)

// Status describes the healthiness of a service or tbot overall.
type Status uint

const (
	// Initializing means no status has been reported for the service.
	Initializing Status = iota

	// Healthy means the service is healthy and ready to serve traffic or it has
	// recently succeeded generating an output.
	Healthy

	// Unhealthy means the service is failing to serve traffic or generate output.
	Unhealthy
)

// String implements fmt.Stringer.
func (s Status) String() string {
	switch s {
	case Initializing:
		return "initializing"
	case Healthy:
		return "healthy"
	case Unhealthy:
		return "unhealthy"
	default:
		return "<unknown status>"
	}
}

// MarshalJSON implements json.Marshaler.
func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// MarshalJSON implements json.Unmarshaler.
func (s *Status) UnmarshalJSON(j []byte) error {
	var str string
	if err := json.Unmarshal(j, &str); err != nil {
		return err
	}
	switch str {
	case "healthy":
		*s = Healthy
	case "unhealthy":
		*s = Unhealthy
	default:
		*s = Initializing
	}
	return nil
}

// HTTPStatusCode returns the HTTP response code that represents this status.
func (s Status) HTTPStatusCode() int {
	switch s {
	case Healthy:
		return http.StatusOK
	default:
		return http.StatusServiceUnavailable
	}
}
