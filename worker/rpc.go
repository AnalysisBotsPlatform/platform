// Datatypes for RPC communication of the worker controller and workers.
package worker

import (
	"errors"
	"fmt"
	"github.com/AnalysisBotsPlatform/platform/db"
	"sync"
)

// TODO document this
type WorkerAPI struct {
	available_workers map[int64][]waiting_worker
	shared_workers    []waiting_worker
	running_workers   map[int64]chan bool
	guard             *sync.RWMutex
}

// TODO document this
type NewWorker struct {
	User_token string
	Name       string
	Shared     bool
}

// TODO document this
type Task struct {
	Id       int64
	Project  string
	Bot      string
	GH_token string
}

// TODO document this
type Result struct {
	Tid         int64
	Stdout      string
	Stderr      string
	Exit_status int
}

// TODO document this
type waiting_worker struct {
	worker          *db.Worker
	task_assignment chan *db.Task
}

// TODO document this
var (
	InvalidToken  = errors.New("The provided token is not valid!")
	NotPrivileged = errors.New("Only admins can register shared workers!")
	NotValidTask  = errors.New("The provided task is not valid!")
)

// TODO document this
func NewWorkerAPI() *WorkerAPI {
	return &WorkerAPI{
		available_workers: make(map[int64][]waiting_worker),
		shared_workers:    make([]waiting_worker, 0),
		running_workers:   make(map[int64]chan bool),
		guard:             &sync.RWMutex{},
	}
}

// TODO document this
func (api *WorkerAPI) assignTask(task *db.Task) {
	api.guard.Lock()
	defer api.guard.Unlock()

    parentTask, err := db.GetParentTask(task.Id)
    if(err != nil){
        // TODO error handling
    }

	waiting, ok := api.available_workers[parentTask.User.Id]
	if ok {
		if len(waiting) > 0 {
			waiting[0].task_assignment <- task
			waiting = waiting[1:]
			api.available_workers[parentTask.User.Id] = waiting
		}
	} else {
		if len(api.shared_workers) > 0 {
			api.shared_workers[0].task_assignment <- task
			api.shared_workers = api.shared_workers[1:]
		}
	}
}

// TODO document this
func (api *WorkerAPI) cancelTask(tid int64) {
	api.guard.Lock()
	defer api.guard.Unlock()

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

// TODO document this
func (api *WorkerAPI) RegisterNewWorker(worker NewWorker, token *string) error {
	tok, err := db.CreateWorker(worker.User_token, worker.Name, worker.Shared)
	if err != nil { // TODO handle invalid token and not privileged
		err = InvalidToken
	}
	*token = tok

	return err
}

// TODO document this
func (api *WorkerAPI) RegisterWorker(worker string, ack *bool) error {
	err := db.SetWorkerActive(worker)
	*ack = err == nil
	if err != nil {
		err = InvalidToken
	}

	return err
}

// TODO document this
func (api *WorkerAPI) UnregisterWorker(worker string, ack *bool) error {
	err := db.SetWorkerInactive(worker)
	*ack = err == nil
	if err != nil {
		err = InvalidToken
	}

	return err
}

// TODO document this
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

// TODO document this
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

	var ok bool
	if pending != nil {
		_, ok = api.running_workers[pending.Id]
	}
	if pending == nil || ok {
		waiting := api.addAvailableWorker(worker)
		api.guard.Unlock()
		pending = <-waiting.task_assignment
		api.guard.Lock()
	}


// TODO fill this
	task.Id = pending.Id
//	task.Project = pending.Project.Name
//	task.Bot = pending.Bot.Name
//	task.GH_token = pending.User.Token
	api.running_workers[task.Id] = make(chan bool, 1)

	return nil
}

// TODO document this
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

// TODO document this
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

// TODO document this
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
	db.UpdateTaskResult(result.Tid, output, result.Exit_status)
	cancel <- false
	*ack = true

	return nil
}
