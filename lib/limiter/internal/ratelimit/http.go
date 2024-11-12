/*
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

package ratelimit

import (
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
)

func ServeHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	var le *limitExceededError
	if errors.As(err, &le) {
		// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Retry-After#delay-seconds
		w.Header().Set("Retry-After", strconv.Itoa(int(le.delay.Seconds())+1))
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(err.Error()))
		return
	}

	status := http.StatusInternalServerError
	var netError net.Error

	if errors.Is(err, io.EOF) {
		status = http.StatusBadGateway
	} else if errors.As(err, &netError) {
		if netError.Timeout() {
			status = http.StatusGatewayTimeout
		} else {
			status = http.StatusBadGateway
		}
	}

	http.Error(w, http.StatusText(status), status)
}

func ExtractClientIP(r *http.Request) (string, error) {
	// TODO: use net.SplitHostPort to be compatible with IPv6
	token, _, _ := strings.Cut(r.RemoteAddr, ":")
	if token == "" {
		return "", errors.New("failed to extract source IP")
	}

	return token, nil
}
