package main

import (
	"db"
	"fmt"
	//"database/sql"
)

func main(){

	user := db.User{Username: "Peter", Email: "peter@mail.de", Token: "qwerty12345"}
	
	user_id := db.CreateUser(user)
	fmt.Println(user_id)

}