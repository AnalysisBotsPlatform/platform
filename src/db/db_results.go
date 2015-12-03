package db

import (
	"fmt"
	//"bytes"
	"log"
	"errors"
)

// Queries for the Results Table
// (id, output)

// INSERT A NEW RESULT

func CreateResult(output string) *Result {
	var last_id int
	if false {
		err := errors.New("Atleast one argument is empty, null or zero.")
		fmt.Println("ERROR function CreateResult:", err)
	} else {
		err := db.QueryRow("INSERT INTO results (output) VALUES ($1) RETURNING id", result.Output).Scan(&last_id)
		if err != nil {
			log.Fatal(err) 
		}
	}
	return &Result{Id: last_id, Output: output}
}

// GET RESULT BY ID

func GetResultById() *Result {
	var result Result
	
	return &result
}

// UPDATE COLUMN OUTPUT