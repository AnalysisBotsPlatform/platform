package db

import (
	"fmt"
	//"bytes"
	"log"
	"errors"
	"time"
)

// Queries for the Tasks Table
// (id, uid, pid, bid, rid, start_time, end_time, status, exit_status)

// INSERT A NEW TASK

func CreateTask(token string, pid int, bid int) *Task {
	var last_id int
	var status int
	var exit_status int
	var project Project
	var user User
	var bot Bot
	var result Result
	
	if token == "" || pid == 0 || bid == 0 {
		err := errors.New("Atleast one argument is empty, null or zero.")
		fmt.Println("ERROR function CreateTask:", err)
	} else {
		err1 := db.QueryRow("SELECT id FROM users WHERE token = $1", token).Scan(&uid)
		if err1 != nil {
			log.Fatal(err)
		}
		result = db.CreateResult("")
		rid = result.Id
		err2 := db.QueryRow("INSERT INTO tasks (uid, pid, bid, rid) VALUES ($1, $2, $3, $4) RETURNING id,status,exit_status", uid, pid, bid, rid).Scan(&last_id, &status, &exit_status)
		if err2 != nil {
			log.Fatal(err) 
		}
		
		project = db.GetProjectById(pid)
		user = db.GetUserById(uid)
		bot = db.GetBotById(bid)	
	}
	return &Task{Id: last_id, Project: project, User: user, Bot: bot, Start_time: start_time, End_time: end_time, Status: status, Exit_status: exit_status, Result: result}
}

// GET TASKS BY USER TOKEN

func GetTasksByToken(token string) []Task {
	var tasks []Task
	
	if token == "" {
		err := errors.New("The User's token must be not empty")
		fmt.Println("ERROR function GetTasksByToken:", err)
	} else {
		user := db.GetUserByToken(token)
		uid := user.Id
		rows, err := db.Query("SELECT * FROM tasks WHERE uid = $1", uid)
		if err != nil {
			log.Fatal(err)
		}
		for rows.Next(){
			var id int
			var pid int
			var bid int
			var rid int
			var stime time.Time
			var etime time.Time
			var status int
			var exit_status int
			
	   		if err := rows.Scan(&id, &uid, &pid, &bid, &rid, &stime, &etime, &status, &exit_status); err != nil {
            	log.Fatal(err)
       		}
       	
       		project := db.GetProjectById(pid)
       		user := db.GetUserByToken(token)
       		bot := db.GetBotById(bid)
       		result := db.GetResultById(rid)
       	
       		task := Task{Id: id, Project: project, User: user, Bot: bot, Start_time: stime, End_time: etime, Status: status, Exit_Status: exit_status, Result: result}     	
       		tasks = append(tasks, task)
		}
		if err := rows.Err(); err != nil {
        	log.Fatal(err)
		}
	}
	return tasks
}

// GET TASK BY ID

func GetTaskById(tid int) *Task {
	var id int
	var uid int
	var pid int
	var bid int
	var rid int
	var stime time.Time
	var etime time.Time
	var status int
	var exit_status int
	
	err := db.QueryRow("SELECT * FROM tasks WHERE id=$1", tid).Scan(&id, &uid, &pid, &bid, &rid, &stime, &etime, &status, &exit_status)
	if err != nil {
		log.Fatal(err)
	}
	
	project := db.GetProjectById(pid)
    user := db.GetUserById(uid)
    bot := db.GetBotById(bid)
    result := db.GetResultById(rid)
	
	return &Task{Id: id, Project: project, User: user, Bot: bot, Start_time: stime, End_time: etime, Status: status, Exit_Status: exit_status, Result: result}
}

// UPDATE COLUMN UID

// UPDATE COLUMN PID

// UPDATE COLUMN BID

// UPDATE COLUMN START_TIME

// UPDATE COLUMN END_TIME

// UPDATE COLUMN STATUS

// UPDATE COLUMN EXIT_STATUS