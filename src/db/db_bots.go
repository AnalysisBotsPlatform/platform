package db

import (
	"fmt"
	"bytes"
	"log"
	"errors"
	"database/sql"
)

var db *sql.DB = OpenDB()

// Queries for the Bots Table
// (id, name, description, tags, fs_path)

// INSERT A NEW BOT

func createBot(bot *Bot) int {
	var last_id int = 0
	if bot == nil {
		err := errors.New("The bot must be not nil")
		fmt.Println("ERROR function createBot:", err)
	} else {
		query := "INSERT INTO bots (name, description, tags, fs_path) VALUES ($1, $2, $3, $4) RETURNING last_id"
		db.QueryRow(query, bot.Name, bot.Description, bot.Tags, bot.Fs_path)
	}
	return last_id
}

// SELECT AN EXISTING BOT

func getBot(botId int) Bot {
	bot := Bot{Id: botId}
	if botId == 0 {
		err := errors.New("The bot id must be greater 0")
		fmt.Println("ERROR function getBot:", err)
	} else {
		query := "SELECT * FROM bots WHERE id = $1"
		rows, err := db.Query(query, botId)
		if err != nil {
			log.Fatal(err) 
		}
		for rows.Next(){
	   		if err := rows.Scan(&bot.Id, &bot.Name, &bot.Description, &bot.Tags, &bot.Fs_path); err != nil {
                log.Fatal(err)
       		}
		}
		if err := rows.Err(); err != nil {
        	log.Fatal(err)
		}
	}
	return bot
}

// UPDATE COLUMN NAME

func setBotName(botId int, name string) {
	query := "UPDATE bots SET name = $2 WHERE id = $1"
	_, err := db.Exec(query, botId, name)
	if err != nil {
		log.Fatal(err) 
	}
}

// UPDATE COLUMN DESCRIPTION

func setBotDescription(botId int, description string) {
	query := "UPDATE bots SET description = $2 WHERE id = $1"
	_, err := db.Exec(query, botId, description)
	if err != nil {
		log.Fatal(err) 
	}
}

// UPDATE COLUMN TAGS

func setBotTags(botId int, tags []string) {
	var buffer bytes.Buffer
	for _, element := range tags {
		buffer.WriteString(element)
	}
	str := buffer.String()
	query := "UPDATE bots SET tags = $2 WHERE id = $1"
	_, err := db.Exec(query, botId, str)
	if err != nil {
		log.Fatal(err) 
	}
}

// UPDATE COLUMN FS_PATH

func setBotFSpath(botId int, fs_path string) {
	query := "UPDATE bots SET fs_path = $2 WHERE id = $1"
	_, err := db.Exec(query, botId, fs_path)
	if err != nil {
		log.Fatal(err) 
	}
}