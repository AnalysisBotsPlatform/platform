package db

import (
	"fmt"
	//"bytes"
	"log"
	"errors"
)

// Queries for the Members Table
// (id, uid, pid)

// INSERT A NEW MEMBER

func CreateMember(token string, pid int) *Member {
	var last_id int
	var uid int
	var user User
	var project Project
	if token == "" || pid == 0 {
		err := errors.New("Atleast one argument is empty, null or zero.")
		fmt.Println("ERROR function CreateMember:", err)
	} else {
		err1 := db.QueryRow("SELECT id FROM users WHERE token = $1", token).Scan(&uid)
		if err1 != nil {
			log.Fatal(err) 
		}
		err2 := db.QueryRow("INSERT INTO members (uid, pid) VALUES ($1, $2) RETURNING id", uid, pid).Scan(&last_id)
		if err2 != nil {
			log.Fatal(err) 
		}
		user = db.GetUserById(uid)
		project = db.GetProjectById(pid)
	}
	return &Member{Id: last_id,User: &user,Project: &project}
}

// GET MEMBER BY TOKEN

func GetMemberByToken(token string) *Member {
	var member Member

	return &member
}

// UPDATE COLUMN UID

// UPDATE COLUMN PID