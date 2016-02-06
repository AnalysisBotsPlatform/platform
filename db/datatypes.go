// Basic datatypes
package db

import (
	"time"
)

// Token length
const Token_length = 32

// Statuses of a task
const (
	Pending   = iota
	Scheduled = iota
	Running   = iota
	Canceled  = iota
	Succeeded = iota
	Failed    = iota
)

// User
type User struct {
	Id           int64
	GH_Id        int64
	User_name    string
	Real_name    string
	Email        string
	Token        string
	Worker_token string
	Admin        bool
}

// User statistics
type User_statistics struct {
	GH_projects      int64
	Bots_used        int64
	Tasks_unfinished int64
	Tasks_total      int64
}

// API token
type API_token struct {
	Token string
	Uid   int64
	Name  string
}

// API statistics
type API_statistics struct {
	Was_accessed       bool
	Last_access        time.Time
	Interval           string
	Remaining_accesses int64
}

// GitHub project
type Project struct {
	Id        int64
	GH_Id     int64
	Name      string
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
	Worker      *Worker
	Start_time  *time.Time
	End_time    *time.Time
	Status      int64
	Exit_status int64
	Output      string
	Patch       string
}

// A worker executes tasks
type Worker struct {
	Id           int64
	Uid          int64
	Token        string
	Name         string
	Last_contact time.Time
	Active       bool
	Shared       bool
}

func (t *Task) StatusString() string {
	switch {
	case t.Status == Pending:
		return "Pending"
	case t.Status == Scheduled:
		return "Scheduled"
	case t.Status == Running:
		return "Running"
	case t.Status == Canceled:
		return "Canceled"
	case t.Status == Succeeded:
		return "Succeeded"
	case t.Status == Failed:
		return "Failed"
	default:
		return "Ups! This should not happen ..."
	}
}

func (t *Task) IsPending() bool {
	return t.Status == Pending
}

func (t *Task) IsScheduled() bool {
	return t.Status == Scheduled
}

func (t *Task) IsRunning() bool {
	return t.Status == Running
}

func (t *Task) IsCanceled() bool {
	return t.Status == Canceled
}

func (t *Task) IsSucceeded() bool {
	return t.Status == Succeeded
}

func (t *Task) IsFailed() bool {
	return t.Status == Failed
}
