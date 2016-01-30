// Background worker.
package worker

import (
	"fmt"
	"github.com/AnalysisBotsPlatform/platform/db"
	"net"
	"net/rpc"
	"strconv"
)

const dummy int64 = 13

// Maximal duration in seconds for each task.
const max_task_time int64 = 60

// WorkerAPI instance used to interact with the workers.
var api *WorkerAPI

// Initialization of the worker. Sets up the RPC infrastructure.
func Init(port string) error {
	api = NewWorkerAPI()
	rpc.Register(api)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return err
	}

	go rpc.Accept(listener)

	return nil
}

// Creates a new task. This includes the following steps:
// - Creating a database entry.
// - Creating a new communication channel.
// - Starting an asynchronous task.
// The task id of the newly created task is returned.
func CreateNewTask(token string, pid string, bid string) (int64, error) {

    //TODO implement this
    
//  task, err := db.CreateNewTask(token, pid, bid)
//	if err != nil {
//		return -1, err
//	}
//
//	api.assignTask(task)

	return dummy, nil
}


func CreateNewEventTask(tid string){
    // TODO implement this
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

func UpdatePeriodTimer(){
    
}
