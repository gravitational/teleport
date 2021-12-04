package main

import (
	"context"
	"flag"
	"log"
	"sort"
	"time"

	"golang.org/x/mod/semver"

	"github.com/gravitational/trace"

	go_github "github.com/google/go-github/v41/github"
)

func main() {
	tag, err := parseFlags()
	if err != nil {
		log.Fatalf("Failed to parse flags; %v.", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := check(ctx, newGitHub(), "gravitational", "teleport", tag); err != nil {
		log.Fatalf("Check failed: %v.", err)
	}
}

func parseFlags() (string, error) {
	tag := flag.String("tag", "", "tag to validate")
	flag.Parse()

	if *tag == "" {
		return "", trace.BadParameter("tag missing")
	}
	return *tag, nil
}

func check(ctx context.Context, gh github, organization string, repository string, tag string) error {
	releases, err := gh.ListReleases(context.Background(), "gravitational", "teleport")
	if err != nil {
		return trace.Wrap(err)
	}
	sort.SliceStable(releases, func(i int, j int) bool {
		return releases[i] > releases[j]
	})
	if len(releases) == 0 {
		return trace.BadParameter("failed to find any releases on GitHub")
	}

	if semver.Compare(tag, releases[0]) <= 0 {
		return trace.BadParameter("found newer version of release, not releasing. Latest release: %v, tag: %v", releases[0], tag)
	}

	return nil
}

type github interface {
	ListReleases(ctx context.Context, organization string, repository string) ([]string, error)
}

type ghClient struct {
	client *go_github.Client
}

func newGitHub() *ghClient {
	return &ghClient{
		client: go_github.NewClient(nil),
	}
}

func (c *ghClient) ListReleases(ctx context.Context, organization string, repository string) ([]string, error) {
	var n int
	var releases []string

	opt := &go_github.ListOptions{
		Page:    0,
		PerPage: 100,
	}
	for {
		page, resp, err := c.client.Repositories.ListReleases(ctx,
			organization,
			repository,
			opt)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, p := range page {
			releases = append(releases, p.GetTagName())
		}

		n += 1
		if n == 100 {
			break
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return releases, nil
}
