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

package scripts

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	regexp "regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/google/safetext/shsprintf"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/utils"
)

// appURIPattern is a regexp excluding invalid characters from application URIs.
var appURIPattern = regexp.MustCompile(`^[-\w/:. ]+$`)

// ErrorBashScript is used to display friendly error message when
// there is an error prepping the actual script.
var ErrorBashScript = []byte(`
#!/bin/sh
echo -e "An error has occurred. \nThe token may be expired or invalid. \nPlease check log for further details."
exit 1
`)

// InstallNodeBashScript is the script that will run on user's machine
// to install teleport and join a teleport cluster.
//
//go:embed node-join/install.sh
var installNodeBashScriptRaw string

var installNodeBashScript = template.Must(template.New("nodejoin").Parse(installNodeBashScriptRaw))

// InstallNodeScriptOptions contains the options configuring the install-node script.
type InstallNodeScriptOptions struct {
	// Required for installation
	InstallOptions InstallScriptOptions

	// Required for joining
	Token      string
	CAPins     []string
	JoinMethod types.JoinMethod

	// Required for service configuration
	Labels        types.Labels
	LabelMatchers types.Labels

	AppServiceEnabled bool
	AppName           string
	AppURI            string

	DatabaseServiceEnabled  bool
	DiscoveryServiceEnabled bool
	DiscoveryGroup          string
}

// GetNodeInstallScript generates an agent installation script which will:
// - install Teleport
// - configure the Teleport agent joining
// - configure the Teleport agent services (currently support ssh, app, database, and discovery)
// - start the agent
func GetNodeInstallScript(ctx context.Context, opts InstallNodeScriptOptions) (string, error) {
	// Computing installation-related values

	// By default, it will use `stable/v<majorVersion>`, eg stable/v12
	repoChannel := ""

	switch opts.InstallOptions.AutoupdateStyle {
	case NoAutoupdate, UpdaterBinaryAutoupdate:
	case PackageManagerAutoupdate:
		// Note: This is a cloud-specific repo. We could use the new stable/rolling
		// repo in non-cloud case, but the script has never support enabling autoupdates
		// in a non-cloud cluster.
		// We will prefer using the new updater binary for autoupdates in self-hosted setups.
		repoChannel = automaticupgrades.DefaultCloudChannelName
	default:
		return "", trace.BadParameter("unsupported autoupdate style: %v", opts.InstallOptions.AutoupdateStyle)
	}

	// Computing joining-related values
	hostname, portStr, err := utils.SplitHostPort(opts.InstallOptions.ProxyAddr)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Computing service configuration-related values
	labelsList := []string{}
	for labelKey, labelValues := range opts.Labels {
		labelKey = shsprintf.EscapeDefaultContext(labelKey)
		for i := range labelValues {
			labelValues[i] = shsprintf.EscapeDefaultContext(labelValues[i])
		}
		labels := strings.Join(labelValues, " ")
		labelsList = append(labelsList, fmt.Sprintf("%s=%s", labelKey, labels))
	}

	var dbServiceResourceLabels []string
	if opts.DatabaseServiceEnabled {
		dbServiceResourceLabels, err = marshalLabelsYAML(opts.LabelMatchers, 6)
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	var appServerResourceLabels []string

	if opts.AppServiceEnabled {
		if errs := validation.IsDNS1035Label(opts.AppName); len(errs) > 0 {
			return "", trace.BadParameter("appName %q must be a valid DNS subdomain: https://goteleport.com/docs/enroll-resources/application-access/guides/connecting-apps/#application-name", opts.AppName)
		}
		if !appURIPattern.MatchString(opts.AppURI) {
			return "", trace.BadParameter("appURI %q contains invalid characters", opts.AppURI)
		}

		appServerResourceLabels, err = marshalLabelsYAML(opts.Labels, 4)
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	if opts.DiscoveryServiceEnabled {
		if opts.DiscoveryGroup == "" {
			return "", trace.BadParameter("discovery group is required")
		}
	}

	var buf bytes.Buffer

	// TODO(hugoShaka): burn this map and replace it by something saner in a future PR.

	// This section relies on Go's default zero values to make sure that the settings
	// are correct when not installing an app.
	err = installNodeBashScript.Execute(&buf, map[string]interface{}{
		"token":    shsprintf.EscapeDefaultContext(opts.Token),
		"hostname": hostname,
		"port":     portStr,
		// The install.sh script has some manually generated configs and some
		// generated by the `teleport <service> config` commands. The old bash
		// version used space delimited values whereas the teleport command uses
		// a comma delimeter. The Old version can be removed when the install.sh
		// file has been completely converted over.
		"caPinsOld":               strings.Join(opts.CAPins, " "),
		"caPins":                  strings.Join(opts.CAPins, ","),
		"packageName":             opts.InstallOptions.TeleportFlavor,
		"repoChannel":             repoChannel,
		"installUpdater":          opts.InstallOptions.AutoupdateStyle.String(),
		"version":                 shsprintf.EscapeDefaultContext(opts.InstallOptions.TeleportVersion),
		"appInstallMode":          strconv.FormatBool(opts.AppServiceEnabled),
		"appServerResourceLabels": appServerResourceLabels,
		"appName":                 shsprintf.EscapeDefaultContext(opts.AppName),
		"appURI":                  shsprintf.EscapeDefaultContext(opts.AppURI),
		"joinMethod":              shsprintf.EscapeDefaultContext(string(opts.JoinMethod)),
		"labels":                  strings.Join(labelsList, ","),
		"databaseInstallMode":     strconv.FormatBool(opts.DatabaseServiceEnabled),
		// No one knows why this field is in snake case ¯\_(ツ)_/¯
		// Also, even if the name is similar to appServerResourceLabels, they must not be confused.
		// appServerResourceLabels are labels to apply on the declared app, while
		// db_service_resource_labels are labels matchers for the service to select resources to serve.
		"db_service_resource_labels": dbServiceResourceLabels,
		"discoveryInstallMode":       strconv.FormatBool(opts.DiscoveryServiceEnabled),
		"discoveryGroup":             shsprintf.EscapeDefaultContext(opts.DiscoveryGroup),
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	return buf.String(), nil
}

// TODO(hugoShaka): burn the indentation thing, this is too fragile and show be handled
// by the template itself.

// marshalLabelsYAML returns a list of strings, each one containing a
// label key and list of value's pair.
// This is used to create yaml sections within the join scripts.
//
// The arg `extraListIndent` allows adding `extra` indent space on
// top of the default space already used, for the default yaml listing
// format (the listing values with the dashes). If `extraListIndent`
// is zero, it's equivalent to using default space only (which is 4 spaces).
func marshalLabelsYAML(resourceMatcherLabels types.Labels, extraListIndent int) ([]string, error) {
	if len(resourceMatcherLabels) == 0 {
		return []string{"{}"}, nil
	}

	ret := []string{}

	// Consistently iterate over fields
	labelKeys := make([]string, 0, len(resourceMatcherLabels))
	for k := range resourceMatcherLabels {
		labelKeys = append(labelKeys, k)
	}

	sort.Strings(labelKeys)

	for _, labelName := range labelKeys {
		labelValues := resourceMatcherLabels[labelName]
		bs, err := yaml.Marshal(map[string]apiutils.Strings{labelName: labelValues})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		labelStr := strings.TrimSpace(string(bs))
		if len(labelValues) > 1 && extraListIndent > 0 {
			labelStr = addExtraListIndentToYAMLLabelStr(labelStr, extraListIndent)
		}

		ret = append(ret, labelStr)
	}

	return ret, nil
}

func addExtraListIndentToYAMLLabelStr(labelStr string, indent int) string {
	words := strings.Split(labelStr, "\n")
	// Skip the first word, since that is the label key.
	// Add extra spaces defined by `yamlListIndent` arg.
	for i := 1; i < len(words); i++ {
		words[i] = fmt.Sprintf("%s%s", strings.Repeat(" ", indent), words[i])
	}

	return strings.Join(words, "\n")
}
