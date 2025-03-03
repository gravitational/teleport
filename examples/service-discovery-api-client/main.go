// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/strslice"
	docker "github.com/docker/docker/client"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	teleport "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

const (
	// Assign proxyAddr to the host and port of your Teleport Proxy Service instance
	proxyAddr      string = ""
	teleportImage  string = "public.ecr.aws/gravitational/teleport:11.0.3"
	initTimeout           = time.Duration(30) * time.Second
	updateInterval        = time.Duration(5) * time.Second
	tokenTTL              = time.Duration(5) * time.Minute
	networkName    string = "bridge"
	managementPort string = "15672"
	tokenLenBytes         = 16
	rabbitMQImage  string = "rabbitmq:3-management"
)

type tokenDemoApp struct {
	dockerClient   *docker.Client
	teleportClient *teleport.Client
}

func (t *tokenDemoApp) listRegisteredAppURLs(ctx context.Context) (map[string]types.AppServer, error) {
	m := make(map[string]types.AppServer)

	for {
		req := proto.ListResourcesRequest{
			ResourceType: "app_server",
			Limit:        10,
		}
		resp, err := t.teleportClient.ListResources(
			ctx,
			req,
		)

		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, r := range resp.Resources {
			if p, ok := r.(types.AppServer); ok {
				m[p.GetApp().GetURI()] = p
			}
		}

		// No more pages to request
		if resp.NextKey == "" {
			break
		}

		req.StartKey = resp.NextKey
	}

	return m, nil
}

func (t *tokenDemoApp) listAppContainerURLs(ctx context.Context, image string) (map[string]struct{}, error) {
	c, err := t.dockerClient.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "ancestor",
			Value: image,
		}),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	l := make(map[string]struct{})

	for _, r := range c {
		b, ok := r.NetworkSettings.Networks[networkName]
		// Not connected to the chosen network, so skip it
		if !ok {
			continue
		}

		u, err := url.Parse("http://" + net.JoinHostPort(
			b.IPAddress,
			managementPort,
		))

		if err != nil {
			return nil, trace.Wrap(err)
		}

		l[u.String()] = struct{}{}
	}

	return l, nil
}

func cryptoRandomHex(len int) (string, error) {
	randomBytes := make([]byte, len)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", trace.Wrap(err)
	}
	return hex.EncodeToString(randomBytes), nil
}

func (t *tokenDemoApp) createAppToken(ctx context.Context) (string, error) {
	n, err := cryptoRandomHex(tokenLenBytes)
	if err != nil {
		return "", trace.Wrap(err)
	}

	tok, err := types.NewProvisionTokenFromSpec(
		n,
		time.Now().Add(tokenTTL),
		types.ProvisionTokenSpecV2{
			Roles: types.SystemRoles{types.RoleApp},
		})

	if err := t.teleportClient.CreateToken(ctx, tok); err != nil {
		return "", trace.Wrap(err)
	}
	return n, nil
}

func (t *tokenDemoApp) startApplicationServiceContainer(
	ctx context.Context,
	token string,
	u url.URL,
) error {

	name := strings.ReplaceAll(u.Hostname(), ".", "-")
	resp, err := t.dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: teleportImage,
			Entrypoint: strslice.StrSlice{
				"/usr/bin/dumb-init",
				"teleport",
				"start",
				"--roles=app",
				"--auth-server=" + proxyAddr,
				"--token=" + token,
				"--app-name=rabbitmq-" + name,
				"--app-uri=" + u.String(),
			},
		},
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return trace.Wrap(err)
	}

	err = t.dockerClient.ContainerStart(
		ctx,
		resp.ID,
		container.StartOptions{},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (t *tokenDemoApp) pruneAppServiceInstance(ctx context.Context, p types.AppServer) error {
	host := p.GetHostname()

	if err := t.teleportClient.DeleteApplicationServer(
		ctx,
		p.GetNamespace(),
		p.GetHostID(),
		p.GetName(),
	); err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("Deleted unnecessary Application Service record:", p.GetName())

	// Don't check errors when removing the container, since it may already
	// have been removed.
	t.dockerClient.ContainerStop(ctx, host, container.StopOptions{})
	t.dockerClient.ContainerRemove(ctx, host, container.RemoveOptions{})

	fmt.Println("Deleted unnecessary Application Service container:", host)
	return nil
}

func (t *tokenDemoApp) reconcileApps() error {
	ctx := context.Background()
	apps, err := t.listRegisteredAppURLs(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	urls, err := t.listAppContainerURLs(ctx, rabbitMQImage)
	if err != nil {
		return trace.Wrap(err)
	}

	for u, _ := range urls {
		if _, ok := apps[u]; ok {
			continue
		}
		tok, err := t.createAppToken(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println("Created a new application token for URL: " + u)

		r, err := url.Parse(u)
		if err != nil {
			return trace.Wrap(err)
		}

		err = t.startApplicationServiceContainer(ctx, tok, *r)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Println("Started an Application Service container to proxy URL: " + u)
	}

	for a, p := range apps {
		_, ok := urls[a]
		if ok {
			continue
		}

		if err := t.pruneAppServiceInstance(ctx, p); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func newTokenDemoApp() *tokenDemoApp {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()
	creds := teleport.LoadIdentityFile("auth.pem")

	t, err := teleport.New(ctx, teleport.Config{
		Addrs:       []string{proxyAddr},
		Credentials: []teleport.Credentials{creds},
		DialOpts: []grpc.DialOption{
			grpc.WithReturnConnectionError(),
		},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("Connected to Teleport")

	d, err := docker.NewClientWithOpts(
		docker.WithAPIVersionNegotiation(),
	)
	if err != nil {
		panic(err)
	}
	fmt.Println("Connected to the Docker daemon")

	return &tokenDemoApp{
		teleportClient: t,
		dockerClient:   d,
	}

}

func main() {
	fmt.Println("Starting the application")
	app := newTokenDemoApp()

	k := time.NewTicker(updateInterval)
	defer k.Stop()
	for {
		<-k.C
		if err := app.reconcileApps(); err != nil {
			panic(err)
		}
	}
}
