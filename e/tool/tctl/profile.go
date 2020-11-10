package main

import (
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/tctl/common"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// loadConfigFromProfile applies config from ~/.tsh/ profile if it's present
//
// DELETE IN: 5.1.0.
//
// This function is enterprise only until 5.1 release
// that open sources RBAC. Otherwise OSS `tctl` admin users will have too much
// control over the teleport instance
func loadConfigFromProfile(ccf *common.GlobalCLIFlags, cfg *service.Config) (*common.AuthServiceClientConfig, error) {
	if ccf.IdentityFilePath != "" {
		return nil, trace.NotFound("identity has been supplied, skip loading the config")
	}

	proxyAddr := ""
	if len(ccf.AuthServerAddr) != 0 {
		proxyAddr = cfg.AuthServers[0].Addr
	}

	profile, _, err := client.Status("", proxyAddr)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	// client is already logged in using tsh login and profile is not expired
	if profile == nil {
		return nil, trace.NotFound("profile is not found")
	}
	if profile.IsExpired(clockwork.NewRealClock()) {
		return nil, trace.BadParameter("your credentials have expired, please login using `tsh login`")
	}

	log.Debugf("Found active profile: %v %v.", profile.ProxyURL, profile.Username)

	c := client.MakeDefaultConfig()
	if err := c.LoadProfile("", proxyAddr); err != nil {
		return nil, trace.Wrap(err)
	}
	keyStore, err := client.NewFSLocalKeyStore(c.KeysDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	webProxyHost, _ := c.WebProxyHostPort()
	key, err := keyStore.GetKey(webProxyHost, c.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authConfig := &common.AuthServiceClientConfig{}
	authConfig.TLS, err = key.TeleportClientTLSConfig(cfg.CipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authConfig.TLS.InsecureSkipVerify = ccf.Insecure
	authConfig.SSH, err = client.ProxyClientSSHConfig(key, keyStore)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Do not override auth servers from command line
	if len(ccf.AuthServerAddr) == 0 {
		webProxyAddr, err := utils.ParseAddr(c.WebProxyAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		log.Debugf("Setting auth server to web proxy %v.", webProxyAddr)
		cfg.AuthServers = []utils.NetAddr{*webProxyAddr}
	}

	return authConfig, nil
}
