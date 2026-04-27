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

// Command gen renders the teleport-kube-agent Helm chart and emits Go source
// with typed constructors for every rendered Kubernetes resource.
//
// The chart is rendered in-process via the helm.sh/helm/v3 Go SDK. This
// command is in a separate Go module to keep the dependencies isolated
// from the main module to enable dead code elimination.
//
// It is meant to be invoked via go:generate from kubeagent/manifest.go.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"slices"
	"strings"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/getter"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	fs := flag.NewFlagSet("kubeagent-gen", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	chartDir := fs.String("chart", "", "path to the teleport-kube-agent chart directory")
	valuesFile := fs.String("values", "", "path to the values file to render with")
	outFile := fs.String("out", "zz_generated.go", "output file")

	if err := fs.Parse(arguments); err != nil {
		return fmt.Errorf("parsing arguments: %w", err)
	}

	if *chartDir == "" || *valuesFile == "" {
		return errors.New("both -chart and -values are required")
	}

	// Load the chart once and reuse it below.
	ch, err := loader.Load(*chartDir)
	if err != nil {
		return fmt.Errorf("loading chart from %q: %w", *chartDir, err)
	}

	// Render the full helm chart matrix. Every variant gets its own set
	// of constructors keyed by the options. A public dispatcher per
	// resource selects the right one based on Options at runtime.
	renders := map[variant]map[resourceID]runtime.Object{}
	for _, v := range allVariants {
		objs, err := renderHelmChart(ch, renderOptions{valuesFile: *valuesFile, roles: strings.Split(v.roles, ","), updater: v.updater, ha: v.ha})
		if err != nil {
			return fmt.Errorf("render %s: %w", v.suffix(), err)
		}
		renders[v] = objs
	}

	// Serialize the loaded chart to JSON for embedding in the helm
	// release-storage Secret. helm reads this on `helm upgrade` to
	// re-render the chart with new values.
	chartJSON, err := json.Marshal(ch)
	if err != nil {
		return fmt.Errorf("marshaling chart to JSON: %w", err)
	}

	var buf bytes.Buffer
	if err := generateSourceCode(&buf, renders, chartJSON); err != nil {
		return fmt.Errorf("emit: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Write unformatted output to a file to make debugging easier.
		debugPath := *outFile + ".debug"
		_ = os.WriteFile(debugPath, buf.Bytes(), 0o644)
		return fmt.Errorf("gofmt failed: %w (unformatted output written to %s)", err, debugPath)
	}

	if err := os.WriteFile(*outFile, formatted, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", *outFile, err)
	}

	return nil
}

// resourceID uniquely identifies a rendered Kubernetes object.
type resourceID struct {
	Kind string
	Name string
}

// compare orders resources by Kind, then Name.
func (a resourceID) compare(b resourceID) int {
	if c := strings.Compare(a.Kind, b.Kind); c != 0 {
		return c
	}
	return strings.Compare(a.Name, b.Name)
}

// renderOptions captures the chart-value overrides applied per render.
type renderOptions struct {
	valuesFile string
	// roles is rendered as `--set roles=<comma-joined>`.
	roles []string
	// updater is rendered as `--set updater.enabled=<bool>`.
	updater bool
	// ha drives the HA-related --set flags. When true:
	// highAvailability.replicaCount=2 and
	// highAvailability.podDisruptionBudget.enabled=true. When false: 1 and false.
	ha bool
}

// renderHelmChart renders the helm chart and returns the decoded objects keyed by Kind and Name.
func renderHelmChart(ch *chart.Chart, opts renderOptions) (map[resourceID]runtime.Object, error) {
	rendered, err := renderTemplates(ch, opts)
	if err != nil {
		return nil, err
	}

	out := map[resourceID]runtime.Object{}
	for path, content := range rendered {
		if strings.TrimSpace(content) == "" ||
			!strings.Contains(path, "/templates/") ||
			!strings.HasSuffix(path, ".yaml") &&
				!strings.HasSuffix(path, ".yml") {
			continue
		}

		objs, err := decodeObjects(content)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}

		for _, obj := range objs {
			if sts, ok := obj.(*appsv1.StatefulSet); ok {
				delete(sts.Spec.Template.Annotations, "checksum/config")
			}

			var id resourceID
			if gvks, _, err := scheme.Scheme.ObjectKinds(obj); err == nil && len(gvks) > 0 {
				id.Kind = gvks[0].Kind
			}
			if mo, ok := obj.(interface{ GetName() string }); ok {
				id.Name = mo.GetName()
			}

			if _, exists := out[id]; exists {
				return nil, fmt.Errorf("duplicate resource %s/%s (rendered by %s)", id.Kind, id.Name, path)
			}
			out[id] = obj
		}
	}
	return out, nil
}

// variant identifies one combination of the (roles, updater, ha) render
// matrix. Each variant yields its own set of constructors emitted by the
// generator. A public dispatcher per resource then maps Options to the
// appropriate constructor.
type variant struct {
	roles   string
	updater bool
	ha      bool
}

// allVariants is the full Cartesian product of configurations the chart
// conditionalizes on. When a new helm chart configuration is added it must
// be reflected here for the generator to pick it up.
var allVariants = []variant{
	{roles: "kube", updater: false, ha: false},
	{roles: "kube", updater: false, ha: true},
	{roles: "kube", updater: true, ha: false},
	{roles: "kube", updater: true, ha: true},
	{roles: "kube,app,discovery", updater: false, ha: false},
	{roles: "kube,app,discovery", updater: false, ha: true},
	{roles: "kube,app,discovery", updater: true, ha: false},
	{roles: "kube,app,discovery", updater: true, ha: true},
}

// suffix returns the Go-identifier suffix used in per variant
// constructor names.
func (v variant) suffix() string {
	var s strings.Builder
	for _, r := range strings.Split(v.roles, ",") {
		s.WriteString(strings.ToUpper(r[:1]))
		s.WriteString(r[1:])
	}

	if v.updater {
		s.WriteString("UpdaterEnabled")
	} else {
		s.WriteString("UpdaterDisabled")
	}

	if v.ha {
		s.WriteString("HAEnabled")
	} else {
		s.WriteString("HADisabled")
	}

	return s.String()
}

func (v variant) optsExpr() string {
	var rolesConst string
	switch v.roles {
	case "kube":
		rolesConst = "RoleKube"
	case "kube,app,discovery":
		rolesConst = "RoleKubeAppDiscovery"
	default:
		panic(fmt.Sprintf("unexpected roles combination %q", v.roles))
	}

	return fmt.Sprintf("opts.Roles == %s && opts.Updater == %t && opts.HighAvailability == %t", rolesConst, v.updater, v.ha)
}

// renderTemplates runs the helm rendering engine in-process and returns its
// raw map of template path to rendered content.
func renderTemplates(ch *chart.Chart, opts renderOptions) (map[string]string, error) {
	var setValues []string
	if len(opts.roles) > 0 {
		setValues = append(setValues, "roles="+strings.Join(opts.roles, `\,`))
	}

	setValues = append(setValues, fmt.Sprintf("updater.enabled=%t", opts.updater))

	replicas := 1
	if opts.ha {
		replicas = 2
	}
	setValues = append(setValues,
		fmt.Sprintf("highAvailability.replicaCount=%d", replicas),
		fmt.Sprintf("highAvailability.podDisruptionBudget.enabled=%t", opts.ha),
	)

	valOpts := &values.Options{
		ValueFiles: []string{opts.valuesFile},
		Values:     setValues,
	}

	vals, err := valOpts.MergeValues(getter.All(cli.New()))
	if err != nil {
		return nil, fmt.Errorf("merging values: %w", err)
	}

	const (
		// releaseName is the helm release name passed to the engine
		releaseName = "teleport-kube-agent"

		// placeholderNamespace is the release namespace passed to the helm engine
		placeholderNamespace = "ns-placeholder"
	)

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

	return engine.Render(ch, rv)
}

// decodeObjects parses the content of a rendered template into the typed
// runtime.Objects it contains. Multi-doc inputs are returned in
// document order. Empty or whitespace docs are skipped.
func decodeObjects(content string) ([]runtime.Object, error) {
	decoder := scheme.Codecs.UniversalDeserializer()
	reader := utilyaml.NewYAMLReader(bufio.NewReader(strings.NewReader(content)))
	var out []runtime.Object
	for i := 0; ; i++ {
		doc, err := reader.Read()
		switch {
		case errors.Is(err, io.EOF):
			return out, nil
		case err != nil:
			return nil, fmt.Errorf("doc %d read: %w", i, err)
		case len(bytes.TrimSpace(doc)) == 0:
			continue
		}

		obj, _, err := decoder.Decode(doc, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("doc %d: %w", i, err)
		}
		out = append(out, obj)
	}
}

// generateSourceCode creates and writes the Go source from the rendered
// Kubernetes resources to w. For each (Kind, Name) it emits one
// variant-keyed constructor per variant where the resource exists,
// plus a public dispatcher that selects the right variant at runtime
// based on Options.
func generateSourceCode(w io.Writer, renders map[variant]map[resourceID]runtime.Object, chartJSON []byte) error {
	var body strings.Builder
	usedAliases := map[string]bool{}

	kubeOnlyTeleportConfig := extractTeleportConfig(renders[variant{roles: "kube", updater: true, ha: true}])
	if kubeOnlyTeleportConfig == "" {
		return errors.New("no teleport.yaml found in ConfigMap rendered with roles=kube")
	}
	allRolesTeleportConfig := extractTeleportConfig(renders[variant{roles: "kube,app,discovery", updater: true, ha: true}])
	if allRolesTeleportConfig == "" {
		return errors.New("no teleport.yaml found in ConfigMap rendered with roles=kube,app,discovery")
	}

	fmt.Fprintln(&body, "// Potential teleport.yaml payloads. Picked at runtime based on the specified roles.")
	fmt.Fprintf(&body, "const teleportConfigKube = %s\n\n", backtickQuote(kubeOnlyTeleportConfig))
	fmt.Fprintf(&body, "const teleportConfigKubeAppDiscovery = %s\n\n", backtickQuote(allRolesTeleportConfig))

	fmt.Fprintln(&body, "// chartJSON is the helm chart serialized to JSON. Embedded into the")
	fmt.Fprintln(&body, "// helm release-storage Secret at runtime so `helm upgrade` can")
	fmt.Fprintln(&body, "// re-render the chart with new values.")
	fmt.Fprintf(&body, "var chartJSON = []byte(%s)\n\n", backtickQuote(string(chartJSON)))

	// Collect every (Kind, Name) seen in any variant, plus the per-variant
	// type names so the dispatcher can declare its return type. The type
	// name is identical across variants for a given (Kind, Name).
	allIDs := map[resourceID]bool{}
	for _, r := range renders {
		for id := range r {
			allIDs[id] = true
		}
	}
	ids := make([]resourceID, 0, len(allIDs))
	for id := range allIDs {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, resourceID.compare)

	for _, id := range ids {
		var typeName string
		for _, v := range allVariants {
			obj, ok := renders[v][id]
			if !ok {
				continue
			}

			emittedType, err := emitConstructor(&body, id, obj, usedAliases, v.suffix())
			if err != nil {
				return fmt.Errorf("emit %s/%s/%s: %w", id.Kind, id.Name, v.suffix(), err)
			}

			typeName = emittedType
		}
		emitDispatcher(&body, id, typeName, renders)
	}

	fmt.Fprintln(w, "// Code generated by kubeagent/internal/gen. DO NOT EDIT.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "package kubeagent")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "import (")
	// Iterate packageAliases in sorted path order so the pre-gofmt buffer
	// (written to zz_generated.go.debug on gofmt failure) is deterministic.
	// go/format.Source will re-sort the imports in the final output.
	paths := make([]string, 0, len(packageAliases))
	for path := range packageAliases {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		alias := packageAliases[path]
		if !usedAliases[alias] {
			continue
		}

		fmt.Fprintf(w, "\t%s %q\n", alias, path)
	}
	fmt.Fprintln(w, ")")
	fmt.Fprintln(w)
	_, err := io.WriteString(w, body.String())
	return err
}

// emitDispatcher writes the public function that switches on Options
// and forwards to the appropriate constructor.
func emitDispatcher(w io.Writer, id resourceID, typeName string, renders map[variant]map[resourceID]runtime.Object) {
	dispatcherName := defaultConstructorName(id)
	fmt.Fprintf(w, "func %s(opts Options) *%s {\n", dispatcherName, typeName)
	fmt.Fprintln(w, "\tswitch {")
	for _, v := range allVariants {
		if _, ok := renders[v][id]; !ok {
			continue
		}
		fmt.Fprintf(w, "\tcase %s:\n", v.optsExpr())
		fmt.Fprintf(w, "\t\treturn %s%s(opts)\n", dispatcherName, v.suffix())
	}
	fmt.Fprintln(w, "\t}")
	fmt.Fprintf(w, "\tpanic(%q)\n", fmt.Sprintf("%s: no rendered variant for the given Options", dispatcherName))
	fmt.Fprintln(w, "}")
	fmt.Fprintln(w)
}

func defaultConstructorName(id resourceID) string {
	switch {
	case strings.HasSuffix(id.Name, "-updater"):
		return "genUpdater" + id.Kind
	case id.Kind == "Secret" && id.Name == "teleport-kube-agent-join-token":
		return "genSecret"
	default:
		return "gen" + id.Kind
	}
}

// emitConstructor writes a `func genFoo(opts Options) *Type { return
// &Type{...} }` declaration for obj. Returns the rendered Go type name so
// the caller can reuse it on the dispatcher's return type.
func emitConstructor(w io.Writer, id resourceID, obj runtime.Object, used map[string]bool, suffix string) (string, error) {
	constructorName := defaultConstructorName(id) + suffix

	if cm, ok := obj.(*corev1.ConfigMap); ok {
		// The data is populated at runtime based on the Teleport roles.
		obj = cm.DeepCopy()
		obj.(*corev1.ConfigMap).Data = nil
	}

	p := newPrinter(used)
	p.writeRootPointer(obj)

	fmt.Fprintf(w, "func %s(opts Options) *%s {\n", constructorName, p.rootTypeName)
	fmt.Fprintf(w, "\treturn %s\n", p.String())
	fmt.Fprintf(w, "}\n\n")
	return p.rootTypeName, nil
}

// extractTeleportConfig returns the teleport.yaml contents specified
// in the ConfigMap.
func extractTeleportConfig(objects map[resourceID]runtime.Object) string {
	obj, ok := objects[resourceID{Kind: "ConfigMap", Name: "teleport-kube-agent"}]
	if !ok {
		return ""
	}
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return ""
	}
	return cm.Data["teleport.yaml"]
}

// backtickQuote wraps s in backticks for embedding as a Go raw string literal.
// Falls back to strconv.Quote (via %q) if s contains a backtick.
func backtickQuote(s string) string {
	if strings.ContainsRune(s, '`') {
		return fmt.Sprintf("%q", s)
	}
	return "`" + s + "`"
}
