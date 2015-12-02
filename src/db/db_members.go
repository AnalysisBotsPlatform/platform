package db

import (
	"fmt"
	//"bytes"
	//"log"
	"errors"
)

// Queries for the Members Table
// (id, uid, pid)

// INSERT A NEW MEMBER

func createMember(member *Member) int {
	var last_id int = 0
	if member == nil {
		err := errors.New("The member must be not nil")
		fmt.Println("ERROR function createMember:", err)
	} else {
		query := "INSERT INTO member (uid, pid) VALUES ($1, $2) RETURNING last_id"
		db.QueryRow(query, member.User.Id, member.Project.Id)
	}
	return last_id
}

// SELECT AN EXISTING MEMBER

// UPDATE COLUMN UID

// UPDATE COLUMN PID