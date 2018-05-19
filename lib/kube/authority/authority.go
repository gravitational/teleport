package authority

import (
	"context"
	"io/ioutil"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
	certificates "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	watch "k8s.io/apimachinery/pkg/watch"
)

// Cert is a certificate
type Cert struct {
	// Cert is a signed certificate PEM block
	Cert []byte
	// CA is a PEM block with trusted CA
	CA []byte
}

// ProcessCSR processes CSR request with local k8s certificate authority
// and returns certificate PEM signed by CA
func ProcessCSR(csrPEM []byte) (*Cert, error) {
	caPEM, err := ioutil.ReadFile(teleport.KubeCAPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, _, err := kubeutils.GetKubeClient("")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	id := "teleport-" + uuid.New()
	requests := clt.CertificatesV1beta1().CertificateSigningRequests()
	csr, err := requests.Create(&certificates.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: id,
		},
		Spec: certificates.CertificateSigningRequestSpec{
			Request: csrPEM,
			Usages: []certificates.KeyUsage{
				certificates.UsageDigitalSignature,
				certificates.UsageKeyEncipherment,
				certificates.UsageServerAuth,
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Delete CSR as it seems to be hanging forever if not deleted manually.
	defer func() {
		if err := requests.Delete(id, &metav1.DeleteOptions{}); err != nil {
			log.Warningf("Failed to delete CSR: %v.", err)
		}
	}()
	csr.Status.Conditions = append(csr.Status.Conditions, certificates.CertificateSigningRequestCondition{
		Type:           certificates.CertificateApproved,
		Reason:         "TeleportApprove",
		Message:        "This CSR was approved by Teleport.",
		LastUpdateTime: metav1.Now(),
	})
	result, err := requests.UpdateApproval(csr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if result.Status.Certificate != nil {
		log.Debugf("Received certificate right after approval, returning.")
		return &Cert{Cert: result.Status.Certificate, CA: caPEM}, nil
	}

	ctx, cancel := context.WithTimeout(context.TODO(), defaults.CSRSignTimeout)
	defer cancel()

	watchForCert := func() ([]byte, error) {
		watcher, err := requests.Watch(metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				Kind: teleport.KubeKindCSR,
			},
			FieldSelector: fields.Set{teleport.KubeMetadataNameSelector: id}.String(),
			Watch:         true,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cert, err := waitForCSR(ctx, watcher.ResultChan())
		watcher.Stop()
		if err == nil {
			return cert, nil
		}
		return nil, trace.Wrap(err)
	}
	// this could be infinite loop, but is limited to certain number
	// of iterations just in case.
	for i := 0; i < int(defaults.CSRSignTimeout/time.Second); i++ {
		cert, err := watchForCert()
		if err == nil {
			return &Cert{Cert: cert, CA: caPEM}, nil
		}
		if !trace.IsRetryError(err) {
			return nil, trace.Wrap(err)
		}
		select {
		case <-time.After(time.Second):
			log.Debugf("Retry after network error: %v.", err)
		case <-ctx.Done():
			return nil, trace.BadParameter(timeoutCSRMessage)
		}
	}
	return nil, trace.BadParameter(timeoutCSRMessage)
}

const timeoutCSRMessage = "timeout while waiting for Kubernetes certificate"

func waitForCSR(ctx context.Context, eventsC <-chan watch.Event) ([]byte, error) {
	for {
		select {
		case event, ok := <-eventsC:
			if !ok {
				return nil, trace.Retry(nil, "events channel closed")
			}
			csr, ok := event.Object.(*certificates.CertificateSigningRequest)
			if !ok {
				log.Warnf("Unexpected resource type: %T, expected %T.", event.Object, &certificates.CertificateSigningRequest{})
				continue
			}
			if csr.Status.Certificate != nil {
				return csr.Status.Certificate, nil
			}
			log.Debugf("CSR got updated, but certificate is not ready yet: %v.", csr.Status.Conditions)
		case <-ctx.Done():
			return nil, trace.BadParameter(timeoutCSRMessage)
		}
	}
}
