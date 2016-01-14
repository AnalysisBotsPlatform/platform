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

// directory where to find the projects
//TODO check this!
const projects_directory = "projects"

// 
// TODO doc this
var cache_directory string

// length of the directory names
// NOTE: if the project gets many users, this number should be raised
// TODO check this
const directory_length = 8

// global memory for the channel
// TODO check this
var channels map[int64]chan bool

// TODO check this
// Initialization of the worker
// sets up the channel and the cache directory
func Init(path_to_cache string) {
	channels = make(map[int64]chan bool)
	cache_directory = path_to_cache
}

// TODO check this
// creats a new task by calling the database with the used information
// this information is: the authtoken, the projectIdentifier, the botIdentifier
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

// TODO check this
// Cancels the running task specified by its id using the channel
// also updates the database 
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

// checks whether the channel recieved a message
// TODO: check this
func tryReceive(chn chan bool) bool {
	select {
	case <-chn:
		return true
	default:
		return false
	}
}

// TODO: check this
// checks if the task was canceled and updates the db if applicable
func checkForCanclation(tid int64, chn chan bool) bool {
	if tryReceive(chn) {
		db.UpdateTaskStatus(tid, db.Cancled)
		return true
	}
	return false
}

// TODO check this
// waits for the channel to report to cancel the process cmd
// kills the cmd if message received
func waitForCanclation(returnChn, cancleChn, abortWait chan bool,
	cmd *exec.Cmd) {
	select {
	case <-cancleChn:
		cmd.Process.Kill()
		returnChn <- true
	case <-abortWait:
	}
}

// TODO document this
//
//
//
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

// TODO check this
//  cleans up the project cache
func cleanProjectCache(directory string) {
	rmDirectoryCmd := exec.Command("rm", "-rf", directory)
	if err := rmDirectoryCmd.Run(); err != nil {
		// TODO handle error
		fmt.Println(err)
	}
}

// TODO check this
// creates the project cache directory if nesessary
// fetches the bot from DockerHub using exec.command
// generally creates a new clone of the repository and works on its
// NOTE: this may be changed
// calls the cleanProjectCache to clean it up
// runs the bot on project, also updates the database
func runTask(task *db.Task, chn chan bool) {
	defer close(chn)
	// create project cache directory if necessary
	dir := fmt.Sprintf("%s/%s", cache_directory, projects_directory)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
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
		fmt.Println(err)
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
				db.UpdateTaskResult(task.Id, "Cannot create project cache!", -1)
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
		// TODO clean up container
	case <-execChn:
		abortChn <- true
		db.UpdateTaskResult(task.Id, output, exit_code)
	}
}
