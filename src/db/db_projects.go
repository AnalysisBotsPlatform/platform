package db

import (
	"fmt"
	//"bytes"
	//"log"
	"errors"
)

// Queries for the Projects Table
// (id, name, uid, clone_url, fs_path)

// INSERT A NEW PROJECT

func createProject(project *Project) int {
	var last_id int = 0
	if project == nil {
		err := errors.New("The project must be not nil")
		fmt.Println("ERROR function createProject:", err)
	} else {
		query := "INSERT INTO projects (name, uid, clone_url, fs_path) VALUES ($1, $2, $3, $4) RETURNING last_id"
		db.QueryRow(query, project.Name, project.Owner.Id, project.Clone_url, project.Fs_path)
	}
	return last_id
}

// SELECT AN EXISTING PROJECT

// UPDATE COLUMN NAME

// UPDATE COLUMN UID

// UPDATE COLUMN CLONE_URL

// UPDATE COLUMN FS_PATH