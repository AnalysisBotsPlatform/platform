// Basic datatypes
package db

import (
	"time"
)

//
// ## Constants ##
//

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


// Github Events
const (
	wildcard                    = iota //0
	commit_comment              = iota
	create                      = iota
	delete                      = iota
	deployment                  = iota
	deployment_status           = iota //5
	fork                        = iota
	gollum                      = iota
	issue_comment               = iota
	issues                      = iota
	member                      = iota //10
	page_build                  = iota
	public                      = iota
	pull_request_review_comment = iota
	pull_request                = iota 
	push                        = iota //15
	release                     = iota
	status                      = iota
	team_add                    = iota 
	watch                       = iota
)

// user friendly names of the GitHub events
var Event_names = {
	"Every Event",
	"Commit Comment",
	"Create",
	"Delete",
	"Deployment",
	"Deployment Status",
	"Fork",
	"Gollum",
	"Issue Comment",
	"Issues",
	"Member",
	"Page Build",
	"Public",
	"Pull Request Review Comment",
	"Pull Request",
	"Push",
	"Release",
	"Status",
	"Team Add",
	"Watch",
}

// Statuses of a scheduled, event or one time task
const (
	Active   = iota
	Complete = iota
)

// Trigger for a task
const (
	Hourly  = iota // every hour
	Daily   = iota // every day
	Weekly  = iota // every week
	Unique  = iota // just once
	Instant = iota // immediately
	Event   = iota // when event occurs
)

//
// ## Data Structures ##
//

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

// abstract super class of the tasks
type group_task struct {
	id      int64
	user    *User
	project *Project
	bot     *Bot
}

// A task is a bot's execution on a project
type Task struct {
	Id          int64
	Gid         int64
	User        *User
	Project     *Project
	Bot         *Bot
	Start_time  *time.Time
	End_time    *time.Time
	Status      int64
	Exit_status int64
	Output      string
	Patch       string
}

// Scheduled task
type ScheduledTask struct {
	Id      int64
	User    *User
	Project *Project
	Bot     *Bot
	Name    string
	Status  int64
	Next    time.Time
	Cron    string
}

// Scheduled task with its executions 
type ScheduledTaskInstances struct {
	Task        *ScheduledTask
	Child_tasks []*Task
}

// One time task
type OneTimeTask struct {
	Id        int64
	User      *User
	Project   *Project
	Bot       *Bot
	Name      string
	Status    int64
	Exec_time time.Time
}

// One time with its executions
type OneTimeTaskInstances struct {
	Task        *OneTimeTask
	Child_tasks []*Task
}

// Instant task
type InstantTask struct {
	Id      int64
	User    *User
	Project *Project
	Bot     *Bot
}

// Instant task with its instances
type InstantTaskInstances struct {
	Task        *InstantTask
	Child_tasks []*Task
}

// Event task
type EventTask struct {
	Id      int64
	User    *User
	Project *Project
	Bot     *Bot
	Name    string
	Status  int64
	Event   int64
	HookId  int64
}

// Even task with its executions
type EventTaskInstances struct {
	Task        *EventTask
	Child_tasks []*Task
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

//
// ## Helper Functions ##
//

// Converts the status of a task to the corresponding string representation
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

// Converts the status from an int to the corresponding string representation
func task_group_status_string(status int64) string {
	switch {
	case status == Active:
		return "Active"
	case status == Complete:
		return "Complete"
	default:
		return "Ups! This should not happen ..."
	}
}

// Converts the status of a task to the corresponding string representation
func (t *ScheduledTask) StatusString() string {
	return task_group_status_string(t.Status)
}

// Checks if the task is active
func (t *ScheduledTask) IsActive() bool {
	return t.Status == Active
}

// Checks if the task is complete
func (t *ScheduledTask) IsComplete() bool {
	return t.Status == Complete
}

// Converts the status of a task to the corresponding string representation
func (t *EventTask) StatusString() string {
	return task_group_status_string(t.Status)
}

// Checks if the task is active
func (t *EventTask) IsActive() bool {
	return t.Status == Active
}

// Checks if the task is complete
func (t *EventTask) IsComplete() bool {
	return t.Status == Complete
}

// Converts the status of a task to the corresponding string representation
func (t *OneTimeTask) StatusString() string {
	return task_group_status_string(t.Status)
}

// Checks if the task is active
func (t *OneTimeTask) IsActive() bool {
	return t.Status == Active
}

// Checks if the task is complete
func (t *OneTimeTask) IsComplete() bool {
	return t.Status == Complete
}

// Converts the event of a task to the corresponding string representation
func (t *EventTask) EventString() string {
	switch {
	case t.Event == wildcard:
		return "*"
	case t.Event == commit_comment:
		return "commit_comment"
	case t.Event == create:
		return "create"
	case t.Event == delete:
		return "delete"
	case t.Event == deployment:
		return "deployment"
	case t.Event == deployment_status:
		return "deployment_status"
	case t.Event == fork:
		return "fork"
	case t.Event == gollum:
		return "gollum"
	case t.Event == issue_comment:
		return "issue_comment"
	case t.Event == issues:
		return "issues"
	case t.Event == member:
		return "member"
	case t.Event == membership:
		return "membership"
	case t.Event == page_build:
		return "page_build"
	case t.Event == public:
		return "public"
	case t.Event == pull_request_review_comment:
		return "pull_request_review_comment"
	case t.Event == pull_request:
		return "pull_request"
	case t.Event == push:
		return "push"
	case t.Event == repository:
		return "repository"
	case t.Event == release:
		return "release"
	case t.Event == status:
		return "status"
	case t.Event == team_add:
		return "team_add"
	case t.Event == watch:
		return "watch"
	default:
		return "Ups! This should not happen ..."
	}
}

// Check if the task is pending
func (t *Task) IsPending() bool {
	return t.Status == Pending
}

// Check if the task is scheduled
func (t *Task) IsScheduled() bool {
	return t.Status == Scheduled
}

// Check if the task is running
func (t *Task) IsRunning() bool {
	return t.Status == Running
}

// Check if the task is canceled
func (t *Task) IsCanceled() bool {
	return t.Status == Canceled
}

// Check if the task is succeeded
func (t *Task) IsSucceeded() bool {
	return t.Status == Succeeded
}

// Check if the task is failed
func (t *Task) IsFailed() bool {
	return t.Status == Failed
}
