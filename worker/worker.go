// Background worker.
package worker

import (
	"fmt"
	"github.com/AnalysisBotsPlatform/platform/db"
	"net"
	"net/rpc"
	"os"
	"strconv"
)

// Maximal duration in seconds for each task.
const max_task_time int64 = 60

// Cache subdirectory where Git patches are located.
const patches_directory = "patches"

// Absolute path to patch files directory.
var patches_path string

// WorkerAPI instance used to interact with the workers.
var api *WorkerAPI

// Initialization of the worker. Sets up the RPC infrastructure.
func Init(port, cache_path string) error {
	api = NewWorkerAPI()
	rpc.Register(api)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return err
	}

	go rpc.Accept(listener)

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

// This function cancles all tasks which succeeded the 'max_task_time'
func CancleTimedOverTasks() {
	tasks, _ := db.GetTimedOverTasks(max_task_time)
	for _, e := range tasks {
		Cancle(strconv.FormatInt(e, 10))
	}
}
