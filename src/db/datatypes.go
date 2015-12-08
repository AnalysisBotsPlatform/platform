// Basic datatypes
package db

import (
	"time"
)

// Statuses of a task
const (
	Pending   = iota
	Running   = iota
	Cancled   = iota
	Succeeded = iota
	Failed    = iota
)

// User
type User struct {
	Id        int64
	GH_Id     int64
	User_name string
	Real_name string
	Email     string
	Token     string
}

// GitHub project
type Project struct {
	Id        int64
	GH_Id     int64
	Name      string
	Owner     *User
	Clone_url string
	Fs_path   string
}

// Analysis bot
type Bot struct {
	Id          int64
	Name        string
	Description string
	Tags        []string
	Fs_path     string
}

// User project relation
type Member struct {
	User    *User
	Project *Project
}

// A task is a bot's execution on a project
type Task struct {
	Id          int64
	Project     *Project
	User        *User
	Bot         *Bot
	Start_time  *time.Time
	End_time    *time.Time
	Status      int64
	Exit_status int64
	Output      string
}

func (t *Task) StatusString() string {
	switch {
	case t.Status == Pending:
		return "Pending"
	case t.Status == Running:
		return "Running"
	case t.Status == Cancled:
		return "Cancled"
	case t.Status == Succeeded:
		return "Succeeded"
	case t.Status == Failed:
		return "Failed"
	default:
		return "Ups! This should not happen ..."
	}
}
