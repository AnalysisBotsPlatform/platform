package db

import (
	"fmt"
	//"bytes"
	//"log"
	"errors"
)

// Queries for the Results Table
// (id, output)

// INSERT A NEW RESULT

func createResult(result *Result) int {
	var last_id int = 0
	if result == nil {
		err := errors.New("The result must be not nil")
		fmt.Println("ERROR function createResult:", err)
	} else {
		query := "INSERT INTO results (output) VALUES ($1) RETURNING last_id"
		db.QueryRow(query, result.Output)
	}
	return last_id
}

// SELECT AN EXISTING RESULT

// UPDATE COLUMN OUTPUT