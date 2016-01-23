// Background worker.
package worker

import (
	"fmt"
	"github.com/AnalysisBotsPlatform/platform/db"
	"github.com/AnalysisBotsPlatform/platform/utils"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// Maximal duration in seconds for each task.
const max_task_time int64 = 60

// Directory (within the cache directory) where projects are found.
const projects_directory = "projects"

// Directory where the application can store temporary data on the file system.
var cache_directory string

// Length of the directory names for the cloned GitHub projects.
// NOTE: If the project gets many users, this number should be raised.
const directory_length = 8

// Channel store for the task channels. These channels are used to interact with
// the running tasks (right now only cancelation signal are sent).
var channels map[int64]chan bool

// Initialization of the worker. Sets up the channel store and the cache
// directory is set.
func Init(path_to_cache string) {
	channels = make(map[int64]chan bool)
	cache_directory = path_to_cache
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

	chn := make(chan bool, 1)
	channels[task.Id] = chn
	go runTask(task, chn)

	return task.Id, nil
}

// Cancels the running task specified by the given task id using the channel.
// Also updates the database entry accordingly.
func Cancle(tid string) error {
	id, err := strconv.ParseInt(tid, 10, 64)
	if err != nil {
		return err
	}

	defer func() {
		recover()
		db.UpdateTaskStatus(id, db.Cancled)
	}()
	if chn, ok := channels[id]; ok {
		chn <- true
		delete(channels, id)
	}
	db.UpdateTaskStatus(id, db.Cancled)

	return nil
}

// This function cancles all tasks which succeeded the 'max_task_time'
func CancleTimedOverTasks() {
	tasks, _ := db.GetTimedOverTasks(max_task_time)
	for _, e := range tasks {
		Cancle(strconv.FormatInt(e, 10))
	}
}

// Checks whether a cancelation signal was sent on the channel.
func tryReceive(chn chan bool) bool {
	select {
	case <-chn:
		return true
	default:
		return false
	}
}

// Checks if the task was canceled and updates the database if applicable.
func checkForCanclation(tid int64, chn chan bool) bool {
	if tryReceive(chn) {
		db.UpdateTaskStatus(tid, db.Cancled)
		return true
	}
	return false
}

// Waits on the channel for a cancelation action. If such an action is received,
// the corresponding process is terminated (Docker container) and the `execBot`
// function is able to continue its execution.
func waitForCanclation(returnChn, cancleChn, abortWait chan bool,
	cmd *exec.Cmd) {
	select {
	case <-cancleChn:
		cmd.Process.Kill()
		returnChn <- true
	case <-abortWait:
	}
}

// Executes the task, i.e. runs the Bot as a Docker container and waits for its
// completion. After the tasks execution terminated, the output and exit_code
// output arguments are set appropriatly. In addition a signal is sent on the
// cancelation channel which allows `waitForCancelation` to continue.
func execBot(returnChn chan bool, cmd *exec.Cmd, stdout, stderr io.ReadCloser,
	output *string, exit_code *int) {
	out, _ := ioutil.ReadAll(stdout)
	err, _ := ioutil.ReadAll(stderr)
	*output = fmt.Sprintf("Stdout:\n%s\nStderr:\n%s", out, err)
	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				*exit_code = status.ExitStatus()
			}
		}
	} else {
		*exit_code = 0
	}
	defer func() { recover() }()
	returnChn <- true
}

// Cleans up the project cache, i.e. the cloned project is removed from file
// system.
func cleanProjectCache(directory string) {
	rmDirectoryCmd := exec.Command("rm", "-rf", directory)
	if err := rmDirectoryCmd.Run(); err != nil {
		// TODO handle error
		fmt.Println(err)
	}
}

// Preperation steps:
// - Creates the project cache directory if nesessary.
// - Fetches the bot from DockerHub.
// General:
// - Creates a new clone of the repository.
// NOTE: Later this may be changed to a pull instead of clone, i.e. a cloned
// repository is reused.
// - The Bot is executed on the cloned project. This includes the creation of a
// new Docker container from the Bot's Docker image.
// - Waits for completion of the Bot's execution.
// - Cleans up project cache directory (removes clone).
// - Updates database entry accordingly (sets exit status and output)
func runTask(task *db.Task, chn chan bool) {
	defer close(chn)
	// create project cache directory if necessary
	dir := fmt.Sprintf("%s/%s", cache_directory, projects_directory)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if _, err := os.Stat(cache_directory); os.IsNotExist(err) {
			if err := os.Mkdir(cache_directory, 0755); err != nil {
				db.UpdateTaskResult(task.Id, "Cannot create cache directory!",
					-1)
				return
			}
		}
		if err := os.Mkdir(dir, 0755); err != nil {
			db.UpdateTaskResult(task.Id, "Cannot create project cache!", -1)
			return
		}
	}

	// fetch Bot from DockerHub
	dockerPullCmd := exec.Command("docker", "pull", task.Bot.Fs_path)
	if err := dockerPullCmd.Run(); err != nil {
		// NOTE This should not happen. Either docker is not available or the
		// Bot was removed from the DockerHub. One might want to invalidate the
		// Bot in case err is an ExitError.
		db.UpdateTaskResult(task.Id, fmt.Sprint(err), -1)
		return
	}
	if checkForCanclation(task.Id, chn) {
		return
	}

	// NOTE reuse cloned project
	token := task.User.Token
	name := task.Project.Name
	directory := ""
	path := ""
	exists := true
	for exists {
		path = fmt.Sprintf("%s/%s", projects_directory,
			utils.RandString(directory_length))
		directory = fmt.Sprintf("%s/%s", cache_directory, path)
		if _, err := os.Stat(directory); os.IsNotExist(err) {
			if err := os.Mkdir(directory, 0755); err != nil {
				db.UpdateTaskResult(task.Id,
					"Cannot create project target directory!", -1)
				return
			}
			exists = false
		}
	}
	gitPullCmd := exec.Command("git", "clone",
		fmt.Sprintf("https://%s@github.com/%s.git", token, name), directory)
	if err := gitPullCmd.Run(); err != nil {
		// TODO handle error
		fmt.Println(err)
	}
	defer cleanProjectCache(directory)
	if checkForCanclation(task.Id, chn) {
		return
	}

	// run Bot on Project
	botCmd := exec.Command("docker", "run", "--rm", "-v",
		fmt.Sprintf("%s:/%s:ro", directory, path), task.Bot.Fs_path, path)
	cancleChn := make(chan bool)
	abortChn := make(chan bool)
	execChn := make(chan bool)
	defer close(cancleChn)
	defer close(abortChn)
	defer close(execChn)

	stdout, _ := botCmd.StdoutPipe()
	stderr, _ := botCmd.StderrPipe()
	botCmd.Start()
	db.UpdateTaskStatus(task.Id, db.Running)

	output := ""
	exit_code := 0
	go waitForCanclation(cancleChn, chn, abortChn, botCmd)
	go execBot(execChn, botCmd, stdout, stderr, &output, &exit_code)
	select {
	case <-cancleChn:
		// nop
	case <-execChn:
		abortChn <- true
		db.UpdateTaskResult(task.Id, output, exit_code)
	}
}
