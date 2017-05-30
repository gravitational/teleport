package saml2

import (
	"bytes"
	"compress/flate"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"

	"encoding/xml"

	"github.com/beevik/etree"
	"github.com/russellhaering/gosaml2/types"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/russellhaering/goxmldsig/etreeutils"
)

func (sp *SAMLServiceProvider) validationContext() *dsig.ValidationContext {
	ctx := dsig.NewDefaultValidationContext(sp.IDPCertificateStore)
	ctx.Clock = sp.Clock
	return ctx
}

// validateResponseAttributes validates a SAML Response's tag and attributes. It does
// not inspect child elements of the Response at all.
func (sp *SAMLServiceProvider) validateResponseAttributes(response *types.Response) error {
	if response.Destination != sp.AssertionConsumerServiceURL {
		return ErrInvalidValue{
			Key:      DestinationAttr,
			Expected: sp.AssertionConsumerServiceURL,
			Actual:   response.Destination,
		}
	}

	if response.Version != "2.0" {
		return ErrInvalidValue{
			Reason:   ReasonUnsupported,
			Key:      "SAML version",
			Expected: "2.0",
			Actual:   response.Version,
		}
	}

	return nil
}

func (sp *SAMLServiceProvider) unmarshalResponse(el *etree.Element) (*types.Response, error) {
	response := &types.Response{}

	doc := etree.NewDocument()
	doc.SetRoot(el)
	data, err := doc.WriteToBytes()
	if err != nil {
		return nil, err
	}

	err = xml.Unmarshal(data, response)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (sp *SAMLServiceProvider) getDecryptCert() (*tls.Certificate, error) {
	if sp.SPKeyStore == nil {
		return nil, fmt.Errorf("no decryption certs available")
	}

	//This is the tls.Certificate we'll use to decrypt any encrypted assertions
	var decryptCert tls.Certificate

	switch crt := sp.SPKeyStore.(type) {
	case dsig.TLSCertKeyStore:
		// Get the tls.Certificate directly if possible
		decryptCert = tls.Certificate(crt)

	default:

		//Otherwise, construct one from the results of GetKeyPair
		pk, cert, err := sp.SPKeyStore.GetKeyPair()
		if err != nil {
			return nil, fmt.Errorf("error getting keypair: %v", err)
		}

		decryptCert = tls.Certificate{
			Certificate: [][]byte{cert},
			PrivateKey:  pk,
		}
	}

	return &decryptCert, nil
}

func (sp *SAMLServiceProvider) decryptAssertions(response *types.Response) error {
	for _, ea := range response.EncryptedAssertions {
		decryptCert, err := sp.getDecryptCert()
		if err != nil {
			return err
		}

		assertion, err := ea.Decrypt(decryptCert)
		if err != nil {
			return err
		}

		response.Assertions = append(response.Assertions, *assertion)
	}

	return nil
}

func (sp *SAMLServiceProvider) validateElementSignature(el *etree.Element) (*etree.Element, error) {
	return sp.validationContext().Validate(el)
}

func (sp *SAMLServiceProvider) validateAssertionSignatures(el *etree.Element) error {
	validateAssertion := func(ctx etreeutils.NSContext, unverifiedAssertion *etree.Element) error {
		if unverifiedAssertion.Parent() != el {
			return fmt.Errorf("found assertion with unexpected parent element: %s", unverifiedAssertion.Parent().Tag)
		}

		detatched, err := etreeutils.NSDetatch(ctx, unverifiedAssertion)
		if err != nil {
			return err
		}

		assertion, err := sp.validationContext().Validate(detatched)
		if err != nil {
			return err
		}

		// Replace the original unverified Assertion with the verified one. Note that
		// at this point only the Assertion (and not the parent Response) can be trusted
		// as having been signed by the IdP.
		if el.RemoveChild(unverifiedAssertion) == nil {
			// Out of an abundance of caution, check to make sure an Assertion was actually
			// removed. If it wasn't a programming error has occurred.
			panic("unable to remove assertion")
		}

		el.AddChild(assertion)

		return nil
	}

	return etreeutils.NSFindIterate(el, SAMLAssertionNamespace, AssertionTag, validateAssertion)
}

//ValidateEncodedResponse both decodes and validates, based on SP
//configuration, an encoded, signed response. It will also appropriately
//decrypt a response if the assertion was encrypted
func (sp *SAMLServiceProvider) ValidateEncodedResponse(encodedResponse string) (*types.Response, error) {
	raw, err := base64.StdEncoding.DecodeString(encodedResponse)
	if err != nil {
		return nil, err
	}

	doc := etree.NewDocument()
	err = doc.ReadFromBytes(raw)
	if err != nil {
		// Attempt to inflate the response in case it happens to be compressed (as with one case at saml.oktadev.com)
		buf, err := ioutil.ReadAll(flate.NewReader(bytes.NewReader(raw)))
		if err != nil {
			return nil, err
		}

		doc = etree.NewDocument()
		err = doc.ReadFromBytes(buf)
		if err != nil {
			return nil, err
		}
	}

	if doc.Root() == nil {
		return nil, fmt.Errorf("unable to parse response")
	}

	el := doc.Root()

	if !sp.SkipSignatureValidation {
		el, err = sp.validateElementSignature(el)
		if err == dsig.ErrMissingSignature {
			// The Response wasn't signed. It is possible that the Assertion inside of
			// the Response was signed.

			// Unfortunately we just blew away our Response
			el = doc.Root()

			err = sp.validateAssertionSignatures(el)
			if err != nil {
				return nil, err
			}
		} else if err != nil || el == nil {
			return nil, err
		}
	}

	decodedResponse, err := sp.unmarshalResponse(el)
	if err != nil {
		return nil, err
	}

	err = sp.decryptAssertions(decodedResponse)
	if err != nil {
		return nil, err
	}

	err = sp.Validate(decodedResponse)
	if err != nil {
		return nil, err
	}

	return decodedResponse, nil
}
