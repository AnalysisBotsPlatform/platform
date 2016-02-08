// Background worker.
package worker

import (
	"fmt"
	"github.com/AnalysisBotsPlatform/platform/db"
	"github.com/gorhill/cronexpr"
	"net"
    "errors"
	"net/rpc"
    "time"
)

const dummy int64 = 13

// Maximal duration in seconds for each task.
const max_task_time int64 = 60

// WorkerAPI instance used to interact with the workers.
var api *WorkerAPI

// ticker to coordinate periodic tasks
var timer *time.Timer

// channel to cancel period runner
var pauseChan chan bool

var runningTasks map[int64]chan bool



// Initialization of the worker. Sets up the RPC infrastructure.
func Init(port string) error {
	api = NewWorkerAPI()
	rpc.Register(api)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return err
	}

	pauseChan = make(chan bool)
	runningTasks = make(map[int64]chan bool)


	go rpc.Accept(listener)

	return nil
}

// TODO document this
func CreateNewTask(parentTaskId int64) error{

    newTask, tErr := db.CreateNewChildTask(parentTaskId)
    if(tErr != nil){
        fmt.Printf("\tError Create Task for Parent Task (%d): %s\n", parentTaskId, tErr.Error())
        return tErr
    }
    api.assignTask(newTask)
    return nil
}


func RunScheduledTask(stid int64){
	cancelChan := make(chan bool, 1)
	runningTasks[stid] = cancelChan
	go runScheduledTask(stid, cancelChan)
}


func RunOneTimeTask(otid int64){
	cancelChan := make(chan bool)
	runningTasks[otid] = cancelChan
	go runOneTimeTask(otid, cancelChan)
}

// ############################
// TODO
// cancel GroupTasks
// ############################

func CancelScheduledTask(stid int64) error{
    fmt.Println("\tWorker Cancel Scheduled Task")
	cancelChan, ok := runningTasks[stid]
    if(ok){
        cancelChan <- true
    } else{
        return errors.New("The provided id does not correspond to a running task.")
    }
	err := db.UpdateScheduledTaskStatus(stid, db.Complete)
	runningChildren, gErr := db.GetRunningChildren(stid)
	if(gErr != nil){
		return gErr
	}

	for _, childTask := range runningChildren{
		cancel(childTask.Id)
	}

	return err
}

func CancelOneTimeTask(stid int64) error{
	cancelChan, ok := runningTasks[stid]
    if(ok){
        cancelChan <- true
    } else{
        return errors.New("The provided id does not correspond to a running task.")
    }
	err := db.UpdateOneTimeTaskStatus(stid, db.Complete)
	runningChildren, gErr := db.GetRunningChildren(stid)
	if(gErr != nil){
		return gErr
	}

	for _, childTask := range runningChildren{
		cancel(childTask.Id)
	}

	return err
}


func CancelEventTask(stid int64) error{
	err := db.UpdateEventTaskStatus(stid, db.Complete)
	runningChildren, gErr := db.GetRunningChildren(stid)
	if(gErr != nil){
		return gErr
	}

	for _, childTask := range runningChildren{
		cancel(childTask.Id)
	}

	return err
}

func CancelInstantTask(stid int64) error{
    cancelChan, ok := runningTasks[stid]
    if(ok){
        cancelChan <- true
    } else{
        return errors.New("The provided id does not correspond to a running task.")
    }
	runningChildren, gErr := db.GetRunningChildren(stid)
	if(gErr != nil){
		return gErr
	}

	for _, childTask := range runningChildren{
		cancel(childTask.Id)
	}

	return nil
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


func StopPeriodRunners(){
    close(pauseChan)
}

func runScheduledTask(stid int64, cancelChan chan bool){
    fmt.Printf("New Scheduled Task Runner (id: %d)\n", stid)
	for{
		scheduledTask, err := db.GetScheduledTask(stid)
		if(err != nil){
			db.UpdateScheduledTaskStatus(stid, db.Complete)
			return
		}
		nextTime := cronexpr.MustParse(scheduledTask.Cron).Next(time.Now())
		sleepTime := nextTime.Sub(time.Now())
		uErr := db.UpdateNextScheduleTime(scheduledTask.Id, nextTime)
		if(uErr != nil){
			db.UpdateScheduledTaskStatus(stid, db.Complete)
			return
		}
		select{
		case <- time.After(sleepTime):
            fmt.Printf("\tExecute Scheduled Task: %d\n", stid)
			CreateNewTask(stid)
		case <- cancelChan:
            fmt.Printf("\t(id: %d) Cancel Execution", stid)
			db.UpdateScheduledTaskStatus(stid, db.Complete)
			return;
		case <- pauseChan:
            fmt.Printf("\t(id: %d) Pause Execution", stid)
			return;
		}
	}
}

func runOneTimeTask(otid int64, cancelChan chan bool){
    fmt.Printf("New One Time Task Runner (id: %d)\n", otid)
	oneTimeTask, err := db.GetOneTimeTask(otid)
	if(err != nil){
        fmt.Printf("\t(id: %d) Error during retrieving Task from db: %s", otid, err.Error())
		return;
	}
    duration := oneTimeTask.Exec_time.Sub(time.Now().UTC())
    fmt.Printf("\t(id: %d) Sleep for \"%d\" nanosec (time: %s)", otid, duration, time.Now().UTC().String())
	select{
        case <- time.After(duration):
        fmt.Printf("\tExecute One Time Task: %d\n", otid)
		CreateNewTask(otid)
        db.UpdateOneTimeTaskStatus(otid, db.Complete)
	case <- cancelChan:
		return;
	case <- pauseChan:
        fmt.Printf("\t(id: %d) Pause Execution", otid)
		return;
	}
}
