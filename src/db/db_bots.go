package db

import (
	"fmt"
	//"bytes"
	"log"
	"errors"
	"database/sql"
)

var db *sql.DB = OpenDB()

// Queries for the Bots Table
// (id, name, description, tags, fs_path)

// INSERT A NEW BOT

func CreateBot(name string, description string, tags []string, fs_path string) *Bot {
	var last_id int
	
	if name == "" || description == "" || fs_path == "" {
		err := errors.New("Atleast one argument is empty, null or zero.")
		fmt.Println("ERROR function CreateBot:", err)
	} else {
		query := "INSERT INTO bots (name, description, tags, fs_path) VALUES ($1, $2, $3, $4) RETURNING id"
		err := db.QueryRow(query, name, description, tags, fs_path).Scan(&last_id)
		if err != nil {
			log.Fatal(err) 
		}
	}
	return &Bot{Id: last_id, Name: name, Description: description, Tags: tags, Fs_path: fs_path}
}

// GET BOT BY ID

func GetBotById(bid int) *Bot {
	var bot Bot
	
	if bid == 0 {
		err := errors.New("The bot id must be not zero")
		fmt.Println("ERROR function GetBotById:", err)
	} else {
		_, err := db.Query("SELECT * FROM bots WHERE id = $1", bid).Scan(&bot.Id, &bot.Name, &bot.Description, &bot.Tags, &bot.Fs_path)
		if err != nil {
			log.Fatal(err) 
		}
	}
	return &bot
}

// GET ALL BOTS

	func GetAllBots() []Bot {
		var bots []Bot
		rows, err := db.Query("SELECT * FROM bots ORDER BY name ASC")
		for rows.Next(){
			var bot Bot
	   		if err := rows.Scan(&bot.Id, &bot.Name, &bot.Description, &bot.Tags, &bot.Fs_path); err != nil {
                log.Fatal(err)
       		}
       		bots := append(bots,bot)
		}
		if err := rows.Err(); err != nil {
        	log.Fatal(err)
		}
		return bots
	}

// SET BOT NAME

func SetBotName(bid int, name string) {
	_, err := db.Exec("UPDATE bots SET name = $2 WHERE id = $1", bid, name)
	if err != nil {
		log.Fatal(err) 
	}
}

// SET BOT DESCRIPTION

func SetBotDescription(bid int, description string) {
	_, err := db.Exec("UPDATE bots SET description = $2 WHERE id = $1", bid, description)
	if err != nil {
		log.Fatal(err) 
	}
}

// SET BOT TAGS

func SetBotTags(bid int, tags []string) {
	_, err := db.Exec("UPDATE bots SET tags = $2 WHERE id = $1", bid, tags)
	if err != nil {
		log.Fatal(err)
	}
}

// SET BOT FS_PATH

func SetBotFSpath(botId int, fs_path string) {
	_, err := db.Exec("UPDATE bots SET fs_path = $2 WHERE id = $1", botId, fs_path)
	if err != nil {
		log.Fatal(err) 
	}
}