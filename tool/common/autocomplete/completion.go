package autocomplete

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"slices"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
	"github.com/gravitational/teleport/tool/tctl/common/resources"
)

const cacheExpiry = 15 * time.Minute
const DefaultCache = "/tmp/autocompletecache.json"

type cache struct {
	client              authclient.Client
	filepath            string
	resourceGettersFunc map[string]resourceGetter
}

type cacheStorage struct {
	Resources map[string]*cacheEntry `json:"resources"`
}

type cacheEntry struct {
	ResourceNames    []string  `json:"resource_names"`
	TimeSinceUpdated time.Time `json:"time_since_updated"`
}

type resourceGetter func(ctx context.Context) ([]string, error)

const (
	HostKey           = "nodes_by_hostname"
	RecordingsKey     = "recordings"
	ActiveSessionsKey = "active_sessions"
	ClusterLoginKey   = "cluster_login"
)

var virtualResourceKeys = []string{
	HostKey,
	RecordingsKey,
	ActiveSessionsKey,
	ClusterLoginKey,
}

func NewAutoComplete(clt *authclient.Client, tc *client.TeleportClient) *cache {
	filePath := DefaultCache

	updaters := make(map[string]resourceGetter, len(resources.Handlers())+1)
	if clt != nil {
		for kind, handler := range resources.Handlers() {
			updaters[kind] = func(ctx context.Context) ([]string, error) {
				if handler.Singleton() {
					return []string{""}, nil
				}
				resources, err := handler.Get(ctx, clt, services.Ref{
					Kind: kind,
				}, resources.GetOpts{})
				if err != nil {
					return nil, trace.Wrap(err)
				}
				names := make([]string, 0, len(resources.Resources()))
				for _, resource := range resources.Resources() {
					names = append(names, resource.GetName())
				}
				return names, nil
			}
		}
	}
	if tc != nil {
		updaters[HostKey] = func(ctx context.Context) ([]string, error) {
			nodes, err := tc.ListNodesWithFilters(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			nodeHosts := make([]string, 0, len(nodes))
			for _, node := range nodes {
				nodeHosts = append(nodeHosts, node.GetHostname())
			}
			return nodeHosts, nil
		}
		updaters[RecordingsKey] = func(ctx context.Context) ([]string, error) {
			fromUTC, toUTC, err := defaults.SearchSessionRange(clockwork.NewRealClock(), "", "", "")
			if err != nil {
				return nil, trace.Wrap(err)
			}
			auditEvents, err := tc.SearchSessionEvents(ctx, fromUTC, toUTC, apidefaults.DefaultChunkSize, types.EventOrderDescending, 100)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			type sessionEvent interface {
				GetSessionID() string
			}
			var sessionIDs []string
			for _, event := range auditEvents {
				switch event.GetType() {
				case events.SessionEndEvent,
					events.WindowsDesktopSessionEndEvent,
					events.DatabaseSessionEndEvent,
					events.AppSessionChunkEvent:
					sessionIDs = append(sessionIDs, event.(sessionEvent).GetSessionID())
				}
			}
			return sessionIDs, nil
		}
		updaters[ActiveSessionsKey] = func(ctx context.Context) ([]string, error) {
			clt, err := tc.ConnectToCluster(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			defer clt.Close()
			sessions, err := clt.AuthClient.GetActiveSessionTrackers(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			sessionIDs := make([]string, 0, len(sessions))
			for _, session := range sessions {
				sessionIDs = append(sessionIDs, session.GetSessionID())
			}
			return sessionIDs, nil
		}
		updaters[ClusterLoginKey] = func(ctx context.Context) ([]string, error) {
			clusterClient, err := tc.ConnectToCluster(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			defer clusterClient.Close()
			rootClusterName := clusterClient.RootClusterName()
			rootAuthClient, err := clusterClient.ConnectToRootCluster(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			defer rootAuthClient.Close()
			leafClusters, err := rootAuthClient.GetRemoteClusters(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			clusters := []string{rootClusterName}
			for _, cluster := range leafClusters {
				clusters = append(clusters, cluster.GetName())
			}
			return clusters, nil
		}
	}
	return &cache{filepath: filePath, resourceGettersFunc: updaters}
}

func (c *cache) update(ctx context.Context, kind string) error {
	// get the list of resources
	updatedResourcesFn := c.resourceGettersFunc[kind]
	updatedResources, err := updatedResourcesFn(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	storage, err := c.readFromFile()
	if err != nil {
		return trace.Wrap(err)
	}
	storage.Resources[kind] = &cacheEntry{
		ResourceNames:    updatedResources,
		TimeSinceUpdated: time.Now(),
	}

	err = c.writeToFile(storage)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(err)
}

func (c *cache) Update(ctx context.Context) error {
	now := time.Now()
	storage, err := c.readFromFile()
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	for kind := range c.resourceGettersFunc {
		entry := storage.Resources[kind]
		if entry != nil && entry.TimeSinceUpdated.After(now.Add(cacheExpiry)) {
			continue
		}
		if err := c.update(ctx, kind); err != nil {
			// TODO debug log here
			continue
		}
	}
	return nil
}

func (c *cache) Get(kind string) ([]string, error) {
	cacheStorage, err := c.readFromFile()
	if err != nil {
		return nil, err
	}

	// when empty, signifies tctl resource command (tctl get ...)
	resources := []string{}
	if kind == "" {
		for kind, res := range cacheStorage.Resources {
			if slices.Contains(virtualResourceKeys, kind) {
				continue
			}
			for _, resource := range res.ResourceNames {
				resources = append(resources, kind+"/"+resource)
			}
		}
		return resources, nil
	}
	if entry := cacheStorage.Resources[kind]; entry != nil {
		return cacheStorage.Resources[kind].ResourceNames, nil
	}
	return []string{}, nil
}

// HintAction returns a kingpin HintAction function that provides autocomplete
// suggestions for the given resource kind from the local cache.
func HintAction(kind string) func() []string {
	return func() []string {
		c := NewAutoComplete(nil, nil)
		names, err := c.Get(kind)
		if err != nil {
			return nil
		}
		return names
	}
}

func (c *cache) readFromFile() (cacheStorage, error) {
	data, err := os.ReadFile(c.filepath)
	if err != nil {
		sysErr := trace.ConvertSystemError(err)
		if trace.IsNotFound(sysErr) {
			return cacheStorage{
				Resources: map[string]*cacheEntry{},
			}, nil
		}
		return cacheStorage{}, trace.ConvertSystemError(err)
	}
	var storage cacheStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return cacheStorage{}, trace.Wrap(err)
	}
	if storage.Resources == nil {
		storage.Resources = map[string]*cacheEntry{}
	}
	return storage, nil
}

func (c *cache) writeToFile(storage cacheStorage) error {
	data, err := json.Marshal(storage)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(os.WriteFile(c.filepath, data, 0600))
}

const completionCommand = "update-completions"

type CompletionCommand struct {
	app    *kingpin.Application
	comCmd *kingpin.CmdClause
}

func (c *CompletionCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config) {
	c.app = app
	c.comCmd = app.Command(completionCommand, "Update local completions cache").Hidden()
}

func (c *CompletionCommand) TryRun(ctx context.Context, cmd string, getClient commonclient.InitFunc) (match bool, err error) {
	switch cmd {
	case c.comCmd.FullCommand():
	default:
		return false, nil
	}
	clt, close, err := getClient(ctx)
	if err != nil {
		return true, trace.Wrap(err)
	}
	defer close(ctx)
	cache := NewAutoComplete(clt, nil)
	return true, trace.Wrap(cache.Update(ctx))
}

func UpdateCompletionsInBackground() error {
	executable, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}
	cmd := exec.Command(executable, completionCommand)
	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(cmd.Process.Release())
}
