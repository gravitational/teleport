package vnet

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log/slog"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"gvisor.dev/gvisor/pkg/tcpip"
)

func buildTcpAppHandlers(ctx context.Context, tc *client.TeleportClient) (map[tcpip.Address]tcpHandler, error) {
	// TODO: dynamically update apps list
	// TODO: get IPs from labels
	apps, err := tc.ListApps(ctx, nil /*filters*/)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	handlers := make(map[tcpip.Address]tcpHandler)

	fmt.Println("\nSetting up VNet IPs for all apps:")
	fmt.Println("IP          	App")
	var nextIp uint32 = 100<<24 + 64<<16 + 0<<8 + 2
	for _, app := range apps {
		addr := tcpip.AddrFrom4([4]byte{byte(nextIp >> 24), byte(nextIp >> 16), byte(nextIp >> 8), byte(nextIp)})
		appName := app.GetName()
		appPublicAddr := app.GetPublicAddr()
		fmt.Printf("%s	%s\n", addr, appName)
		handlers[addr] = proxyToApp(tc, appName, appPublicAddr)
		nextIp += 1
	}
	return handlers, nil
}

func proxyToApp(tc *client.TeleportClient, appName, appPublicAddr string) tcpHandler {
	return func(ctx context.Context, conn io.ReadWriteCloser) error {
		cert, err := appCert(ctx, tc, appName, appPublicAddr)
		if err != nil {
			return trace.Wrap(err, "getting cert for app %s", appName)
		}
		appConn, err := dialApp(ctx, tc, cert)
		if err != nil {
			return trace.Wrap(err, "dialing app %s", appName)
		}
		return trace.Wrap(utils.ProxyConn(ctx, conn, appConn))
	}
}

func dialApp(ctx context.Context, tc *client.TeleportClient, cert *tls.Certificate) (*tls.Conn, error) {
	alpnDialerConfig := apiclient.ALPNDialerConfig{
		ALPNConnUpgradeRequired: tc.TLSRoutingConnUpgradeRequired,
		TLSConfig: &tls.Config{
			NextProtos:   common.ProtocolsToString([]common.Protocol{common.ProtocolTCP}),
			Certificates: []tls.Certificate{*cert},
		},
		GetClusterCAs: func(context.Context) (*x509.CertPool, error) { return tc.LocalAgent().ClientCertPool(tc.SiteName) },
	}
	tlsConn, err := apiclient.DialALPN(ctx, tc.WebProxyAddr, alpnDialerConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tlsConn, nil
}

func appCert(ctx context.Context, tc *client.TeleportClient, appName, appPublicAddr string) (*tls.Certificate, error) {
	slog.Debug("Getting cert for app", slog.String("app", appName))
	key, err := tc.LocalAgent().GetKey(tc.SiteName, client.WithAppCerts{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO: check cert expiry
	cert, ok := key.AppTLSCerts[appName]
	if !ok {
		if err := appLogin(ctx, tc, appName, appPublicAddr); err != nil {
			return nil, trace.Wrap(err)
		}
		key, err := tc.LocalAgent().GetKey(tc.SiteName, client.WithAppCerts{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cert, ok = key.AppTLSCerts[appName]
		if !ok {
			return nil, trace.Errorf("unable to log in to app %q", appName)
		}
	}
	tlsCert, err := key.TLSCertificate(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &tlsCert, nil
}

func appLogin(ctx context.Context, tc *client.TeleportClient, appName, appPublicAddr string) error {
	slog.Debug("Logging in to app", slog.String("app", appName))
	currentProfile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	request := types.CreateAppSessionRequest{
		Username:    tc.Username,
		PublicAddr:  appPublicAddr,
		ClusterName: tc.SiteName,
	}
	webSession, err := tc.CreateAppSession(ctx, request)
	if err != nil {
		return trace.Wrap(err)
	}

	certReissueParams := client.ReissueParams{
		RouteToCluster: currentProfile.Cluster,
		RouteToApp: proto.RouteToApp{
			Name:        appName,
			SessionID:   webSession.GetName(),
			PublicAddr:  appPublicAddr,
			ClusterName: tc.SiteName,
		},
		AccessRequests: currentProfile.ActiveRequests.AccessRequests,
	}
	err = tc.ReissueUserCerts(ctx, client.CertCacheKeep, certReissueParams)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
