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

package cloud

import (
	"context"
	"sync"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/trace"
)

// iamTask defines an IAM task for the database.
type iamTask struct {
	// isSetup indicates the task is a setup task if true, a teardown task if
	// false.
	isSetup bool
	// database is the database to configure.
	database types.Database
	// retryCount is the number of retries performed.
	retryCount int
}

// run executes this task.
func (t *iamTask) run(ctx context.Context, clients common.CloudClients, awsPolicyClient aws.InlinePolicyClient) error {
	configurator, err := newAWS(ctx, awsConfig{
		clients:         clients,
		awsPolicyClient: awsPolicyClient,
		database:        t.database,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if t.isSetup {
		return configurator.setupIAM(ctx)
	}
	return configurator.teardownIAM(ctx)
}

// iamTaskQueue defines a queue for IAM tasks.
type iamTaskQueue struct {
	queue []iamTask
	mu    sync.Mutex
}

// addTask adds a task to the queue.
func (q *iamTaskQueue) addTask(task iamTask) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.queue = append(q.queue, task)
}

// addTasksForRetry adds back tasks for retries.
func (q *iamTaskQueue) addTasksForRetry(tasks []iamTask, maxRetry int) {
	q.mu.Lock()
	defer q.mu.Unlock()

	retryTasks := []iamTask{}
	for _, task := range tasks {
		task.retryCount++
		if task.retryCount <= maxRetry {
			retryTasks = append(retryTasks, task)
		}
	}

	if len(retryTasks) == 0 {
		return
	}

	// Add retry tasks to front of the queue to preserve order.
	q.queue = append(retryTasks, q.queue...)
}

// take tasks from tasks queue.
func (q *iamTaskQueue) take() (take []iamTask) {
	q.mu.Lock()
	defer q.mu.Unlock()

	take, q.queue = q.queue, take
	return
}
