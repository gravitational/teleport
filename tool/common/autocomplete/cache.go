package autocomplete

import (
	"encoding/json"
	"os"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/auth/authclient"
)

type cache struct {
	client               authclient.Client
	filepath             string
	resourceUpdatersFunc map[string]func() ([]string, error)

	storage cacheStorage
}

type cacheStorage struct {
	resources map[string]*cacheEntry
}

type cacheEntry struct {
	resourceNames    []string
	timeSinceUpdated time.Time
}

func NewCache(filePath string, resourceUpdaters map[string]func() ([]string, error)) *cache {
	return &cache{filepath: filePath, resourceUpdatersFunc: resourceUpdaters}
}

func (c *cache) update(kind string) error {
	// get the list of resources
	updatedResourcesFn := c.resourceUpdatersFunc[kind]
	updatedResources, err := updatedResourcesFn()
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

func (c *cache) Get(kind string) ([]string, error) {
	cacheStorage, err := c.readFromFile()
	if err != nil {
		return nil, err
	}
	// if cache is old retrieve from server and write to file
	// TODO
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
