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
			out <- AllDoneMsg{}
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
			indegree[t.ID] = len(t.DependsOn)
			for _, dep := range t.DependsOn {
				dependents[dep] = append(dependents[dep], t.ID)
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

		// markFinished decrements pending and closes done when zero.
		markFinished := func() {
			mu.Lock()
			pending--
			p := pending
			mu.Unlock()
			if p == 0 {
				close(done)
			}
		}

		// skipDependents recursively skips tasks whose dependency failed.
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
				out <- TaskSkippedMsg{
					ID:     childID,
					Label:  child.Label,
					Reason: fmt.Sprintf("dependency %q failed", parentID),
				}
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

					// Acquire global worker slot.
					workerSem <- struct{}{}

					wg.Add(1)
					go func(t *Task) {
						defer wg.Done()
						defer func() { <-workerSem }()

						// Acquire resource semaphores.
						for _, res := range t.Resources {
							if sem, ok := resSems[res]; ok {
								sem <- struct{}{}
							}
						}

						mu.Lock()
						state[t.ID] = Running
						mu.Unlock()

						out <- TaskStartedMsg{ID: t.ID, Label: t.Label}

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

						out <- TaskDoneMsg{
							ID:       t.ID,
							Label:    t.Label,
							Err:      err,
							Critical: t.Critical,
						}

						if err != nil {
							skipDependents(t.ID)
						}

						// Unblock dependents.
						for _, childID := range dependents[t.ID] {
							mu.Lock()
							childState := state[childID]
							mu.Unlock()
							if childState == Skipped {
								continue
							}

							mu.Lock()
							indegree[childID]--
							ready := indegree[childID] == 0
							mu.Unlock()

							if ready {
								readyCh <- childID
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

		// Wait for all tasks.
		<-done
		wg.Wait()
	}()

	return out
}
