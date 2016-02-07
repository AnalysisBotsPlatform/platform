// Background worker.
package worker

import (
	"errors"
	"fmt"
	"github.com/AnalysisBotsPlatform/platform/db"
	"log"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

// Initialization of the worker. Sets up the RPC infrastructure.
func Init(port, cache_path string) error {
	api = NewWorkerAPI()
	rpc.Register(api)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return err
	}

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
func CreateNewTask(token string, pid string, bid string) (int64, error) {
	task, err := db.CreateNewTask(token, pid, bid)
	if err != nil {
		return -1, err
	}

	api.assignTask(task)

	return task.Id, nil
}

// Cancels the running task specified by the given task id using the channel.
// Also updates the database entry accordingly.
func Cancle(tid string) error {
	id, err := strconv.ParseInt(tid, 10, 64)
	if err != nil {
		return err
	}

	api.cancelTask(id)

	return nil
}

// Perform unregister action for worker. This continues a potentially blocked
// execution of GetTask.
func DeleteWorker(worker_token string) {
	var ack bool
	api.UnregisterWorker(worker_token, &ack)
}

// This function cancles all tasks which succeeded the 'max_task_time'
func CancleTimedOverTasks() {
	tasks, _ := db.GetTimedOverTasks(max_task_time)
	for _, e := range tasks {
		Cancle(strconv.FormatInt(e, 10))
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
