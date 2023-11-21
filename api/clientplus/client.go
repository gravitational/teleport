package clientplus

import (
	"context"
	"crypto/tls"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"net"
)

type Config struct {
	Credential CredentialProvider
	Connector  Connector
}

type Connector interface {
	Dial(context.Context, string) (net.Conn, error)
}

type Client struct {
	config Config
	conn   *grpc.ClientConn
}

type CredentialProvider interface {
	GetTLSCredential(ctx context.Context) (*tls.Certificate, error)
	GetSSHCredential(ctx context.Context) (*ssh.Certificate, error)
}

func tryConnectors(ctx context.Context) (Connector, error) {

}

func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Connector != nil {
		return startClient(cfg)
	}

	return tryConnectors(ctx)
}

func startClient(cfg Config) *Client {
	return &Client{}
}
