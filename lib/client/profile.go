/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package client

import (
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// ProfileStore is a storage interface for client profile data.
type ProfileStore interface {
	// CurrentProfile returns the current profile.
	CurrentProfile() (string, error)

	// ListProfiles returns a list of all profiles.
	ListProfiles() ([]string, error)

	// GetProfile returns the requested profile.
	GetProfile(profileName string) (*profile.Profile, error)

	// SaveProfile saves the given profile. If makeCurrent
	// is true, it makes this profile current.
	SaveProfile(profile *profile.Profile, setCurrent bool) error
}

// MemProfileStore is an in-memory implementation of ProfileStore.
type MemProfileStore struct {
	// currentProfile is the currently selected profile.
	currentProfile string
	// profiles is a map of proxyHosts to profiles.
	profiles map[string]*profile.Profile
}

// NewMemProfileStore creates a new instance of MemProfileStore.
func NewMemProfileStore() *MemProfileStore {
	return &MemProfileStore{
		profiles: make(map[string]*profile.Profile),
	}
}

// CurrentProfile returns the current profile.
func (ms *MemProfileStore) CurrentProfile() (string, error) {
	if ms.currentProfile == "" {
		return "", trace.NotFound("current-profile is not set")
	}
	return ms.currentProfile, nil
}

// ListProfiles returns a list of all profiles.
func (ms *MemProfileStore) ListProfiles() ([]string, error) {
	var profileNames []string
	for profileName := range ms.profiles {
		profileNames = append(profileNames, profileName)
	}
	return profileNames, nil
}

// GetProfile returns the requested profile.
func (ms *MemProfileStore) GetProfile(profileName string) (*profile.Profile, error) {
	if profile, ok := ms.profiles[profileName]; ok {
		return profile.Copy(), nil
	}
	return nil, trace.NotFound("profile for proxy host %q not found", profileName)
}

// SaveProfile saves the given profile. If makeCurrent
// is true, it makes this profile current.
func (ms *MemProfileStore) SaveProfile(profile *profile.Profile, makecurrent bool) error {
	ms.profiles[profile.Name()] = profile.Copy()
	if makecurrent {
		ms.currentProfile = profile.Name()
	}
	return nil
}

// FSProfileStore is an on-disk implementation of the ProfileStore interface.
//
// The FS store uses the file layout outlined in `api/utils/keypaths.go`.
type FSProfileStore struct {
	// log holds the structured logger.
	log logrus.FieldLogger

	// Dir is the directory where all keys are stored.
	Dir string
}

// NewFSProfileStore creates a new instance of FSProfileStore.
func NewFSProfileStore(dirPath string) *FSProfileStore {
	dirPath = profile.FullProfilePath(dirPath)
	return &FSProfileStore{
		log: logrus.WithField(teleport.ComponentKey, teleport.ComponentKeyStore),
		Dir: dirPath,
	}
}

// CurrentProfile returns the current profile.
func (fs *FSProfileStore) CurrentProfile() (string, error) {
	profileName, err := profile.GetCurrentProfileName(fs.Dir)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return profileName, nil
}

// ListProfiles returns a list of all profiles.
func (fs *FSProfileStore) ListProfiles() ([]string, error) {
	profileNames, err := profile.ListProfileNames(fs.Dir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return profileNames, nil
}

// GetProfile returns the requested profile.
func (fs *FSProfileStore) GetProfile(profileName string) (*profile.Profile, error) {
	profile, err := profile.FromDir(fs.Dir, profileName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return profile, nil
}

// SaveProfile saves the given profile. If makeCurrent
// is true, it makes this profile current.
func (fs *FSProfileStore) SaveProfile(profile *profile.Profile, makeCurrent bool) error {
	if err := os.MkdirAll(fs.Dir, os.ModeDir|profileDirPerms); err != nil {
		return trace.ConvertSystemError(err)
	}

	err := profile.SaveToDir(fs.Dir, makeCurrent)
	return trace.Wrap(err)
}

// ProfileStatus combines metadata from the logged in profile and associated
// SSH certificate.
type ProfileStatus struct {
	// Name is the profile name.
	Name string

	// Dir is the directory where profile is located.
	Dir string

	// ProxyURL is the URL the web client is accessible at.
	ProxyURL url.URL

	// Username is the Teleport username.
	Username string

	// Roles is a list of Teleport Roles this user has been assigned.
	Roles []string

	// Logins are the Linux accounts, also known as principals in OpenSSH terminology.
	Logins []string

	// KubeEnabled is true when this profile is configured to connect to a
	// kubernetes cluster.
	KubeEnabled bool

	// KubeUsers are the kubernetes users used by this profile.
	KubeUsers []string

	// KubeGroups are the kubernetes groups used by this profile.
	KubeGroups []string

	// Databases is a list of database services this profile is logged into.
	Databases []tlsca.RouteToDatabase

	// Apps is a list of apps this profile is logged into.
	Apps []tlsca.RouteToApp

	// ValidUntil is the time at which this SSH certificate will expire.
	ValidUntil time.Time

	// GetKeyRingError is any error encountered while loading the KeyRing.
	GetKeyRingError error

	// Extensions is a list of enabled SSH features for the certificate.
	Extensions []string

	// CriticalOptions is a map of SSH critical options for the certificate.
	CriticalOptions map[string]string

	// Cluster is a selected cluster
	Cluster string

	// Traits hold claim data used to populate a role at runtime.
	Traits wrappers.Traits

	// ActiveRequests tracks the privilege escalation requests applied
	// during certificate construction.
	ActiveRequests []string

	// AWSRoleARNs is a list of allowed AWS role ARNs user can assume.
	AWSRolesARNs []string

	// AzureIdentities is a list of allowed Azure identities user can assume.
	AzureIdentities []string

	// GCPServiceAccounts is a list of allowed GCP service accounts user can assume.
	GCPServiceAccounts []string

	// AllowedResourceIDs is a list of resources the user can access. An empty
	// list means there are no resource-specific restrictions.
	AllowedResourceIDs []types.ResourceID

	// IsVirtual is set when this profile does not actually exist on disk,
	// probably because it was constructed from an identity file. When set,
	// certain profile functions - particularly those that return paths to
	// files on disk - must be accompanied by fallback logic when those paths
	// do not exist.
	IsVirtual bool

	// SAMLSingleLogoutEnabled is whether SAML SLO (single logout) is enabled, this can only be true if this is a SAML SSO session
	// using an auth connector with a SAML SLO URL configured.
	SAMLSingleLogoutEnabled bool
}

// profileOptions contains fields needed to initialize a profile beyond those
// derived directly from a Key.
type profileOptions struct {
	ProfileName             string
	ProfileDir              string
	WebProxyAddr            string
	Username                string
	SiteName                string
	KubeProxyAddr           string
	IsVirtual               bool
	SAMLSingleLogoutEnabled bool
}

// profileFromkey returns a ProfileStatus for the given key and options.
func profileStatusFromKey(key *Key, opts profileOptions) (*ProfileStatus, error) {
	sshCert, err := key.SSHCert()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshIdent, err := sshca.DecodeIdentity(sshCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Extract from the certificate how much longer it will be valid for.
	validUntil := time.Unix(int64(sshIdent.ValidBefore), 0)

	// Extract roles from certificate. Note, if the certificate is in old format,
	// this will be empty.
	roles := slices.Clone(sshIdent.Roles)
	sort.Strings(roles)

	// Extract extensions from certificate. This lists the abilities of the
	// certificate (like can the user request a PTY, port forwarding, etc.)
	var extensions []string
	for ext := range sshCert.Extensions {
		if ext == teleport.CertExtensionTeleportRoles ||
			ext == teleport.CertExtensionTeleportTraits ||
			ext == teleport.CertExtensionTeleportRouteToCluster ||
			ext == teleport.CertExtensionTeleportActiveRequests ||
			ext == teleport.CertExtensionAllowedResources {
			continue
		}
		extensions = append(extensions, ext)
	}
	sort.Strings(extensions)

	tlsCert, err := key.TeleportTLSCertificate()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsID, err := tlsca.FromSubject(tlsCert.Subject, time.Time{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	databases, err := findActiveDatabases(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appCerts, err := key.AppTLSCertificates()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var apps []tlsca.RouteToApp
	for _, cert := range appCerts {
		tlsID, err := tlsca.FromSubject(cert.Subject, time.Time{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if tlsID.RouteToApp.PublicAddr != "" {
			apps = append(apps, tlsID.RouteToApp)
		}
	}

	return &ProfileStatus{
		Name: opts.ProfileName,
		Dir:  opts.ProfileDir,
		ProxyURL: url.URL{
			Scheme: "https",
			Host:   opts.WebProxyAddr,
		},
		Username:                opts.Username,
		Logins:                  sshCert.ValidPrincipals,
		ValidUntil:              validUntil,
		Extensions:              extensions,
		CriticalOptions:         sshCert.CriticalOptions,
		Roles:                   roles,
		Cluster:                 opts.SiteName,
		Traits:                  sshIdent.Traits,
		ActiveRequests:          sshIdent.ActiveRequests,
		KubeEnabled:             opts.KubeProxyAddr != "",
		KubeUsers:               tlsID.KubernetesUsers,
		KubeGroups:              tlsID.KubernetesGroups,
		Databases:               databases,
		Apps:                    apps,
		AWSRolesARNs:            tlsID.AWSRoleARNs,
		AzureIdentities:         tlsID.AzureIdentities,
		GCPServiceAccounts:      tlsID.GCPServiceAccounts,
		IsVirtual:               opts.IsVirtual,
		AllowedResourceIDs:      sshIdent.AllowedResourceIDs,
		SAMLSingleLogoutEnabled: opts.SAMLSingleLogoutEnabled,
	}, nil
}

// IsExpired returns true if profile is not expired yet
func (p *ProfileStatus) IsExpired(now time.Time) bool {
	return p.ValidUntil.Sub(now) <= 0
}

// virtualPathWarnOnce is used to ensure warnings about missing virtual path
// environment variables are consolidated into a single message and not spammed
// to the console.
var virtualPathWarnOnce sync.Once

// virtualPathFromEnv attempts to retrieve the path as defined by the given
// formatter from the environment.
func (p *ProfileStatus) virtualPathFromEnv(kind VirtualPathKind, params VirtualPathParams) (string, bool) {
	if !p.IsVirtual {
		return "", false
	}

	for _, envName := range VirtualPathEnvNames(kind, params) {
		if val, ok := os.LookupEnv(envName); ok {
			return val, true
		}
	}

	// If we can't resolve any env vars, this will return garbage which we
	// should at least warn about. As ugly as this is, arguably making every
	// profile path lookup fallible is even uglier.
	log.Debugf("Could not resolve path to virtual profile entry of type %s "+
		"with parameters %+v.", kind, params)

	virtualPathWarnOnce.Do(func() {
		log.Errorf("A virtual profile is in use due to an identity file " +
			"(`-i ...`) but this functionality requires additional files on " +
			"disk and may fail. Consider using a compatible wrapper " +
			"application (e.g. Machine ID) for this command.")
	})

	return "", false
}

// CACertPathForCluster returns path to the cluster CA certificate for this profile.
//
// It's stored in  <profile-dir>/keys/<proxy>/cas/<cluster>.pem by default.
func (p *ProfileStatus) CACertPathForCluster(cluster string) string {
	// Return an env var override if both valid and present for this identity.
	if path, ok := p.virtualPathFromEnv(VirtualPathCA, VirtualPathCAParams(types.HostCA)); ok {
		return path
	}

	return filepath.Join(keypaths.ProxyKeyDir(p.Dir, p.Name), "cas", cluster+".pem")
}

// KeyPath returns path to the private key for this profile.
//
// It's kept in <profile-dir>/keys/<proxy>/<user>.
func (p *ProfileStatus) KeyPath() string {
	// Return an env var override if both valid and present for this identity.
	if path, ok := p.virtualPathFromEnv(VirtualPathKey, nil); ok {
		return path
	}

	return keypaths.UserKeyPath(p.Dir, p.Name, p.Username)
}

// DatabaseCertPathForCluster returns path to the specified database access
// certificate for this profile, for the specified cluster.
//
// It's kept in <profile-dir>/keys/<proxy>/<user>-db/<cluster>/<name>-x509.pem
//
// If the input cluster name is an empty string, the selected cluster in the
// profile will be used.
func (p *ProfileStatus) DatabaseCertPathForCluster(clusterName string, databaseName string) string {
	if clusterName == "" {
		clusterName = p.Cluster
	}

	if path, ok := p.virtualPathFromEnv(VirtualPathDatabase, VirtualPathDatabaseParams(databaseName)); ok {
		return path
	}

	return keypaths.DatabaseCertPath(p.Dir, p.Name, p.Username, clusterName, databaseName)
}

// OracleWalletDir returns path to the specified database access
// certificate for this profile, for the specified cluster.
//
// It's kept in <profile-dir>/keys/<proxy>/<user>-db/<cluster>/dbname-wallet/
//
// If the input cluster name is an empty string, the selected cluster in the
// profile will be used.
func (p *ProfileStatus) OracleWalletDir(clusterName string, databaseName string) string {
	if clusterName == "" {
		clusterName = p.Cluster
	}

	if path, ok := p.virtualPathFromEnv(VirtualPathDatabase, VirtualPathDatabaseParams(databaseName)); ok {
		return path
	}

	return keypaths.DatabaseOracleWalletDirectory(p.Dir, p.Name, p.Username, clusterName, databaseName)
}

// DatabaseLocalCAPath returns the specified db 's self-signed localhost CA path for
// this profile.
//
// It's kept in <profile-dir>/keys/<proxy>/<user>-db/proxy-localca.pem
func (p *ProfileStatus) DatabaseLocalCAPath() string {
	if path, ok := p.virtualPathFromEnv(VirtualPathDatabase, nil); ok {
		return path
	}
	return filepath.Join(keypaths.DatabaseDir(p.Dir, p.Name, p.Username), "proxy-localca.pem")
}

// AppCertPath returns path to the specified app access certificate
// for this profile.
//
// It's kept in <profile-dir>/keys/<proxy>/<user>-app/<cluster>/<name>-x509.pem
func (p *ProfileStatus) AppCertPath(cluster, name string) string {
	if cluster == "" {
		cluster = p.Cluster
	}
	if path, ok := p.virtualPathFromEnv(VirtualPathApp, VirtualPathAppParams(name)); ok {
		return path
	}

	return keypaths.AppCertPath(p.Dir, p.Name, p.Username, cluster, name)
}

// AppLocalCAPath returns the specified app's self-signed localhost CA path for
// this profile.
//
// It's kept in <profile-dir>/keys/<proxy>/<user>-app/<cluster>/<name>-localca.pem
func (p *ProfileStatus) AppLocalCAPath(cluster, name string) string {
	if cluster == "" {
		cluster = p.Cluster
	}
	return keypaths.AppLocalCAPath(p.Dir, p.Name, p.Username, cluster, name)
}

// KubeConfigPath returns path to the specified kubeconfig for this profile.
//
// It's kept in <profile-dir>/keys/<proxy>/<user>-kube/<cluster>/<name>-kubeconfig
func (p *ProfileStatus) KubeConfigPath(name string) string {
	if path, ok := p.virtualPathFromEnv(VirtualPathKubernetes, VirtualPathKubernetesParams(name)); ok {
		return path
	}

	return keypaths.KubeConfigPath(p.Dir, p.Name, p.Username, p.Cluster, name)
}

// KubeCertPathForCluster returns path to the specified kube access certificate
// for this profile, for the specified cluster name.
//
// It's kept in <profile-dir>/keys/<proxy>/<username>-kube/<cluster>/<name>-x509.pem
func (p *ProfileStatus) KubeCertPathForCluster(teleportCluster, kubeCluster string) string {
	if teleportCluster == "" {
		teleportCluster = p.Cluster
	}
	if path, ok := p.virtualPathFromEnv(VirtualPathKubernetes, VirtualPathKubernetesParams(kubeCluster)); ok {
		return path
	}
	return keypaths.KubeCertPath(p.Dir, p.Name, p.Username, teleportCluster, kubeCluster)
}

// DatabaseServices returns a list of database service names for this profile.
func (p *ProfileStatus) DatabaseServices() (result []string) {
	for _, db := range p.Databases {
		result = append(result, db.ServiceName)
	}
	return result
}

// DatabasesForCluster returns a list of databases for this profile, for the
// specified cluster name.
func (p *ProfileStatus) DatabasesForCluster(clusterName string) ([]tlsca.RouteToDatabase, error) {
	if clusterName == "" || clusterName == p.Cluster {
		return p.Databases, nil
	}

	idx := KeyIndex{
		ProxyHost:   p.Name,
		Username:    p.Username,
		ClusterName: clusterName,
	}

	store := NewFSKeyStore(p.Dir)
	key, err := store.GetKey(idx, WithDBCerts{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return findActiveDatabases(key)
}

// AppsForCluster returns a list of apps for this profile, for the
// specified cluster name.
func (p *ProfileStatus) AppsForCluster(clusterName string) ([]tlsca.RouteToApp, error) {
	if clusterName == "" || clusterName == p.Cluster {
		return p.Apps, nil
	}

	idx := KeyIndex{
		ProxyHost:   p.Name,
		Username:    p.Username,
		ClusterName: clusterName,
	}

	store := NewFSKeyStore(p.Dir)
	key, err := store.GetKey(idx, WithAppCerts{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return findActiveApps(key)
}

// AppNames returns a list of app names this profile is logged into.
func (p *ProfileStatus) AppNames() (result []string) {
	for _, app := range p.Apps {
		result = append(result, app.Name)
	}
	return result
}

// ProfileNameFromProxyAddress converts proxy address to profile name or
// returns the current profile if the proxyAddr is not set.
func ProfileNameFromProxyAddress(store ProfileStore, proxyAddr string) (string, error) {
	if proxyAddr == "" {
		profileName, err := store.CurrentProfile()
		return profileName, trace.Wrap(err)
	}

	profileName, err := utils.Host(proxyAddr)
	return profileName, trace.Wrap(err)
}

// AccessInfo returns the complete services.AccessInfo for this profile.
func (p *ProfileStatus) AccessInfo() *services.AccessInfo {
	return &services.AccessInfo{
		Username:           p.Username,
		Roles:              p.Roles,
		Traits:             p.Traits,
		AllowedResourceIDs: p.AllowedResourceIDs,
	}
}
