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

package controller

import (
	"context"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gravitational/teleport/lib/autoupdate/agent"
)

const updaterIDFile = agent.BinaryName + ".id"

type StatusWriter struct {
	kclient.Client
	// Scheme for Kubernetes objects.
	Scheme *runtime.Scheme
	// UpdateID is the unique ID of the updater.
	UpdateID uuid.UUID
	// UpdateGroup is the group of the updater.
	UpdateGroup string
	// ProxyAddress is the address of the proxy.
	ProxyAddress string
}

// writeStatus updates the configmap with the latest status of the updater.
func (c *StatusWriter) writeStatus(ctx context.Context, owner metav1.Object, version string, failed bool) error {
	log := ctrllog.FromContext(ctx)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      owner.GetName() + "-updater",
			Namespace: owner.GetNamespace(),
		},
	}
	op, err := controllerutil.CreateOrUpdate(ctx, c.Client, cm, func() error {
		if err := c.generateData(ctx, cm, version, failed, time.Now()); err != nil {
			return trace.Wrap(err)
		}
		// Setting the owner ref ensures that the configmap is reset if the statefulset is deleted.
		return controllerutil.SetOwnerReference(owner, cm, c.Scheme)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	log.V(1).Info("updater configmap reconciled",
		"name", cm.Name,
		"namespace", owner.GetNamespace(),
		"op", string(op),
		"failed", failed,
		"version", version,
	)
	return nil
}

// generateData generates the data for the configmap based on the provided version and failed flag.
func (c *StatusWriter) generateData(ctx context.Context, cm *corev1.ConfigMap, version string, failed bool, updateTime time.Time) error {
	log := ctrllog.FromContext(ctx)
	spec := agent.UpdateSpec{
		Enabled: true,
		Proxy:   c.ProxyAddress,
		Group:   c.UpdateGroup,
	}

	var config agent.UpdateConfig
	switch s := cm.Data[agent.UpdateConfigName]; {
	case len(s) > 0:
		err := yaml.Unmarshal([]byte(s), &config)
		// Update spec to match controller configuration
		config.Spec = spec
		if err == nil {
			break
		}
		log.Error(err, "overwriting corrupted configmap data")
		fallthrough
	default:
		config = agent.UpdateConfig{
			Version: agent.UpdateConfigV1,
			Kind:    agent.UpdateConfigKind,
			Spec:    spec,
			Status: agent.UpdateStatus{
				IDFile: filepath.Join("/etc/updater-config", updaterIDFile),
				Active: agent.Revision{
					Version: version,
					// TODO: add enterprise and FIPS flags
				},
			},
		}
	}
	// Update the status if the update failed or if the version is present and different from the active one.
	if failed ||
		(version != "" && config.Status.Active.Version != version) {
		config.Status.Active.Version = version
		config.Status.LastUpdate = &agent.LastUpdate{
			Success: !failed,
			Time:    updateTime.UTC().Truncate(time.Millisecond),
			// TODO: set target version for update, requires GetVersion refactor
		}
	}
	out, err := yaml.Marshal(config)
	if err != nil {
		return trace.Wrap(err)
	}
	cm.Data = map[string]string{
		updaterIDFile:          c.UpdateID.String(),
		agent.UpdateConfigName: string(out),
	}
	return nil
}
