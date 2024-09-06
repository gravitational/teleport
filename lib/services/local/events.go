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

package local

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/kubewaitingcontainer"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// EventsService implements service to watch for events
type EventsService struct {
	*logrus.Entry
	backend backend.Backend
}

// NewEventsService returns new events service instance
func NewEventsService(b backend.Backend) *EventsService {
	return &EventsService{
		Entry:   logrus.WithFields(logrus.Fields{teleport.ComponentKey: "Events"}),
		backend: b,
	}
}

// NewWatcher returns a new event watcher
func (e *EventsService) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	if len(watch.Kinds) == 0 {
		return nil, trace.BadParameter("global watches are not supported yet")
	}

	validKinds := make([]types.WatchKind, 0, len(watch.Kinds))
	var parsers []resourceParser
	var prefixes []backend.Key
	for _, kind := range watch.Kinds {
		if kind.Name != "" && kind.Kind != types.KindNamespace {
			if watch.AllowPartialSuccess {
				continue
			}
			return nil, trace.BadParameter("watch with Name is only supported for Namespace resource")
		}
		var parser resourceParser
		switch kind.Kind {
		case types.KindCertAuthority:
			parser = newCertAuthorityParser(kind.LoadSecrets, kind.Filter)
		case types.KindToken:
			parser = newProvisionTokenParser()
		case types.KindStaticTokens:
			parser = newStaticTokensParser()
		case types.KindClusterAuditConfig:
			parser = newClusterAuditConfigParser()
		case types.KindClusterNetworkingConfig:
			parser = newClusterNetworkingConfigParser()
		case types.KindClusterAuthPreference:
			parser = newAuthPreferenceParser()
		case types.KindSessionRecordingConfig:
			parser = newSessionRecordingConfigParser()
		case types.KindUIConfig:
			parser = newUIConfigParser()
		case types.KindClusterName:
			parser = newClusterNameParser()
		case types.KindNamespace:
			parser = newNamespaceParser(kind.Name)
		case types.KindRole:
			parser = newRoleParser()
		case types.KindUser:
			parser = newUserParser()
		case types.KindNode:
			parser = newNodeParser()
		case types.KindProxy:
			parser = newProxyParser()
		case types.KindAuthServer:
			parser = newAuthServerParser()
		case types.KindTunnelConnection:
			parser = newTunnelConnectionParser()
		case types.KindReverseTunnel:
			parser = newReverseTunnelParser()
		case types.KindAccessRequest:
			p, err := newAccessRequestParser(kind.Filter)
			if err != nil {
				if watch.AllowPartialSuccess {
					continue
				}
				return nil, trace.Wrap(err)
			}
			parser = p
		case types.KindAppServer:
			parser = newAppServerV3Parser()
		case types.KindWebSession:
			switch kind.SubKind {
			case types.KindSAMLIdPSession:
				parser = newSAMLIdPSessionParser(kind.LoadSecrets)
			case types.KindSnowflakeSession:
				parser = newSnowflakeSessionParser(kind.LoadSecrets)
			case types.KindAppSession:
				parser = newAppSessionParser(kind.LoadSecrets)
			case types.KindWebSession:
				parser = newWebSessionParser(kind.LoadSecrets)
			default:
				if watch.AllowPartialSuccess {
					continue
				}
				return nil, trace.BadParameter("watcher on object subkind %q is not supported", kind.SubKind)
			}
		case types.KindWebToken:
			parser = newWebTokenParser()
		case types.KindRemoteCluster:
			parser = newRemoteClusterParser()
		case types.KindKubeServer:
			parser = newKubeServerParser()
		case types.KindDatabaseServer:
			parser = newDatabaseServerParser()
		case types.KindDatabaseService:
			parser = newDatabaseServiceParser()
		case types.KindDatabase:
			parser = newDatabaseParser()
		case types.KindDatabaseObject:
			parser = newDatabaseObjectParser()
		case types.KindApp:
			parser = newAppParser()
		case types.KindLock:
			parser = newLockParser()
		case types.KindNetworkRestrictions:
			parser = newNetworkRestrictionsParser()
		case types.KindWindowsDesktopService:
			parser = newWindowsDesktopServicesParser()
		case types.KindWindowsDesktop:
			parser = newWindowsDesktopsParser()
		case types.KindInstaller:
			parser = newInstallerParser()
		case types.KindKubernetesCluster:
			parser = newKubeClusterParser()
		case types.KindCrownJewel:
			parser = newCrownJewelParser()
		case types.KindPlugin:
			parser = newPluginParser(kind.LoadSecrets)
		case types.KindSAMLIdPServiceProvider:
			parser = newSAMLIDPServiceProviderParser()
		case types.KindUserGroup:
			parser = newUserGroupParser()
		case types.KindOktaImportRule:
			parser = newOktaImportRuleParser()
		case types.KindOktaAssignment:
			parser = newOktaAssignmentParser()
		case types.KindIntegration:
			parser = newIntegrationParser()
		case types.KindDiscoveryConfig:
			parser = newDiscoveryConfigParser()
		case types.KindHeadlessAuthentication:
			p, err := newHeadlessAuthenticationParser(kind.Filter)
			if err != nil {
				if watch.AllowPartialSuccess {
					continue
				}
				return nil, trace.Wrap(err)
			}
			parser = p
		case types.KindAccessList:
			parser = newAccessListParser()
		case types.KindAuditQuery:
			parser = newAuditQueryParser()
		case types.KindSecurityReport:
			parser = newSecurityReportParser()
		case types.KindSecurityReportState:
			parser = newSecurityReportStateParser()
		case types.KindUserLoginState:
			parser = newUserLoginStateParser()
		case types.KindAccessListMember:
			parser = newAccessListMemberParser()
		case types.KindAccessListReview:
			parser = newAccessListReviewParser()
		case types.KindKubeWaitingContainer:
			parser = newKubeWaitingContainerParser()
		case types.KindNotification:
			parser = newUserNotificationParser()
		case types.KindGlobalNotification:
			parser = newGlobalNotificationParser()
		case types.KindAccessMonitoringRule:
			parser = newAccessMonitoringRuleParser()
		case types.KindBotInstance:
			parser = newBotInstanceParser()
		case types.KindInstance:
			parser = newInstanceParser()
		case types.KindDevice:
			parser = newDeviceParser()
		case types.KindAccessGraphSecretPrivateKey:
			parser = newAccessGraphSecretPrivateKeyParser()
		case types.KindAccessGraphSecretAuthorizedKey:
			parser = newAccessGraphSecretAuthorizedKeyParser()
		case types.KindAccessGraphSettings:
			parser = newAccessGraphSettingsParser()
		case types.KindSPIFFEFederation:
			parser = newSPIFFEFederationParser()
		case types.KindStaticHostUser:
			parser = newStaticHostUserParser()
		default:
			if watch.AllowPartialSuccess {
				continue
			}
			return nil, trace.BadParameter("watcher on object kind %q is not supported", kind.Kind)
		}
		prefixes = append(prefixes, parser.prefixes()...)
		parsers = append(parsers, parser)
		validKinds = append(validKinds, kind)
	}

	if len(validKinds) == 0 {
		return nil, trace.BadParameter("none of the requested kinds can be watched")
	}

	origNumPrefixes := len(prefixes)
	redundantNumPrefixes := len(backend.RemoveRedundantPrefixes(prefixes))
	if origNumPrefixes != redundantNumPrefixes {
		// If you've hit this error, the prefixes in two or more of your parsers probably overlap, meaning
		// one prefix will also contain another as a subset. Look into using backend.ExactKey instead of
		// backend.NewKey in your parser.
		return nil, trace.BadParameter("redundant prefixes detected in events, which will result in event parsers not aligning with their intended prefix (this is a bug)")
	}

	w, err := e.backend.NewWatcher(ctx, backend.Watch{
		Name:            watch.Name,
		Prefixes:        prefixes,
		QueueSize:       watch.QueueSize,
		MetricComponent: watch.MetricComponent,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newWatcher(w, e.Entry, parsers, validKinds), nil
}

func newWatcher(backendWatcher backend.Watcher, l *logrus.Entry, parsers []resourceParser, kinds []types.WatchKind) *watcher {
	w := &watcher{
		backendWatcher: backendWatcher,
		Entry:          l,
		parsers:        parsers,
		eventsC:        make(chan types.Event),
		kinds:          kinds,
	}
	go w.forwardEvents()
	return w
}

type watcher struct {
	*logrus.Entry
	parsers        []resourceParser
	backendWatcher backend.Watcher
	eventsC        chan types.Event
	kinds          []types.WatchKind
}

func (w *watcher) Error() error {
	return nil
}

func (w *watcher) parseEvent(e backend.Event) ([]types.Event, []error) {
	if e.Type == types.OpInit {
		return []types.Event{{Type: e.Type, Resource: types.NewWatchStatus(w.kinds)}}, nil
	}
	events := []types.Event{}
	errs := []error{}
	for _, p := range w.parsers {
		if p.match(e.Item.Key) {
			resource, err := p.parse(e)
			if err != nil {
				errs = append(errs, trace.Wrap(err))
				continue
			}
			// if resource is nil, then it was well-formed but is being filtered out.
			if resource == nil {
				continue
			}
			events = append(events, types.Event{Type: e.Type, Resource: resource})
		}
	}
	return events, errs
}

func (w *watcher) forwardEvents() {
	for {
		select {
		case <-w.backendWatcher.Done():
			return
		case event := <-w.backendWatcher.Events():
			converted, errs := w.parseEvent(event)
			for _, err := range errs {
				// not found errors are expected, for example
				// when namespace prefix is watched, it captures
				// node events as well, and there could be no
				// handler registered for nodes, only for namespaces
				if !trace.IsNotFound(err) {
					w.Warning(trace.DebugReport(err))
				}
			}
			for _, c := range converted {
				select {
				case w.eventsC <- c:
				case <-w.backendWatcher.Done():
					return
				}
			}
		}
	}
}

// Events returns channel with events
func (w *watcher) Events() <-chan types.Event {
	return w.eventsC
}

// Done returns the channel signaling the closure
func (w *watcher) Done() <-chan struct{} {
	return w.backendWatcher.Done()
}

// Close closes the watcher and releases
// all associated resources
func (w *watcher) Close() error {
	return w.backendWatcher.Close()
}

// resourceParser is an interface
// for parsing resource from backend byte event stream
type resourceParser interface {
	// parse parses resource from the backend event
	parse(event backend.Event) (types.Resource, error)
	// match returns true if event key matches
	match(key backend.Key) bool
	// prefixes returns prefixes to watch
	prefixes() []backend.Key
}

// baseParser is a partial implementation of resourceParser for the most common
// resource types (stored under a static prefix).
type baseParser struct {
	matchPrefixes []backend.Key
}

func newBaseParser(prefixes ...backend.Key) baseParser {
	return baseParser{matchPrefixes: prefixes}
}

func (p baseParser) prefixes() []backend.Key {
	return p.matchPrefixes
}

func (p baseParser) match(key backend.Key) bool {
	for _, prefix := range p.matchPrefixes {
		if key.HasPrefix(prefix) {
			return true
		}
	}
	return false
}

func newCertAuthorityParser(loadSecrets bool, filter map[string]string) *certAuthorityParser {
	var caFilter types.CertAuthorityFilter
	caFilter.FromMap(filter)
	return &certAuthorityParser{
		loadSecrets: loadSecrets,
		baseParser:  newBaseParser(backend.NewKey(authoritiesPrefix)),
		filter:      caFilter,
	}
}

type certAuthorityParser struct {
	baseParser
	loadSecrets bool
	filter      types.CertAuthorityFilter
}

func (p *certAuthorityParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		caType, name, err := baseTwoKeys(event.Item.Key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &types.ResourceHeader{
			Kind:    types.KindCertAuthority,
			SubKind: caType,
			Version: types.V2,
			Metadata: types.Metadata{
				Name:      name,
				Namespace: apidefaults.Namespace,
			},
		}, nil
	case types.OpPut:
		ca, err := services.UnmarshalCertAuthority(event.Item.Value,
			services.WithExpires(event.Item.Expires), services.WithRevision(event.Item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !p.filter.Match(ca) {
			return nil, nil
		}
		// never send private signing keys over event stream?
		// this might not be true
		setSigningKeys(ca, p.loadSecrets)
		return ca, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newProvisionTokenParser() *provisionTokenParser {
	return &provisionTokenParser{
		baseParser: newBaseParser(backend.NewKey(tokensPrefix)),
	}
}

type provisionTokenParser struct {
	baseParser
}

func (p *provisionTokenParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindToken, types.V2, 0)
	case types.OpPut:
		token, err := services.UnmarshalProvisionToken(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return token, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newStaticTokensParser() *staticTokensParser {
	return &staticTokensParser{
		baseParser: newBaseParser(backend.NewKey(clusterConfigPrefix, staticTokensPrefix)),
	}
}

type staticTokensParser struct {
	baseParser
}

func (p *staticTokensParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		h, err := resourceHeader(event, types.KindStaticTokens, types.V2, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		h.SetName(types.MetaNameStaticTokens)
		return h, nil
	case types.OpPut:
		tokens, err := services.UnmarshalStaticTokens(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return tokens, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newClusterAuditConfigParser() *clusterAuditConfigParser {
	return &clusterAuditConfigParser{
		baseParser: newBaseParser(backend.NewKey(clusterConfigPrefix, auditPrefix)),
	}
}

type clusterAuditConfigParser struct {
	baseParser
}

func (p *clusterAuditConfigParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		h, err := resourceHeader(event, types.KindClusterAuditConfig, types.V2, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		h.SetName(types.MetaNameClusterAuditConfig)
		return h, nil
	case types.OpPut:
		clusterAuditConfig, err := services.UnmarshalClusterAuditConfig(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return clusterAuditConfig, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newClusterNetworkingConfigParser() *clusterNetworkingConfigParser {
	return &clusterNetworkingConfigParser{
		baseParser: newBaseParser(backend.NewKey(clusterConfigPrefix, networkingPrefix)),
	}
}

type clusterNetworkingConfigParser struct {
	baseParser
}

func (p *clusterNetworkingConfigParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		h, err := resourceHeader(event, types.KindClusterNetworkingConfig, types.V2, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		h.SetName(types.MetaNameClusterNetworkingConfig)
		return h, nil
	case types.OpPut:
		clusterNetworkingConfig, err := services.UnmarshalClusterNetworkingConfig(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return clusterNetworkingConfig, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newAuthPreferenceParser() *authPreferenceParser {
	return &authPreferenceParser{
		baseParser: newBaseParser(backend.NewKey(authPrefix, preferencePrefix, generalPrefix)),
	}
}

type authPreferenceParser struct {
	baseParser
}

func (p *authPreferenceParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		h, err := resourceHeader(event, types.KindClusterAuthPreference, types.V2, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		h.SetName(types.MetaNameClusterAuthPreference)
		return h, nil
	case types.OpPut:
		ap, err := services.UnmarshalAuthPreference(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ap, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newUIConfigParser() *uiConfigParser {
	return &uiConfigParser{
		baseParser: newBaseParser(backend.NewKey(clusterConfigPrefix, uiPrefix)),
	}
}

type uiConfigParser struct {
	baseParser
}

func (p *uiConfigParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		h, err := resourceHeader(event, types.KindUIConfig, types.V1, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		h.SetName(types.MetaNameUIConfig)
		return h, nil
	case types.OpPut:
		ap, err := services.UnmarshalUIConfig(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ap, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newSessionRecordingConfigParser() *sessionRecordingConfigParser {
	return &sessionRecordingConfigParser{
		baseParser: newBaseParser(backend.NewKey(clusterConfigPrefix, sessionRecordingPrefix)),
	}
}

type sessionRecordingConfigParser struct {
	baseParser
}

func (p *sessionRecordingConfigParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		h, err := resourceHeader(event, types.KindSessionRecordingConfig, types.V2, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		h.SetName(types.MetaNameSessionRecordingConfig)
		return h, nil
	case types.OpPut:
		ap, err := services.UnmarshalSessionRecordingConfig(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ap, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newClusterNameParser() *clusterNameParser {
	return &clusterNameParser{
		baseParser: newBaseParser(backend.NewKey(clusterConfigPrefix, namePrefix)),
	}
}

type clusterNameParser struct {
	baseParser
}

func (p *clusterNameParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		h, err := resourceHeader(event, types.KindClusterName, types.V2, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		h.SetName(types.MetaNameClusterName)
		return h, nil
	case types.OpPut:
		clusterName, err := services.UnmarshalClusterName(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return clusterName, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newNamespaceParser(name string) *namespaceParser {
	prefix := backend.NewKey(namespacesPrefix)
	if name != "" {
		prefix = backend.NewKey(namespacesPrefix, name, paramsPrefix)
	}
	return &namespaceParser{
		baseParser: newBaseParser(prefix),
	}
}

type namespaceParser struct {
	baseParser
}

func (p *namespaceParser) match(key backend.Key) bool {
	// namespaces are stored under key '/namespaces/<namespace-name>/params'
	// and this code matches similar pattern
	return p.baseParser.match(key) &&
		key.HasSuffix(backend.Key(paramsPrefix)) &&
		len(key.Components()) == 3
}

func (p *namespaceParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindNamespace, types.V2, 1)
	case types.OpPut:
		namespace, err := services.UnmarshalNamespace(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return namespace, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newRoleParser() *roleParser {
	return &roleParser{
		baseParser: newBaseParser(backend.NewKey(rolesPrefix)),
	}
}

type roleParser struct {
	baseParser
}

func (p *roleParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindRole, types.V7, 1)
	case types.OpPut:
		resource, err := services.UnmarshalRole(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return resource, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newAccessRequestParser(m map[string]string) (*accessRequestParser, error) {
	var filter types.AccessRequestFilter
	if err := filter.FromMap(m); err != nil {
		return nil, trace.Wrap(err)
	}
	return &accessRequestParser{
		filter:      filter,
		matchPrefix: backend.NewKey(accessRequestsPrefix),
		matchSuffix: backend.NewKey(paramsPrefix),
	}, nil
}

type accessRequestParser struct {
	filter      types.AccessRequestFilter
	matchPrefix backend.Key
	matchSuffix backend.Key
}

func (p *accessRequestParser) prefixes() []backend.Key {
	return []backend.Key{p.matchPrefix}
}

func (p *accessRequestParser) match(key backend.Key) bool {
	if !key.HasPrefix(p.matchPrefix) {
		return false
	}
	if !key.HasSuffix(p.matchSuffix) {
		return false
	}
	return true
}

func (p *accessRequestParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindAccessRequest, types.V3, 1)
	case types.OpPut:
		req, err := itemToAccessRequest(event.Item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !p.filter.Match(req) {
			return nil, nil
		}
		return req, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newUserParser() *userParser {
	return &userParser{
		baseParser: newBaseParser(backend.NewKey(webPrefix, usersPrefix)),
	}
}

type userParser struct {
	baseParser
}

func (p *userParser) match(key backend.Key) bool {
	// users are stored under key '/web/users/<username>/params'
	// and this code matches similar pattern
	return p.baseParser.match(key) &&
		key.HasSuffix(backend.Key(paramsPrefix)) &&
		len(key.Components()) == 4
}

func (p *userParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindUser, types.V2, 1)
	case types.OpPut:
		resource, err := services.UnmarshalUser(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return resource, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newNodeParser() *nodeParser {
	return &nodeParser{
		baseParser: newBaseParser(backend.NewKey(nodesPrefix, apidefaults.Namespace)),
	}
}

type nodeParser struct {
	baseParser
}

func (p *nodeParser) parse(event backend.Event) (types.Resource, error) {
	return parseServer(event, types.KindNode)
}

func newProxyParser() *proxyParser {
	return &proxyParser{
		baseParser: newBaseParser(backend.NewKey(proxiesPrefix)),
	}
}

type proxyParser struct {
	baseParser
}

func (p *proxyParser) parse(event backend.Event) (types.Resource, error) {
	return parseServer(event, types.KindProxy)
}

func newAuthServerParser() *authServerParser {
	return &authServerParser{
		baseParser: newBaseParser(backend.NewKey(authServersPrefix)),
	}
}

type authServerParser struct {
	baseParser
}

func (p *authServerParser) parse(event backend.Event) (types.Resource, error) {
	return parseServer(event, types.KindAuthServer)
}

func newTunnelConnectionParser() *tunnelConnectionParser {
	return &tunnelConnectionParser{
		baseParser: newBaseParser(backend.NewKey(tunnelConnectionsPrefix)),
	}
}

type tunnelConnectionParser struct {
	baseParser
}

func (p *tunnelConnectionParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		clusterName, name, err := baseTwoKeys(event.Item.Key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &types.ResourceHeader{
			Kind:    types.KindTunnelConnection,
			SubKind: clusterName,
			Version: types.V2,
			Metadata: types.Metadata{
				Name:      name,
				Namespace: apidefaults.Namespace,
			},
		}, nil
	case types.OpPut:
		resource, err := services.UnmarshalTunnelConnection(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return resource, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newReverseTunnelParser() *reverseTunnelParser {
	return &reverseTunnelParser{
		baseParser: newBaseParser(backend.NewKey(reverseTunnelsPrefix)),
	}
}

type reverseTunnelParser struct {
	baseParser
}

func (p *reverseTunnelParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindReverseTunnel, types.V2, 0)
	case types.OpPut:
		resource, err := services.UnmarshalReverseTunnel(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return resource, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newAppServerV3Parser() *appServerV3Parser {
	return &appServerV3Parser{
		baseParser: newBaseParser(backend.NewKey(appServersPrefix, apidefaults.Namespace)),
	}
}

type appServerV3Parser struct {
	baseParser
}

func (p *appServerV3Parser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		hostID, name, err := baseTwoKeys(event.Item.Key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &types.AppServerV3{
			Kind:    types.KindAppServer,
			Version: types.V3,
			Metadata: types.Metadata{
				Name:        name,
				Namespace:   apidefaults.Namespace,
				Description: hostID, // Pass host ID via description field for the cache.
			},
		}, nil
	case types.OpPut:
		return services.UnmarshalAppServer(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newSAMLIdPSessionParser(loadSecrets bool) *webSessionParser {
	return &webSessionParser{
		baseParser:  newBaseParser(backend.NewKey(samlIdPPrefix, sessionsPrefix)),
		loadSecrets: loadSecrets,
		hdr: types.ResourceHeader{
			Kind:    types.KindWebSession,
			SubKind: types.KindSAMLIdPSession,
			Version: types.V2,
		},
	}
}

func newSnowflakeSessionParser(loadSecrets bool) *webSessionParser {
	return &webSessionParser{
		baseParser:  newBaseParser(backend.NewKey(snowflakePrefix, sessionsPrefix)),
		loadSecrets: loadSecrets,
		hdr: types.ResourceHeader{
			Kind:    types.KindWebSession,
			SubKind: types.KindSnowflakeSession,
			Version: types.V2,
		},
	}
}

func newAppSessionParser(loadSecrets bool) *webSessionParser {
	return &webSessionParser{
		baseParser:  newBaseParser(backend.NewKey(appsPrefix, sessionsPrefix)),
		loadSecrets: loadSecrets,
		hdr: types.ResourceHeader{
			Kind:    types.KindWebSession,
			SubKind: types.KindAppSession,
			Version: types.V2,
		},
	}
}

func newWebSessionParser(loadSecrets bool) *webSessionParser {
	return &webSessionParser{
		baseParser:  newBaseParser(backend.NewKey(webPrefix, sessionsPrefix)),
		loadSecrets: loadSecrets,
		hdr: types.ResourceHeader{
			Kind:    types.KindWebSession,
			SubKind: types.KindWebSession,
			Version: types.V2,
		},
	}
}

type webSessionParser struct {
	baseParser
	loadSecrets bool
	hdr         types.ResourceHeader
}

func (p *webSessionParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeaderWithTemplate(event, p.hdr, 0)
	case types.OpPut:
		resource, err := services.UnmarshalWebSession(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !p.loadSecrets {
			return resource.WithoutSecrets(), nil
		}
		return resource, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newWebTokenParser() *webTokenParser {
	return &webTokenParser{
		baseParser: newBaseParser(backend.NewKey(webPrefix, tokensPrefix)),
	}
}

type webTokenParser struct {
	baseParser
}

func (p *webTokenParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindWebToken, types.V1, 0)
	case types.OpPut:
		resource, err := services.UnmarshalWebToken(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return resource, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newKubeServerParser() *kubeServerParser {
	return &kubeServerParser{
		baseParser: newBaseParser(backend.NewKey(kubeServersPrefix)),
	}
}

type kubeServerParser struct {
	baseParser
}

func (p *kubeServerParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		hostID, name, err := baseTwoKeys(event.Item.Key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &types.KubernetesServerV3{
			Kind:    types.KindKubeServer,
			Version: types.V3,
			Metadata: types.Metadata{
				Name:        name,
				Namespace:   apidefaults.Namespace,
				Description: hostID, // Pass host ID via description field for the cache.
			},
		}, nil
	case types.OpPut:
		return services.UnmarshalKubeServer(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newDatabaseServerParser() *databaseServerParser {
	return &databaseServerParser{
		baseParser: newBaseParser(backend.NewKey(dbServersPrefix, apidefaults.Namespace)),
	}
}

type databaseServerParser struct {
	baseParser
}

func (p *databaseServerParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		hostID, name, err := baseTwoKeys(event.Item.Key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &types.DatabaseServerV3{
			Kind:    types.KindDatabaseServer,
			Version: types.V3,
			Metadata: types.Metadata{
				Name:        name,
				Namespace:   apidefaults.Namespace,
				Description: hostID, // Pass host ID via description field for the cache.
			},
		}, nil
	case types.OpPut:
		return services.UnmarshalDatabaseServer(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newDatabaseServiceParser() *databaseServiceParser {
	return &databaseServiceParser{
		baseParser: newBaseParser(backend.NewKey(databaseServicePrefix)),
	}
}

type databaseServiceParser struct {
	baseParser
}

func (p *databaseServiceParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindDatabaseService, types.V1, 0)
	case types.OpPut:
		return services.UnmarshalDatabaseService(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newKubeClusterParser() *kubeClusterParser {
	return &kubeClusterParser{
		baseParser: newBaseParser(backend.NewKey(kubernetesPrefix)),
	}
}

type kubeClusterParser struct {
	baseParser
}

func (p *kubeClusterParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindKubernetesCluster, types.V3, 0)
	case types.OpPut:
		return services.UnmarshalKubeCluster(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newCrownJewelParser() *crownJewelParser {
	return &crownJewelParser{
		baseParser: newBaseParser(backend.NewKey(crownJewelsKey)),
	}
}

type crownJewelParser struct {
	baseParser
}

func (p *crownJewelParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindCrownJewel, types.V1, 0)
	case types.OpPut:
		r, err := services.UnmarshalCrownJewel(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(r), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newAppParser() *appParser {
	return &appParser{
		baseParser: newBaseParser(backend.NewKey(appPrefix)),
	}
}

type appParser struct {
	baseParser
}

func (p *appParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindApp, types.V3, 0)
	case types.OpPut:
		return services.UnmarshalApp(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newDatabaseParser() *databaseParser {
	return &databaseParser{
		baseParser: newBaseParser(backend.NewKey(databasesPrefix)),
	}
}

type databaseParser struct {
	baseParser
}

func (p *databaseParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindDatabase, types.V3, 0)
	case types.OpPut:
		return services.UnmarshalDatabase(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newDatabaseObjectParser() *databaseObjectParser {
	return &databaseObjectParser{
		baseParser: newBaseParser(backend.NewKey(databaseObjectPrefix)),
	}
}

type databaseObjectParser struct {
	baseParser
}

func (p *databaseObjectParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindDatabaseObject, types.V1, 0)
	case types.OpPut:
		//nolint:staticcheck // SA1019. Using this unmarshaler for json compatibility.
		resource, err := services.FastUnmarshalProtoResourceDeprecated[*dbobjectv1.DatabaseObject](event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(resource), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func parseServer(event backend.Event, kind string) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, kind, types.V2, 0)
	case types.OpPut:
		resource, err := services.UnmarshalServer(event.Item.Value,
			kind,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return resource, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newRemoteClusterParser() *remoteClusterParser {
	return &remoteClusterParser{
		matchPrefix: backend.NewKey(remoteClustersPrefix),
	}
}

type remoteClusterParser struct {
	matchPrefix backend.Key
}

func (p *remoteClusterParser) prefixes() []backend.Key {
	return []backend.Key{p.matchPrefix}
}

func (p *remoteClusterParser) match(key backend.Key) bool {
	return key.HasPrefix(p.matchPrefix)
}

func (p *remoteClusterParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindRemoteCluster, types.V3, 0)
	case types.OpPut:
		resource, err := services.UnmarshalRemoteCluster(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return resource, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newLockParser() *lockParser {
	return &lockParser{
		baseParser: newBaseParser(backend.NewKey(locksPrefix)),
	}
}

type lockParser struct {
	baseParser
}

func (p *lockParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindLock, types.V2, 0)
	case types.OpPut:
		return services.UnmarshalLock(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newNetworkRestrictionsParser() *networkRestrictionsParser {
	return &networkRestrictionsParser{
		matchPrefix: backend.NewKey(restrictionsPrefix, network),
	}
}

type networkRestrictionsParser struct {
	matchPrefix backend.Key
}

func (p *networkRestrictionsParser) prefixes() []backend.Key {
	return []backend.Key{p.matchPrefix}
}

func (p *networkRestrictionsParser) match(key backend.Key) bool {
	return key.HasPrefix(p.matchPrefix)
}

func (p *networkRestrictionsParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindNetworkRestrictions, types.V1, 0)
	case types.OpPut:
		resource, err := services.UnmarshalNetworkRestrictions(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return resource, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newWindowsDesktopServicesParser() *windowsDesktopServicesParser {
	return &windowsDesktopServicesParser{
		baseParser: newBaseParser(backend.NewKey(windowsDesktopServicesPrefix, "")),
	}
}

type windowsDesktopServicesParser struct {
	baseParser
}

func (p *windowsDesktopServicesParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindWindowsDesktopService, types.V3, 0)
	case types.OpPut:
		return services.UnmarshalWindowsDesktopService(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newWindowsDesktopsParser() *windowsDesktopsParser {
	return &windowsDesktopsParser{
		baseParser: newBaseParser(backend.NewKey(windowsDesktopsPrefix, "")),
	}
}

type windowsDesktopsParser struct {
	baseParser
}

func (p *windowsDesktopsParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		hostID, name, err := baseTwoKeys(event.Item.Key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &types.ResourceHeader{
			Kind:    types.KindWindowsDesktop,
			Version: types.V3,
			Metadata: types.Metadata{
				Name:        name,
				Namespace:   apidefaults.Namespace,
				Description: hostID, // pass ID via description field for the cache
			},
		}, nil
	case types.OpPut:
		return services.UnmarshalWindowsDesktop(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

type installerParser struct {
	baseParser
}

func newInstallerParser() *installerParser {
	return &installerParser{
		baseParser: newBaseParser(backend.NewKey(clusterConfigPrefix, scriptsPrefix, installerPrefix)),
	}
}

func (p *installerParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		h, err := resourceHeader(event, types.KindInstaller, types.V1, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return h, nil
	case types.OpPut:
		inst, err := services.UnmarshalInstaller(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return inst, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

type pluginParser struct {
	baseParser
	loadSecrets bool
}

func newPluginParser(loadSecrets bool) *pluginParser {
	return &pluginParser{
		baseParser:  newBaseParser(backend.NewKey(pluginsPrefix)),
		loadSecrets: loadSecrets,
	}
}

func (p *pluginParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		h, err := resourceHeader(event, types.KindPlugin, types.V1, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return h, nil
	case types.OpPut:
		plugin, err := services.UnmarshalPlugin(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return plugin, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newSAMLIDPServiceProviderParser() *samlIDPServiceProviderParser {
	return &samlIDPServiceProviderParser{
		baseParser: newBaseParser(backend.NewKey(samlIDPServiceProviderPrefix)),
	}
}

type samlIDPServiceProviderParser struct {
	baseParser
}

func (p *samlIDPServiceProviderParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindSAMLIdPServiceProvider, types.V1, 0)
	case types.OpPut:
		return services.UnmarshalSAMLIdPServiceProvider(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newUserGroupParser() *userGroupParser {
	return &userGroupParser{
		baseParser: newBaseParser(backend.NewKey(userGroupPrefix)),
	}
}

type userGroupParser struct {
	baseParser
}

func (p *userGroupParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindUserGroup, types.V1, 0)
	case types.OpPut:
		return services.UnmarshalUserGroup(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newOktaImportRuleParser() *oktaImportRuleParser {
	return &oktaImportRuleParser{
		baseParser: newBaseParser(backend.NewKey(oktaImportRulePrefix)),
	}
}

type oktaImportRuleParser struct {
	baseParser
}

func (p *oktaImportRuleParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindOktaImportRule, types.V1, 0)
	case types.OpPut:
		return services.UnmarshalOktaImportRule(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newOktaAssignmentParser() *oktaAssignmentParser {
	return &oktaAssignmentParser{
		baseParser: newBaseParser(backend.NewKey(oktaAssignmentPrefix)),
	}
}

type oktaAssignmentParser struct {
	baseParser
}

func (p *oktaAssignmentParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindOktaAssignment, types.V1, 0)
	case types.OpPut:
		return services.UnmarshalOktaAssignment(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newIntegrationParser() *integrationParser {
	return &integrationParser{
		baseParser: newBaseParser(backend.NewKey(integrationsPrefix)),
	}
}

type integrationParser struct {
	baseParser
}

func (p *integrationParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindIntegration, types.V1, 0)
	case types.OpPut:
		return services.UnmarshalIntegration(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newDiscoveryConfigParser() *discoveryConfigParser {
	return &discoveryConfigParser{
		baseParser: newBaseParser(backend.NewKey(discoveryConfigPrefix)),
	}
}

type discoveryConfigParser struct {
	baseParser
}

func (p *discoveryConfigParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindDiscoveryConfig, types.V1, 0)
	case types.OpPut:
		return services.UnmarshalDiscoveryConfig(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newHeadlessAuthenticationParser(m map[string]string) (*headlessAuthenticationParser, error) {
	var filter types.HeadlessAuthenticationFilter
	if err := filter.FromMap(m); err != nil {
		return nil, trace.Wrap(err)
	}

	return &headlessAuthenticationParser{
		baseParser: newBaseParser(backend.NewKey(headlessAuthenticationPrefix)),
		filter:     filter,
	}, nil
}

type headlessAuthenticationParser struct {
	baseParser
	filter types.HeadlessAuthenticationFilter
}

func (p *headlessAuthenticationParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindHeadlessAuthentication, types.V1, 0)
	case types.OpPut:
		ha, err := unmarshalHeadlessAuthentication(event.Item.Value)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !p.filter.Match(ha) {
			return nil, nil
		}
		return ha, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newAccessListParser() *accessListParser {
	return &accessListParser{
		baseParser: newBaseParser(backend.ExactKey(accessListPrefix)),
	}
}

type accessListParser struct {
	baseParser
}

func (p *accessListParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindAccessList, types.V1, 0)
	case types.OpPut:
		return services.UnmarshalAccessList(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newAuditQueryParser() *auditQueryParser {
	return &auditQueryParser{
		baseParser: newBaseParser(backend.NewKey(AuditQueryPrefix)),
	}
}

type auditQueryParser struct {
	baseParser
}

func (p *auditQueryParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindAuditQuery, types.V1, 0)
	case types.OpPut:
		return services.UnmarshalAuditQuery(event.Item.Value,
			services.WithExpires(event.Item.Expires),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newSecurityReportParser() *securityReportParser {
	return &securityReportParser{
		baseParser: newBaseParser(backend.NewKey(SecurityReportPrefix)),
	}
}

type securityReportParser struct {
	baseParser
}

func (p *securityReportParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindSecurityReport, types.V1, 0)
	case types.OpPut:
		return services.UnmarshalSecurityReport(event.Item.Value,
			services.WithExpires(event.Item.Expires),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newSecurityReportStateParser() *securityReportStateParser {
	return &securityReportStateParser{
		baseParser: newBaseParser(backend.NewKey(SecurityReportStatePrefix)),
	}
}

type securityReportStateParser struct {
	baseParser
}

func (p *securityReportStateParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindSecurityReportState, types.V1, 0)
	case types.OpPut:
		return services.UnmarshalSecurityReportState(event.Item.Value,
			services.WithExpires(event.Item.Expires),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newUserLoginStateParser() *userLoginStateParser {
	return &userLoginStateParser{
		baseParser: newBaseParser(backend.NewKey(userLoginStatePrefix)),
	}
}

type userLoginStateParser struct {
	baseParser
}

func (p *userLoginStateParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindUserLoginState, types.V1, 0)
	case types.OpPut:
		return services.UnmarshalUserLoginState(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newAccessListMemberParser() *accessListMemberParser {
	return &accessListMemberParser{
		baseParser: newBaseParser(backend.ExactKey(accessListMemberPrefix)),
	}
}

type accessListMemberParser struct {
	baseParser
}

func (p *accessListMemberParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		accessList, name, err := baseTwoKeys(event.Item.Key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &types.ResourceHeader{
			Kind:    types.KindAccessListMember,
			Version: types.V1,
			Metadata: types.Metadata{
				Name:        name,
				Namespace:   apidefaults.Namespace,
				Description: accessList, // pass access list description field for the cache
			},
		}, nil
	case types.OpPut:
		return services.UnmarshalAccessListMember(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newAccessListReviewParser() *accessListReviewParser {
	return &accessListReviewParser{
		baseParser: newBaseParser(backend.ExactKey(accessListReviewPrefix)),
	}
}

type accessListReviewParser struct {
	baseParser
}

func (p *accessListReviewParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		accessList, name, err := baseTwoKeys(event.Item.Key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &types.ResourceHeader{
			Kind:    types.KindAccessListReview,
			Version: types.V1,
			Metadata: types.Metadata{
				Name:        name,
				Namespace:   apidefaults.Namespace,
				Description: accessList, // pass access list description field for the cache
			},
		}, nil
	case types.OpPut:
		return services.UnmarshalAccessListReview(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newKubeWaitingContainerParser() *kubeWaitingContainerParser {
	return &kubeWaitingContainerParser{
		baseParser: newBaseParser(backend.NewKey(kubeWaitingContPrefix)),
	}
}

type kubeWaitingContainerParser struct {
	baseParser
}

func (p *kubeWaitingContainerParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		parts := event.Item.Key.Components()
		if len(parts) != 6 {
			return nil, trace.BadParameter("malformed key for %s event: %s", types.KindKubeWaitingContainer, event.Item.Key)
		}

		resource, err := kubewaitingcontainer.NewKubeWaitingContainer(
			string(parts[5]),
			&kubewaitingcontainerpb.KubernetesWaitingContainerSpec{
				Username:      string(parts[1]),
				Cluster:       string(parts[2]),
				Namespace:     string(parts[3]),
				PodName:       string(parts[4]),
				ContainerName: string(parts[5]),
				Patch:         []byte("{}"),                       // default to empty patch. It doesn't matter for delete ops.
				PatchType:     kubewaitingcontainer.JSONPatchType, // default to JSON patch. It doesn't matter for delete ops.
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(resource), nil
	case types.OpPut:
		resource, err := services.UnmarshalKubeWaitingContainer(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(resource), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newAccessMonitoringRuleParser() *AccessMonitoringRuleParser {
	return &AccessMonitoringRuleParser{
		baseParser: newBaseParser(backend.ExactKey(accessMonitoringRulesPrefix)),
	}
}

type AccessMonitoringRuleParser struct {
	baseParser
}

func (p *AccessMonitoringRuleParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindAccessMonitoringRule, types.V1, 0)
	case types.OpPut:
		r, err := services.UnmarshalAccessMonitoringRule(event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(r), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newUserNotificationParser() *userNotificationParser {
	return &userNotificationParser{
		baseParser: newBaseParser(backend.NewKey(notificationsUserSpecificPrefix)),
	}
}

type userNotificationParser struct {
	baseParser
}

func (p *userNotificationParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		parts := event.Item.Key.Components()
		if len(parts) != 4 {
			return nil, trace.BadParameter("malformed key for %s event: %s", types.KindNotification, event.Item.Key)
		}

		notification := &notificationsv1.Notification{
			Kind:    types.KindNotification,
			Version: types.V1,
			Spec: &notificationsv1.NotificationSpec{
				Username: string(parts[2]),
			},
			Metadata: &headerv1.Metadata{
				Name: string(parts[3]),
			},
		}

		return types.Resource153ToLegacy(notification), nil
	case types.OpPut:
		notification, err := services.UnmarshalNotification(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(notification), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newGlobalNotificationParser() *globalNotificationParser {
	return &globalNotificationParser{
		baseParser: newBaseParser(backend.NewKey(notificationsGlobalPrefix)),
	}
}

type globalNotificationParser struct {
	baseParser
}

func (p *globalNotificationParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		parts := event.Item.Key.Components()
		if len(parts) != 3 {
			return nil, trace.BadParameter("malformed key for %s event: %s", types.KindGlobalNotification, event.Item.Key)
		}

		globalNotification := &notificationsv1.GlobalNotification{
			Kind:    types.KindGlobalNotification,
			Version: types.V1,
			Spec: &notificationsv1.GlobalNotificationSpec{
				Notification: &notificationsv1.Notification{
					Spec: &notificationsv1.NotificationSpec{},
				},
			},
			Metadata: &headerv1.Metadata{
				Name: string(parts[2]),
			},
		}

		return types.Resource153ToLegacy(globalNotification), nil
	case types.OpPut:
		globalNotification, err := services.UnmarshalGlobalNotification(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(globalNotification), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newBotInstanceParser() *botInstanceParser {
	return &botInstanceParser{
		baseParser: newBaseParser(backend.NewKey(botInstancePrefix)),
	}
}

type botInstanceParser struct {
	baseParser
}

func (p *botInstanceParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		parts := event.Item.Key.Components()
		if len(parts) != 3 {
			return nil, trace.BadParameter("malformed key for %s event: %s", types.KindBotInstance, event.Item.Key)
		}

		botInstance := &machineidv1.BotInstance{
			Kind:    types.KindBotInstance,
			Version: types.V1,
			Spec: &machineidv1.BotInstanceSpec{
				BotName:    string(parts[1]),
				InstanceId: string(parts[2]),
			},
			Metadata: &headerv1.Metadata{
				Name: string(parts[2]),
			},
		}

		return types.Resource153ToLegacy(botInstance), nil
	case types.OpPut:
		botInstance, err := services.UnmarshalBotInstance(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(botInstance), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newInstanceParser() *instanceParser {
	return &instanceParser{
		baseParser: newBaseParser(backend.NewKey(instancePrefix)),
	}
}

type instanceParser struct {
	baseParser
}

func (p *instanceParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindInstance, types.V1, 0)
	case types.OpPut:
		instance, err := generic.FastUnmarshal[*types.InstanceV1](event.Item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return instance, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newStaticHostUserParser() *staticHostUserParser {
	return &staticHostUserParser{
		baseParser: newBaseParser(backend.NewKey(staticHostUserPrefix)),
	}
}

type staticHostUserParser struct {
	baseParser
}

func (p *staticHostUserParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindStaticHostUser, types.V1, 0)
	case types.OpPut:
		resource, err := services.UnmarshalProtoResource[*userprovisioningpb.StaticHostUser](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(resource), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func resourceHeader(event backend.Event, kind, version string, offset int) (types.Resource, error) {
	name, err := base(event.Item.Key, offset)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &types.ResourceHeader{
		Kind:    kind,
		Version: version,
		Metadata: types.Metadata{
			Name:      name,
			Namespace: apidefaults.Namespace,
		},
	}, nil
}

func resourceHeaderWithTemplate(event backend.Event, hdr types.ResourceHeader, offset int) (types.Resource, error) {
	name, err := base(event.Item.Key, offset)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &types.ResourceHeader{
		Kind:    hdr.Kind,
		SubKind: hdr.SubKind,
		Version: hdr.Version,
		Metadata: types.Metadata{
			Name:      name,
			Namespace: apidefaults.Namespace,
		},
	}, nil
}

// WaitForEvent waits for the event matched by the specified event matcher in the given watcher.
func WaitForEvent(ctx context.Context, watcher types.Watcher, m EventMatcher, clock clockwork.Clock) (types.Resource, error) {
	tick := clock.NewTicker(defaults.WebHeadersTimeout)
	defer tick.Stop()

	select {
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return nil, trace.BadParameter("expected init event, got %v instead", event.Type)
		}
	case <-watcher.Done():
		// Watcher closed, probably due to a network error.
		return nil, trace.ConnectionProblem(watcher.Error(), "watcher is closed")
	case <-tick.Chan():
		return nil, trace.LimitExceeded("timed out waiting for initialize event")
	}

	for {
		select {
		case event := <-watcher.Events():
			res, err := m.Match(event)
			if err == nil {
				return res, nil
			}
			if !trace.IsCompareFailed(err) {
				logrus.WithError(err).Debug("Failed to match event.")
			}
		case <-watcher.Done():
			// Watcher closed, probably due to a network error.
			return nil, trace.ConnectionProblem(watcher.Error(), "watcher is closed")
		case <-tick.Chan():
			return nil, trace.LimitExceeded("timed out waiting for event")
		}
	}
}

func newDeviceParser() *deviceParser {
	return &deviceParser{
		baseParser: newBaseParser(backend.NewKey(devicetrust.DevicesIDPrefix...)),
	}
}

type deviceParser struct {
	baseParser
}

func (p *deviceParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		name, err := base(event.Item.Key, 0 /* offset */)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		device := &types.DeviceV1{
			ResourceHeader: types.ResourceHeader{
				Kind:    types.KindDevice,
				Version: types.V1,
				Metadata: types.Metadata{
					Name: name,
				},
			},
		}
		return device, nil
	case types.OpPut:
		device, err := services.UnmarshalDeviceFromBackendItem(event.Item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// [devicepb.Device] doesn't satisfy types.Resource interface or Resource153 interface,
		// so we need to convert it to types.DeviceV1 so it satisfy types.Resource interface.
		dev := types.DeviceToResource(device)
		// Set the expires and revision fields.
		dev.SetExpiry(event.Item.Expires)
		dev.SetRevision(event.Item.Revision)
		return dev, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newAccessGraphSecretPrivateKeyParser() *accessGraphSecretPrivateKeyParser {
	return &accessGraphSecretPrivateKeyParser{
		baseParser: newBaseParser(backend.NewKey(privateKeysPrefix)),
	}
}

type accessGraphSecretPrivateKeyParser struct {
	baseParser
}

func (p *accessGraphSecretPrivateKeyParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		deviceID, name, err := baseTwoKeys(event.Item.Key)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		privateKey := &accessgraphsecretsv1pb.PrivateKey{
			Kind:    types.KindAccessGraphSecretPrivateKey,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: name,
			},
			Spec: &accessgraphsecretsv1pb.PrivateKeySpec{
				DeviceId: deviceID,
			},
		}

		return types.Resource153ToLegacy(privateKey), nil
	case types.OpPut:
		pk, err := services.UnmarshalAccessGraphPrivateKey(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(pk), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newAccessGraphSecretAuthorizedKeyParser() *accessGraphSecretAuthorizedKeyParser {
	return &accessGraphSecretAuthorizedKeyParser{
		baseParser: newBaseParser(backend.NewKey(authorizedKeysPrefix)),
	}
}

type accessGraphSecretAuthorizedKeyParser struct {
	baseParser
}

func (p *accessGraphSecretAuthorizedKeyParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		hostID, name, err := baseTwoKeys(event.Item.Key)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		authorizedKey := &accessgraphsecretsv1pb.AuthorizedKey{
			Kind:    types.KindAccessGraphSecretAuthorizedKey,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: name,
			},
			Spec: &accessgraphsecretsv1pb.AuthorizedKeySpec{
				HostId: hostID,
			},
		}

		return types.Resource153ToLegacy(authorizedKey), nil
	case types.OpPut:
		ak, err := services.UnmarshalAccessGraphAuthorizedKey(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(ak), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

// Match matches the specified resource event by applying itself
func (r EventMatcherFunc) Match(event types.Event) (types.Resource, error) {
	return r(event)
}

// EventMatcherFunc matches the specified resource event.
// Implements EventMatcher
type EventMatcherFunc func(types.Event) (types.Resource, error)

// EventMatcher matches a specific resource event
type EventMatcher interface {
	// Match matches the specified event.
	// Returns the matched resource if successful.
	// Returns trace.CompareFailedError for no match.
	Match(types.Event) (types.Resource, error)
}

// base returns the key component that is offset
// components before the last component.
func base(key backend.Key, offset int) (string, error) {
	parts := key.Components()
	if len(parts) < offset+1 {
		return "", trace.NotFound("failed parsing %v", string(key))
	}
	return string(parts[len(parts)-offset-1]), nil
}

// baseTwoKeys returns two last keys
func baseTwoKeys(key backend.Key) (string, string, error) {
	parts := key.Components()
	if len(parts) < 2 {
		return "", "", trace.NotFound("failed parsing %v", string(key))
	}
	return string(parts[len(parts)-2]), string(parts[len(parts)-1]), nil
}

func newAccessGraphSettingsParser() *accessGraphSettingsParser {
	return &accessGraphSettingsParser{
		baseParser: newBaseParser(backend.NewKey(clusterConfigPrefix, accessGraphSettingsPrefix)),
	}
}

type accessGraphSettingsParser struct {
	baseParser
}

func (p *accessGraphSettingsParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		h, err := resourceHeader(event, types.KindAccessGraphSettings, types.V1, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		h.SetName(types.MetaNameAccessGraphSettings)
		return h, nil
	case types.OpPut:
		settings, err := services.UnmarshalAccessGraphSettings(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return types.Resource153ToLegacy(settings), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newSPIFFEFederationParser() *spiffeFederationParser {
	return &spiffeFederationParser{
		baseParser: newBaseParser(backend.NewKey(spiffeFederationPrefix)),
	}
}

type spiffeFederationParser struct {
	baseParser
}

func (p *spiffeFederationParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindSPIFFEFederation, types.V1, 0)
	case types.OpPut:
		federation, err := services.UnmarshalSPIFFEFederation(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision))
		if err != nil {
			return nil, trace.Wrap(err, "unmarshalling resource from event")
		}
		return types.Resource153ToLegacy(federation), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
