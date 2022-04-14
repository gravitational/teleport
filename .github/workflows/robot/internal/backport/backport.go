/*
Copyright 2022 Gravitational, Inc.

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

package backport

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func Backport() error {
	dir, err := ioutil.TempDir("/tmp", "clone")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	fmt.Printf("--> dir: %v.\n", dir)

	//auth := &http.BasicAuth{Username: os.Getenv("GITHUB_TOKEN")}

	// We instantiate a new repository targeting the given path (the .git folder)
	r, err := git.PlainOpen("/home/rjones/go/src/github.com/gravitational/teleport")

	//r, err := git.PlainClone(dir, false, &git.CloneOptions{
	//	URL:      "https://github.com/gravitational/teleport.git",
	//	Progress: os.Stdout,
	//	Auth:     auth,
	//	//URL:           headURL,
	//	//SingleBranch:  true,
	//	//ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", headRef)),
	//	//Auth:          auth,
	//	//Progress:      os.Stdout,
	//})
	if err != nil {
		fmt.Printf("--> here -1!\n")
		log.Fatal(err)
	}

	file, err := ioutil.TempFile("/home/rjones/go/src/github.com/gravitational/teleport", "")
	if err != nil {
		fmt.Printf("--> here 0: %v.\n", err)
		log.Fatal(err)
	}
	defer os.Remove(file.Name())

	f := strings.TrimPrefix(file.Name(), "/home/rjones/go/src/github.com/gravitational/teleport")

	w, err := r.Worktree()
	if err != nil {
		fmt.Printf("--> here 1: %v.\n", err)
		log.Fatal(err)
	}

	_, err = w.Add(f)
	if err != nil {
		fmt.Printf("--> here 2: %v.\n", err)
		log.Fatal(err)
	}

	_, err = w.Commit("Fix.", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Foo Bar",
			Email: "foo@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		fmt.Printf("--> here 3: %v.\n", err)
		log.Fatal(err)
	}

	err = r.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   nil,
		Progress:   os.Stdout,
		Auth:       &http.BasicAuth{Username: ""},
	})
	if err != nil {
		fmt.Printf("--> here 4: %v.\n", err)
	}

	return nil
}
