package main

import (
	"db"
	"fmt"
	"time"
	//"database/sql"
)

func main(){

	// initialize structs

	user := db.User {Id: 22, Username: "jamesbond007", Realname: "James Bond", Email: "james@bond.uk", Token: "007"}
	user_id := db.CreateUser(user)
	//--> Problem mit Nebenlaeufigkeit
	project := db.Project {Name: "Skyfall", Owner: user, Clone_url: "http://project-skyfall.com", Fs_path: "github/bond/project/skyfall"}
	project_id := db.CreateProject(project)
	member := db.Member {User: user, Project: project}
	member_id := db.CreateMember(member)
	bot := db.Bot {Name: "DB-7", Description: "This bot is TOP SECRET", Tags: []string{"bond", "db7", "secret"}, Fs_path: "github/bond/bot/db-7"}
	bot_id := db.CreateBot(bot)
	result := db.Result {Output: "Mission completed"}
	result_id := db.CreateResult(result)
	task := db.Task{Project: project, User: user, Bot: bot, Start_time: time.Now(), End_time: time.Now(), Status: 0, Exit_status: 1, Result: result}
	task_id := db.CreateTask(task)
	// initialize variables
	
	
	
	
	
	
	
	
	
	
	
	// print ids
	
	fmt.Println("User_id: ", user_id)
	fmt.Println("Project_id: ", project_id)
	fmt.Println("Member_id: ", member_id)
	fmt.Println("Bot_id: ", bot_id)
	fmt.Println("Result_id: ", result_id)
	fmt.Println("Task_id: ", task_id)

}