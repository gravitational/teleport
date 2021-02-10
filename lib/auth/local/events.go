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
	"sort"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/resource"
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
			parser = newClusterConfigParser()
		case types.KindClusterAuthPreference:
			parser = newAuthPreferenceParser()
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
		prefixes = append(prefixes, parser.prefix())
		parsers = append(parsers, parser)
	}
	// sort so that longer prefixes get first
	sort.Slice(parsers, func(i, j int) bool { return len(parsers[i].prefix()) > len(parsers[j].prefix()) })
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

func (w *watcher) parseEvent(e backend.Event) (*services.Event, error) {
	for _, p := range w.parsers {
		if e.Type == backend.OpInit {
			return &services.Event{Type: e.Type}, nil
		}
		if p.match(e.Item.Key) {
			resource, err := p.parse(e)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			// if resource is nil, then it was well-formed but is being filtered out.
			if resource == nil {
				return nil, nil
			}
			return &services.Event{Type: e.Type, Resource: resource}, nil
		}
	}
	return nil, trace.NotFound("no match found for %v %v", e.Type, string(e.Item.Key))
}

func (w *watcher) forwardEvents() {
	for {
		select {
		case <-w.backendWatcher.Done():
			return
		case event := <-w.backendWatcher.Events():
			converted, err := w.parseEvent(event)
			if err != nil {
				// not found errors are expected, for example
				// when namespace prefix is watched, it captures
				// node events as well, and there could be no
				// handler registered for nodes, only for namespaces
				if !trace.IsNotFound(err) {
					w.Warning(trace.DebugReport(err))
				}
				continue
			}
			// event is being filtered out
			if converted == nil {
				continue
			}
			select {
			case w.eventsC <- *converted:
			case <-w.backendWatcher.Done():
				return
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
	// prefix returns prefix to watch
	prefix() []byte
}

// baseParser is a partial implementation of resourceParser for the most common
// resource types (stored under a static prefix).
type baseParser struct {
	matchPrefix []byte
}

func (p baseParser) prefix() []byte {
	return p.matchPrefix
}

func (p baseParser) match(key []byte) bool {
	return bytes.HasPrefix(key, p.matchPrefix)
}

func newCertAuthorityParser(loadSecrets bool) *certAuthorityParser {
	return &certAuthorityParser{
		loadSecrets: loadSecrets,
		baseParser:  baseParser{matchPrefix: backend.Key(authoritiesPrefix)},
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
		ca, err := resource.UnmarshalCertAuthority(event.Item.Value,
			resource.WithResourceID(event.Item.ID), resource.WithExpires(event.Item.Expires), resource.SkipValidation())
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
		baseParser: baseParser{matchPrefix: backend.Key(tokensPrefix)},
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
		token, err := resource.UnmarshalProvisionToken(event.Item.Value,
			resource.WithResourceID(event.Item.ID),
			resource.WithExpires(event.Item.Expires),
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
		baseParser: baseParser{matchPrefix: backend.Key(clusterConfigPrefix, staticTokensPrefix)},
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
		tokens, err := resource.UnmarshalStaticTokens(event.Item.Value,
			resource.WithResourceID(event.Item.ID),
			resource.WithExpires(event.Item.Expires),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return tokens, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newClusterConfigParser() *clusterConfigParser {
	return &clusterConfigParser{
		baseParser: baseParser{matchPrefix: backend.Key(clusterConfigPrefix, generalPrefix)},
	}
}

type clusterConfigParser struct {
	baseParser
}

func (p *clusterConfigParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		h, err := resourceHeader(event, services.KindClusterConfig, services.V3, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		h.SetName(services.MetaNameClusterConfig)
		return h, nil
	case backend.OpPut:
		clusterConfig, err := resource.UnmarshalClusterConfig(
			event.Item.Value,
			resource.WithResourceID(event.Item.ID),
			resource.WithExpires(event.Item.Expires),
			resource.SkipValidation(),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return clusterConfig, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func newAuthPreferenceParser() *authPreferenceParser {
	return &authPreferenceParser{
		baseParser: baseParser{matchPrefix: backend.Key(authPrefix, preferencePrefix, generalPrefix)},
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
		ap, err := resource.UnmarshalAuthPreference(
			event.Item.Value,
			resource.WithResourceID(event.Item.ID),
			resource.WithExpires(event.Item.Expires),
			resource.SkipValidation(),
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
		baseParser: baseParser{matchPrefix: backend.Key(clusterConfigPrefix, namePrefix)},
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
		clusterName, err := resource.UnmarshalClusterName(event.Item.Value,
			resource.WithResourceID(event.Item.ID),
			resource.WithExpires(event.Item.Expires),
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
	p := &namespaceParser{}
	if name == "" {
		p.matchPrefix = backend.Key(namespacesPrefix)
	} else {
		p.matchPrefix = backend.Key(namespacesPrefix, name, paramsPrefix)
	}
	return p
}

type namespaceParser struct {
	baseParser
}

func (p *namespaceParser) match(key []byte) bool {
	// namespaces are stored under key '/namespaces/<namespace-name>/params'
	// and this code matches similar pattern
	return bytes.HasPrefix(key, p.matchPrefix) &&
		bytes.HasSuffix(key, []byte(paramsPrefix)) &&
		bytes.Count(key, []byte{backend.Separator}) == 3
}

func (p *namespaceParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		return resourceHeader(event, services.KindNamespace, services.V2, 1)
	case backend.OpPut:
		namespace, err := resource.UnmarshalNamespace(event.Item.Value,
			resource.WithResourceID(event.Item.ID),
			resource.WithExpires(event.Item.Expires),
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
		baseParser: baseParser{matchPrefix: backend.Key(rolesPrefix)},
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
		resource, err := resource.UnmarshalRole(event.Item.Value,
			resource.WithResourceID(event.Item.ID),
			resource.WithExpires(event.Item.Expires),
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

func (p *accessRequestParser) prefix() []byte {
	return p.matchPrefix
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
		baseParser: baseParser{matchPrefix: backend.Key(webPrefix, usersPrefix)},
	}
}

type userParser struct {
	baseParser
}

func (p *userParser) match(key []byte) bool {
	// users are stored under key '/web/users/<username>/params'
	// and this code matches similar pattern
	return bytes.HasPrefix(key, p.matchPrefix) &&
		bytes.HasSuffix(key, []byte(paramsPrefix)) &&
		bytes.Count(key, []byte{backend.Separator}) == 4
}

func (p *userParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		return resourceHeader(event, services.KindUser, services.V2, 1)
	case backend.OpPut:
		resource, err := resource.UnmarshalUser(event.Item.Value,
			resource.WithResourceID(event.Item.ID),
			resource.WithExpires(event.Item.Expires),
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
		baseParser: baseParser{matchPrefix: backend.Key(nodesPrefix, defaults.Namespace)},
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
		baseParser: baseParser{matchPrefix: backend.Key(proxiesPrefix)},
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
		baseParser: baseParser{matchPrefix: backend.Key(authServersPrefix)},
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
		baseParser: baseParser{matchPrefix: backend.Key(tunnelConnectionsPrefix)},
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
		resource, err := resource.UnmarshalTunnelConnection(event.Item.Value,
			resource.WithResourceID(event.Item.ID),
			resource.WithExpires(event.Item.Expires),
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
		baseParser: baseParser{matchPrefix: backend.Key(reverseTunnelsPrefix)},
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
		resource, err := resource.UnmarshalReverseTunnel(event.Item.Value,
			resource.WithResourceID(event.Item.ID),
			resource.WithExpires(event.Item.Expires),
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
		baseParser: baseParser{matchPrefix: backend.Key(appsPrefix, serversPrefix, defaults.Namespace)},
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
		baseParser: baseParser{matchPrefix: backend.Key(appsPrefix, sessionsPrefix)},
		hdr: services.ResourceHeader{
			Kind:    services.KindWebSession,
			SubKind: services.KindAppSession,
			Version: services.V2,
		},
	}
}

func newWebSessionParser() *webSessionParser {
	return &webSessionParser{
		baseParser: baseParser{matchPrefix: backend.Key(webPrefix, sessionsPrefix)},
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
		resource, err := resource.UnmarshalWebSession(event.Item.Value,
			resource.WithResourceID(event.Item.ID),
			resource.WithExpires(event.Item.Expires),
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
		baseParser: baseParser{matchPrefix: backend.Key(webPrefix, tokensPrefix)},
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
		resource, err := resource.UnmarshalWebToken(event.Item.Value,
			resource.WithResourceID(event.Item.ID),
			resource.WithExpires(event.Item.Expires),
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
		baseParser: baseParser{matchPrefix: backend.Key(kubeServicesPrefix)},
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
		baseParser: baseParser{matchPrefix: backend.Key(dbServersPrefix, defaults.Namespace)},
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
		return resource.UnmarshalDatabaseServer(
			event.Item.Value,
			resource.WithResourceID(event.Item.ID),
			resource.WithExpires(event.Item.Expires),
			resource.SkipValidation())
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

func parseServer(event backend.Event, kind string) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		return resourceHeader(event, kind, services.V2, 0)
	case backend.OpPut:
		resource, err := resource.UnmarshalServer(event.Item.Value,
			kind,
			resource.WithResourceID(event.Item.ID),
			resource.WithExpires(event.Item.Expires),
			resource.SkipValidation(),
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

func (p *remoteClusterParser) prefix() []byte {
	return p.matchPrefix
}

func (p *remoteClusterParser) match(key []byte) bool {
	return bytes.HasPrefix(key, p.matchPrefix)
}

func (p *remoteClusterParser) parse(event backend.Event) (services.Resource, error) {
	switch event.Type {
	case backend.OpDelete:
		return resourceHeader(event, services.KindRemoteCluster, services.V3, 0)
	case backend.OpPut:
		resource, err := resource.UnmarshalRemoteCluster(event.Item.Value,
			resource.WithResourceID(event.Item.ID),
			resource.WithExpires(event.Item.Expires),
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
