// Background worker.
package worker

import (
	"errors"
	"fmt"
	"github.com/AnalysisBotsPlatform/platform/db"
	"github.com/gorhill/cronexpr"
	"log"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// Maximal duration in seconds for each task.
const max_task_time int64 = 60

// Cache subdirectory where projects are cloned to.
const projects_directory = "projects"

// Cache subdirectory where Git patches are located.
const patches_directory = "patches"

// Absolute path to patch files directory.
var projects_path string

// Absolute path to patch files directory.
var patches_path string

// WorkerAPI instance used to interact with the workers.
var api *WorkerAPI

// Custom error messages.
var (
	PatchFailure = errors.New("Patch cannot be applied!")
)

// ticker to coordinate periodic tasks
var timer *time.Timer

// channel to cancel period runner
var pauseChan chan bool

var runningTasks map[int64]chan bool

// Initialization of the worker. Sets up the RPC infrastructure.
func Init(port, cache_path string) error {
	api = NewWorkerAPI()
	rpc.Register(api)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return err
	}

	pauseChan = make(chan bool)
	runningTasks = make(map[int64]chan bool)

	recoverActiveTasks()

	go rpc.Accept(listener)

	projects_path = fmt.Sprintf("%s/%s", cache_path, projects_directory)
	if _, err := os.Stat(projects_path); os.IsNotExist(err) {
		fmt.Println("Project cache directory does not exist!")
		fmt.Printf("Create project cache directory %s\n", projects_path)
		if err := os.MkdirAll(projects_path, 0755); err != nil {
			fmt.Println("Project cache directory cannot be created!")
			return err
		}
	}

	patches_path = fmt.Sprintf("%s/%s", cache_path, patches_directory)
	if _, err := os.Stat(patches_path); os.IsNotExist(err) {
		fmt.Println("Patch cache directory does not exist!")
		fmt.Printf("Create patch cache directory %s\n", patches_path)
		if err := os.MkdirAll(patches_path, 0755); err != nil {
			fmt.Println("Patch cache directory cannot be created!")
			return err
		}
	}

	return nil
}

// Returns the path to the patch directory
func GetPatchPath() string {
	return patches_path
}

// Creates a new task. This includes the following steps:
// - Creating a database entry.
// - Creating a new communication channel.
// - Starting an asynchronous task.
// The task id of the newly created task is returned.
// TODO document this
func CreateNewTask(parentTaskId int64) error {

	newTask, tErr := db.CreateNewChildTask(parentTaskId)
	if tErr != nil {
		return tErr
	}
	api.assignTask(newTask)
	return nil
}

func RunScheduledTask(stid int64) {
	cancelChan := make(chan bool, 1)
	runningTasks[stid] = cancelChan
	go runScheduledTask(stid, cancelChan)
}

func RunOneTimeTask(otid int64) {
	cancelChan := make(chan bool)
	runningTasks[otid] = cancelChan
	go runOneTimeTask(otid, cancelChan)
}

// ############################
// TODO
// cancel GroupTasks
// ############################

func CancelScheduledTask(stid int64) error {
	cancelChan, ok := runningTasks[stid]
	if ok {
		cancelChan <- true
	} else {
		return errors.New("The provided id does not correspond to a running task.")
	}
	err := db.UpdateScheduledTaskStatus(stid, db.Complete)
	runningChildren, gErr := db.GetRunningChildren(stid)
	if gErr != nil {
		return gErr
	}

	for _, childTask := range runningChildren {
		cancel(childTask.Id)
	}

	return err
}

func CancelOneTimeTask(stid int64) error {
	cancelChan, ok := runningTasks[stid]
	if ok {
		cancelChan <- true
	} else {
		return errors.New("The provided id does not correspond to a running task.")
	}
	err := db.UpdateOneTimeTaskStatus(stid, db.Complete)
	runningChildren, gErr := db.GetRunningChildren(stid)
	if gErr != nil {
		return gErr
	}

	for _, childTask := range runningChildren {
		cancel(childTask.Id)
	}

	return err
}

func CancelEventTask(stid int64) error {
	err := db.UpdateEventTaskStatus(stid, db.Complete)
	runningChildren, gErr := db.GetRunningChildren(stid)
	if gErr != nil {
		return gErr
	}

	for _, childTask := range runningChildren {
		cancel(childTask.Id)
	}

	return err
}

func CancelInstantTask(stid int64) error {
	cancelChan, ok := runningTasks[stid]
	if ok {
		cancelChan <- true
	} else {
		return errors.New("The provided id does not correspond to a running task.")
	}
	runningChildren, gErr := db.GetRunningChildren(stid)
	if gErr != nil {
		return gErr
	}

	for _, childTask := range runningChildren {
		cancel(childTask.Id)
	}

	return nil
}

// Perform unregister action for worker. This continues a potentially blocked
// execution of GetTask.
func DeleteWorker(worker_token string) {
	var ack bool
	api.UnregisterWorker(worker_token, &ack)
}

// Cancels the running task specified by the given task id using the channel.
// Also updates the database entry accordingly.
func cancel(tid int64) {

	api.cancelTask(tid)

}

// This function cancles all tasks which succeeded the 'max_task_time'
func CancleTimedOverTasks() {
	tasks, _ := db.GetTimedOverTasks(max_task_time)
	for _, e := range tasks {
		cancel(e)
	}
}

func StopPeriodRunners() {
	close(pauseChan)
}

func runScheduledTask(stid int64, cancelChan chan bool) {
	for {
		scheduledTask, err := db.GetScheduledTask(stid)
		if err != nil {
			db.UpdateScheduledTaskStatus(stid, db.Complete)
			return
		}
		nextTime := cronexpr.MustParse(scheduledTask.Cron).Next(time.Now())
		sleepTime := nextTime.Sub(time.Now())
		uErr := db.UpdateNextScheduleTime(scheduledTask.Id, nextTime)
		if uErr != nil {
			db.UpdateScheduledTaskStatus(stid, db.Complete)
			return
		}
		select {
		case <-time.After(sleepTime):
			CreateNewTask(stid)
		case <-cancelChan:
			db.UpdateScheduledTaskStatus(stid, db.Complete)
			return
		case <-pauseChan:
			return
		}
	}
}

func runOneTimeTask(otid int64, cancelChan chan bool) {
	oneTimeTask, err := db.GetOneTimeTask(otid)
	if err != nil {
		return
	}
	duration := oneTimeTask.Exec_time.Sub(time.Now().UTC())
	select {
	case <-time.After(duration):
		CreateNewTask(otid)
		db.UpdateOneTimeTaskStatus(otid, db.Complete)
	case <-cancelChan:
		return
	case <-pauseChan:
		return
	}
}

func recoverActiveTasks() {
	sched_ids, err := db.GetScheduledTaskIdsWithStatus(db.Active)
	if err == nil {
		for _, id := range sched_ids {
			RunScheduledTask(id)
		}
	}
	oneTime_ids, err := db.GetOneTimeTaskIdsWithStatus(db.Active)
	if err == nil {
		for _, id := range oneTime_ids {
			RunOneTimeTask(id)
		}
	}
}

// Apply the patch to the project on the given branch.
func CommitPatch(task *db.Task, branch_name string) error {
	clone_path := fmt.Sprintf("%s/%d", projects_path, task.Id)

	// clone branch where to commit patch
	clone_cmd := exec.Command("git", "clone",
		// clone URL
		fmt.Sprintf("https://%s@github.com/%s.git", task.User.Token,
			task.Project.Name),
		// default branch
		"--branch", branch_name,
		// clone only default branch
		"--single-branch",
		// target directory
		clone_path)
	if out, err := clone_cmd.CombinedOutput(); err != nil {
		log.Println(string(out))
		return PatchFailure
	}
	defer os.RemoveAll(clone_path)

	// apply patch
	patch_file, err := filepath.Abs(fmt.Sprintf("%s/%s", patches_path,
		task.Patch))
	if err != nil {
		return PatchFailure
	}
	patch_cmd := exec.Command("git", "am", patch_file)
	patch_cmd.Dir = clone_path
	if out, err := patch_cmd.CombinedOutput(); err != nil {
		log.Println(string(out))
		return PatchFailure
	}

	// push changes
	push_cmd := exec.Command("git", "push")
	push_cmd.Dir = clone_path
	if out, err := push_cmd.CombinedOutput(); err != nil {
		log.Println(string(out))
		return PatchFailure
	}

	return nil
}
