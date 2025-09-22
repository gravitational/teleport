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

package concurrentqueue

import (
	"sync"
)

type config struct {
	workers   int
	capacity  int
	inputBuf  int
	outputBuf int
}

// Option is a queue configuration option.
type Option func(*config)

// Workers is the number of concurrent workers to be used for the
// queue.  Defaults to 4.
func Workers(w int) Option {
	return func(cfg *config) {
		cfg.workers = w
	}
}

// Capacity is the amount of "in flight" items the queue can hold, not including
// any extra capacity in the input/output buffers. Assuming unbuffered input/output,
// this is the number of items that can be pushed to a queue before it begins to exhibit
// backpressure.  A value of 8-16x the number of workers is probably a reasonable choice
// for most applications.  Note that a queue always has a capacity at least equal to the
// number of workers, so `New(fn,Workers(7),Capacity(3))` results in a queue with capacity
// `7`. Defaults to 64.
func Capacity(c int) Option {
	return func(cfg *config) {
		cfg.capacity = c
	}
}

// InputBuf is the amount of buffer to be used in the input/push channel. Defaults to 0 (unbuffered).
func InputBuf(b int) Option {
	return func(cfg *config) {
		cfg.inputBuf = b
	}
}

// OutputBuf is the amount of buffer to be used in the output/pop channel.  Allocating output
// buffer space may improve performance when items are able to be popped in quick succession.
// Defaults to 0 (unbuffered).
func OutputBuf(b int) Option {
	return func(cfg *config) {
		cfg.outputBuf = b
	}
}

// item is the internal "work item" used by the queue.  it holds a value, and a nonce indicating the
// order in which the value was received.
type item[I any] struct {
	value I
	nonce uint64
}

// Queue is a data processing helper which uses a worker pool to apply a closure to a series of
// values concurrently, preserving the correct ordering of results.  It is essentially the concurrent
// equivalent of this:
//
//	for msg := range inputChannel {
//	    outputChannel <- workFunction(msg)
//	}
//
// In order to prevent indefinite memory growth within the queue due to slow consumption and/or
// workers, the queue will exert backpressure over its input channel once a configurable capacity
// is reached.
type Queue[I any, O any] struct {
	input     chan I
	output    chan O
	closeOnce sync.Once
	done      chan struct{}
}

// Push accesses the queue's input channel.  The type of sent values must match
// that expected by the queue's work function.  If the queue was configured with
// a buffered input/push channel, non-blocking sends can be used as a heuristic for
// detecting backpressure due to queue capacity.  This is not a perfect test, but
// the rate of false positives will be extremely low for a queue with a decent
// capacity and non-trivial work function.
func (q *Queue[I, O]) Push() chan<- I {
	return q.input
}

// Pop accesses the queue's output channel.  The type of the received value
// will match the output of the work function.
func (q *Queue[I, O]) Pop() <-chan O {
	return q.output
}

// Done signals closure of the queue.
func (q *Queue[I, O]) Done() <-chan struct{} {
	return q.done
}

// Close permanently terminates all background operations.  If the queue is not drained before
// closure, items may be lost.
func (q *Queue[I, O]) Close() error {
	q.closeOnce.Do(func() {
		close(q.done)
	})
	return nil
}

// New builds a new queue instance around the supplied work function.
func New[I any, O any](workfn func(I) O, opts ...Option) *Queue[I, O] {
	const defaultWorkers = 4
	const defaultCapacity = 64

	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.workers < 1 {
		cfg.workers = defaultWorkers
	}
	if cfg.capacity < 1 {
		cfg.capacity = defaultCapacity
	}

	// capacity must be at least equal to the number of workers or else workers
	// will always be idle.
	if cfg.capacity < cfg.workers {
		cfg.capacity = cfg.workers
	}

	q := &Queue[I, O]{
		input:  make(chan I, cfg.inputBuf),
		output: make(chan O, cfg.outputBuf),
		done:   make(chan struct{}),
	}

	go q.run(workfn, cfg)

	return q
}

// run spawns background tasks and then blocks on collection/reordering routine.
func (q *Queue[I, O]) run(workfn func(I) O, cfg config) {
	// internal worker input/output channels. due to the semaphore channel below,
	// sends on these channels never block, as they are each allocated with sufficient
	// capacity to hold all in-flight items.
	workerIn, workerOut := make(chan item[I], cfg.capacity), make(chan item[O], cfg.capacity)

	// semaphore channel used to limit the number of "in flight" items.  a message is added prior to accepting
	// every input and removed upon emission of every output.  this allows us to exert backpressure and prevent
	// uncapped memory growth due to a slow worker.  this also keeps the queue's "capacity" consistent, regardless
	// of whether we are experiencing general slowness or a "head of line blocking" type scenario.
	sem := make(chan struct{}, cfg.capacity)

	// spawn workers
	for range cfg.workers {
		go func() {
			for {
				var itm item[I]
				select {
				case itm = <-workerIn:
				case <-q.done:
					return
				}

				out := item[O]{
					value: workfn(itm.value),
					nonce: itm.nonce,
				}

				select {
				case workerOut <- out:
				default:
					panic("cq worker output channel already full (semaphore violation)")
				}
			}
		}()
	}

	go q.distribute(workerIn, sem)

	q.collect(workerOut, sem)
}

// distribute takes inbound work items, applies a nonce, and then distributes
// them to the workers.
func (q *Queue[I, O]) distribute(workerIn chan<- item[I], sem chan struct{}) {
	var nonce uint64
	for {
		// we are about to accept an input, add an item to the in-flight semaphore channel
		select {
		case sem <- struct{}{}:
		case <-q.done:
			return
		}

		var value I
		select {
		case value = <-q.input:
		case <-q.done:
			return
		}

		select {
		case workerIn <- item[I]{value: value, nonce: nonce}:
		default:
			panic("cq worker input channel already full (semaphore violation)")
		}

		nonce++
	}
}

// collect takes the potentially disordered worker output and unifies it into
// an ordered output.
func (q *Queue[I, O]) collect(workerOut <-chan item[O], sem chan struct{}) {
	// items that cannot be emitted yet (due to arriving out of order),
	// stored in mapping of nonce => value.
	queue := make(map[uint64]O)

	// the nonce of the item we need to emit next.  incremented upon
	// successful emission.
	var nonce uint64

	// output value to be emitted (if any).  note that nil is a valid
	// output value, so we cannot inspect this value directly to
	// determine our state.
	var out O

	// emit indicates whether or not we should be attempting to emit
	// the output value.
	var emit bool

	for {
		outc := q.output
		if !emit {
			// we do not have the next output item yet, do not attempt to send
			outc = nil
		}

		select {
		case itm := <-workerOut:
			if itm.nonce == nonce {
				// item matches current nonce, proceed directly to emitting state
				out, emit = itm.value, true
			} else {
				// item does not match current nonce, store it in queue
				queue[itm.nonce] = itm.value
			}
		case outc <- out:
			// successfully sent current item, increment nonce and setup next
			// output if it is present.
			nonce++
			out, emit = queue[nonce]
			delete(queue, nonce)

			// event has been emitted, remove an item from in-flight semaphore channel
			select {
			case <-sem:
			default:
				panic("cq sem channel already empty (semaphore violation)")
			}
		case <-q.done:
			return
		}

	}
}
