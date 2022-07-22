/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package etcd

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"time"
)

const (
	metricsURL = "https://127.0.0.1:3379"
)

type Instance struct {
	process *exec.Cmd
	dataDir string
	kill    context.CancelFunc
	stdout  bytes.Buffer
	stderr  bytes.Buffer
}

func (etcd *Instance) Stop() {
	log.Printf("Killing etcd process")
	etcd.kill()

	log.Printf("Waiting for etcd process to exit...")
	switch err := etcd.process.Wait().(type) {
	case nil:
		break

	case *exec.ExitError:
		// we expect a return code of -1 because we've just killed the process
		// above. If we get something else then etcd failed earlier for some
		// reason and we should print diagnostic output
		if err.ExitCode() != -1 {
			log.Printf("Etcd exited with unexpected status %d", err.ExitCode())
			log.Printf("stderr:\n%s", etcd.stderr.String())
		}

	default:
		log.Printf("Failed: %s", err)
	}

	log.Printf("Removing data dir")
	if err := os.RemoveAll(etcd.dataDir); err != nil {
		log.Printf("Failed removing data dir: %s", err)
	}
}

// Start starts the etcd server using the keys extpected by the
// integration and unit tests
func Start(ctx context.Context, workspace string, env ...string) (*Instance, error) {
	etcdBinary, err := exec.LookPath("etcd")
	if err != nil {
		return nil, trace.Wrap(err, "can't find etcd binary")
	}

	dataDir, err := os.MkdirTemp(os.TempDir(), "teleport-etcd-*")
	if err != nil {
		return nil, trace.Wrap(err, "can't create temp dir")
	}

	certsDir := path.Join(workspace, "examples", "etcd", "certs")

	etcdInstance := &Instance{}
	var processCtx context.Context
	processCtx, etcdInstance.kill = context.WithCancel(context.Background())

	etcdInstance.process = exec.CommandContext(processCtx, etcdBinary,
		"--name", "teleportstorage",
		"--data-dir", dataDir,
		"--initial-cluster-state", "new",
		"--cert-file", path.Join(certsDir, "server-cert.pem"),
		"--key-file", path.Join(certsDir, "server-key.pem"),
		"--trusted-ca-file", path.Join(certsDir, "ca-cert.pem"),
		"--advertise-client-urls=https://127.0.0.1:2379",
		"--listen-client-urls=https://127.0.0.1:2379",
		"--listen-metrics-urls", metricsURL,
		"--client-cert-auth",
	)
	etcdInstance.process.Dir = workspace
	etcdInstance.process.Stdout = &etcdInstance.stdout
	etcdInstance.process.Stderr = &etcdInstance.stderr

	if len(env) > 0 {
		etcdInstance.process.Env = append(os.Environ(), env...)
	}

	log.Printf("Launching etcd (%s)", etcdInstance.process.Path)
	if err = etcdInstance.process.Start(); err != nil {
		return nil, trace.Wrap(err, "failed starting etcd")
	}

	timeoutCtx, cancelTimeout := context.WithTimeout(ctx, 5*time.Second)
	defer cancelTimeout()

	if err := waitForEtcdToStart(timeoutCtx, certsDir); err != nil {
		log.Printf("Etcd failed to come up. Tidying up.")
		etcdInstance.Stop()
		return nil, trace.Wrap(err, "failed while waiting for etcd to start")
	}

	return etcdInstance, nil
}

// waitForEtcdToStart polls an etcd server (as started with Start()) until either
// a) the etcd service reports itself as healthy, or
// b) the supplied context expires
func waitForEtcdToStart(ctx context.Context, certDir string) error {
	log.Printf("Waiting for etcd to come up...")

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	client, err := newHTTPSTransport(certDir)
	if err != nil {
		return trace.Wrap(err, "failed to create https client")
	}

	for {
		select {
		case <-ticker.C:
			healthy, err := pollForHealth(ctx, client, metricsURL+"/health")
			if err != nil {
				return trace.Wrap(err)
			}
			if healthy {
				log.Printf("Etcd reporting healthy")
				return nil
			}

		case <-ctx.Done():
			return trace.Errorf("timed out waiting for etcd to start")
		}
	}
}

// newHTTPSTransport creates an HTTP client configured to use TLS with an etcd server
// as started with Start()
func newHTTPSTransport(certDir string) (*http.Client, error) {
	caCertPath := path.Join(certDir, "ca-cert.pem")
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed reading CA cert from %s", caCertPath)
	}

	clientCert, err := tls.LoadX509KeyPair(
		path.Join(certDir, "client-cert.pem"), path.Join(certDir, "client-key.pem"))
	if err != nil {
		return nil, trace.Wrap(err, "failed reading client cert")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig.RootCAs = x509.NewCertPool()
	if !transport.TLSClientConfig.RootCAs.AppendCertsFromPEM(caCert) {
		return nil, trace.Errorf("Failed adding CA cert")
	}
	transport.TLSClientConfig.Certificates = []tls.Certificate{clientCert}

	return &http.Client{Transport: transport}, nil
}

// pollForHealth polls the etcd metrecs endpoint for status information, returning
// true if the service reports itself as healthy
func pollForHealth(ctx context.Context, client *http.Client, url string) (bool, error) {
	type health struct {
		Health string `json:"health"`
		Reason string `json:"reason"`
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	request, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, url, nil)
	if err != nil {
		return false, trace.Wrap(err, "Failed constructing poll request")
	}

	// A request failure is considered "not healthy" rather than an error, as
	// the etcd server may just not be up yet.
	response, err := client.Do(request)
	if err != nil {
		return false, nil
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return false, trace.Wrap(err, "failed reading poll response body")
	}

	status := health{}
	if err = json.Unmarshal(body, &status); err != nil {
		return false, trace.Wrap(err, "failed parsing poll response")
	}

	return status.Health == "true", nil
}
