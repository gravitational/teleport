// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"log"
	"path"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/gravitational/trace"
)

func main() {
	args, err := parseCommandLine()
	if err != nil {
		flag.Usage()
		log.Fatal(err.Error())
	}

	rootRepo, err := git.PlainOpen(args.repoDir)
	if err != nil {
		log.Fatalf("Failed opening repository %s: %s", args.repoDir, err)
	}

	repos, err := scanForSubmodules(rootRepo, args.recursive)
	if err != nil {
		log.Fatalf("Failed scanning repository: %s", err)
	}

	if err = tagRepos(repos, args.tag, args.annotation, args.force); err != nil {
		log.Fatalf("Failed during tagging: %s", err)
	}

	if args.push {
		if err = pushTags(repos, args.remote, args.tag, args.force); err != nil {
			log.Fatalf("Failed during tagging: %s", err)
		}
	}
}

// scanForSubmodules collects all of the submodules that the root repoitory has
// coinfigured (optionally recursively) and compiles them into a single list
func scanForSubmodules(root *git.Repository, recursive bool) ([]*git.Repository, error) {
	queue := []*git.Repository{root}
	result := []*git.Repository{root}
	var repo *git.Repository

	for len(queue) > 0 {
		repo, queue = queue[len(queue)-1], queue[:len(queue)-1]
		w, err := repo.Worktree()
		if err != nil {
			return nil, trace.Wrap(err, "failed fetching worktree")
		}

		repoSubmodules, err := w.Submodules()
		if err != nil {
			return nil, trace.Wrap(err, "failed fetching submodules")
		}

		for _, m := range repoSubmodules {
			log.Printf("Found submodule %s at %s", m.Config().Name, w.Filesystem.Join(w.Filesystem.Root(), m.Config().Path))

			submoduleRepo, err := m.Repository()
			if err != nil {
				return nil, trace.Wrap(err, "Failed fetching submodule repository")
			}

			result = append(result, submoduleRepo)

			if recursive {
				queue = append(queue, submoduleRepo)
			}
		}
	}

	return result, nil
}

// tagRepos tage all repos in the supplied list with a given tag and optional
// annotation.
//
// The default behaviour is to halt at the first error, but if force is `true`,
// then `tagRepos` will ignore some errors and try to continue. Ignored errors
// include:
//   - An existing tag with the same name
//   - Failure to extract repo metadata
//   - failure to tag a single repo.
//
// In the case of an existing tag, `force` will cause the existing tag to be
// deleted and re-created. in all other cases, `tagRepos` will give up on the
// current repsitory and move on to the next one.
func tagRepos(repos []*git.Repository, tag, annotation string, force bool) error {
	var opts *git.CreateTagOptions
	if annotation != "" {
		opts = &git.CreateTagOptions{
			Message: annotation,
		}
	}

	for _, repo := range repos {
		w, err := repo.Worktree()
		if err != nil {
			if !force {
				return trace.Wrap(err, "Failed extracting worktree")
			}
			log.Printf("WARNING: failed extracting woirktree")
			continue
		}

		log.Printf("Tagging %s", w.Filesystem.Root())

		tagExists, err := statTag(repo, tag)
		if err != nil {
			return trace.Wrap(err, "failed enumerating tags")
		}

		if tagExists {
			if !force {
				return trace.BadParameter("tag already exists")
			}
			log.Printf("WARNING: tag \"%s\" already exists. Deleting.", tag)
			err = repo.DeleteTag(tag)
			if err != nil {
				log.Printf("WARNING: Faled to delete tag: %s", err)
				continue
			}
		}

		head, err := repo.Head()
		if err != nil {
			return trace.Wrap(err, "failed fetching head revision")
		}

		log.Printf("Tagging %s => %s", w.Filesystem.Root(), head.String())
		_, err = repo.CreateTag(tag, head.Hash(), opts)
		if err != nil {
			if !force {
				return trace.Wrap(err, "")
			}
			log.Printf("Failed to set tag: %s", err)
		}
	}

	return nil
}

// pushTags pushed the created tags to all of the listed submodules.
//
// The default behaviour is to halt at the first error, but if force is `true`,
// then `pushTags` will ignore some errors and try to continue. Ignored errors
// include:
//   - Failure to extract repo metadata
//   - failure to push to a repo.
//
// When an operation on a reposiotory failes (and force is `true`), `pushTags`
// will give up on the current repsitory and move on to the next one.
func pushTags(repos []*git.Repository, remote string, tag string, force bool) error {
	refname := path.Join("refs", "tags", tag)
	refspec := fmt.Sprintf("%s:%s", refname, refname)

	for _, repo := range repos {
		w, err := repo.Worktree()
		if err != nil {
			if !force {
				return trace.Wrap(err, "Failed extracting worktree")
			}
			log.Printf("WARNING: failed extracting woirktree")
			continue
		}

		log.Printf("Pushing: %s", w.Filesystem.Root())

		opts := git.PushOptions{
			RemoteName: remote,
			RefSpecs:   []config.RefSpec{config.RefSpec(refspec)},
		}

		if err := repo.Push(&opts); err != nil {
			if !force {
				return trace.Wrap(err, "failed pushing tags")
			}
			log.Printf("WARNING: failed pushing tag: %s", err)
		}
	}

	return nil
}

// statTag checks to see if a tag with the given name already exists in a
// repository, either as a simple lightweight tag, or a full annotated tag.
func statTag(repo *git.Repository, tag string) (bool, error) {
	tagpath := "refs/tags/" + tag
	res := false
	simpleTags, err := repo.Tags()
	if err != nil {
		return false, trace.Wrap(err, "failed listing tags")
	}
	simpleTags.ForEach(func(t *plumbing.Reference) error {
		if t.Name().String() == tagpath {
			res = true
			return nil
		}
		return nil
	})

	if err != nil {
		return false, trace.Wrap(err, "Failed enumerating tags")
	}

	return res, nil
}
