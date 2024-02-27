package types

import "time"

// GetVersion returns the resource version.
func (k *CrownJewel) GetVersion() string {
	return k.Version
}

// GetKind returns the resource kind.
func (k *CrownJewel) GetKind() string {
	return k.Kind
}

// GetSubKind returns the app resource subkind.
func (k *CrownJewel) GetSubKind() string {
	return k.SubKind
}

// SetSubKind sets the app resource subkind.
func (k *CrownJewel) SetSubKind(sk string) {
	k.SubKind = sk
}

// GetResourceID returns the app resource ID.
func (k *CrownJewel) GetResourceID() int64 {
	return k.Metadata.ID
}

// SetResourceID sets the resource ID.
func (k *CrownJewel) SetResourceID(id int64) {
	k.Metadata.ID = id
}

// GetRevision returns the revision
func (k *CrownJewel) GetRevision() string {
	return k.Metadata.GetRevision()
}

// SetRevision sets the revision
func (k *CrownJewel) SetRevision(rev string) {
	k.Metadata.SetRevision(rev)
}

// GetMetadata returns the resource metadata.
func (k *CrownJewel) GetMetadata() Metadata {
	return k.Metadata
}

// Origin returns the origin value of the resource.
func (k *CrownJewel) Origin() string {
	return k.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (k *CrownJewel) SetOrigin(origin string) {
	k.Metadata.SetOrigin(origin)
}

// GetNamespace returns the kube resource namespace.
func (k *CrownJewel) GetNamespace() string {
	return k.Metadata.Namespace
}

// SetExpiry sets the kube resource expiration time.
func (k *CrownJewel) SetExpiry(expiry time.Time) {
	k.Metadata.SetExpiry(expiry)
}

// Expiry returns the kube resource expiration time.
func (k *CrownJewel) Expiry() time.Time {
	return k.Metadata.Expiry()
}

// GetName returns the kube resource name.
func (k *CrownJewel) GetName() string {
	return k.Metadata.Name
}

// SetName sets the resource name.
func (k *CrownJewel) SetName(name string) {
	k.Metadata.Name = name
}

func (k *CrownJewel) SetMetadata(meta Metadata) {
	k.Metadata = meta
}

func (k *CrownJewel) CheckAndSetDefaults() error {
	k.setStaticFields()
	return k.Metadata.CheckAndSetDefaults()
}

func (k *CrownJewel) setStaticFields() {
	k.Kind = KindCrownJewel
	k.Version = V1
}

func (k *CrownJewel) String() string {
	return k.Metadata.String()
}

func (k *CrownJewel) GetAllLabels() map[string]string {
	return k.Metadata.Labels
}

func (k *CrownJewel) GetStaticLabels() map[string]string {
	return k.Metadata.Labels
}

func (k *CrownJewel) SetStaticLabels(l map[string]string) {
	 k.Metadata.Labels=l
}

func (k *CrownJewel) SetLabels(labels map[string]string) {
	k.Metadata.Labels = labels
}

func (k *CrownJewel) GetLabels() map[string]string {
	return k.Metadata.Labels
}

func (k *CrownJewel) GetLabel(kk string) (string, bool) {
	val, ok := k.Metadata.Labels[kk]
	return val, ok
}

func (k *CrownJewel) MatchSearch(values []string) bool {
	return true
}


