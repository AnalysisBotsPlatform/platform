package db

import (
	"fmt"
	//"bytes"
	//"log"
	"errors"
)

// Queries for the Tasks Table
// (id, uid, pid, bid, start_time, end_time, status, exit_status)

// INSERT A NEW TASK

func createTask(task *Task) int {
	var last_id int = 0
	if task == nil {
		err := errors.New("The task must be not nil")
		fmt.Println("ERROR function createTask:", err)
	} else {
		query := "INSERT INTO tasks (uid, pid, bid, start_time, end_time, status, exit_status) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING last_id"
		db.QueryRow(query, task.User.Id, task.Project.Id, task.Bot.Id, task.Start_time, task.End_time, task.Status, task.Exit_status)
	}
	return last_id
}

// SELECT AN EXISTING TASK

// UPDATE COLUMN UID

// UPDATE COLUMN PID

// UPDATE COLUMN BID

// UPDATE COLUMN START_TIME

// UPDATE COLUMN END_TIME

// UPDATE COLUMN STATUS

// UPDATE COLUMN EXIT_STATUS