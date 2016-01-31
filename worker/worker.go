// Background worker.
package worker

import (
	"fmt"
	"github.com/AnalysisBotsPlatform/platform/db"
	"net"
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
var cancelChan chan bool

// channel to send signal for next tasks
var timeChan <-chan time.Time

// Initialization of the worker. Sets up the RPC infrastructure.
func Init(port string) error {
	api = NewWorkerAPI()
	rpc.Register(api)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return err
	}
    
    timer = time.NewTimer(1*time.Hour)
    timeChan = timer.C
    
    cancelChan = make(chan bool, 1)
    
    UpdatePeriodTimer()
    
    go runPeriodicTasks()

	go rpc.Accept(listener)

	return nil
}

// TODO document this
func CreateNewTask(parentTaskId int64) error{

    newTask, tErr := db.CreateNewChildTask(parentTaskId)
    if(tErr != nil){
        // TODO error handling
        return tErr
    }
    api.assignTask(newTask)
    return nil
}



// Cancels the running task specified by the given task id using the channel.
// Also updates the database entry accordingly.
func Cancle(tid int64) {
	
	api.cancelTask(tid)

}

// This function cancles all tasks which succeeded the 'max_task_time'
func CancleTimedOverTasks() {
	tasks, _ := db.GetTimedOverTasks(max_task_time)
	for _, e := range tasks {
		Cancle(e)
	}
}

func UpdatePeriodTimer(){
    nextExecTime, err := db.GetMinimalNextTime()
    if(err != nil){
        // TODO error handling
    }
    sleepTime := nextExecTime.Sub(time.Now())
    timer.Reset(sleepTime)
}




func runPeriodicTasks(){
    for{
        select{
            case <- timeChan:
            scheduledTasks, err := db.GetOverdueScheduledTasks(time.Now())
            if(err != nil){
                // TODO error handling
            }
            for _, task := range scheduledTasks{
                err := CreateNewTask(task.Id)
                if(err != nil){
                    continue
                }
                updateScheduleTimeAndStatus(task)
                UpdatePeriodTimer()
            }
            
            case <- cancelChan:
                return;
        }
    }
}

func ComputeDate(t time.Time, day int) time.Time{
    
    currentDay := int(t.Weekday())
    var dayDiff int
    if(day >= currentDay){
        dayDiff = day - currentDay
    }else{
        dayDiff = 7 - (currentDay - day)
    }
    
    return t.AddDate(0,0, dayDiff)
    
}


// TODO document this

// TODO error handling

func updateScheduleTimeAndStatus(task *db.ScheduledTask){
    taskType := task.Type
    tid := task.Id
    switch(taskType){
        case db.Hourly:
            hours, hErr := db.GetHourlyTaskHours(tid)
            if(hErr != nil){
                // TODO error handling
            }
            scheduledTime := task.Next
            *scheduledTime = scheduledTime.Add(time.Duration(hours)*time.Hour)
            db.UpdateNextScheduleTime(tid, scheduledTime)
            break;
        case db.Daily:
            scheduledTime := task.Next
            *scheduledTime = scheduledTime.AddDate(0, 0, 1)
            db.UpdateNextScheduleTime(tid, scheduledTime)
            break;
        case db.Weekly:
            scheduledTime := task.Next
            *scheduledTime = scheduledTime.AddDate(0, 0, 7)
            db.UpdateNextScheduleTime(tid, scheduledTime)
            break;
        
        case db.OneTime:
            db.UpdateScheduledTaskStatus(tid, db.Complete)
            break;
        
        case db.Instant:
            db.UpdateScheduledTaskStatus(tid, db.Complete)
            break;
    }
}
