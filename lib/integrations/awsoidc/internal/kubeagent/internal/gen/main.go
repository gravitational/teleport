// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Command gen renders the teleport-kube-agent Helm chart at build time and
// emits a compact Go source file containing compressed manifests, Helm hook
// metadata, and a slim chart object for Helm release-secret interop.
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"strings"

	"github.com/coreos/go-semver/semver"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"

	"github.com/gravitational/teleport/lib/integrations/awsoidc/internal/kubeagent"
)

const (
	releaseName          = "teleport-kube-agent"
	placeholderNamespace = "ns-placeholder"
	placeholderVersion   = "99.99.99"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("kubeagent-gen", flag.ContinueOnError)
	fs.SetOutput(stderr)

	chartDir := fs.String("chart", "", "path to the teleport-kube-agent chart directory")
	valuesFile := fs.String("values", "", "path to the values file to render with")
	outFile := fs.String("out", "zz_generated.go", "output file")

	if err := fs.Parse(arguments); err != nil {
		return fmt.Errorf("parsing arguments: %w", err)
	}

	if *chartDir == "" || *valuesFile == "" {
		return errors.New("both -chart and -values are required")
	}

	ch, err := loader.Load(*chartDir)
	if err != nil {
		return fmt.Errorf("loading chart from %q: %w", *chartDir, err)
	}

	renders := make(map[chartVariant]string, len(allChartVariants))
	for _, v := range allChartVariants {
		rendered, err := renderTemplates(ch, renderOptions{
			valuesFile: *valuesFile,
			roles:      strings.Split(v.roles, ","),
			updater:    v.updater,
			ha:         v.ha,
			enterprise: v.enterprise,
			skipHooks:  true,
		})
		if err != nil {
			return fmt.Errorf("render %s: %w", v.suffix(), err)
		}

		_, manifests, err := releaseutil.SortManifests(rendered, nil, releaseutil.InstallOrder)
		if err != nil {
			return fmt.Errorf("sort manifests %s: %w", v.suffix(), err)
		}

		renders[v] = joinManifestContent(manifests)
	}

	hooksByEnterprise := map[bool][]*release.Hook{}
	for _, v := range allHookVariants {
		renderedWithHooks, err := renderTemplates(ch, renderOptions{
			valuesFile: *valuesFile,
			roles:      strings.Split(RoleKubeAppDiscovery, ","),
			updater:    true,
			ha:         true,
			enterprise: v.enterprise,
			skipHooks:  false,
		})
		if err != nil {
			return fmt.Errorf("render hooks enterprise=%t: %w", v.enterprise, err)
		}

		hooks, _, err := releaseutil.SortManifests(renderedWithHooks, nil, releaseutil.InstallOrder)
		if err != nil {
			return fmt.Errorf("sort hooks enterprise=%t: %w", v.enterprise, err)
		}

		hooksByEnterprise[v.enterprise] = hooks
	}

	minifiedChart := struct {
		Metadata *chart.Metadata        `json:"metadata"`
		Values   map[string]interface{} `json:"values"`
	}{
		Metadata: ch.Metadata,
		Values:   ch.Values,
	}
	chartJSON, err := json.Marshal(minifiedChart)
	if err != nil {
		return fmt.Errorf("marshal minified chart: %w", err)
	}

	var buf bytes.Buffer
	if err := generateSource(&buf, renders, hooksByEnterprise, chartJSON); err != nil {
		return fmt.Errorf("generate source: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		_ = os.WriteFile(*outFile+".debug", buf.Bytes(), 0o644)
		return fmt.Errorf("gofmt generated source: %w", err)
	}

	switch *outFile {
	case "-":
		if _, err := bytes.NewReader(formatted).WriteTo(stdout); err != nil {
			return fmt.Errorf("write to stdout: %w", err)
		}
	default:
		if err := os.WriteFile(*outFile, formatted, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", *outFile, err)
		}
	}

	return nil
}

type hookVariant struct {
	enterprise bool
}

var allHookVariants = []hookVariant{
	{enterprise: false},
	{enterprise: true},
}

const (
	RoleKube             = "kube"
	RoleKubeAppDiscovery = "kube,app,discovery"
)

type renderOptions struct {
	valuesFile string
	roles      []string
	updater    bool
	ha         bool
	enterprise bool
	skipHooks  bool
}

func renderTemplates(ch *chart.Chart, opts renderOptions) (map[string]string, error) {
	replicas := 1
	if opts.ha {
		replicas = 2
	}

	setValues := []string{
		fmt.Sprintf("roles=%s", strings.Join(opts.roles, `\,`)),
		fmt.Sprintf("enterprise=%t", opts.enterprise),
		fmt.Sprintf("updater.enabled=%t", opts.updater),
		fmt.Sprintf("skipHooks=%t", opts.skipHooks),
		fmt.Sprintf("teleportVersionOverride=%s", placeholderVersion),
		fmt.Sprintf("highAvailability.replicaCount=%d", replicas),
		fmt.Sprintf("highAvailability.podDisruptionBudget.enabled=%t", opts.ha),
	}

	valOpts := &values.Options{
		ValueFiles: []string{opts.valuesFile},
		Values:     setValues,
	}
	vals, err := valOpts.MergeValues(getter.All(cli.New()))
	if err != nil {
		return nil, fmt.Errorf("merging values: %w", err)
	}

	relOpts := chartutil.ReleaseOptions{
		Name:      releaseName,
		Namespace: placeholderNamespace,
		Revision:  1,
		IsInstall: true,
	}
	rv, err := chartutil.ToRenderValues(ch, vals, relOpts, chartutil.DefaultCapabilities)
	if err != nil {
		return nil, fmt.Errorf("preparing render values: %w", err)
	}

	rendered, err := engine.Render(ch, rv)
	if err != nil {
		return nil, err
	}

	for name := range rendered {
		if strings.HasSuffix(name, "NOTES.txt") {
			delete(rendered, name)
		}
	}

	return rendered, nil
}

type chartVariant struct {
	roles      string
	updater    bool
	ha         bool
	enterprise bool
}

var allChartVariants = []chartVariant{
	{roles: RoleKube, updater: false, ha: false, enterprise: false},
	{roles: RoleKube, updater: false, ha: false, enterprise: true},
	{roles: RoleKube, updater: false, ha: true, enterprise: false},
	{roles: RoleKube, updater: false, ha: true, enterprise: true},
	{roles: RoleKube, updater: true, ha: false, enterprise: false},
	{roles: RoleKube, updater: true, ha: false, enterprise: true},
	{roles: RoleKube, updater: true, ha: true, enterprise: false},
	{roles: RoleKube, updater: true, ha: true, enterprise: true},
	{roles: RoleKubeAppDiscovery, updater: false, ha: false, enterprise: false},
	{roles: RoleKubeAppDiscovery, updater: false, ha: false, enterprise: true},
	{roles: RoleKubeAppDiscovery, updater: false, ha: true, enterprise: false},
	{roles: RoleKubeAppDiscovery, updater: false, ha: true, enterprise: true},
	{roles: RoleKubeAppDiscovery, updater: true, ha: false, enterprise: false},
	{roles: RoleKubeAppDiscovery, updater: true, ha: false, enterprise: true},
	{roles: RoleKubeAppDiscovery, updater: true, ha: true, enterprise: false},
	{roles: RoleKubeAppDiscovery, updater: true, ha: true, enterprise: true},
}

func (v chartVariant) Options() kubeagent.ChartOptions {
	return kubeagent.ChartOptions{
		Namespace:        placeholderNamespace,
		ProxyAddr:        "proxy-placeholder:443",
		AuthToken:        "token-placeholder",
		KubeClusterName:  "cluster-placeholder",
		Roles:            kubeagent.TeleportSystemRoles(v.roles),
		Enterprise:       v.enterprise,
		Updater:          v.updater,
		UpdaterChannel:   "channel-placeholder",
		HighAvailability: v.ha,
		RequestedVersion: semver.New(placeholderVersion),
	}
}

func (v chartVariant) suffix() string {
	var b strings.Builder
	for _, r := range strings.Split(v.roles, ",") {
		b.WriteString(strings.ToUpper(r[:1]))
		b.WriteString(r[1:])
	}

	if v.enterprise {
		b.WriteString("Enterprise")
	} else {
		b.WriteString("OSS")
	}

	if v.updater {
		b.WriteString("UpdaterEnabled")
	} else {
		b.WriteString("UpdaterDisabled")
	}

	if v.ha {
		b.WriteString("HAEnabled")
	} else {
		b.WriteString("HADisabled")
	}

	return b.String()
}

func (v chartVariant) condition() string {
	var roleConst string
	switch v.roles {
	case RoleKube:
		roleConst = "RoleKube"
	case RoleKubeAppDiscovery:
		roleConst = "RoleKubeAppDiscovery"
	default:
		panic("unknown roles")
	}

	return fmt.Sprintf("opts.Enterprise == %t && opts.Roles == %s && opts.Updater == %t && opts.HighAvailability == %t", v.enterprise, roleConst, v.updater, v.ha)
}

func joinManifestContent(manifests []releaseutil.Manifest) string {
	var b strings.Builder
	for i, m := range manifests {
		if i > 0 {
			b.WriteString("---\n")
		}
		b.WriteString(m.Content)
		if !strings.HasSuffix(m.Content, "\n") {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func generateSource(w io.Writer, renders map[chartVariant]string, hooks map[bool][]*release.Hook, chartJSON []byte) error {
	fmt.Fprintln(w, "// Code generated by kubeagent/internal/gen. DO NOT EDIT.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "package kubeagent")
	fmt.Fprintln(w)

	fmt.Fprintln(w, `const generatedVersion = "`+placeholderVersion+`"`)
	fmt.Fprintln(w)

	for _, v := range allChartVariants {
		fmt.Fprintf(w, "const compressedManifest%s = %q\n\n", v.suffix(), compressString(renders[v]))
	}

	fmt.Fprintln(w, "func compressedManifest(opts ChartOptions) (string, bool) {")
	fmt.Fprintln(w, "\tswitch {")
	for _, v := range allChartVariants {
		fmt.Fprintf(w, "\tcase %s:\n", v.condition())
		fmt.Fprintf(w, "\t\treturn compressedManifest%s, true\n", v.suffix())
	}
	fmt.Fprintln(w, "\tdefault:")
	fmt.Fprintln(w, "\t\treturn \"\", false")
	fmt.Fprintln(w, "\t}")
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)

	writeGeneratedHooks(w, "generatedHooksOSS", hooks[false])
	writeGeneratedHooks(w, "generatedHooksEnterprise", hooks[true])

	fmt.Fprintf(w, "const chartJSON = %s\n", backtickQuote(string(chartJSON)))
	return nil
}

func writeGeneratedHooks(w io.Writer, name string, hooks []*release.Hook) {
	fmt.Fprintf(w, "var %s = []generatedHook{\n", name)
	for _, h := range hooks {
		fmt.Fprintln(w, "\t{")
		fmt.Fprintf(w, "\t\tName: %q,\n", h.Name)
		fmt.Fprintf(w, "\t\tKind: %q,\n", h.Kind)
		fmt.Fprintf(w, "\t\tPath: %q,\n", h.Path)
		fmt.Fprintf(w, "\t\tManifest: %q,\n", compressString(h.Manifest))
		fmt.Fprintf(w, "\t\tEvents: %#v,\n", hookEvents(h.Events))
		fmt.Fprintf(w, "\t\tWeight: %d,\n", h.Weight)
		fmt.Fprintf(w, "\t\tDeletePolicies: %#v,\n", hookDeletePolicies(h.DeletePolicies))
		fmt.Fprintln(w, "\t},")
	}
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)
}

func hookEvents(events []release.HookEvent) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		out = append(out, event.String())
	}
	return out
}

func hookDeletePolicies(policies []release.HookDeletePolicy) []string {
	out := make([]string, 0, len(policies))
	for _, policy := range policies {
		out = append(out, string(policy))
	}
	return out
}

func compressString(s string) string {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte(s)); err != nil {
		panic(err)
	}
	if err := gz.Close(); err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func backtickQuote(s string) string {
	if strings.ContainsRune(s, '`') {
		return fmt.Sprintf("%q", s)
	}
	return "`" + s + "`"
}
