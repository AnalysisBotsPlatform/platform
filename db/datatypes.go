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
	Running   = iota
	Cancelled   = iota
	Succeeded = iota
	Failed    = iota
)

// Days of the week
const (
	Monday   	= iota
	Tuesday   = iota
	Wednesday = iota
	Thursday 	= iota
	Friday   	= iota
	Saturday  = iota
	Sunday		= iota
)

// Github Events
const (
	wildcard										= iota//0
	commit_comment							= iota
	create											= iota
	delete											= iota
	deployment									= iota
	deployment_status						= iota//5
	fork												= iota
	gollum											= iota
	issue_comment								= iota
	issues											= iota
	member											= iota//10
	membership									= iota
	page_build									= iota
	public											= iota
	pull_request_review_comment	= iota
	pull_request								= iota//15
	push												= iota
	repository									= iota
	release											= iota
	status											= iota
	team_add										= iota//20
	watch												= iota
)

// Schedule Status
const (
	Active 		= iota
	Cancelled = iota
	Complete 	= iota
)

// Schedule Types
const (
	Hourly	= iota
	Daily 	= iota
	Weekly 	= iota
	Single 	= iota
	Event 	= iota
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
type ScheduledTask struct {
	Id          int64
	Name				string
	Tasks				[]*Task
	User        *User
	Project     *Project
	Bot         *Bot
	Status      int64
	Type				int64
	Event 			int64
	Next      	*time.Time
}

// A task is a bot's execution on a project
type Task struct {
	Id          	int64
	ScheduledTask *ScheduledTask
	Worker      	*Worker
	Start_time  	*time.Time
	End_time    	*time.Time
	Status      	int64
	Exit_status 	int64
	Output      	string
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
	case t.Status == Running:
		return "Running"
	case t.Status == :
		return "Cancelled"
	case t.Status == Succeeded:
		return "Succeeded"
	case t.Status == Failed:
		return "Failed"
	default:
		return "Ups! This should not happen ..."
	}
}
