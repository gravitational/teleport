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

package ui

import (
	"strings"

	inventoryv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/inventory/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
)

// UnifiedInstance represents either a Teleport instance or a bot instance for the WebUI.
type UnifiedInstance struct {
	// ID is the unique identifier for this item
	ID string `json:"id"`
	// Type is the type of instance, either "instance" or "bot_instance"
	Type string `json:"type"`
	// Instance contains the instance data if type is "instance"
	Instance *InstanceData `json:"instance,omitempty"`
	// BotInstance contains the bot instance data if type is "bot_instance"
	BotInstance *BotInstanceData `json:"botInstance,omitempty"`
}

// InstanceData represents a teleport instance item for the WebUI
type InstanceData struct {
	// Name is the hostname of the instance
	Name string `json:"name"`
	// Version is the version
	Version string `json:"version"`
	// Services is the list of services running on this instance
	Services []string `json:"services"`
	// Upgrader contains information about the external upgrader
	Upgrader *UpgraderInfo `json:"upgrader,omitempty"`
}

// BotInstanceData represents a bot instance item for the WebUI
type BotInstanceData struct {
	// Name is the name of the bot
	Name string `json:"name"`
	// Version is the bot version
	Version string `json:"version"`
}

// UpgraderInfo contains information about an external upgrader
type UpgraderInfo struct {
	// Type is the upgrader type
	Type string `json:"type"`
	// Version is the upgrader version
	Version string `json:"version"`
	// Group is the updater group
	Group string `json:"group"`
}

// MakeUnifiedInstance creates a UnifiedInstance from a UnifiedInstanceItem proto
func MakeUnifiedInstance(item *inventoryv1.UnifiedInstanceItem) UnifiedInstance {
	if instance := item.GetInstance(); instance != nil {
		return makeInstanceUnifiedItem(instance)
	}
	if botInstance := item.GetBotInstance(); botInstance != nil {
		return makeBotInstanceUnifiedItem(botInstance)
	}
	return UnifiedInstance{}
}

func makeInstanceUnifiedItem(instance *types.InstanceV1) UnifiedInstance {
	services := make([]string, 0, len(instance.Spec.Services))
	for _, service := range instance.Spec.Services {
		services = append(services, string(service))
	}

	instanceData := &InstanceData{
		Name:     instance.Spec.Hostname,
		Version:  strings.TrimPrefix(instance.Spec.Version, "v"),
		Services: services,
	}

	if instance.Spec.ExternalUpgrader != "" || instance.Spec.UpdaterInfo != nil {
		instanceData.Upgrader = &UpgraderInfo{
			Type: instance.Spec.ExternalUpgrader,
		}
		if instance.Spec.UpdaterInfo != nil {
			instanceData.Upgrader.Group = instance.Spec.UpdaterInfo.UpdateGroup
			// UpdaterV2Info doesn't have a version field so we hardcode "v2"
			instanceData.Upgrader.Version = "v2"
		}
	}

	return UnifiedInstance{
		ID:       instance.Metadata.Name,
		Type:     "instance",
		Instance: instanceData,
	}
}

func makeBotInstanceUnifiedItem(botInstance *machineidv1.BotInstance) UnifiedInstance {
	botData := &BotInstanceData{
		Name: botInstance.Spec.BotName,
	}

	if botInstance.Status != nil && len(botInstance.Status.LatestHeartbeats) > 0 {
		heartbeat := botInstance.Status.LatestHeartbeats[0]
		botData.Version = strings.TrimPrefix(heartbeat.Version, "v")
	}

	return UnifiedInstance{
		ID:          botInstance.Metadata.Name,
		Type:        "bot_instance",
		BotInstance: botData,
	}
}
