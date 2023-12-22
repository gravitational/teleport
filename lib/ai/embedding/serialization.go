/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package embedding

import (
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/types"
)

// SerializeNode converts a serializable resource into text ready to be fed to an
// embedding model. The YAML serialization function was chosen over JSON and
// CSV as it provided better results.
func SerializeResource(resource types.Resource) ([]byte, error) {
	switch resource.GetKind() {
	case types.KindNode:
		return SerializeNode(resource.(types.Server))
	case types.KindKubernetesCluster:
		return SerializeKubeCluster(resource.(types.KubeCluster))
	case types.KindApp:
		return SerializeApp(resource.(types.Application))
	case types.KindDatabase:
		return SerializeDatabase(resource.(types.Database))
	case types.KindWindowsDesktop:
		return SerializeWindowsDesktop(resource.(types.WindowsDesktop))
	default:
		return nil, trace.BadParameter("unknown resource kind %q", resource.GetKind())
	}
}

// SerializeNode converts a type.Server into text ready to be fed to an
// embedding model. The YAML serialization function was chosen over JSON and
// CSV as it provided better results.
func SerializeNode(node types.Server) ([]byte, error) {
	a := struct {
		Name    string            `yaml:"name"`
		Kind    string            `yaml:"kind"`
		SubKind string            `yaml:"subkind"`
		Labels  map[string]string `yaml:"labels"`
	}{
		// Create artificial Name file for the node "name". Using node.GetName() as Name seems to confuse the model.
		Name:    node.GetHostname(),
		Kind:    types.KindNode,
		SubKind: node.GetSubKind(),
		Labels:  node.GetAllLabels(),
	}
	text, err := yaml.Marshal(&a)
	return text, trace.Wrap(err)
}

// SerializeKubeCluster converts a type.KubeCluster into text ready to be fed to an
// embedding model. The YAML serialization function was chosen over JSON and
// CSV as it provided better results.
func SerializeKubeCluster(cluster types.KubeCluster) ([]byte, error) {
	a := struct {
		Name    string            `yaml:"name"`
		Kind    string            `yaml:"kind"`
		SubKind string            `yaml:"subkind"`
		Labels  map[string]string `yaml:"labels"`
	}{
		Name:    cluster.GetName(),
		Kind:    types.KindKubernetesCluster,
		SubKind: cluster.GetSubKind(),
		Labels:  cluster.GetAllLabels(),
	}
	text, err := yaml.Marshal(&a)
	return text, trace.Wrap(err)
}

// SerializeApp converts a type.Application into text ready to be fed to an
// embedding model. The YAML serialization function was chosen over JSON and
// CSV as it provided better results.
func SerializeApp(app types.Application) ([]byte, error) {
	a := struct {
		Name        string            `yaml:"name"`
		Kind        string            `yaml:"kind"`
		SubKind     string            `yaml:"subkind"`
		Labels      map[string]string `yaml:"labels"`
		Description string            `yaml:"description"`
	}{
		Name:        app.GetName(),
		Kind:        types.KindApp,
		SubKind:     app.GetSubKind(),
		Labels:      app.GetAllLabels(),
		Description: app.GetDescription(),
	}
	text, err := yaml.Marshal(&a)
	return text, trace.Wrap(err)
}

// SerializeDatabase converts a type.Database into text ready to be fed to an
// embedding model. The YAML serialization function was chosen over JSON and
// CSV as it provided better results.
func SerializeDatabase(db types.Database) ([]byte, error) {
	a := struct {
		Name        string            `yaml:"name"`
		Kind        string            `yaml:"kind"`
		SubKind     string            `yaml:"subkind"`
		Labels      map[string]string `yaml:"labels"`
		Type        string            `yaml:"type"`
		Description string            `yaml:"description"`
	}{
		Name:        db.GetName(),
		Kind:        types.KindDatabase,
		SubKind:     db.GetSubKind(),
		Labels:      db.GetAllLabels(),
		Type:        db.GetType(),
		Description: db.GetDescription(),
	}
	text, err := yaml.Marshal(&a)
	return text, trace.Wrap(err)
}

// SerializeWindowsDesktop converts a type.WindowsDesktop into text ready to be fed to an
// embedding model. The YAML serialization function was chosen over JSON and
// CSV as it provided better results.
func SerializeWindowsDesktop(desktop types.WindowsDesktop) ([]byte, error) {
	a := struct {
		Name     string            `yaml:"name"`
		Kind     string            `yaml:"kind"`
		SubKind  string            `yaml:"subkind"`
		Labels   map[string]string `yaml:"labels"`
		Address  string            `yaml:"address"`
		ADDomain string            `yaml:"ad_domain"`
	}{
		Name:     desktop.GetName(),
		Kind:     types.KindKubernetesCluster,
		SubKind:  desktop.GetSubKind(),
		Labels:   desktop.GetAllLabels(),
		Address:  desktop.GetAddr(),
		ADDomain: desktop.GetDomain(),
	}
	text, err := yaml.Marshal(&a)
	return text, trace.Wrap(err)
}
