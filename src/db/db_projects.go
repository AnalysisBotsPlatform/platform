package db

import (
	"fmt"
	//"bytes"
	"log"
	"errors"
)

// Queries for the Projects Table
// (id, name, uid, clone_url, fs_path)

// INSERT AN NEW PROJECT

func CreateProject(name string, token string, clone_url string, fs_path string) *Project {
	var last_id int
	var uid int
	var owner User
	
	if name == "" || token == "" || clone_url == "" || fs_path == "" {
		err := errors.New("Atleast one argument is empty, null or zero.")
		fmt.Println("ERROR function CreateProject:", err)
	} else {
		err1 := db.QueryRow("SELECT id FROM users WHERE token = $1", token).Scan(&uid)
		if err1 != nil {
			log.Fatal(err)
		}
		err2 := db.QueryRow("INSERT INTO projects (name, uid, clone_url, fs_path) VALUES ($1, $2, $3, $4) RETURNING id", name, uid, clone_url, fs_path).Scan(&last_id)
		if err2 != nil {
			log.Fatal(err) 
		}
		owner = db.GetUserById(uid)
	}
	return &Project{Id: last_id, Name: name, Owner: owner, Clone_url: clone_url, Fs_path: fs_path}
}

// GET PROJECTS BY USER TOKEN

func GetProjectsByToken(token string) []Project {
	var projects []Project
	
	if token == "" {
		err := errors.New("The User's token must be not empty")
		fmt.Println("ERROR function GetProjectsByToken:", err)
	} else {
		uid := db.GetUserByToken(token).Id
		rows, err := db.Query("SELECT * FROM projects WHERE uid = &1 ", uid)
		if err != nil {
			log.Fatal(err)
		}
		for rows.Next(){
			var id int
			var name string
			var clone_url string
			var fs_path string
			
			if err := rows.Scan(&id, &name, &uid, &clone_url, &fs_path); err != nil {
            	log.Fatal(err)
       		}
       		
       		owner := db.GetUserByToken(token)
			project := &Project{Id: id, Name: name, Owner: owner, Clone_url: clone_url, Fs_path: fs_path} 
			
			projects = append(projects, project)
		}
		if err := rows.Err(); err != nil {
        	log.Fatal(err)
		}
	}
	return projects
}

// GET PROJECT BY TOKEN AND PID

func GetProjectByTokenPid(token string, pid int) *Project {
	var project Project
	
	if token == "" || pid == 0 {
		err := errors.New("The User's token must be not empty and process id not zero")
		fmt.Println("ERROR function GetProjectByTokenPid:", err)
	} else {
		uid := db.GetMemberByToken(token).User.Id
		owner := db.GetUserByToken(token)
		project.Owner = owner
		err := db.QueryRow("SELECT id, name, clone_url, fs_path FROM projects WHERE id = $1 AND uid = $2", pid, uid).Scan(&project.Id, &project.name, &project.clone_url, &project.fs_path)
		if err != nil {
			log.Fatal(err) 
		}
	}
	return &project
}

// UPDATE COLUMN NAME

// UPDATE COLUMN UID

// UPDATE COLUMN CLONE_URL

// UPDATE COLUMN FS_PATH