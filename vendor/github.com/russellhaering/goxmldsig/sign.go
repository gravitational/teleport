package dsig

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	_ "crypto/sha1"
	_ "crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/beevik/etree"
	"github.com/russellhaering/goxmldsig/etreeutils"
)

type SigningContext struct {
	Hash          crypto.Hash
	KeyStore      X509KeyStore
	IdAttribute   string
	Prefix        string
	Canonicalizer Canonicalizer
}

func NewDefaultSigningContext(ks X509KeyStore) *SigningContext {
	return &SigningContext{
		Hash:          crypto.SHA256,
		KeyStore:      ks,
		IdAttribute:   DefaultIdAttr,
		Prefix:        DefaultPrefix,
		Canonicalizer: MakeC14N11Canonicalizer(),
	}
}

func (ctx *SigningContext) SetSignatureMethod(algorithmID string) error {
	hash, ok := signatureMethodsByIdentifier[algorithmID]
	if !ok {
		return fmt.Errorf("Unknown SignatureMethod: %s", algorithmID)
	}

	ctx.Hash = hash

	return nil
}

func (ctx *SigningContext) digest(el *etree.Element) ([]byte, error) {
	canonical, err := ctx.Canonicalizer.Canonicalize(el)
	if err != nil {
		return nil, err
	}

	hash := ctx.Hash.New()
	_, err = hash.Write(canonical)
	if err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}

func (ctx *SigningContext) constructSignedInfo(el *etree.Element, enveloped bool) (*etree.Element, error) {
	digestAlgorithmIdentifier, ok := digestAlgorithmIdentifiers[ctx.Hash]
	if !ok {
		return nil, errors.New("unsupported hash mechanism")
	}

	signatureMethodIdentifier, ok := signatureMethodIdentifiers[ctx.Hash]
	if !ok {
		return nil, errors.New("unsupported signature method")
	}

	digest, err := ctx.digest(el)
	if err != nil {
		return nil, err
	}

	signedInfo := &etree.Element{
		Tag:   SignedInfoTag,
		Space: ctx.Prefix,
	}

	// /SignedInfo/CanonicalizationMethod
	canonicalizationMethod := ctx.createNamespacedElement(signedInfo, CanonicalizationMethodTag)
	canonicalizationMethod.CreateAttr(AlgorithmAttr, string(ctx.Canonicalizer.Algorithm()))

	// /SignedInfo/SignatureMethod
	signatureMethod := ctx.createNamespacedElement(signedInfo, SignatureMethodTag)
	signatureMethod.CreateAttr(AlgorithmAttr, signatureMethodIdentifier)

	// /SignedInfo/Reference
	reference := ctx.createNamespacedElement(signedInfo, ReferenceTag)

	dataId := el.SelectAttrValue(ctx.IdAttribute, "")
	if dataId == "" {
		return nil, errors.New("Missing data ID")
	}

	reference.CreateAttr(URIAttr, "#"+dataId)

	// /SignedInfo/Reference/Transforms
	transforms := ctx.createNamespacedElement(reference, TransformsTag)
	if enveloped {
		envelopedTransform := ctx.createNamespacedElement(transforms, TransformTag)
		envelopedTransform.CreateAttr(AlgorithmAttr, EnvelopedSignatureAltorithmId.String())
	}
	canonicalizationAlgorithm := ctx.createNamespacedElement(transforms, TransformTag)
	canonicalizationAlgorithm.CreateAttr(AlgorithmAttr, string(ctx.Canonicalizer.Algorithm()))

	// /SignedInfo/Reference/DigestMethod
	digestMethod := ctx.createNamespacedElement(reference, DigestMethodTag)
	digestMethod.CreateAttr(AlgorithmAttr, digestAlgorithmIdentifier)

	// /SignedInfo/Reference/DigestValue
	digestValue := ctx.createNamespacedElement(reference, DigestValueTag)
	digestValue.SetText(base64.StdEncoding.EncodeToString(digest))

	return signedInfo, nil
}

func (ctx *SigningContext) ConstructSignature(el *etree.Element, enveloped bool) (*etree.Element, error) {
	signedInfo, err := ctx.constructSignedInfo(el, enveloped)
	if err != nil {
		return nil, err
	}

	sig := &etree.Element{
		Tag:   SignatureTag,
		Space: ctx.Prefix,
	}

	xmlns := "xmlns"
	if ctx.Prefix != "" {
		xmlns += ":" + ctx.Prefix
	}

	sig.CreateAttr(xmlns, Namespace)
	sig.AddChild(signedInfo)

	// When using xml-c14n11 (ie, non-exclusive canonicalization) the canonical form
	// of the SignedInfo must declare all namespaces that are in scope at it's final
	// enveloped location in the document. In order to do that, we're going to construct
	// a series of cascading NSContexts to capture namespace declarations:

	// First get the context surrounding the element we are signing.
	rootNSCtx, err := etreeutils.NSBuildParentContext(el)
	if err != nil {
		return nil, err
	}

	// Then capture any declarations on the element itself.
	elNSCtx, err := rootNSCtx.SubContext(el)
	if err != nil {
		return nil, err
	}

	// Followed by declarations on the Signature (which we just added above)
	sigNSCtx, err := elNSCtx.SubContext(sig)
	if err != nil {
		return nil, err
	}

	// Finally detatch the SignedInfo in order to capture all of the namespace
	// declarations in the scope we've constructed.
	detatchedSignedInfo, err := etreeutils.NSDetatch(sigNSCtx, signedInfo)
	if err != nil {
		return nil, err
	}

	digest, err := ctx.digest(detatchedSignedInfo)
	if err != nil {
		return nil, err
	}

	key, cert, err := ctx.KeyStore.GetKeyPair()
	if err != nil {
		return nil, err
	}

	rawSignature, err := rsa.SignPKCS1v15(rand.Reader, key, ctx.Hash, digest)
	if err != nil {
		return nil, err
	}

	signatureValue := ctx.createNamespacedElement(sig, SignatureValueTag)
	signatureValue.SetText(base64.StdEncoding.EncodeToString(rawSignature))

	keyInfo := ctx.createNamespacedElement(sig, KeyInfoTag)
	x509Data := ctx.createNamespacedElement(keyInfo, X509DataTag)
	x509Certificate := ctx.createNamespacedElement(x509Data, X509CertificateTag)
	x509Certificate.SetText(base64.StdEncoding.EncodeToString(cert))

	return sig, nil
}

func (ctx *SigningContext) createNamespacedElement(el *etree.Element, tag string) *etree.Element {
	child := el.CreateElement(tag)
	child.Space = ctx.Prefix
	return child
}

func (ctx *SigningContext) SignEnveloped(el *etree.Element) (*etree.Element, error) {
	sig, err := ctx.ConstructSignature(el, true)
	if err != nil {
		return nil, err
	}

	ret := el.Copy()
	ret.Child = append(ret.Child, sig)

	return ret, nil
}
