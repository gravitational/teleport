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
	backend          backend.Backend
	getClusterConfig getClusterConfigFunc
}

// NewEventsService returns new events service instance
func NewEventsService(b backend.Backend, getClusterConfig getClusterConfigFunc) *EventsService {
	return &EventsService{
		Entry:            logrus.WithFields(logrus.Fields{trace.Component: "Events"}),
		backend:          b,
		getClusterConfig: getClusterConfig,
	}
}

// NewWatcher returns a new event watcher
func (e *EventsService) NewWatcher(ctx context.Context, watch services.Watch) (services.Watcher, error) {
	if len(watch.Kinds) == 0 {
		return nil, trace.BadParameter("global watches are not supported yet")
	}
	var parsers []resourceParser
	var prefixes [][]byte
	for _, kind := range watch.Kinds {
		if kind.Name != "" && kind.Kind != services.KindNamespace {
			return nil, trace.BadParameter("watch with Name is only supported for Namespace resource")
		}
		var parser resourceParser
		switch kind.Kind {
		case services.KindCertAuthority:
			parser = newCertAuthorityParser(kind.LoadSecrets)
		case services.KindToken:
			parser = newProvisionTokenParser()
		case services.KindStaticTokens:
			parser = newStaticTokensParser()
		case services.KindClusterConfig:
			parser = newClusterConfigParser(e.getClusterConfig)
		case types.KindClusterNetworkingConfig:
			parser = newClusterNetworkingConfigParser()
		case types.KindClusterAuthPreference:
			parser = newAuthPreferenceParser()
		case types.KindSessionRecordingConfig:
			parser = newSessionRecordingConfigParser()
		case services.KindClusterName:
			parser = newClusterNameParser()
		case services.KindNamespace:
			parser = newNamespaceParser(kind.Name)
		case services.KindRole:
			parser = newRoleParser()
		case services.KindUser:
			parser = newUserParser()
		case services.KindNode:
			parser = newNodeParser()
		case services.KindProxy:
			parser = newProxyParser()
		case services.KindAuthServer:
			parser = newAuthServerParser()
		case services.KindTunnelConnection:
			parser = newTunnelConnectionParser()
		case services.KindReverseTunnel:
			parser = newReverseTunnelParser()
		case services.KindAccessRequest:
			p, err := newAccessRequestParser(kind.Filter)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			parser = p
		case services.KindAppServer:
			parser = newAppServerParser()
		case services.KindWebSession:
			switch kind.SubKind {
			case services.KindAppSession:
				parser = newAppSessionParser()
			case services.KindWebSession:
				parser = newWebSessionParser()
			default:
				return nil, trace.BadParameter("watcher on object subkind %q is not supported", kind.SubKind)
			}
		case services.KindWebToken:
			parser = newWebTokenParser()
		case services.KindRemoteCluster:
			parser = newRemoteClusterParser()
		case services.KindKubeService:
			parser = newKubeServiceParser()
		case types.KindDatabaseServer:
			parser = newDatabaseServerParser()
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
		eventsC:        make(chan services.Event),
	}
	go w.forwardEvents()
	return w
}

type watcher struct {
	*logrus.Entry
	parsers        []resourceParser
	backendWatcher backend.Watcher
	eventsC        chan services.Event
}

func (w *watcher) Error() error {
	return nil
}

func (w *watcher) parseEvent(e backend.Event) ([]services.Event, []error) {
	if e.Type == backend.OpInit {
		return []services.Event{{Type: e.Type}}, nil
	}
	events := []services.Event{}
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
			events = append(events, services.Event{Type: e.Type, Resource: resource})
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
func (w *watcher) Events() <-chan services.Event {
	return w.eventsC
}

// Done returns the channel signalling the closure
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
	parse(event backend.Event) (services.Resource, error)
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

func (p *certAuthorityParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		caType, name, err := baseTwoKeys(event.Item.Key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &services.ResourceHeader{
			Kind:    services.KindCertAuthority,
			SubKind: caType,
			Version: services.V2,
			Metadata: services.Metadata{
				Name:      name,
				Namespace: defaults.Namespace,
			},
		}, nil
	case backend.OpPut:
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

func (p *provisionTokenParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		return resourceHeader(event, services.KindToken, services.V2, 0)
	case backend.OpPut:
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

func (p *staticTokensParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		h, err := resourceHeader(event, services.KindStaticTokens, services.V2, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		h.SetName(services.MetaNameStaticTokens)
		return h, nil
	case backend.OpPut:
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

func newClusterConfigParser(getClusterConfig getClusterConfigFunc) *clusterConfigParser {
	prefixes := [][]byte{
		backend.Key(clusterConfigPrefix, generalPrefix),
		backend.Key(clusterConfigPrefix, networkingPrefix),
		backend.Key(clusterConfigPrefix, sessionRecordingPrefix),
	}
	return &clusterConfigParser{
		baseParser:       newBaseParser(prefixes...),
		getClusterConfig: getClusterConfig,
	}
}

type clusterConfigParser struct {
	baseParser
	getClusterConfig getClusterConfigFunc
}

func (p *clusterConfigParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		if !bytes.HasPrefix(event.Item.Key, backend.Key(clusterConfigPrefix, generalPrefix)) {
			return nil, nil
		}
		h, err := resourceHeader(event, services.KindClusterConfig, services.V3, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		h.SetName(services.MetaNameClusterConfig)
		return h, nil
	case backend.OpPut:
		// To ensure backward compatibility, do not use the ClusterConfig
		// resource passed with the event but perform a separate get from the
		// backend. The resource fetched in this way is populated with all the
		// fields expected by legacy event consumers.  DELETE IN 8.0.0
		clusterConfig, err := p.getClusterConfig()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return clusterConfig, nil
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

func (p *clusterNetworkingConfigParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		h, err := resourceHeader(event, types.KindClusterNetworkingConfig, services.V2, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		h.SetName(types.MetaNameClusterNetworkingConfig)
		return h, nil
	case backend.OpPut:
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

func (p *authPreferenceParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		h, err := resourceHeader(event, services.KindClusterAuthPreference, services.V2, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		h.SetName(services.MetaNameClusterAuthPreference)
		return h, nil
	case backend.OpPut:
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

func (p *sessionRecordingConfigParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		h, err := resourceHeader(event, types.KindSessionRecordingConfig, services.V2, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		h.SetName(types.MetaNameSessionRecordingConfig)
		return h, nil
	case backend.OpPut:
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

func (p *clusterNameParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		h, err := resourceHeader(event, services.KindClusterName, services.V2, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		h.SetName(services.MetaNameClusterName)
		return h, nil
	case backend.OpPut:
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

func (p *namespaceParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		return resourceHeader(event, services.KindNamespace, services.V2, 1)
	case backend.OpPut:
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

func (p *roleParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		return resourceHeader(event, services.KindRole, services.V3, 1)
	case backend.OpPut:
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
	var filter services.AccessRequestFilter
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
	filter      services.AccessRequestFilter
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

func (p *accessRequestParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		return resourceHeader(event, services.KindAccessRequest, services.V3, 1)
	case backend.OpPut:
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

func (p *userParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		return resourceHeader(event, services.KindUser, services.V2, 1)
	case backend.OpPut:
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
		baseParser: newBaseParser(backend.Key(nodesPrefix, defaults.Namespace)),
	}
}

type nodeParser struct {
	baseParser
}

func (p *nodeParser) parse(event backend.Event) (services.Resource, error) {
	return parseServer(event, services.KindNode)
}

func newProxyParser() *proxyParser {
	return &proxyParser{
		baseParser: newBaseParser(backend.Key(proxiesPrefix)),
	}
}

type proxyParser struct {
	baseParser
}

func (p *proxyParser) parse(event backend.Event) (services.Resource, error) {
	return parseServer(event, services.KindProxy)
}

func newAuthServerParser() *authServerParser {
	return &authServerParser{
		baseParser: newBaseParser(backend.Key(authServersPrefix)),
	}
}

type authServerParser struct {
	baseParser
}

func (p *authServerParser) parse(event backend.Event) (services.Resource, error) {
	return parseServer(event, services.KindAuthServer)
}

func newTunnelConnectionParser() *tunnelConnectionParser {
	return &tunnelConnectionParser{
		baseParser: newBaseParser(backend.Key(tunnelConnectionsPrefix)),
	}
}

type tunnelConnectionParser struct {
	baseParser
}

func (p *tunnelConnectionParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		clusterName, name, err := baseTwoKeys(event.Item.Key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &services.ResourceHeader{
			Kind:    services.KindTunnelConnection,
			SubKind: clusterName,
			Version: services.V2,
			Metadata: services.Metadata{
				Name:      name,
				Namespace: defaults.Namespace,
			},
		}, nil
	case backend.OpPut:
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

func (p *reverseTunnelParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		return resourceHeader(event, services.KindReverseTunnel, services.V2, 0)
	case backend.OpPut:
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

func newAppServerParser() *appServerParser {
	return &appServerParser{
		baseParser: newBaseParser(backend.Key(appsPrefix, serversPrefix, defaults.Namespace)),
	}
}

type appServerParser struct {
	baseParser
}

func (p *appServerParser) parse(event backend.Event) (services.Resource, error) {
	return parseServer(event, services.KindAppServer)
}

func newAppSessionParser() *webSessionParser {
	return &webSessionParser{
		baseParser: newBaseParser(backend.Key(appsPrefix, sessionsPrefix)),
		hdr: services.ResourceHeader{
			Kind:    services.KindWebSession,
			SubKind: services.KindAppSession,
			Version: services.V2,
		},
	}
}

func newWebSessionParser() *webSessionParser {
	return &webSessionParser{
		baseParser: newBaseParser(backend.Key(webPrefix, sessionsPrefix)),
		hdr: services.ResourceHeader{
			Kind:    services.KindWebSession,
			SubKind: services.KindWebSession,
			Version: services.V2,
		},
	}
}

type webSessionParser struct {
	baseParser
	hdr services.ResourceHeader
}

func (p *webSessionParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		return resourceHeaderWithTemplate(event, p.hdr, 0)
	case backend.OpPut:
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

func (p *webTokenParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		return resourceHeader(event, services.KindWebToken, services.V1, 0)
	case backend.OpPut:
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

func (p *kubeServiceParser) parse(event backend.Event) (services.Resource, error) {
	return parseServer(event, services.KindKubeService)
}

func newDatabaseServerParser() *databaseServerParser {
	return &databaseServerParser{
		baseParser: newBaseParser(backend.Key(dbServersPrefix, defaults.Namespace)),
	}
}

type databaseServerParser struct {
	baseParser
}

func (p *databaseServerParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		hostID, name, err := baseTwoKeys(event.Item.Key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &types.DatabaseServerV3{
			Kind:    types.KindDatabaseServer,
			Version: types.V3,
			Metadata: services.Metadata{
				Name:        name,
				Namespace:   defaults.Namespace,
				Description: hostID, // Pass host ID via description field for the cache.
			},
		}, nil
	case backend.OpPut:
		return services.UnmarshalDatabaseServer(
			event.Item.Value,
			services.WithResourceID(event.Item.ID),
			services.WithExpires(event.Item.Expires),
		)
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func parseServer(event backend.Event, kind string) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		return resourceHeader(event, kind, services.V2, 0)
	case backend.OpPut:
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

func (p *remoteClusterParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		return resourceHeader(event, services.KindRemoteCluster, services.V3, 0)
	case backend.OpPut:
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

func resourceHeader(event backend.Event, kind, version string, offset int) (services.Resource, error) {
	name, err := base(event.Item.Key, offset)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &services.ResourceHeader{
		Kind:    kind,
		Version: version,
		Metadata: services.Metadata{
			Name:      string(name),
			Namespace: defaults.Namespace,
		},
	}, nil
}

func resourceHeaderWithTemplate(event backend.Event, hdr services.ResourceHeader, offset int) (services.Resource, error) {
	name, err := base(event.Item.Key, offset)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &services.ResourceHeader{
		Kind:    hdr.Kind,
		SubKind: hdr.SubKind,
		Version: hdr.Version,
		Metadata: services.Metadata{
			Name:      string(name),
			Namespace: defaults.Namespace,
		},
	}, nil
}

// WaitForEvent waits for the event matched by the specified event matcher in the given watcher.
func WaitForEvent(ctx context.Context, watcher services.Watcher, m EventMatcher, clock clockwork.Clock) (services.Resource, error) {
	tick := clock.NewTicker(defaults.WebHeadersTimeout)
	defer tick.Stop()

	select {
	case event := <-watcher.Events():
		if event.Type != backend.OpInit {
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
func (r EventMatcherFunc) Match(event services.Event) (services.Resource, error) {
	return r(event)
}

// EventMatcherFunc matches the specified resource event.
// Implements EventMatcher
type EventMatcherFunc func(services.Event) (services.Resource, error)

// EventMatcher matches a specific resource event
type EventMatcher interface {
	// Match matches the specified event.
	// Returns the matched resource if successful.
	// Returns trace.CompareFailedError for no match.
	Match(services.Event) (services.Resource, error)
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

// getClusterConfigFunc gets ClusterConfig to facilitate backward compatible
// transition to standalone configuration resources.  DELETE IN 8.0.0
type getClusterConfigFunc func(...services.MarshalOption) (services.ClusterConfig, error)
