package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/teleport/lib/services"

	"github.com/buger/goterm"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
)

type collection interface {
	writeText(w io.Writer) error
	writeJSON(w io.Writer) error
	writeYAML(w io.Writer) error
}

type roleCollection struct {
	roles []services.Role
}

func (r *roleCollection) writeText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	printHeader(t, []string{"Role", "Allowed to login as", "Namespaces", "Node Labels", "Access to resources"})
	if len(r.roles) == 0 {
		_, err := io.WriteString(w, t.String())
		return trace.Wrap(err)
	}
	for _, r := range r.roles {
		fmt.Fprintf(t, "%v\t%v\t%v\t%v\t%v\n",
			r.GetMetadata().Name,
			strings.Join(r.GetLogins(), ","),
			strings.Join(r.GetNamespaces(), ","),
			printNodeLabels(r.GetNodeLabels()),
			printActions(r.GetResources()))
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

func (r *roleCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(r.toMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (r *roleCollection) toMarshal() interface{} {
	if len(r.roles) == 1 {
		return r.roles[0]
	}
	return r.roles
}

func (r *roleCollection) writeYAML(w io.Writer) error {
	data, err := yaml.Marshal(r.toMarshal())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

type namespaceCollection struct {
	namespaces []services.Namespace
}

func (n *namespaceCollection) writeText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	printHeader(t, []string{"Name"})
	if len(n.namespaces) == 0 {
		_, err := io.WriteString(w, t.String())
		return trace.Wrap(err)
	}
	for _, n := range n.namespaces {
		fmt.Fprintf(t, "%v\n", n.Metadata.Name)
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

func (n *namespaceCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(n.toMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (n *namespaceCollection) toMarshal() interface{} {
	if len(n.namespaces) == 1 {
		return n.namespaces[0]
	}
	return n.namespaces
}

func (n *namespaceCollection) writeYAML(w io.Writer) error {
	data, err := yaml.Marshal(n.toMarshal())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func printActions(resources map[string][]string) string {
	pairs := []string{}
	for key, actions := range resources {
		if key == services.Wildcard {
			return fmt.Sprintf("<all resources>: %v", strings.Join(actions, ","))
		}
		pairs = append(pairs, fmt.Sprintf("%v:%v", key, strings.Join(actions, ",")))
	}
	return strings.Join(pairs, ",")
}

func printNodeLabels(labels map[string]string) string {
	pairs := []string{}
	for key, val := range labels {
		if key == services.Wildcard {
			return "<all nodes>"
		}
		pairs = append(pairs, fmt.Sprintf("%v=%v", key, val))
	}
	return strings.Join(pairs, ",")
}
