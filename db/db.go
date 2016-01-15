package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/lib/pq"
	"strconv"
	"strings"
)

/*
The import "pq" is a pure Go postgres driver for Go's database/sql package
See also: https://github.com/lib/pq
*/
var db *sql.DB //pq.Conn

/*
This function connects with the database through the pq driver - otherwise it returns an error
Parameters like username and password must be provided by the db owner
Further parameters shall be adapted:
dbname - the name of the database
sslmode - possible values see in the Postgres' sslmode documentation
*/
func OpenDB(user string, password string) error {
	var err error
	db, err = sql.Open("postgres",
		fmt.Sprintf("user=%s password=%s dbname=analysisbots sslmode=disable",
			user, password))
	return err
}

/*
This function is closing the open database connection 
*/
func CloseDB() {
	defer db.Close()
}

/*
This function returns an interface slice from an interface
*/
func makeSlice(in interface{}) []interface{} {
	return in.([]interface{})
}

/*
This function returns a string map from an interface
*/
func makeStringMap(in interface{}) map[string]interface{} {
	return in.(map[string]interface{})
}

/*
This function return an int64 from an interface
*/
func makeInt64(in interface{}) int64 {
	val, _ := in.(json.Number).Int64()
	return val
}

/*
This function return a string from an interface
*/
func makeString(in interface{}) string {
	if in != nil {
		return in.(string)
	} else {
		return ""
	}
}

//
// Users
//

/*
This function updates/creates an User:
If the User already exists the user will be updated
If the User does not exist the user will be created
*/
func UpdateUser(data interface{}, token string) error {
	// declarations
	values := makeStringMap(data)
	user := User{
		GH_Id:     makeInt64(values["id"]),
		User_name: makeString(values["login"]),
		Real_name: makeString(values["name"]),
		Email:     makeString(values["email"]),
		Token:     token,
	}

	// update user information
	if existsUser(user.GH_Id) {
		updateUser(&user)
	} else {
		createUser(&user)
	}

	return nil
}

/*
This function returns an User from the information provided in 
the database and the users' token
*/
func GetUser(token string) (*User, error) {
	// declarations
	user := User{}
	var user_name, real_name, email sql.NullString

	// fetch user
	if err := db.QueryRow("SELECT * FROM users WHERE token=$1", token).
		Scan(&user.Id, &user.GH_Id, &user_name, &real_name, &email,
		&user.Token); err != nil {
		return nil, err
	}

	//set remaining fields
	if user_name.Valid {
		user.User_name = user_name.String
	}
	if real_name.Valid {
		user.Real_name = real_name.String
	}
	if email.Valid {
		user.Email = email.String
	}

	return &user, nil
}

/*
This function returns an User from the information provided in
the database and specified by the users' token as well as his id
*/
func getUser(uid string, token string) (*User, error) {
	// declarations
	user := User{}
	var user_name, real_name, email sql.NullString

	// fetch user and verify token
	if err := db.QueryRow("SELECT * FROM users WHERE id=$1 AND token=$2", uid,
		token).Scan(&user.Id, &user.GH_Id, &user_name, &real_name, &email,
		&user.Token); err != nil {
		return nil, err
	}

	// set remaining fields
	if user_name.Valid {
		user.User_name = user_name.String
	}
	if real_name.Valid {
		user.Real_name = real_name.String
	}
	if email.Valid {
		user.Email = email.String
	}

	return &user, nil
}

/*
This function checks whether an user exists specified by his github id
*/
func existsUser(gh_id int64) bool {
	err := db.QueryRow("SELECT gh_id FROM users WHERE gh_id = $1", gh_id).
		Scan(&gh_id)
	return err != sql.ErrNoRows
}

/*
This function updates the user information 
provided by the User struct in the database
*/
func updateUser(user *User) {
	// update user information
	db.QueryRow("UPDATE users SET username=$1, realname=$2, email=$3, token=$4"+
		" WHERE gh_id=$5", user.User_name, user.Real_name, user.Email,
		user.Token, user.GH_Id)
}

/*
This function creates an user by storing the information
provided by the User struct in the database
*/
func createUser(user *User) {
	// create user
	db.QueryRow("INSERT INTO users (gh_id, username, realname, email, token)"+
		" VALUES ($1, $2, $3, $4, $5)", user.GH_Id, user.User_name,
		user.Real_name, user.Email, user.Token)
}

//
// Projects
//

/*
This function updates an existing Project
If the Project does not exist a new Project will be created 
If a Project has no related member it will be deleted
*/
func UpdateProjects(values interface{}, token string) ([]*Project, error) {
	// declarations
	projects := make([]*Project, len(makeSlice(values)))
	var uid int64
	if user, err := GetUser(token); err != nil {
		return nil, err
	} else {
		uid = user.Id
	}
	identifier := make(map[int64]bool)

	// update or insert projects
	for i, value := range makeSlice(values) {
		entry := makeStringMap(value)
		gh_id := makeInt64(entry["id"])
		identifier[gh_id] = true
		project := Project{
			GH_Id:     gh_id,
			Name:      makeString(entry["full_name"]),
			Clone_url: makeString(entry["html_url"]),
		}

		if existsProject(project.GH_Id) {
			if err := updateProject(&project, uid); err != nil {
				return nil, err
			}
		} else {
			if err := createProject(&project, uid); err != nil {
				return nil, err
			}
		}

		projects[i] = &project
	}

	// delete projects that no longer exist
	rows, err := db.Query("SELECT gh_id, pid, uid FROM projects"+
		" INNER JOIN members ON projects.id=members.pid WHERE uid=$1", uid)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var gh_id, pid int64
	for rows.Next() {
		if err := rows.Scan(&gh_id, &pid, &uid); err != nil {
			return nil, err
		}
		if _, ok := identifier[gh_id]; !ok {
			db.QueryRow("DELETE FROM members WHERE uid=$1 AND pid=$2", uid, pid)
			var count int64 = -1
			if err := db.QueryRow("SELECT count(*) FROM members WHERE pid=$1",
				pid).Scan(&count); err == nil && count == 0 {
				db.QueryRow("DELETE FROM projects WHERE id=$1", pid)
			}
		}
	}

	return projects, nil
}

/*
This function returns a Project specified by the project id and the user token
The Project struct will be filled with the project's information stored in the database
*/
func GetProject(pid string, token string) (*Project, error) {
	// declarations
	project := Project{}
	var name, clone_url, fs_path sql.NullString
	var owner sql.NullInt64

	// fetch project and verify token
	if err := db.QueryRow("SELECT projects.*, users.token FROM projects"+
		" INNER JOIN members ON projects.id=members.pid"+
		" INNER JOIN users ON members.uid=users.id"+
		" WHERE projects.id=$1 AND users.token=$2", pid, token).
		Scan(&project.Id, &project.GH_Id, &name, &owner, &clone_url, &fs_path,
		&token); err != nil {
		return nil, err
	}

	// set remaining fields
	if name.Valid {
		project.Name = name.String
	}
	if clone_url.Valid {
		project.Clone_url = clone_url.String
	}
	if fs_path.Valid {
		project.Fs_path = fs_path.String
	}
	// TODO fetch owner data

	return &project, nil
}

/*
This function checks whether a Project exists for the given Github ID
*/
func existsProject(gh_id int64) bool {
	err := db.QueryRow("SELECT gh_id FROM projects WHERE gh_id = $1", gh_id).
		Scan(&gh_id)
	return err != sql.ErrNoRows
}

/*
This function fetches the project's information from the database and stores it
in the provided Project - Related members will be updated
*/
func fillProject(project *Project, uid int64) error {
	// declarations
	var name, clone_url, fs_path sql.NullString
	var owner sql.NullInt64

	// fetch project information
	if err := db.QueryRow("SELECT * FROM projects WHERE gh_id=$1",
		project.GH_Id).Scan(&project.Id, &project.GH_Id, &name, &owner,
		&clone_url, &fs_path); err != nil {
		return err
	}

	// set remaining fields
	if name.Valid {
		project.Name = name.String
	}
	if clone_url.Valid {
		project.Clone_url = clone_url.String
	}
	if fs_path.Valid {
		project.Fs_path = fs_path.String
	}

	// update member relation
	if err := db.QueryRow("SELECT * FROM members WHERE uid=$1 AND pid=$2", uid,
		project.Id).Scan(&uid, &project.Id); err == sql.ErrNoRows {
		db.QueryRow("INSERT INTO members VALUES ($1, $2)", uid, project.Id)
	}

	return nil
}

/*
This function updates the Project referenced by the users' id in the database 
with the information given by the Project structure
*/
func updateProject(project *Project, uid int64) error {
	// update project information
	db.QueryRow("UPDATE projects SET name=$1, clone_url=$2 WHERE gh_id=$4",
		project.Name, project.Clone_url, project.GH_Id)

	if err := fillProject(project, uid); err != nil {
		return err
	}

	return nil
}

/*
This function inserts a new Project into the database with the information given by 
the Project structure and sets a reference to the user specified by his id
*/
func createProject(project *Project, uid int64) error {
	// create project
	db.QueryRow("INSERT INTO projects (gh_id, name, clone_url)"+
		" VALUES ($1, $2, $3)", project.GH_Id, project.Name, project.Clone_url)

	if err := fillProject(project, uid); err != nil {
		return err
	}

	return nil
}

//
// Bots
//

/*
This function returns all bots from the database
*/
func GetBots() ([]*Bot, error) {
	//declarations
	var bots []*Bot
	rows, err := db.Query("SELECT * FROM bots")
	if err != nil {
		return nil, err
	}

	// fetch bots
	defer rows.Close()
	for rows.Next() {
		bot := Bot{}
		var description, tags, fs_path sql.NullString

		err := rows.Scan(&bot.Id, &bot.Name, &description, &tags, &fs_path)
		if err != nil {
			return nil, err
		}

		if description.Valid {
			bot.Description = description.String
		}
		if tags.Valid {
			bot.Tags = strings.Split(tags.String[1:len(tags.String)-1], ",")
		}
		if fs_path.Valid {
			bot.Fs_path = fs_path.String
		}

		bots = append(bots, &bot)
	}

	return bots, nil
}

/*
This function returns the bot specified by the bot's id
*/
func GetBot(bid string) (*Bot, error) {
	// declarations
	bot := Bot{}
	var description, tags, fs_path sql.NullString

	err := db.QueryRow("SELECT * FROM bots WHERE id=$1", bid).
		Scan(&bot.Id, &bot.Name, &description, &tags, &fs_path)
	if err != nil {
		return nil, err
	}

	if description.Valid {
		bot.Description = description.String
	}
	if tags.Valid {
		bot.Tags = strings.Split(tags.String[1:len(tags.String)-1], ",")
	}
	if fs_path.Valid {
		bot.Fs_path = fs_path.String
	}

	return &bot, nil
}

//
// Tasks
//

/*
This function returns all tasks from the database specified by the users' token
*/
func GetTasks(token string) ([]*Task, error) {
	//declarations
	var tasks []*Task
	rows, err := db.Query("SELECT tasks.id, users.token FROM tasks"+
		" INNER JOIN users ON tasks.uid=users.id WHERE users.token=$1", token)
	if err != nil {
		return nil, err
	}

	// fetch bots
	defer rows.Close()
	for rows.Next() {
		var tid string
		if err := rows.Scan(&tid, &token); err != nil {
			return nil, err
		}
		task, err := GetTask(tid, token)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

/*
This function returns the task specified by the tasks' id and the users' token
*/
func GetTask(tid string, token string) (*Task, error) {
	// declarations
	task := Task{}
	var pid, uid, bid int64
	var start_time, end_time pq.NullTime
	var exit_status sql.NullInt64
	var output sql.NullString

	// fetch task entry for tid
	if err := db.QueryRow("SELECT * FROM tasks WHERE id=$1", tid).
		Scan(&task.Id, &uid, &pid, &bid, &start_time, &end_time, &task.Status,
		&exit_status, &output); err != nil {
		return nil, err
	}

	// fetch user and verify token
	user, err := getUser(strconv.FormatInt(uid, 10), token)
	if err != nil {
		return nil, err
	}
	task.User = user

	// fetch project
	project, err := GetProject(strconv.FormatInt(pid, 10), token)
	if err != nil {
		return nil, err
	}
	task.Project = project

	// fetch bot
	bot, err := GetBot(strconv.FormatInt(bid, 10))
	if err != nil {
		return nil, err
	}
	task.Bot = bot

	// set remaining fields
	if start_time.Valid {
		task.Start_time = &start_time.Time
	}
	if end_time.Valid {
		task.End_time = &end_time.Time
	}
	if exit_status.Valid {
		task.Exit_status = exit_status.Int64
	}
	if output.Valid {
		task.Output = output.String
	}

	return &task, nil
}
