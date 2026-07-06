// Command tp-enroll calls the Teleport OktaService.CreateIntegration RPC (the
// OAuth-for-Okta enrollment path that has no tctl/YAML equivalent). It reuses
// the caller's tsh profile for auth.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/client"
	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
)

func main() {
	var (
		proxy       = flag.String("proxy", "", "Teleport proxy address, e.g. dev.teleport.sh:443")
		org         = flag.String("org", "", "Okta organization URL")
		oauthID     = flag.String("oauth-id", "", "Okta OAuth service-app client ID")
		metadataURL = flag.String("metadata-url", "", "Okta SAML app metadata URL")
		owners      = flag.String("owner", "", "comma-separated Access List default owners")
		groupFilter = flag.String("group-filter", "", "comma-separated Okta group filters")
		appFilter   = flag.String("app-filter", "", "comma-separated Okta app filters")
		validate    = flag.Bool("validate-only", false, "only run ValidateClientCredentials")
	)
	flag.Parse()
	if *proxy == "" || *org == "" || *oauthID == "" {
		fmt.Fprintln(os.Stderr, "-proxy, -org and -oauth-id are required")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	clt, err := client.New(ctx, client.Config{
		Addrs:       []string{*proxy},
		Credentials: []client.Credentials{client.LoadProfile("", "")},
	})
	if err != nil {
		fatal("connect to Teleport", err)
	}
	defer clt.Close()

	okc := oktapb.NewOktaServiceClient(clt.GetConnection())
	creds := func() *oktapb.OktaAPICredentials { // fresh per RPC — don't share oneof messages
		return &oktapb.OktaAPICredentials{Auth: &oktapb.OktaAPICredentials_OauthId{OauthId: *oauthID}}
	}

	if _, err := okc.ValidateClientCredentials(ctx, &oktapb.ValidateClientCredentialsRequest{
		OktaOrganizationUrl: *org,
		ApiCredentials:      creds(),
	}); err != nil {
		fatal("ValidateClientCredentials", err)
	}
	fmt.Fprintln(os.Stderr, "credentials validated")
	if *validate {
		return
	}

	if *metadataURL == "" || *owners == "" {
		fmt.Fprintln(os.Stderr, "-metadata-url and -owner are required to enroll")
		os.Exit(2)
	}

	resp, err := okc.CreateIntegration(ctx, &oktapb.CreateIntegrationRequest{
		OktaOrganizationUrl:     *org,
		ApiCredentials:          creds(),
		SsoMetadataUrl:          *metadataURL,
		EnableUserSync:          true,
		EnableAppGroupSync:      true,
		EnableAccessListSync:    true,
		EnableBidirectionalSync: true,
		AccessListSettings: &oktapb.AccessListSettings{
			DefaultOwner: splitList(*owners),
			GroupFilters: splitList(*groupFilter),
			AppFilters:   splitList(*appFilter),
		},
	})
	if err != nil {
		fatal("CreateIntegration", err)
	}

	out, _ := json.MarshalIndent(map[string]any{
		"plugin":    resp.GetPlugin().GetMetadata().Name,
		"connector": resp.GetConnectorInfo().GetTeleportConnectorName(),
		"oktaAppId": resp.GetConnectorInfo().GetOktaAppId(),
	}, "", "  ")
	fmt.Println(string(out))
}

func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func fatal(what string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", what, err)
	os.Exit(1)
}
