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

package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/lib/automaticupgrades/constants"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
)

const defaultChannelTimeout = 5 * time.Second

// automaticUpgrades109 implements a version server in the Teleport Proxy following the RFD 109 spec.
// It is configured through the Teleport Proxy configuration and tells agent updaters
// which version they should install.
func (h *Handler) automaticUpgrades109(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	if h.cfg.AutomaticUpgradesChannels == nil {
		return nil, trace.AccessDenied("This proxy is not configured to serve automatic upgrades channels.")
	}

	// The request format is "<channel name>/{version,critical}"
	// As <channel name> might contain "/" we have to split, pop the last part
	// and re-construct the channel name.
	channelAndType := p.ByName("request")

	reqParts := strings.Split(strings.Trim(channelAndType, "/"), "/")
	if len(reqParts) < 2 {
		return nil, trace.BadParameter("path format should be /webapi/automaticupgrades/channel/<channel>/{version,critical}")
	}
	requestType := reqParts[len(reqParts)-1]
	channelName := strings.Join(reqParts[:len(reqParts)-1], "/")

	if channelName == "" {
		return nil, trace.BadParameter("a channel name is required")
	}

	// Finally, we treat the request based on its type
	switch requestType {
	case "version":
		h.log.Debugf("Agent requesting version for channel %s", channelName)
		return h.automaticUpgradesVersion109(w, r, channelName)
	case "critical":
		h.log.Debugf("Agent requesting criticality for channel %s", channelName)
		return h.automaticUpgradesCritical109(w, r, channelName)
	default:
		return nil, trace.BadParameter("requestType path must end with 'version' or 'critical'")
	}
}

// automaticUpgradesVersion109 handles version requests from upgraders
func (h *Handler) automaticUpgradesVersion109(w http.ResponseWriter, r *http.Request, channelName string) (interface{}, error) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultChannelTimeout)
	defer cancel()

	targetVersion, err := h.autoUpdateAgentVersion(ctx, channelName, "" /* updater UUID */)
	if err != nil {
		// If the error is that the upstream channel has no version
		// We gracefully handle by serving "none"
		var NoNewVersionErr *version.NoNewVersionError
		if errors.As(trace.Unwrap(err), &NoNewVersionErr) {
			_, err = w.Write([]byte(constants.NoVersion))
			return nil, trace.Wrap(err)
		}
		// Else we propagate the error
		return nil, trace.Wrap(err)
	}

	// RFD 109 specifies that version from channels must have the leading "v".
	// As h.autoUpdateAgentVersion doesn't, we must add it.
	_, err = fmt.Fprintf(w, "v%s", targetVersion)
	return nil, trace.Wrap(err)
}

// automaticUpgradesCritical109 handles criticality requests from upgraders
func (h *Handler) automaticUpgradesCritical109(w http.ResponseWriter, r *http.Request, channelName string) (interface{}, error) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultChannelTimeout)
	defer cancel()

	// RFD109 agents already retrieve maintenance windows from the CMC, no need to
	// do a maintenance window lookup for them.
	critical, err := h.autoUpdateAgentShouldUpdate(ctx, channelName, "" /* updater UUID */, false /* window lookup */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := "no"
	if critical {
		response = "yes"
	}
	_, err = w.Write([]byte(response))
	return nil, trace.Wrap(err)
}
