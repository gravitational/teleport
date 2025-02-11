package scripts

import (
	"context"
	_ "embed"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/web/scripts/oneoff"
)

type AutoupdateStyle int

const (
	NoAutoupdate AutoupdateStyle = iota
	PackageManagerAutoupdate
	UpdaterBinaryAutoupdate
)

type InstallScriptOptions struct {
	AutoupdateStyle AutoupdateStyle
	// TeleportVersion that should be installed. Without the leading "v".
	TeleportVersion string
	CDNBaseURL      string
	ProxyAddr       string
	TeleportFlavor  string
	FIPS            bool
}

func (o *InstallScriptOptions) Check() error {
	if o.ProxyAddr == "" {
		return trace.BadParameter("Proxy address is required")
	}

	if o.TeleportVersion == "" {
		return trace.BadParameter("Teleport version is required")
	}
	return nil
}

func (o *InstallScriptOptions) OneOffParams() (params oneoff.OneOffScriptParams) {
	// We add the leading v if it's not here
	version := o.TeleportVersion
	if o.TeleportVersion[0] != 'v' {
		version = "v" + o.TeleportVersion
	}

	args := []string{"enable", "--proxy", o.ProxyAddr}
	if o.CDNBaseURL != "" {
		args = append(args, "--base-url", o.CDNBaseURL)
	}

	return oneoff.OneOffScriptParams{
		TeleportBin:     "teleport-update",
		TeleportArgs:    strings.Join(args, " "),
		CDNBaseURL:      o.CDNBaseURL,
		TeleportVersion: version,
		TeleportFlavor:  o.TeleportFlavor,
		SuccessMessage:  "Teleport successfully installed.",
		TeleportFIPS:    o.FIPS,
	}
}

func GetInstallScript(ctx context.Context, opts InstallScriptOptions) (string, error) {
	switch opts.AutoupdateStyle {
	case NoAutoupdate, PackageManagerAutoupdate:
		return getLegacyInstallScript(ctx, opts)
	case UpdaterBinaryAutoupdate:
		return getUpdaterInstallScript(ctx, opts)
	default:
		return "", trace.BadParameter("unsupported autoupdate style: %v", opts.AutoupdateStyle)
	}
}

//go:embed install/install.sh
var legacyInstallScript string

func getLegacyInstallScript(ctx context.Context, opts InstallScriptOptions) (string, error) {
	return legacyInstallScript, nil
}

func getUpdaterInstallScript(ctx context.Context, opts InstallScriptOptions) (string, error) {
	if err := opts.Check(); err != nil {
		return "", trace.Wrap(err, "invalid install script parameters")
	}

	scriptParams := opts.OneOffParams()

	return oneoff.BuildScript(scriptParams)
}
