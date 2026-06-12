/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/autoupdate/agent"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/config/transform"
	"github.com/gravitational/teleport/lib/defaults"
)

var scopedConfigureMigrateJoinMethods = []string{
	string(types.JoinMethodToken),
	string(types.JoinMethodIAM),
	string(types.JoinMethodEC2),
	string(types.JoinMethodGCP),
	string(types.JoinMethodAzure),
	string(types.JoinMethodAzureDevops),
	string(types.JoinMethodOracle),
	string(types.JoinMethodKubernetes),
	string(types.JoinMethodBoundKeypair),
}

type configureMigrateFlags struct {
	input             string
	installSuffix     string
	output            string
	proxyServer       string
	authServer        string
	joinMethod        string
	token             string
	tokenName         string
	tokenSecretFile   string
	dataDir           string
	disableServices   string
	labels            []string
	diff              bool
	force             bool
	test              bool
	parsedDisable     []string
	parsedLabels      map[string]string
	stdout            io.Writer
	stderr            io.Writer
	normalizedOutput  string
	normalizedDataDir string
}

func (f *configureMigrateFlags) CheckAndSetDefaults() error {
	if f.input == "" {
		f.input = defaults.ConfigFilePath
	}
	if f.stdout == nil {
		f.stdout = os.Stdout
	}
	if f.stderr == nil {
		f.stderr = os.Stderr
	}
	if f.joinMethod == "" {
		f.joinMethod = string(types.JoinMethodToken)
	}
	if !slices.Contains(scopedConfigureMigrateJoinMethods, f.joinMethod) {
		return trace.BadParameter("unsupported join method %q", f.joinMethod)
	}
	if f.proxyServer == "" && f.authServer == "" {
		return trace.BadParameter("one of --proxy-server or --auth-server is required")
	}
	if f.proxyServer != "" && f.authServer != "" {
		return trace.BadParameter("only one of --proxy-server or --auth-server can be set")
	}
	if f.token != "" && (f.tokenName != "" || f.tokenSecretFile != "") {
		return trace.BadParameter("--token cannot be combined with --token-name or --token-secret-file; --token uses legacy single-value token semantics, while --token-name and --token-secret-file split the scoped token name and secret")
	}
	if f.token != "" {
		f.tokenName = f.token
	}
	if f.tokenName == "" {
		return trace.BadParameter("--token-name is required")
	}
	switch types.JoinMethod(f.joinMethod) {
	case types.JoinMethodToken:
		if f.token == "" && f.tokenSecretFile == "" {
			return trace.BadParameter("--token-secret-file is required when --join-method=token")
		}
	default:
		if f.tokenSecretFile != "" {
			return trace.BadParameter("--token-secret-file is only supported when --join-method=token")
		}
	}
	if f.installSuffix == "" && (f.output == "" || f.dataDir == "") {
		return trace.BadParameter("--install-suffix is required unless both --output and --data-dir are set")
	}
	if f.installSuffix != "" {
		if err := validateInstallSuffix(f.installSuffix); err != nil {
			return trace.Wrap(err)
		}
	}
	if f.dataDir == "" {
		f.normalizedDataDir = defaultMigrateDataDir(f.installSuffix)
	} else {
		f.normalizedDataDir = f.dataDir
	}
	if f.output == "" || f.output == teleport.SchemeFile {
		if f.installSuffix == "" {
			return trace.BadParameter("--install-suffix is required when --output=file uses the default migrated config path")
		}
		f.normalizedOutput = teleport.SchemeFile + "://" + defaultMigrateConfigPath(f.installSuffix)
	} else if f.output == teleport.SchemeStdout {
		f.normalizedOutput = teleport.SchemeStdout + "://"
	} else {
		f.normalizedOutput = f.output
	}
	f.parsedDisable = parseDisableServices(f.disableServices)
	labels, err := parseMigrateLabels(f.labels)
	if err != nil {
		return trace.Wrap(err)
	}
	f.parsedLabels = labels
	return nil
}

func onConfigureMigrate(flags configureMigrateFlags) error {
	if err := flags.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	raw, err := os.ReadFile(flags.input)
	if err != nil {
		return trace.Wrap(trace.ConvertSystemError(err), "failed reading input config %q", flags.input)
	}
	doc, err := transform.Load(raw)
	if err != nil {
		return trace.Wrap(err, "failed parsing input config %q", flags.input)
	}

	result, err := transform.ApplyMigration(doc, transform.MigrateParams{
		InstallSuffix:   flags.installSuffix,
		ProxyServer:     flags.proxyServer,
		AuthServer:      flags.authServer,
		JoinMethod:      types.JoinMethod(flags.joinMethod),
		TokenName:       flags.tokenName,
		TokenSecretPath: flags.tokenSecretFile,
		DataDir:         flags.normalizedDataDir,
		DisableServices: flags.parsedDisable,
		ExtraSSHLabels:  flags.parsedLabels,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	outputRaw, err := result.Document.Render()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := validateMigratedConfig(outputRaw); err != nil {
		return trace.Wrap(err)
	}

	for _, change := range result.LogPathsChanged {
		fmt.Fprintf(flags.stderr, "NOTICE: rewrote %s from %q to %q to avoid log-file collisions.\n", change.Path, change.Old, change.New)
	}
	if result.PIDFileChanged != nil {
		change := result.PIDFileChanged
		fmt.Fprintf(flags.stderr, "NOTICE: rewrote %s from %q to %q to avoid PID-file collisions.\n", change.Path, change.Old, change.New)
	}
	for _, service := range result.DisableServicesNotFound {
		fmt.Fprintf(flags.stderr, "NOTICE: --disable-services=%s was requested, but no matching service section exists.\n", service)
	}
	for _, section := range result.ServicesDisabled {
		fmt.Fprintf(flags.stderr, "NOTICE: disabled %s in the migrated config; the original agent continues serving it.\n", section)
	}
	for _, warning := range result.ListenerWarnings {
		fmt.Fprintf(flags.stderr, "WARNING: %s\n", warning)
	}
	for _, notice := range result.Notices {
		fmt.Fprintf(flags.stderr, "NOTICE: %s\n", notice)
	}
	if types.JoinMethod(flags.joinMethod) == types.JoinMethodBoundKeypair {
		fmt.Fprintln(flags.stderr, "NOTICE: bound_keypair joins require the registration secret step outside this command.")
	}

	outputPath, outputIsStdout, err := migrateOutputPath(flags.normalizedOutput)
	if err != nil {
		return trace.Wrap(err)
	}
	if flags.diff {
		if outputPath != "" && !flags.force {
			refuse, err := wouldRefuseOverwrite(outputPath)
			if err != nil {
				return trace.Wrap(err)
			}
			if refuse {
				fmt.Fprintf(flags.stderr, "WARNING: output file %q exists and is non-empty; writing would require --force.\n", outputPath)
			}
		}
		outputName := flags.normalizedOutput
		if outputPath != "" {
			outputName = outputPath
		}
		diff, err := transform.DiffDocuments(doc, result.Document, flags.input, outputName)
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = fmt.Fprint(flags.stdout, diff)
		return trace.Wrap(err)
	}
	if flags.test {
		fmt.Fprintf(flags.stderr, "OK %s (migrated output validated)\n", flags.input)
		return nil
	}
	if outputIsStdout {
		fmt.Fprintln(flags.stderr, "NOTICE: stdout output is redacted; use --output=file:// to write a usable config")
		redacted, err := result.Document.Redact(transform.DefaultRedactionRules()).Render()
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = flags.stdout.Write(redacted)
		return trace.Wrap(err)
	}
	if err := writeMigratedConfig(outputPath, outputRaw, flags.force); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(flags.stdout, "Wrote migrated Teleport configuration to %q.\n", outputPath)
	return nil
}

func validateMigratedConfig(raw []byte) error {
	// TODO(scopes): add scope-aware semantic validation when configure --test
	// grows scoped validation.
	_, err := config.ReadConfig(bytes.NewReader(raw))
	return trace.Wrap(err)
}

func parseDisableServices(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []string
	seen := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		service := strings.TrimSpace(part)
		if service == "" {
			continue
		}
		if _, ok := seen[service]; ok {
			continue
		}
		seen[service] = struct{}{}
		out = append(out, service)
	}
	return out
}

func parseMigrateLabels(raw []string) (map[string]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	labels := make(map[string]string)
	for _, label := range raw {
		key, value, ok := strings.Cut(label, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, trace.BadParameter("labels must be in key=value form")
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if existing, ok := labels[key]; ok && existing != value {
			return nil, trace.BadParameter("label %q was specified with multiple values", key)
		}
		labels[key] = value
	}
	return labels, nil
}

// Mirrors onConfigDump's --output parsing, including its url.Parse
// limitations (paths containing '?' or '#' are mangled); keep the two in sync.
func migrateOutputPath(output string) (path string, stdout bool, err error) {
	uri, err := url.Parse(output)
	if err != nil {
		return "", false, trace.BadParameter("could not parse output value %q", output)
	}
	switch uri.Scheme {
	case teleport.SchemeStdout:
		return "", true, nil
	case teleport.SchemeFile, "":
		if uri.Path == "" {
			return "", false, trace.BadParameter("missing path in --output=%q", output)
		}
		if !filepath.IsAbs(uri.Path) {
			return "", false, trace.BadParameter("please use absolute path for file %v", uri.Path)
		}
		return uri.Path, false, nil
	default:
		return "", false, trace.BadParameter("unsupported --output=%v, use file:// or stdout://", uri.Scheme)
	}
}

func writeMigratedConfig(path string, raw []byte, force bool) error {
	if !force {
		refuse, err := wouldRefuseOverwrite(path)
		if err != nil {
			return trace.Wrap(err)
		}
		if refuse {
			return trace.AlreadyExists("will not overwrite existing non-empty file %v; pass --force to overwrite", path)
		}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return trace.Wrap(trace.ConvertSystemError(err), "error creating config file directory %s", dir)
	}
	tempFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return trace.Wrap(trace.ConvertSystemError(err), "error creating temporary config file")
	}
	tempName := tempFile.Name()
	defer os.Remove(tempName)
	if err := tempFile.Chmod(0o600); err != nil {
		tempFile.Close()
		return trace.Wrap(trace.ConvertSystemError(err), "error setting temporary config file permissions")
	}
	if _, err := tempFile.Write(raw); err != nil {
		tempFile.Close()
		return trace.Wrap(trace.ConvertSystemError(err), "error writing temporary config file")
	}
	if err := tempFile.Close(); err != nil {
		return trace.Wrap(err, "error closing temporary config file")
	}
	if err := os.Rename(tempName, path); err != nil {
		return trace.Wrap(trace.ConvertSystemError(err), "error moving temporary config file into place")
	}
	return nil
}

func wouldRefuseOverwrite(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err == nil {
		return strings.TrimSpace(string(raw)) != "", nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, trace.Wrap(trace.ConvertSystemError(err), "failed reading existing output file %q", path)
}

// Keep in sync with lib/autoupdate/agent.Namespace path derivation
// (/etc/teleport_<suffix>.yaml, /var/lib/teleport_<suffix>).
func defaultMigrateConfigPath(suffix string) string {
	return filepath.Join(filepath.Dir(defaults.ConfigFilePath), "teleport_"+suffix+".yaml")
}

func defaultMigrateDataDir(suffix string) string {
	return filepath.Join(filepath.Dir(defaults.DataDir), "teleport_"+suffix)
}

// validateInstallSuffix wraps agent.ValidateNamespaceName and additionally
// rejects a leading '-': it is accepted by the updater's regex but is a
// CLI-parsing hazard for the generated runbook commands.
func validateInstallSuffix(suffix string) error {
	if strings.HasPrefix(suffix, "-") {
		return trace.BadParameter("invalid namespace name %s, must be alphanumeric", suffix)
	}
	return trace.Wrap(agent.ValidateNamespaceName(suffix))
}
