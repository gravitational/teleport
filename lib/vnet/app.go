package vnet

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

func proxyToApp(tc *client.TeleportClient, appName, appPublicAddr string) tcpHandler {
	return func(ctx context.Context, connector tcpConnector) error {
		cert, err := appCert(ctx, tc, appName, appPublicAddr)
		if err != nil {
			return trace.Wrap(err, "getting cert for app %s", appName)
		}
		appConn, err := dialApp(ctx, tc, cert)
		if err != nil {
			return trace.Wrap(err, "dialing app %s", appName)
		}
		conn, err := connector()
		if err != nil {
			return trace.Wrap(err)
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
