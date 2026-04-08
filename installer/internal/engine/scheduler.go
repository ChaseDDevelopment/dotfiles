package engine

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const defaultTaskTimeout = 10 * time.Minute

// Run executes tasks concurrently respecting dependency order and
// resource locks. It returns a channel that emits TaskStartedMsg,
// TaskDoneMsg, TaskSkippedMsg, and a final AllDoneMsg.
//
// maxWorkers caps the number of concurrent goroutines. Resource
// semaphores (one per Resource type) further serialize tasks that
// share exclusive resources like apt or cargo.
func Run(ctx context.Context, tasks []Task, maxWorkers int) <-chan any {
	out := make(chan any, 128)

	go func() {
		defer func() {
			send(ctx, out, AllDoneMsg{})
			close(out)
		}()

		if len(tasks) == 0 {
			return
		}

		// Build DAG structures.
		byID := make(map[string]*Task, len(tasks))
		indegree := make(map[string]int, len(tasks))
		dependents := make(map[string][]string) // parent → children
		state := make(map[string]TaskState, len(tasks))

		for i := range tasks {
			t := &tasks[i]
			byID[t.ID] = t
			state[t.ID] = Queued
		}

		// Validate dependencies and compute indegree. Unknown deps
		// are silently stripped to prevent deadlocks.
		for i := range tasks {
			t := &tasks[i]
			var validDeps []string
			for _, dep := range t.DependsOn {
				if _, ok := byID[dep]; !ok {
					// Unknown dependency — strip it to prevent
					// deadlock from an unresolvable indegree.
					continue
				}
				validDeps = append(validDeps, dep)
				dependents[dep] = append(dependents[dep], t.ID)
			}
			tasks[i].DependsOn = validDeps
			indegree[t.ID] = len(validDeps)
		}

		// Cycle detection via Kahn's algorithm.
		{
			tempIn := make(map[string]int, len(indegree))
			for k, v := range indegree {
				tempIn[k] = v
			}
			var queue []string
			for id, deg := range tempIn {
				if deg == 0 {
					queue = append(queue, id)
				}
			}
			processed := 0
			for len(queue) > 0 {
				id := queue[0]
				queue = queue[1:]
				processed++
				for _, child := range dependents[id] {
					tempIn[child]--
					if tempIn[child] == 0 {
						queue = append(queue, child)
					}
				}
			}
			if processed != len(tasks) {
				var cycled []string
				for id, deg := range tempIn {
					if deg > 0 {
						cycled = append(cycled, id)
					}
				}
				send(ctx, out, TaskDoneMsg{
					ID:       cycled[0],
					Label:    byID[cycled[0]].Label,
					Err:      fmt.Errorf("dependency cycle involving: %v", cycled),
					Critical: true,
				})
				return
			}
		}

		// Resource semaphores — one slot per resource type.
		resSems := map[Resource]chan struct{}{
			ResApt:   make(chan struct{}, 1),
			ResCargo: make(chan struct{}, 1),
		}

		// Global worker semaphore.
		workerSem := make(chan struct{}, maxWorkers)

		var mu sync.Mutex
		var wg sync.WaitGroup

		// readyCh feeds tasks that have zero remaining dependencies.
		readyCh := make(chan string, len(tasks))

		// Enqueue initially ready tasks.
		for id, deg := range indegree {
			if deg == 0 {
				readyCh <- id
			}
		}

		// Track how many tasks are still pending (not finished).
		pending := len(tasks)

		// done is closed when all tasks are processed.
		done := make(chan struct{})
		var doneOnce sync.Once

		// markFinished decrements pending and closes done when zero.
		// Protected by sync.Once to prevent double-close panic if a
		// task is both skipped and dispatched in a race window.
		markFinished := func() {
			mu.Lock()
			pending--
			p := pending
			mu.Unlock()
			if p == 0 {
				doneOnce.Do(func() { close(done) })
			}
		}

		// skipDependents recursively skips tasks whose dependency
		// failed. All state transitions are atomic under mu.
		var skipDependents func(parentID string)
		skipDependents = func(parentID string) {
			for _, childID := range dependents[parentID] {
				mu.Lock()
				if state[childID] != Queued {
					mu.Unlock()
					continue
				}
				state[childID] = Skipped
				mu.Unlock()

				child := byID[childID]
				send(ctx, out, TaskSkippedMsg{
					ID:     childID,
					Label:  child.Label,
					Reason: fmt.Sprintf("dependency %q failed", parentID),
				})
				markFinished()
				skipDependents(childID)
			}
		}

		// Start dispatcher goroutine that consumes readyCh.
		go func() {
			for {
				select {
				case id := <-readyCh:
					task := byID[id]

					// Acquire global worker slot (context-aware).
					select {
					case workerSem <- struct{}{}:
					case <-ctx.Done():
						return
					case <-done:
						return
					}

					wg.Add(1)
					go func(t *Task) {
						defer wg.Done()
						defer func() { <-workerSem }()

						// Acquire resource semaphores (context-aware).
						// Track acquired resources so we can release
						// them if context cancels mid-acquisition.
						var acquired []Resource
						canRun := true
						for _, res := range t.Resources {
							if sem, ok := resSems[res]; ok {
								select {
								case sem <- struct{}{}:
									acquired = append(acquired, res)
								case <-ctx.Done():
									canRun = false
								}
								if !canRun {
									break
								}
							}
						}
						if !canRun {
							for _, res := range acquired {
								<-resSems[res]
							}
							return
						}

						mu.Lock()
						if state[t.ID] != Queued {
							// Task was skipped by skipDependents while
							// waiting for worker/resource slots. Release
							// resources and exit — skipDependents already
							// called markFinished.
							mu.Unlock()
							for _, res := range t.Resources {
								if sem, ok := resSems[res]; ok {
									<-sem
								}
							}
							return
						}
						state[t.ID] = Running
						mu.Unlock()

						send(ctx, out, TaskStartedMsg{ID: t.ID, Label: t.Label})

						// Run with timeout.
						tCtx, cancel := context.WithTimeout(ctx, defaultTaskTimeout)
						err := t.Run(tCtx)
						cancel()

						// Release resource semaphores.
						for _, res := range t.Resources {
							if sem, ok := resSems[res]; ok {
								<-sem
							}
						}

						mu.Lock()
						if err != nil {
							state[t.ID] = Failed
						} else {
							state[t.ID] = Succeeded
						}
						mu.Unlock()

						send(ctx, out, TaskDoneMsg{
							ID:       t.ID,
							Label:    t.Label,
							Err:      err,
							Critical: t.Critical,
						})

						if err != nil {
							skipDependents(t.ID)
						}

						// Unblock dependents — single critical
						// section per child to prevent TOCTOU race
						// with skipDependents.
						for _, childID := range dependents[t.ID] {
							mu.Lock()
							if state[childID] == Skipped {
								mu.Unlock()
								continue
							}
							indegree[childID]--
							ready := indegree[childID] == 0
							mu.Unlock()

							if ready {
								select {
								case readyCh <- childID:
								case <-ctx.Done():
									return
								}
							}
						}

						markFinished()
					}(task)

				case <-done:
					return
				case <-ctx.Done():
					return
				}
			}
		}()

		// Wait for all tasks or cancellation.
		select {
		case <-done:
		case <-ctx.Done():
		}
		wg.Wait()
	}()

	return out
}

// send writes a message to the output channel, aborting if the
// context is cancelled. This prevents goroutine leaks when the
// TUI stops reading events (e.g., on critical failure abort).
func send(ctx context.Context, ch chan<- any, msg any) {
	select {
	case ch <- msg:
	case <-ctx.Done():
	}
}
