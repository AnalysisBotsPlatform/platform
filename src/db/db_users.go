package db

import (
	"fmt"
	//"bytes"
	"log"
	"errors"
)

// Queries for the Users Table
// (id, username, realname, email, token)

// INSERT A NEW USER

func CreateUser(user User) int {
	var last_id int
	if &user == nil {
		err := errors.New("The user must be not nil")
		fmt.Println("ERROR function createUser:", err)
	} else {
		query := "INSERT INTO users(username, realname, email, token) VALUES ($1, $2, $3, $4) RETURNING id"
		err := db.QueryRow(query, user.Username, user.Realname, user.Email, user.Token).Scan(&last_id)
		if err != nil {
			log.Fatal(err) 
		}
	}
	return last_id
}

// SELECT AN EXISTING USER

// UPDATE COLUMN USERNAME

// UPDATE COLUMN REALNAME

// UPDATE COLUMN EMAIL

// UPDATE COLUMN TOKEN