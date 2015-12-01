// Basic datatypes
package db

import (
	"time"
	"container/list"
)

// Statuses of a task
const (
	Pending = iota
	Running = iota
	Cancled = iota
	Succeeded = iota
	Failed = iota
)

// User
type User struct {
	Uid int
	User_name string
	Real_name string
	Email string
	Token string
}

// GitHub project
type Project struct {
	Pid int
	Name string
	Owner *User
	Clone_url string
	Fs_path string
}

// A task is a bot's execution on a project
type Task struct {
	Tid int
	Project *Project
	User *User
	Bot *Bot
	Start_time time.Time
	End_time time.Time
	Status int
	Exit_status int
	Result *Result
}

// Analysis bot
type Bot struct {
	Bid int
	Name string
	Description string
	Tags list.List
	Fs_path string
}

// User project relation
type Member struct {
	User *User
	Project *Project
}

// Result of a bot's execution
type Result struct {
	Rid int
	Text string
}
