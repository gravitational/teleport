package autocomplete

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
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

	storage cacheStorage
}

type cacheStorage struct {
	resources map[string]*cacheEntry
}

type cacheEntry struct {
	resourceNames    []string
	timeSinceUpdated time.Time
}

type resourceGetter func(ctx context.Context) ([]string, error)

const hostKey = "nodes_by_hostname"

func NewCache(filePath string, clt *authclient.Client, tc *client.TeleportClient) *cache {
	if filePath == "" {
		filePath = DefaultCache
	}
	updaters := make(map[string]resourceGetter, len(resources.Handlers())+1)
	if clt != nil {
		for kind, handler := range resources.Handlers() {
			updaters[kind] = func(ctx context.Context) ([]string, error) {

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
		updaters[hostKey] = func(ctx context.Context) ([]string, error) {
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
	}
	return &cache{filepath: filePath, resourceGettersFunc: updaters}
}

func (c *cache) update(ctx context.Context, kind string) error {
	// get the list of resources
	updatedResourcesFn := c.resourceGettersFunc[kind]
	updatedResources, err := updatedResourcesFn(ctx)
	if err != nil {
		return err
	}
	c.storage.resources[kind] = &cacheEntry{
		resourceNames:    updatedResources,
		timeSinceUpdated: time.Now(),
	}

	err = c.writeToFile(c.storage)
	if err != nil {
		return err
	}
	return err
}

func (c *cache) Update(ctx context.Context) error {
	now := time.Now()
	for kind, entry := range c.storage.resources {
		if !entry.timeSinceUpdated.After(now.Add(cacheExpiry)) {
			continue
		}
		if err := c.update(ctx, kind); err != nil {
			return trace.Wrap(err)
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
		for _, res := range cacheStorage.resources {
			resources = append(resources, res.resourceNames...)
		}
		return resources, nil
	}
	return cacheStorage.resources[kind].resourceNames, nil
}

func (c *cache) readFromFile() (cacheStorage, error) {
	data, err := os.ReadFile(c.filepath)
	if err != nil {
		return cacheStorage{}, trace.Wrap(err)
	}
	var storage cacheStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return cacheStorage{}, trace.Wrap(err)
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
	cache := NewCache(DefaultCache, clt, nil)
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
