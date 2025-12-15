// YSM - Yandere SQL Manager
// Copyright (C) 2025 blubskye
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// Source code: https://github.com/blubskye/yandere_sql_manager

package worker

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
)

// Task represents a unit of work to be processed
type Task func() error

// Result represents the result of a task execution
type Result struct {
	Index int
	Error error
	Data  interface{}
}

// Pool manages a pool of worker goroutines
type Pool struct {
	workers    int
	tasks      chan indexedTask
	results    chan Result
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	running    atomic.Bool
	completed  atomic.Int64
	total      atomic.Int64
	errors     atomic.Int64
}

type indexedTask struct {
	index int
	task  Task
}

// NewPool creates a new worker pool with the specified number of workers
// If workers <= 0, it defaults to the number of CPUs
func NewPool(workers int) *Pool {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Pool{
		workers: workers,
		tasks:   make(chan indexedTask, workers*2),
		results: make(chan Result, workers*2),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start begins the worker pool
func (p *Pool) Start() {
	if p.running.Load() {
		return
	}
	p.running.Store(true)

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// worker processes tasks from the task channel
func (p *Pool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case task, ok := <-p.tasks:
			if !ok {
				return
			}

			err := task.task()
			p.completed.Add(1)
			if err != nil {
				p.errors.Add(1)
			}

			select {
			case p.results <- Result{Index: task.index, Error: err}:
			case <-p.ctx.Done():
				return
			}
		}
	}
}

// Submit adds a task to the pool
func (p *Pool) Submit(task Task) {
	p.SubmitIndexed(int(p.total.Load()), task)
}

// SubmitIndexed adds a task with a specific index to the pool
func (p *Pool) SubmitIndexed(index int, task Task) {
	p.total.Add(1)
	select {
	case p.tasks <- indexedTask{index: index, task: task}:
	case <-p.ctx.Done():
	}
}

// Results returns the results channel
func (p *Pool) Results() <-chan Result {
	return p.results
}

// Wait waits for all tasks to complete and closes channels
func (p *Pool) Wait() {
	close(p.tasks)
	p.wg.Wait()
	close(p.results)
	p.running.Store(false)
}

// Stop cancels all pending tasks and stops the pool
func (p *Pool) Stop() {
	p.cancel()
	p.Wait()
}

// Progress returns (completed, total, errors)
func (p *Pool) Progress() (int64, int64, int64) {
	return p.completed.Load(), p.total.Load(), p.errors.Load()
}

// Workers returns the number of workers
func (p *Pool) Workers() int {
	return p.workers
}

// IsRunning returns whether the pool is running
func (p *Pool) IsRunning() bool {
	return p.running.Load()
}

// BatchProcessor processes items in parallel batches
type BatchProcessor struct {
	pool       *Pool
	batchSize  int
	items      []interface{}
	processor  func(item interface{}) error
	mu         sync.Mutex
	results    []error
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(workers, batchSize int) *BatchProcessor {
	return &BatchProcessor{
		pool:      NewPool(workers),
		batchSize: batchSize,
	}
}

// Process processes all items with the given processor function
func (bp *BatchProcessor) Process(items []interface{}, processor func(item interface{}) error) []error {
	bp.items = items
	bp.processor = processor
	bp.results = make([]error, len(items))

	bp.pool.Start()

	// Submit all tasks
	for i, item := range items {
		idx := i
		itm := item
		bp.pool.Submit(func() error {
			err := processor(itm)
			bp.mu.Lock()
			bp.results[idx] = err
			bp.mu.Unlock()
			return err
		})
	}

	// Wait and collect results
	go func() {
		bp.pool.Wait()
	}()

	// Drain results channel
	for range bp.pool.Results() {
		// Results are already stored in bp.results
	}

	return bp.results
}

// Progress returns the progress of batch processing
func (bp *BatchProcessor) Progress() (int64, int64, int64) {
	return bp.pool.Progress()
}

// ParallelExecute executes functions in parallel and returns all errors
func ParallelExecute(workers int, tasks ...Task) []error {
	if len(tasks) == 0 {
		return nil
	}

	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > len(tasks) {
		workers = len(tasks)
	}

	errors := make([]error, len(tasks))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)

	for i, task := range tasks {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func(idx int, t Task) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			errors[idx] = t()
		}(i, task)
	}

	wg.Wait()
	return errors
}

// ParallelMap applies a function to all items in parallel
func ParallelMap[T any, R any](workers int, items []T, fn func(T) (R, error)) ([]R, []error) {
	if len(items) == 0 {
		return nil, nil
	}

	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > len(items) {
		workers = len(items)
	}

	results := make([]R, len(items))
	errors := make([]error, len(items))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)

	for i, item := range items {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, itm T) {
			defer wg.Done()
			defer func() { <-sem }()

			results[idx], errors[idx] = fn(itm)
		}(i, item)
	}

	wg.Wait()
	return results, errors
}

// Pipeline represents a processing pipeline with multiple stages
type Pipeline struct {
	stages []pipelineStage
}

type pipelineStage struct {
	name     string
	workers  int
	process  func(in <-chan interface{}) <-chan interface{}
}

// NewPipeline creates a new processing pipeline
func NewPipeline() *Pipeline {
	return &Pipeline{}
}

// AddStage adds a processing stage to the pipeline
func (p *Pipeline) AddStage(name string, workers int, process func(in <-chan interface{}) <-chan interface{}) *Pipeline {
	p.stages = append(p.stages, pipelineStage{
		name:    name,
		workers: workers,
		process: process,
	})
	return p
}

// Run executes the pipeline with the given input
func (p *Pipeline) Run(input <-chan interface{}) <-chan interface{} {
	current := input
	for _, stage := range p.stages {
		current = stage.process(current)
	}
	return current
}

// FanOut distributes work from one channel to multiple worker channels
func FanOut(input <-chan interface{}, workers int) []<-chan interface{} {
	outputs := make([]chan interface{}, workers)
	for i := range outputs {
		outputs[i] = make(chan interface{})
	}

	go func() {
		defer func() {
			for _, ch := range outputs {
				close(ch)
			}
		}()

		i := 0
		for item := range input {
			outputs[i%workers] <- item
			i++
		}
	}()

	result := make([]<-chan interface{}, workers)
	for i, ch := range outputs {
		result[i] = ch
	}
	return result
}

// FanIn merges multiple channels into one
func FanIn(inputs ...<-chan interface{}) <-chan interface{} {
	output := make(chan interface{})
	var wg sync.WaitGroup

	for _, input := range inputs {
		wg.Add(1)
		go func(ch <-chan interface{}) {
			defer wg.Done()
			for item := range ch {
				output <- item
			}
		}(input)
	}

	go func() {
		wg.Wait()
		close(output)
	}()

	return output
}
