package db

import (
    "database/sql"
    "fmt"
    "log"
    "lib/pq"
)

func OpenDB() *sql.DB {
	db, err := sql.Open("postgres", "user=jannisdikeoulias dbname=analysisbot sslmode=disable")
	if err != nil {
		log.Fatal(err) 
	}	
	return db
}

func CloseDB(db *sql.DB){
	defer db.Close()
}

func PrintError(err error){
	if err, ok := err.(*pq.Error); ok {
   		fmt.Println("pq error:", err.Code.Name())
	}
}