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

func CreateUser(username string, realname string, email string, token string) *User {
	var last_id int
	
	if token == "" {
		err := errors.New("The User's token must be not empty")
		fmt.Println("ERROR function CreateUser:", err)
	} else { 
		err := db.QueryRow("INSERT INTO users (username, realname, email, token) VALUES ($1, $2, $3, $4) RETURNING id WHERE NOT EXISTS (UPDATE users SET username=$1, realname=$2, email=$3, token=$4 WHERE id=$5)", username, realname, email, token, db.GetUserByToken.Id).Scan(&last_id)
		if err != nil {
			log.Fatal(err) 
		}
	}
	return &User{Id: last_id, Username: username, Realname: realname, Email: email, Token: token}
}

// GET USER BY TOKEN

func GetUserByToken(token string) *User {
	var user User
	
	if token == "" {
		err := errors.New("The User's token must be not empty")
		fmt.Println("ERROR function GetUserByToken:", err)
	} else { 
		err := db.QueryRow("SELECT * FROM users WHERE token = $1", token).Scan(&user.Id, &user.username, &user.realname, &user.email, &user.token)
		if err != nil {
			log.Fatal(err) 
		}
	}
	return &user
}

// GET USER BY ID

func GetUserById(uid int) *User {
	var user User
	
	if uid == 0 {
		err := errors.New("The User's id must be not zero")
		fmt.Println("ERROR function GetUserById:", err)
	} else { 
		err := db.QueryRow("SELECT * FROM users WHERE id = $1", uid).Scan(&user.Id, &user.username, &user.realname, &user.email, &user.token)
		if err != nil {
			log.Fatal(err) 
		}
	}
	return &user
}



// UPDATE COLUMN USERNAME

// UPDATE COLUMN REALNAME

// UPDATE COLUMN EMAIL

// UPDATE COLUMN TOKEN