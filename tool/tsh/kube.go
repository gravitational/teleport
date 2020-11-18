/*
Copyright 2020 Gravitational, Inc.

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

package main

import (
	"fmt"
	"time"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/trace"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/pkg/apis/clientauthentication"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
)

type kubeCommands struct {
	credentials *kubeCredentialsCommand
	ls          *kubeLSCommand
	login       *kubeLoginCommand
}

func newKubeCommand(app *kingpin.Application) kubeCommands {
	kube := app.Command("kube", "Manage available kubernetes clusters")
	cmds := kubeCommands{
		credentials: newKubeCredentialsCommand(kube),
		ls:          newKubeLSCommand(kube),
		login:       newKubeLoginCommand(kube),
	}
	return cmds
}

type kubeCredentialsCommand struct {
	*kingpin.CmdClause
	kubeCluster     string
	teleportCluster string
}

func newKubeCredentialsCommand(parent *kingpin.CmdClause) *kubeCredentialsCommand {
	c := &kubeCredentialsCommand{
		// This command is always hidden. It's called from the kubeconfig that
		// tsh generates and never by users directly.
		CmdClause: parent.Command("credentials", "Get credentials for kubectl access").Hidden(),
	}
	c.Flag("teleport-cluster", "Name of the teleport cluster to get credentials for.").Required().StringVar(&c.teleportCluster)
	c.Flag("kube-cluster", "Name of the kubernetes cluster to get credentials for.").Required().StringVar(&c.kubeCluster)
	return c
}

func (c *kubeCredentialsCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}

	// Try loading existing keys.
	k, err := tc.LocalAgent().GetKey(client.WithKubeCerts(c.teleportCluster))
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	// Loaded existing credentials and have a cert for this cluster? Return it
	// right away.
	if err == nil {
		crt, err := k.KubeTLSCertificate(c.kubeCluster)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if crt != nil && time.Until(crt.NotAfter) > time.Minute {
			log.Debugf("Re-using existing TLS cert for kubernetes cluster %q", c.kubeCluster)
			return c.writeResponse(k, c.kubeCluster)
		}
		// Otherwise, cert for this k8s cluster is missing or expired. Request
		// a new one.
	}

	log.Debugf("Requesting TLS cert for kubernetes cluster %q", c.kubeCluster)
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		return tc.ReissueUserCerts(cf.Context, client.ReissueParams{
			RouteToCluster:    c.teleportCluster,
			KubernetesCluster: c.kubeCluster,
		})
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// ReissueUserCerts should cache the new cert on disk, re-read them.
	k, err = tc.LocalAgent().GetKey(client.WithKubeCerts(c.teleportCluster))
	if err != nil {
		return trace.Wrap(err)
	}

	return c.writeResponse(k, c.kubeCluster)
}

func (c *kubeCredentialsCommand) writeResponse(key *client.Key, kubeClusterName string) error {
	crt, err := key.KubeTLSCertificate(kubeClusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	resp := &clientauthentication.ExecCredential{
		Status: &clientauthentication.ExecCredentialStatus{
			// Indicate  slightly earlier expiration to avoid the cert expiring
			// mid-request.
			ExpirationTimestamp:   &metav1.Time{Time: crt.NotAfter.Add(-1 * time.Minute)},
			ClientCertificateData: string(key.KubeTLSCerts[kubeClusterName]),
			ClientKeyData:         string(key.Priv),
		},
	}
	data, err := runtime.Encode(kubeCodecs.LegacyCodec(kubeGroupVersion), resp)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Println(string(data))
	return nil
}

type kubeLSCommand struct {
	*kingpin.CmdClause
}

func newKubeLSCommand(parent *kingpin.CmdClause) *kubeLSCommand {
	c := &kubeLSCommand{
		CmdClause: parent.Command("ls", "Get a list of kubernetes clusters"),
	}
	return c
}

func (c *kubeLSCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}

	var currentTeleportCluster string
	var kubeClusters []string
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		pc, err := tc.ConnectToProxy(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer pc.Close()
		ac, err := pc.ConnectToCurrentCluster(cf.Context, true)
		if err != nil {
			return trace.Wrap(err)
		}
		defer ac.Close()

		cn, err := ac.GetClusterName()
		if err != nil {
			return trace.Wrap(err)
		}
		currentTeleportCluster = cn.GetClusterName()

		kubeClusters, err = kubeutils.KubeClusterNames(cf.Context, ac)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var selectedCluster string
	if kc, err := kubeconfig.Load(""); err != nil {
		log.WithError(err).Warning("Failed parsing existing kubeconfig")
	} else {
		selectedCluster = kubeconfig.KubeClusterFromContext(kc.CurrentContext, currentTeleportCluster)
	}

	var t asciitable.Table
	if cf.Quiet {
		t = asciitable.MakeHeadlessTable(2)
	} else {
		t = asciitable.MakeTable([]string{"Kube Cluster Name", "Selected"})
	}
	for _, cluster := range kubeClusters {
		var selectedMark string
		if cluster == selectedCluster {
			selectedMark = "*"
		}
		t.AddRow([]string{cluster, selectedMark})
	}
	fmt.Println(t.AsBuffer().String())

	return nil
}

type kubeLoginCommand struct {
	*kingpin.CmdClause
	kubeCluster string
}

func newKubeLoginCommand(parent *kingpin.CmdClause) *kubeLoginCommand {
	c := &kubeLoginCommand{
		CmdClause: parent.Command("login", "Login to a kubernetes cluster"),
	}
	c.Arg("kube-cluster", "Name of the kubernetes cluster to login to. Check 'tsh kube ls' for a list of available clusters.").Required().StringVar(&c.kubeCluster)
	return c
}

func (c *kubeLoginCommand) run(cf *CLIConf) error {
	profile, _, err := client.Status("", cf.Proxy)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.AccessDenied("not logged in")
		}
		return trace.Wrap(err)
	}
	if profile == nil {
		return trace.AccessDenied("not logged in")
	}
	kc, err := kubeconfig.Load("")
	if err != nil {
		return trace.Wrap(err)
	}

	kubeContext := kubeconfig.ContextName(profile.Cluster, c.kubeCluster)
	if _, ok := kc.Contexts[kubeContext]; !ok {
		fmt.Println(`check 'tsh kube ls' for known clusters
if this is a new cluster, you may need to 'tsh login' again to access it`)
		return trace.BadParameter("kubernetes cluster %q not found", c.kubeCluster)
	}
	kc.CurrentContext = kubeContext
	if err := kubeconfig.Save("", *kc); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Logged into kubernetes cluster %q\n", c.kubeCluster)
	return nil
}

// Required magic boilerplate to use the k8s encoder.

var (
	kubeScheme       = runtime.NewScheme()
	kubeCodecs       = serializer.NewCodecFactory(kubeScheme)
	kubeGroupVersion = schema.GroupVersion{
		Group:   "client.authentication.k8s.io",
		Version: "v1beta1",
	}
)

func init() {
	metav1.AddToGroupVersion(kubeScheme, schema.GroupVersion{Version: "v1"})
	clientauthv1beta1.AddToScheme(kubeScheme)
	clientauthentication.AddToScheme(kubeScheme)
}
