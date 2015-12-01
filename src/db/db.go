package db

import (
    "database/sql"
    "fmt"
    "log"
    "github.com/lib/pq"
)

func openDB() *sql.DB {
	db, err := sql.Open("analysisbot", "user=jannisdikeoulias dbname=postgres sslmode=disable")
	if err != nil {
		log.Fatal(err) 
	}
	
	return db
}

func closeDB(db *sql.DB){
	defer db.Close()
}

func printError(err error){
	if err, ok := err.(*pq.Error); ok {
   		fmt.Println("pq error:", err.Code.Name())
	}
}

func printRows(rows *sql.Rows){
	
}

func main() {

	db := openDB() 
	
	pw := "root"
	rows, err := db.Query("SELECT username FROM users WHERE password = $1", pw)
	if err != nil {
		log.Fatal(err) 
	}
	
	for rows.Next() {
        var username string
        if err := rows.Scan(&username); err != nil {
                log.Fatal(err)
        }
       fmt.Printf("%s password is: %s\n", username, pw)
	}
	if err := rows.Err(); err != nil {
        log.Fatal(err)
	}
	
	printError(err)
	
}
