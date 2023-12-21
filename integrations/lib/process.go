/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package lib

import (
	"context"
	"sync"

	"github.com/gravitational/trace"
)

type Job interface {
	DoJob(context.Context) error
}

type ServiceJob interface {
	Job
	IsReady() bool
	SetReady(ready bool)
	WaitReady(ctx context.Context) (bool, error)
	Done() <-chan struct{}
	Err() error
}

type serviceJob struct {
	mu      sync.Mutex
	do      func(context.Context) error
	ready   bool
	err     error
	readyCh chan struct{}
	doneCh  chan struct{}
}

type Process struct {
	sync.Mutex
	// doneCh is closed when all the jobs are completed.
	doneCh chan struct{}
	// spawn runs a goroutine in the app's context as a job with waiting for
	// its completion on shutdown.
	spawn func(Job, bool)
	// terminate signals the app to terminate gracefully.
	terminate func()
	// cancel signals the app to terminate immediately
	cancel context.CancelFunc
	// onTerminate is a list of callbacks called on terminate.
	onTerminate []jobFunc
	// terminated flags out that process has been signaled for termination.
	terminated bool
	// criticalErrors is a list of errors returned by critical jobs.
	criticalErrors []error
}

type jobFunc func(context.Context) error

type processKey struct{}
type jobKey struct{}

var closedChan = make(chan struct{})

func init() {
	close(closedChan)
}

func NewProcess(ctx context.Context) *Process {
	ctx, cancel := context.WithCancel(ctx)
	doneCh := make(chan struct{})
	process := &Process{
		doneCh:      doneCh,
		cancel:      cancel,
		onTerminate: make([]jobFunc, 0),
	}
	ctx = context.WithValue(ctx, processKey{}, process)

	var jobs sync.WaitGroup

	jobs.Add(1) // Start the main "job". We have to do it for Wait() not being returned beforehand.
	go func() {
		jobs.Wait()
		close(doneCh)
	}()
	process.spawn = func(job Job, critical bool) {
		jobs.Add(1)
		jctx, jcancel := context.WithCancel(context.WithValue(ctx, jobKey{}, job))
		go func() {
			err := job.DoJob(jctx)
			jcancel()
			jobs.Done()
			if err != nil && critical {
				process.Terminate()
			}
		}()
	}

	var once sync.Once
	process.terminate = func() {
		once.Do(func() {
			process.Lock()
			process.terminated = true
			for _, j := range process.onTerminate {
				process.spawn(j, false)
			}
			process.onTerminate = nil
			process.Unlock()
			jobs.Done() // Stop the main "job".
		})
	}

	return process
}

func (p *Process) SpawnJob(job Job) {
	if p == nil {
		panic("spawning a job on a nil process")
	}
	select {
	case <-p.doneCh:
		panic("spawning a job on a finished process")
	default:
		p.spawn(job, false)
	}
}

func (p *Process) SpawnCriticalJob(job Job) {
	if p == nil {
		panic("spawning a job on a nil process")
	}
	select {
	case <-p.doneCh:
		panic("spawning a job on a finished process")
	default:
		p.spawn(job, true)
	}
}

func (p *Process) Spawn(fn func(ctx context.Context) error) {
	p.SpawnJob(jobFunc(fn))
}

func (p *Process) SpawnCritical(fn func(ctx context.Context) error) {
	p.SpawnCriticalJob(jobFunc(fn))
}

func (p *Process) OnTerminate(fn func(ctx context.Context) error) {
	if p == nil {
		panic("calling OnTerminate a nil process")
	}
	p.Lock()
	defer p.Unlock()
	if p.terminated {
		p.Spawn(fn)
	} else {
		p.onTerminate = append(p.onTerminate, fn)
	}
}

// Done channel is used to wait for jobs completion.
func (p *Process) Done() <-chan struct{} {
	if p == nil {
		return closedChan
	}
	return p.doneCh
}

// Terminate signals a process to terminate. You should avoid spawning new jobs after termination.
func (p *Process) Terminate() {
	if p == nil {
		return
	}
	p.terminate()
}

// Shutdown signals a process to terminate and waits for completion of all jobs.
func (p *Process) Shutdown(ctx context.Context) error {
	p.Terminate()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.Done():
		return nil
	}
}

// Close shuts down all process jobs immediately.
func (p *Process) Close() {
	if p == nil {
		return
	}
	p.cancel()
	<-p.doneCh
}

func (p *Process) CriticalError() error {
	return trace.NewAggregate(p.criticalErrors...)
}

func (j jobFunc) DoJob(ctx context.Context) error {
	return j(ctx)
}

func MustGetProcess(ctx context.Context) *Process {
	return ctx.Value(processKey{}).(*Process)
}

func MustGetJob(ctx context.Context) Job {
	return ctx.Value(jobKey{}).(Job)
}

func MustGetServiceJob(ctx context.Context) ServiceJob {
	return MustGetJob(ctx).(ServiceJob)
}

func NewServiceJob(fn func(ctx context.Context) error) ServiceJob {
	job := &serviceJob{
		readyCh: make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
	job.do = func(ctx context.Context) error {
		err := fn(ctx)
		job.finish(err)
		return err
	}
	return job
}

func (job *serviceJob) finish(err error) {
	job.mu.Lock()
	defer job.mu.Unlock()

	select {
	case <-job.readyCh:
	default:
		close(job.readyCh)
	}
	job.err = err
	close(job.doneCh)
}

func (job *serviceJob) DoJob(ctx context.Context) error {
	return job.do(ctx)
}

func (job *serviceJob) IsReady() bool {
	job.mu.Lock()
	defer job.mu.Unlock()

	return job.ready
}

func (job *serviceJob) SetReady(ready bool) {
	job.mu.Lock()
	defer job.mu.Unlock()

	job.ready = ready
	select {
	case <-job.readyCh:
	default:
		close(job.readyCh)
	}
}

func (job *serviceJob) WaitReady(ctx context.Context) (bool, error) {
	select {
	case <-job.readyCh:
		return job.IsReady(), nil
	case <-ctx.Done():
		return false, trace.Wrap(ctx.Err())
	}
}

func (job *serviceJob) Done() <-chan struct{} {
	return job.doneCh
}

func (job *serviceJob) Err() error {
	job.mu.Lock()
	defer job.mu.Unlock()

	return job.err
}
