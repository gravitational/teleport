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
	None AutoupdateStyle = iota
	PackageManager
	UpdaterBinary
)

type InstallScriptOptions struct {
	AutoupdateStyle AutoupdateStyle
	TeleportVersion string
	CDNBaseURL      string
	ProdyAddr       string
	TeleportFlavor  string
	FIPS            bool
}

func GetInstallScript(ctx context.Context, opts InstallScriptOptions) (string, error) {
	switch opts.AutoupdateStyle {
	case None, PackageManager:
		return GetLegacyInstallScript(ctx, opts)
	case UpdaterBinary:
		return GetUpdaterInstallScript(ctx, opts)
	default:
		return "", trace.BadParameter("unsupported autoupdate style: %v", opts.AutoupdateStyle)
	}
}

//go:embed install/install.sh
var legacyInstallScript string

func GetLegacyInstallScript(ctx context.Context, opts InstallScriptOptions) (string, error) {
	return legacyInstallScript, nil
}

func GetUpdaterInstallScript(ctx context.Context, opts InstallScriptOptions) (string, error) {
	if opts.ProdyAddr == "" {
		return "", trace.BadParameter("Proxy address is required")
	}

	if opts.TeleportVersion == "" {
		return "", trace.BadParameter("Teleport version is required")
	}

	// We add the leading v if it's not here
	version := opts.TeleportVersion
	if opts.TeleportVersion[0] != 'v' {
		version = "v" + opts.TeleportVersion
	}

	args := []string{"enable", "--proxy", opts.ProdyAddr}
	if opts.CDNBaseURL != "" {
		args = append(args, "--base-url", opts.CDNBaseURL)
	}

	scriptParams := oneoff.OneOffScriptParams{
		TeleportBin:     "teleport-update",
		TeleportArgs:    strings.Join(args, " "),
		CDNBaseURL:      opts.CDNBaseURL,
		TeleportVersion: version,
		TeleportFlavor:  opts.TeleportFlavor,
		SuccessMessage:  "Teleport successfully installed.",
		TeleportFIPS:    opts.FIPS,
	}
	return oneoff.BuildScript(scriptParams)
}
