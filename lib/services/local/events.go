/*
Copyright 2018-2019 Gravitational, Inc.

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

package local

import (
	"bytes"
	"context"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// EventsService implements service to watch for events
type EventsService struct {
	*logrus.Entry
	backend backend.Backend
}

// NewEventsService returns new events service instance
func NewEventsService(b backend.Backend) *EventsService {
	return &EventsService{
		Entry:   logrus.WithFields(logrus.Fields{trace.Component: "Events"}),
		backend: b,
	}
}

// NewWatcher returns a new event watcher
func (e *EventsService) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	if len(watch.Kinds) == 0 {
		return nil, trace.BadParameter("global watches are not supported yet")
	}
	var parsers []resourceParser
	var prefixes [][]byte
	for _, kind := range watch.Kinds {
		if kind.Name != "" && kind.Kind != types.KindNamespace {
			return nil, trace.BadParameter("watch with Name is only supported for Namespace resource")
		}
		var parser resourceParser
		switch kind.Kind {
		case types.KindCertAuthority:
			parser = newCertAuthorityParser(kind.LoadSecrets)
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
				return nil, trace.Wrap(err)
			}
			parser = p
		case types.KindAppServer:
			switch kind.Version {
			case types.V2: // DELETE IN 9.0.
				parser = newAppServerV2Parser()
			default:
				parser = newAppServerV3Parser()
			}
		case types.KindWebSession:
			switch kind.SubKind {
			case types.KindSnowflakeSession:
				parser = newSnowflakeSessionParser()
			case types.KindAppSession:
				parser = newAppSessionParser()
			case types.KindWebSession:
				parser = newWebSessionParser()
			default:
				return nil, trace.BadParameter("watcher on object subkind %q is not supported", kind.SubKind)
			}
		case types.KindWebToken:
			parser = newWebTokenParser()
		case types.KindRemoteCluster:
			parser = newRemoteClusterParser()
		case types.KindKubeService:
			parser = newKubeServiceParser()
		case types.KindDatabaseServer:
			parser = newDatabaseServerParser()
		case types.KindDatabase:
			parser = newDatabaseParser()
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
		default:
			return nil, trace.BadParameter("watcher on object kind %q is not supported", kind.Kind)
		}
		prefixes = append(prefixes, parser.prefixes()...)
		parsers = append(parsers, parser)
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
	return newWatcher(w, e.Entry, parsers), nil
}

func newWatcher(backendWatcher backend.Watcher, l *logrus.Entry, parsers []resourceParser) *watcher {
	w := &watcher{
		backendWatcher: backendWatcher,
		Entry:          l,
		parsers:        parsers,
		eventsC:        make(chan types.Event),
	}
	go w.forwardEvents()
	return w
}

type watcher struct {
	*logrus.Entry
	parsers        []resourceParser
	backendWatcher backend.Watcher
	eventsC        chan types.Event
}

func (w *watcher) Error() error {
	return nil
}

func (w *watcher) parseEvent(e backend.Event) ([]types.Event, []error) {
	if e.Type == types.OpInit {
		return []types.Event{{Type: e.Type}}, nil
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
	match(key []byte) bool
	// prefixes returns prefixes to watch
	prefixes() [][]byte
}

// baseParser is a partial implementation of resourceParser for the most common
// resource types (stored under a static prefix).
type baseParser struct {
	matchPrefixes [][]byte
}

func newBaseParser(prefixes ...[]byte) baseParser {
	return baseParser{matchPrefixes: prefixes}
}

func (p baseParser) prefixes() [][]byte {
	return p.matchPrefixes
}

func (p baseParser) match(key []byte) bool {
	for _, prefix := range p.matchPrefixes {
		if bytes.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func newCertAuthorityParser(loadSecrets bool) *certAuthorityParser {
	return &certAuthorityParser{
		loadSecrets: loadSecrets,
		baseParser:  newBaseParser(backend.Key(authoritiesPrefix)),
	}
}

type certAuthorityParser struct {
	baseParser
	loadSecrets bool
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
			services.WithResourceID(event.Item.ID), services.WithExpires(event.Item.Expires))
		if err != nil {
			return nil, trace.Wrap(err)
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
		baseParser: newBaseParser(backend.Key(tokensPrefix)),
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
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
		baseParser: newBaseParser(backend.Key(clusterConfigPrefix, staticTokensPrefix)),
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
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
		baseParser: newBaseParser(backend.Key(clusterConfigPrefix, auditPrefix)),
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
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
		baseParser: newBaseParser(backend.Key(clusterConfigPrefix, networkingPrefix)),
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
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
		baseParser: newBaseParser(backend.Key(authPrefix, preferencePrefix, generalPrefix)),
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
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
		baseParser: newBaseParser(backend.Key(clusterConfigPrefix, sessionRecordingPrefix)),
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
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
		baseParser: newBaseParser(backend.Key(clusterConfigPrefix, namePrefix)),
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
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
	prefix := backend.Key(namespacesPrefix)
	if name != "" {
		prefix = backend.Key(namespacesPrefix, name, paramsPrefix)
	}
	return &namespaceParser{
		baseParser: newBaseParser(prefix),
	}
}

type namespaceParser struct {
	baseParser
}

func (p *namespaceParser) match(key []byte) bool {
	// namespaces are stored under key '/namespaces/<namespace-name>/params'
	// and this code matches similar pattern
	return p.baseParser.match(key) &&
		bytes.HasSuffix(key, []byte(paramsPrefix)) &&
		bytes.Count(key, []byte{backend.Separator}) == 3
}

func (p *namespaceParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindNamespace, types.V2, 1)
	case types.OpPut:
		namespace, err := services.UnmarshalNamespace(event.Item.Value,
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
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
		baseParser: newBaseParser(backend.Key(rolesPrefix)),
	}
}

type roleParser struct {
	baseParser
}

func (p *roleParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindRole, types.V3, 1)
	case types.OpPut:
		resource, err := services.UnmarshalRole(event.Item.Value,
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
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
		matchPrefix: backend.Key(accessRequestsPrefix),
		matchSuffix: backend.Key(paramsPrefix),
	}, nil
}

type accessRequestParser struct {
	filter      types.AccessRequestFilter
	matchPrefix []byte
	matchSuffix []byte
}

func (p *accessRequestParser) prefixes() [][]byte {
	return [][]byte{p.matchPrefix}
}

func (p *accessRequestParser) match(key []byte) bool {
	if !bytes.HasPrefix(key, p.matchPrefix) {
		return false
	}
	if !bytes.HasSuffix(key, p.matchSuffix) {
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
		baseParser: newBaseParser(backend.Key(webPrefix, usersPrefix)),
	}
}

type userParser struct {
	baseParser
}

func (p *userParser) match(key []byte) bool {
	// users are stored under key '/web/users/<username>/params'
	// and this code matches similar pattern
	return p.baseParser.match(key) &&
		bytes.HasSuffix(key, []byte(paramsPrefix)) &&
		bytes.Count(key, []byte{backend.Separator}) == 4
}

func (p *userParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindUser, types.V2, 1)
	case types.OpPut:
		resource, err := services.UnmarshalUser(event.Item.Value,
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
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
		baseParser: newBaseParser(backend.Key(nodesPrefix, apidefaults.Namespace)),
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
		baseParser: newBaseParser(backend.Key(proxiesPrefix)),
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
		baseParser: newBaseParser(backend.Key(authServersPrefix)),
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
		baseParser: newBaseParser(backend.Key(tunnelConnectionsPrefix)),
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
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
		baseParser: newBaseParser(backend.Key(reverseTunnelsPrefix)),
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
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
		baseParser: newBaseParser(backend.Key(appServersPrefix, apidefaults.Namespace)),
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newAppServerV2Parser() *appServerV2Parser {
	return &appServerV2Parser{
		baseParser: newBaseParser(backend.Key(appsPrefix, serversPrefix, apidefaults.Namespace)),
	}
}

// DELETE IN 9.0. Deprecated, replaced by applicationServerParser.
type appServerV2Parser struct {
	baseParser
}

func (p *appServerV2Parser) parse(event backend.Event) (types.Resource, error) {
	return parseServer(event, types.KindAppServer)
}

func newSnowflakeSessionParser() *webSessionParser {
	return &webSessionParser{
		baseParser: newBaseParser(backend.Key(snowflakePrefix, sessionsPrefix)),
		hdr: types.ResourceHeader{
			Kind:    types.KindWebSession,
			SubKind: types.KindSnowflakeSession,
			Version: types.V2,
		},
	}
}

func newAppSessionParser() *webSessionParser {
	return &webSessionParser{
		baseParser: newBaseParser(backend.Key(appsPrefix, sessionsPrefix)),
		hdr: types.ResourceHeader{
			Kind:    types.KindWebSession,
			SubKind: types.KindAppSession,
			Version: types.V2,
		},
	}
}

func newWebSessionParser() *webSessionParser {
	return &webSessionParser{
		baseParser: newBaseParser(backend.Key(webPrefix, sessionsPrefix)),
		hdr: types.ResourceHeader{
			Kind:    types.KindWebSession,
			SubKind: types.KindWebSession,
			Version: types.V2,
		},
	}
}

type webSessionParser struct {
	baseParser
	hdr types.ResourceHeader
}

func (p *webSessionParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeaderWithTemplate(event, p.hdr, 0)
	case types.OpPut:
		resource, err := services.UnmarshalWebSession(event.Item.Value,
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return resource, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newWebTokenParser() *webTokenParser {
	return &webTokenParser{
		baseParser: newBaseParser(backend.Key(webPrefix, tokensPrefix)),
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return resource, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newKubeServiceParser() *kubeServiceParser {
	return &kubeServiceParser{
		baseParser: newBaseParser(backend.Key(kubeServicesPrefix)),
	}
}

type kubeServiceParser struct {
	baseParser
}

func (p *kubeServiceParser) parse(event backend.Event) (types.Resource, error) {
	return parseServer(event, types.KindKubeService)
}

func newDatabaseServerParser() *databaseServerParser {
	return &databaseServerParser{
		baseParser: newBaseParser(backend.Key(dbServersPrefix, apidefaults.Namespace)),
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newAppParser() *appParser {
	return &appParser{
		baseParser: newBaseParser(backend.Key(appPrefix)),
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newDatabaseParser() *databaseParser {
	return &databaseParser{
		baseParser: newBaseParser(backend.Key(databasesPrefix)),
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
		)
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
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
		matchPrefix: backend.Key(remoteClustersPrefix),
	}
}

type remoteClusterParser struct {
	matchPrefix []byte
}

func (p *remoteClusterParser) prefixes() [][]byte {
	return [][]byte{p.matchPrefix}
}

func (p *remoteClusterParser) match(key []byte) bool {
	return bytes.HasPrefix(key, p.matchPrefix)
}

func (p *remoteClusterParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindRemoteCluster, types.V3, 0)
	case types.OpPut:
		resource, err := services.UnmarshalRemoteCluster(event.Item.Value,
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
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
		baseParser: newBaseParser(backend.Key(locksPrefix)),
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newNetworkRestrictionsParser() *networkRestrictionsParser {
	return &networkRestrictionsParser{
		matchPrefix: backend.Key(restrictionsPrefix, network),
	}
}

type networkRestrictionsParser struct {
	matchPrefix []byte
}

func (p *networkRestrictionsParser) prefixes() [][]byte {
	return [][]byte{p.matchPrefix}
}

func (p *networkRestrictionsParser) match(key []byte) bool {
	return bytes.HasPrefix(key, p.matchPrefix)
}

func (p *networkRestrictionsParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		return resourceHeader(event, types.KindNetworkRestrictions, types.V1, 0)
	case types.OpPut:
		resource, err := services.UnmarshalNetworkRestrictions(event.Item.Value,
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
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
		baseParser: newBaseParser(backend.Key(windowsDesktopServicesPrefix)),
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newWindowsDesktopsParser() *windowsDesktopsParser {
	return &windowsDesktopsParser{
		baseParser: newBaseParser(backend.Key(windowsDesktopsPrefix)),
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
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
		)
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
			Name:      string(name),
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
			Name:      string(name),
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

// base returns last element delimited by separator, index is
// is an index of the key part to get counting from the end
func base(key []byte, offset int) ([]byte, error) {
	parts := bytes.Split(key, []byte{backend.Separator})
	if len(parts) < offset+1 {
		return nil, trace.NotFound("failed parsing %v", string(key))
	}
	return parts[len(parts)-offset-1], nil
}

// baseTwoKeys returns two last keys
func baseTwoKeys(key []byte) (string, string, error) {
	parts := bytes.Split(key, []byte{backend.Separator})
	if len(parts) < 2 {
		return "", "", trace.NotFound("failed parsing %v", string(key))
	}
	return string(parts[len(parts)-2]), string(parts[len(parts)-1]), nil
}
