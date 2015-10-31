/*
Copyright 2015 Gravitational, Inc.

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
package services

import (
	"time"

	"github.com/gravitational/teleport/lib/backend"
)

type LeaderElectionService struct {
	bk          backend.Backend
	path        []string
	weAreMaster bool
	subscribers []chan Event
	serverID    string
	enabled     bool
}

type Event int

const (
	Leader   = Event(1)
	Follower = Event(2)

	electionInterval = time.Second * 15
	masterLifetime   = time.Second * 30
)

func NewLeaderElectionService(backend backend.Backend, path []string, serverID string) *LeaderElectionService {
	les := LeaderElectionService{
		bk:          backend,
		path:        path,
		weAreMaster: false,
		serverID:    serverID,
		enabled:     true,
	}

	return &les
}

func (les *LeaderElectionService) Subscribe(c chan Event) {
	les.subscribers = append(les.subscribers, c)
}

func (les *LeaderElectionService) Disable() {
	les.enabled = false
	_, _ = les.bk.CompareAndSwap(
		les.path, "master", []byte{},
		masterLifetime, []byte(les.serverID),
	)
}

func (les *LeaderElectionService) AcquireMaster() bool {
	if les.weAreMaster {

		prevVal, err := les.bk.CompareAndSwap(
			les.path, "master", []byte(les.serverID),
			masterLifetime, []byte(les.serverID),
		)
		if err != nil && len(prevVal) == 0 {
			prevVal, err = les.bk.CompareAndSwap(
				les.path, "master", []byte(les.serverID),
				masterLifetime, []byte{},
			)
		}
		if err != nil {
			les.weAreMaster = false
			return false
		}
		return true

	} else {

		_, err := les.bk.CompareAndSwap(
			les.path, "master", []byte(les.serverID),
			masterLifetime, []byte{},
		)
		if err == nil {
			les.weAreMaster = true
			return true
		}
		return false
	}
}

func (les *LeaderElectionService) Start() {
	go les.election()
}

func (les *LeaderElectionService) election() {
	for {
		if les.weAreMaster {
			if !les.AcquireMaster() {
				for _, c := range les.subscribers {
					c <- Follower
				}
			}
		} else {
			if les.AcquireMaster() {
				for _, c := range les.subscribers {
					c <- Leader
				}
			}
		}
		time.Sleep(electionInterval)
	}
}
