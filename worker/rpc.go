// Datatypes for RPC communication of the worker controller and workers.
package worker

import (
	"errors"
	"fmt"
	"github.com/AnalysisBotsPlatform/platform/db"
	"os"
	"strings"
	"sync"
)

// This struct stores all relevant information to handle RPS's for the worker
// API.
type WorkerAPI struct {
	available_workers map[int64][]waiting_worker
	shared_workers    []waiting_worker
	running_workers   map[int64]chan bool
	guard             *sync.RWMutex
}

// Payload for registering a new worker client.
type NewWorker struct {
	User_token string
	Name       string
	Shared     bool
}

// Payload for task assignments.
type Task struct {
	Id       int64
	Project  string
	Bot      string
	GH_token string
	Patch    bool
}

// Payload for returning task results.
type Result struct {
	Tid         int64
	Stdout      string
	Stderr      string
	Exit_status int
	Patch       string
}

// Enable a worker to wait for a new task by adding a channel that delivers the
// task to execute.
type waiting_worker struct {
	worker          *db.Worker
	task_assignment chan *db.Task
}

// Custom error messages.
var (
	InvalidToken  = errors.New("The provided token is not valid!")
	NoTask        = errors.New("No task assigned!")
	NotPrivileged = errors.New("Only admins can register shared workers!")
	NotValidTask  = errors.New("The provided task is not valid!")
)

// Instantiate a new remote API for worker clients.
func NewWorkerAPI() *WorkerAPI {
	return &WorkerAPI{
		available_workers: make(map[int64][]waiting_worker),
		shared_workers:    make([]waiting_worker, 0),
		running_workers:   make(map[int64]chan bool),
		guard:             &sync.RWMutex{},
	}
}

// Assign an available worker to the new task. If there is no worker available
// the task is not assigned. The assignment is done in two phases:
//
// 1. Try to find a worker belonging to the user that started the task.
//
// 2. Only if there is no such worker try to find a shared worker that can
// execute the task.
func (api *WorkerAPI) assignTask(task *db.Task) {
	api.guard.Lock()
	defer api.guard.Unlock()

	waiting, ok := api.available_workers[task.User.Id]
	if ok {
		if len(waiting) > 0 {
			waiting[0].task_assignment <- task
			waiting = waiting[1:]
			api.available_workers[task.User.Id] = waiting
		}
	} else {
		if len(api.shared_workers) > 0 {
			api.shared_workers[0].task_assignment <- task
			api.shared_workers = api.shared_workers[1:]
		}
	}
}

// Cancel the task specified by `tid`, i.e. send an cancel signal to the worker
// executing the task (if applicable) and update the task's status to canceled.
func (api *WorkerAPI) cancelTask(tid int64) {
	api.guard.Lock()
	defer api.guard.Unlock()

	// NOTE reason whether this defer call is needed or not
	defer func() {
		recover()
		db.UpdateTaskStatus(tid, db.Canceled)
	}()
	if cancel, ok := api.running_workers[tid]; ok {
		cancel <- true
		delete(api.running_workers, tid)
	}
	db.UpdateTaskStatus(tid, db.Canceled)
}

// Register a new worker client for the user whose worker registration token is
// passed.
func (api *WorkerAPI) RegisterNewWorker(worker NewWorker, token *string) error {
	tok, err := db.CreateWorker(worker.User_token, worker.Name, worker.Shared)
	if err != nil { // NOTE handle invalid token and not privileged
		err = InvalidToken
	}
	*token = tok

	return err
}

// Marks the given worker as active. Must be called before any attempt to
// execute tasks.
func (api *WorkerAPI) RegisterWorker(worker string, ack *bool) error {
	err := db.SetWorkerActive(worker)
	*ack = err == nil
	if err != nil {
		err = InvalidToken
	}

	return err
}

// Marks the given worker as inactive and removes it from the set of available
// workers. Must be called before the worker client terminates.
func (api *WorkerAPI) UnregisterWorker(worker_token string, ack *bool) error {
	err := db.SetWorkerInactive(worker_token)
	*ack = err == nil
	if err != nil {
		return InvalidToken
	}

	worker, err := db.GetWorker(worker_token)
	*ack = err == nil
	if err != nil {
		return InvalidToken
	}

	api.guard.Lock()
	defer api.guard.Unlock()

	waiting, ok := api.available_workers[worker.Uid]
	if ok {
		for i, ww := range waiting {
			if ww.worker.Id == worker.Id {
				waiting = append(waiting[:i], waiting[i+1:]...)
				ww.task_assignment <- nil
				break
			}
		}
		api.available_workers[worker.Uid] = waiting
	} else {
		for i, ww := range api.shared_workers {
			if ww.worker.Id == worker.Id {
				api.shared_workers = append(api.shared_workers[:i],
					api.shared_workers[i+1:]...)
				ww.task_assignment <- nil
				break
			}
		}
	}

	return nil
}

// Helper to add a worker to the set of available workers. Creates a
// `waiting_worker` and inserts it into the set.
func (api *WorkerAPI) addAvailableWorker(worker *db.Worker) waiting_worker {
	waiting := waiting_worker{
		worker:          worker,
		task_assignment: make(chan *db.Task),
	}

	if worker.Shared {
		api.shared_workers = append(api.shared_workers, waiting)
	} else {
		api.available_workers[worker.Uid] = append(
			api.available_workers[worker.Uid], waiting)
	}

	return waiting
}

// Assign a pending task to the calling worker client. Blocks if there is no
// pending task. Continues execution after a new task was created and assigned
// to this worker.
func (api *WorkerAPI) GetTask(worker_token string, task *Task) error {
	worker, err := db.GetWorker(worker_token)
	if err != nil {
		task = nil
		return InvalidToken
	}

	api.guard.Lock()
	defer api.guard.Unlock()
	pending, err := db.GetPendingTask(worker.Uid, worker.Shared)
	if err != nil {
		task = nil
		return err
	}

	if pending == nil {
		waiting := api.addAvailableWorker(worker)
		api.guard.Unlock()
		pending = <-waiting.task_assignment
		api.guard.Lock()
		if pending == nil {
			return NoTask
		}
	}

	task.Id = pending.Id
	task.Project = pending.Project.Name
	task.Bot = pending.Bot.Name
	task.GH_token = pending.User.Token
	for _, tag := range pending.Bot.Tags {
		if strings.ToLower(tag) == "git patch" {
			task.Patch = true
		}
	}

	api.running_workers[task.Id] = make(chan bool, 1)
	db.UpdateTaskStatus(task.Id, db.Scheduled)

	return nil
}

// Wait for task to complete its execution or until it is canceled. Must be
// called immediately after `GetTask`.
func (api *WorkerAPI) WaitForTaskCancelation(task Task, canceled *bool) error {
	api.guard.RLock()
	cancel, ok := api.running_workers[task.Id]
	if !ok {
		*canceled = false
		return NotValidTask
	}
	api.guard.RUnlock()

	*canceled = <-cancel
	delete(api.running_workers, task.Id)

	return nil
}

// Mark a pending task as running.
func (api *WorkerAPI) PublishTaskStarted(task Task, ack *bool) error {
	api.guard.RLock()
	_, ok := api.running_workers[task.Id]
	if !ok {
		*ack = true
		return NotValidTask
	}
	api.guard.RUnlock()

	db.UpdateTaskStatus(task.Id, db.Running)
	*ack = true

	return nil
}

// Return the task's result back to the server.
func (api *WorkerAPI) PublishTaskResult(result Result, ack *bool) error {
	api.guard.RLock()
	cancel, ok := api.running_workers[result.Tid]
	if !ok {
		*ack = true
		return NotValidTask
	}
	api.guard.RUnlock()

	output := fmt.Sprintf("Stdout:\n%s\nStderr:\n%s", result.Stdout,
		result.Stderr)
	file_name := db.UpdateTaskResult(result.Tid, output, result.Exit_status,
		result.Patch != "")
	cancel <- false
	*ack = true

	if result.Patch != "" {
		file_path := fmt.Sprintf("%s/%s", GetPatchPath(), file_name)
		file, err := os.Create(file_path)
		if err != nil {
			fmt.Println(err)
			return err
		}
		defer file.Close()

		_, err = file.WriteString(result.Patch)
		if err != nil {
			fmt.Println(err)
			return err
		}
	}

	return nil
}
