// Database controller.
package db

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/AnalysisBotsPlatform/platform/utils"
	"github.com/lib/pq"
	"strconv"
	"strings"
	"time"
)

// The import "pq" is a pure Go postgres driver for Go's database/sql package
// See also: https://github.com/lib/pq
var db *sql.DB //pq.Conn

// This function connects with the database through the pq driver - otherwise it
// returns an error
// Parameters like username and password must be provided by the db owner
// Further parameters shall be adapted:
// dbname - the name of the database
// sslmode - possible values see in the Postgres' sslmode documentation
func OpenDB(host, user, password, name string) error {
	var err error
	db, err = sql.Open("postgres",
		fmt.Sprintf("host=%s user=%s password=%s dbname=%s "+
			"sslmode=disable", host, user, password, name))
	return err
}

// This function is closing the open database connection
func CloseDB() {
	defer db.Close()
}

//
// Helper functions
//

// This function returns an interface slice from an interface
func makeSlice(in interface{}) []interface{} {
	return in.([]interface{})
}

// This function returns a string map from an interface
func makeStringMap(in interface{}) map[string]interface{} {
	return in.(map[string]interface{})
}

// This function return an int64 from an interface
func makeInt64(in interface{}) int64 {
	val, _ := in.(json.Number).Int64()
	return val
}

// This function return a string from an interface
func makeString(in interface{}) string {
	if in != nil {
		return in.(string)
	} else {
		return ""
	}
}

// Generates a sequence of random characters (`letterBytes`) of length `n` such
// that it is unique within a particular data set. Thus `db_query` must be
// passed where the sequence can be substituted in terms of `sql.QueryRow`. The
// result of the query must be empty if and only if the sequence does not appear
// in the data set. The result may only yield a single column containing an
// integer.
func nonExistingRandString(n int, db_query string) string {
	sequence := utils.RandString(n)
	var result int
	for err := db.QueryRow(db_query, sequence).Scan(&result); err !=
		sql.ErrNoRows; sequence = utils.RandString(n) {
	}
	return sequence
}

//
// Users
//

// This function updates/creates an User:
// If the User already exists the user will be updated
// If the User does not exist the user will be created
func UpdateUser(data interface{}, token string) error {
	// declarations
	values := makeStringMap(data)
	user := User{
		GH_Id:     makeInt64(values["id"]),
		User_name: makeString(values["login"]),
		Real_name: makeString(values["name"]),
		Email:     makeString(values["email"]),
		Token:     token,
		Worker_token: nonExistingRandString(Token_length,
			"SELECT 42 FROM users WHERE worker_token = $1"),
		Admin: false,
	}

	// update user information
	if existsUser(user.GH_Id) {
		updateUser(&user)
	} else {
		createUser(&user)
	}

	return nil
}

// This function returns an User from the information provided in
// the database and the users' token
func GetUser(token string) (*User, error) {
	// declarations
	user := User{}
	var user_name, real_name, email sql.NullString

	// fetch user
	if err := db.QueryRow("SELECT * FROM users WHERE token=$1", token).
		Scan(&user.Id, &user.GH_Id, &user_name, &real_name, &email,
		&user.Token, &user.Worker_token, &user.Admin); err != nil {
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

// This function returns an User from the information provided in
// the database and specified by the users' token as well as his id
func getUser(uid string, token string) (*User, error) {
	// declarations
	user := User{}
	var user_name, real_name, email sql.NullString

	// fetch user and verify token
	if err := db.QueryRow("SELECT * FROM users WHERE id=$1 AND token=$2", uid,
		token).Scan(&user.Id, &user.GH_Id, &user_name, &real_name, &email,
		&user.Token, &user.Worker_token, &user.Admin); err != nil {
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

// This function checks whether an user exists specified by his github id
func existsUser(gh_id int64) bool {
	err := db.QueryRow("SELECT gh_id FROM users WHERE gh_id = $1", gh_id).
		Scan(&gh_id)
	return err != sql.ErrNoRows
}

// This function updates the user information
// provided by the User struct in the database
func updateUser(user *User) {
	// update user information
	db.QueryRow("UPDATE users SET username=$1, realname=$2, email=$3, token=$4"+
		" WHERE gh_id=$5", user.User_name, user.Real_name, user.Email,
		user.Token, user.GH_Id)
}

// This function creates an user by storing the information
// provided by the User struct in the database
func createUser(user *User) {
	// create user
	db.QueryRow("INSERT INTO users (gh_id, username, realname, email, token, "+
		"worker_token, admin) VALUES ($1, $2, $3, $4, $5, $6, $7)", user.GH_Id,
		user.User_name, user.Real_name, user.Email, user.Token,
		user.Worker_token, user.Admin)
}

//
// Projects
//

// This function updates an existing Project
// If the Project does not exist a new Project will be created
// If a Project has no related member it will be deleted
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
			return nil, nil
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

// This function returns a Project specified by the project id and the user
// token
// The Project struct will be filled with the project's information stored in
// the database
func GetProject(pid string, token string) (*Project, error) {
	// declarations
	project := Project{}
	var name, clone_url, fs_path sql.NullString

	// fetch project and verify token
	if err := db.QueryRow("SELECT projects.*, users.token FROM projects"+
		" INNER JOIN members ON projects.id=members.pid"+
		" INNER JOIN users ON members.uid=users.id"+
		" WHERE projects.id=$1 AND users.token=$2", pid, token).
		Scan(&project.Id, &project.GH_Id, &name, &clone_url, &fs_path,
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

	return &project, nil
}

// This function checks whether a Project exists for the given Github ID
func existsProject(gh_id int64) bool {
	err := db.QueryRow("SELECT gh_id FROM projects WHERE gh_id = $1", gh_id).
		Scan(&gh_id)
	return err != sql.ErrNoRows
}

// This function fetches the project's information from the database and stores
// it in the provided Project - Related members will be updated
func fillProject(project *Project, uid int64) error {
	// declarations
	var name, clone_url, fs_path sql.NullString

	// fetch project information
	if err := db.QueryRow("SELECT * FROM projects WHERE gh_id=$1",
		project.GH_Id).Scan(&project.Id, &project.GH_Id, &name, &clone_url,
		&fs_path); err != nil {
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

// This function updates the Project referenced by the users' id in the database
// with the information given by the Project structure
func updateProject(project *Project, uid int64) error {
	// update project information
	db.QueryRow("UPDATE projects SET name=$1, clone_url=$2 WHERE gh_id=$3",
		project.Name, project.Clone_url, project.GH_Id)

	if err := fillProject(project, uid); err != nil {
		return err
	}

	return nil
}

// This function inserts a new Project into the database with the information
// given by the Project structure and sets a reference to the user specified by
// his id
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

// This function inserts a new Bot to the database unless
// it does not already exist
func AddBot(path, description, tags string) error {
	// check whether bot exists already
	err := db.QueryRow("SELECT id FROM bots WHERE name=$1", path).Scan(&path)
	if err == nil {
		return errors.New("Bot already exists!")
	}

	// escape tags
	single_tags := strings.Split(tags, ",")
	var buffer bytes.Buffer
	delim := "{"
	for _, tag := range single_tags {
		buffer.WriteString(delim + "\"" + strings.TrimSpace(tag) + "\"")
		delim = ","
	}
	buffer.WriteString("}")

	// create bot
	var result string
	if err := db.QueryRow("INSERT INTO bots (name, description, tags, fs_path)"+
		" VALUES ($1, $2, $3, $4)", path, description, buffer.String(), path).
		Scan(&result); err != nil && err != sql.ErrNoRows {
		return err
	}

	return nil
}

// This function returns all bots from the database
func GetBots() ([]*Bot, error) {
	//declarations
	var bots []*Bot
	rows, err := db.Query("SELECT * FROM bots ORDER BY id")
	if err != nil {
		return nil, err
	}

	// fetch bots
	defer rows.Close()
	for rows.Next() {
		bot := Bot{}
		var description, tags, fs_path sql.NullString

		if err := rows.Scan(&bot.Id, &bot.Name, &description, &tags, &fs_path); err != nil {
			return nil, nil
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

// This function returns the bot specified by the bot's id
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
// Workers
//

// Creates a new worker for the given user (identified by the provided
// `user_token`). Returns the identification token for the new worker or an
// error if the user is not privileged to created shared workers.
func CreateWorker(user_token, name string, shared bool) (string, error) {
	// declarations
	var uid int64
	var admin bool

	// get user
	if err := db.QueryRow("SELECT id, worker_token, admin FROM users "+
		"WHERE worker_token = $1", user_token).
		Scan(&uid, &user_token, &admin); err != nil {
		return "", err
	}

	// check permissions
	if !admin && shared {
		return "",
			errors.New("Only admins are allowed to create shared workers!")
	}

	// create worker
	token := nonExistingRandString(Token_length,
		"SELECT 42 FROM workers WHERE token = $1")
	db.QueryRow("INSERT INTO workers (uid, token, name, last_contact, active, "+
		"shared) VALUES ($1, $2, $3, now(), $4, $5)", uid, token, name, false,
		shared)

	return token, nil
}

// Sets the given worker active, i.e. the `active` flag is set and the
// `last_contact` time is updated. If the worker does not exist an error is
// returned.
func SetWorkerActive(token string) error {
	db.QueryRow("UPDATE workers SET active=true, last_contact=now() "+
		"WHERE token=$1", token)

	return nil
}

// Sets the given worker inactive, i.e. the `active` flag is unset and the
// `last_contact` time is updated. If the worker does not exist an error is
// returned.
func SetWorkerInactive(token string) error {
	db.QueryRow("UPDATE workers SET active=false, last_contact=now() "+
		"WHERE token=$1", token)

	return nil
}

// Returns the worker that corresponds to the given token. In case the token is
// invalid an error is returned.
func GetWorker(token string) (*Worker, error) {
	// declarations
	worker := Worker{}

	// get worker
	if err := db.QueryRow("SELECT * FROM workers WHERE token = $1", token).
		Scan(&worker.Id, &worker.Uid, &worker.Token, &worker.Name,
		&worker.Last_contact, &worker.Active, &worker.Shared); err != nil {
		return nil, err
	}

	return &worker, nil
}

//
// Scheduled Tasks
//

//
// TODO document this
//
func GetScheduledTasks(token string) ([]*ScheduledTask, error) {
	//declarations
	var scheduled_tasks []*ScheduledTask
	var stid 						int64

    fmt.Println("Retrieve Tasks...")

	rows, err := db.Query("SELECT scheduled_tasks.id FROM scheduled_tasks"+
		" INNER JOIN users ON scheduled_tasks.uid=users.id WHERE users.token=$1"+
		" ORDER BY scheduled_tasks.id", token)
	if err != nil {
		return nil, err
	}

    fmt.Println("Retrieved Tasks")

	// fetch tasks
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&stid); err != nil {
			return nil, nil
		}
		scheduled_task, _, err := GetScheduledTask(strconv.FormatInt(stid, 10), token, false)
		if err != nil {
			return nil, err
		}
		scheduled_tasks = append(scheduled_tasks, scheduled_task)
	}
	return scheduled_tasks, nil
}

//
// TODO document this
//
func GetScheduledTask(stid string, token string, children bool) (*ScheduledTask, []*Task, error) {
	// declarations
	scheduled_task := ScheduledTask{}
	var name 					sql.NullString
	var pid, uid, bid int64
	var status 				sql.NullInt64
	var stype 				sql.NullInt64
	var event 				sql.NullInt64
	var next 					pq.NullTime

    // TODO !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!

	var tasks []*Task

	// fetch user and verify token
	user, err := getUser(strconv.FormatInt(uid, 10), token)
	if err != nil {
		return nil, nil, err
	}
	scheduled_task.User = user

	// fetch project
	project, err := GetProject(strconv.FormatInt(pid, 10), token)
	if err != nil {
		return nil, nil, err
	}
	scheduled_task.Project = project

	// fetch bot
	bot, err := GetBot(strconv.FormatInt(bid, 10))
	if err != nil {
		return nil, nil, err
	}
	scheduled_task.Bot = bot


	return &scheduled_task, tasks, nil
}

//
// Tasks (INSERT)
//

//
// TODO document this
//
func CreateTaskGroup(uid string, pid string, bid string) (int64, error) {
	return 0, nil
}

//
// TODO document this
// NOTE DONE
//
func CreateScheduleTask(user_token string, pid string, bid string, name string, next time.Time, cron string) (*ScheduledTask, error) {
	// Check if pid, bid are integer values
	if _, err := strconv.ParseInt(pid, 10, 64); err != nil {
		return nil, err
	}
	if _, err := strconv.ParseInt(bid, 10, 64); err != nil {
		return nil, err
	}
	// Check whether the user is allowed to access the project
	if project, err := GetProject(pid, user_token); err != nil {
		return nil, err
	}
	// Retrieve user and bot information
	if user, err := GetUser(user_token); err != nil {
		return nil, err
	}
	if bot, err := GetBot(bid); err != nil {
		return nil, err
	}
	// Create Task Group
	if gid, err := CreateTaskGroup(strconv.FormatInt(user.Id, 10), pid, bid); err != nil {
		return nil, err
	}
	// Create new scheduled task
	task := ScheduleTask{
		Gid:			gid,
		User:			user,
		Project: 	project,
		Bot:			bot,
		Name:			name,
		Status:		status,
		Next:			next,
		Cron:			cron }
	// Insert into database
	if err := db.QueryRow("INSERT INTO schedule_tasks"+
		" (name, gid, status, next, cron)"+
    " VALUES ($1, $2, $3, to_timestamp($4), $5) RETURNING id",
		 name, gid, Active, next.Unix(), cron).
		Scan(&task.Id); err != nil {
    	fmt.Println("CreateScheduleTask: Error: "+err.Error())
			return nil, err
	}
	return &task, nil
}

//
// TODO document this
// NOTE DONE
//
func CreateOneTimeTask(user_token string, pid string, bid string, name string, exec_time time.Time) (*OneTimeTask, error) {
	// Check if pid, bid are integer values
	if _, err := strconv.ParseInt(pid, 10, 64); err != nil {
		return nil, err
	}
	if _, err := strconv.ParseInt(bid, 10, 64); err != nil {
		return nil, err
	}
	// Check whether the user is allowed to access the project
	if project, err := GetProject(pid, user_token); err != nil {
		return nil, err
	}
	// Retrieve user and bot information
	if user, err := GetUser(user_token); err != nil {
		return nil, err
	}
	if bot, err := GetBot(bid); err != nil {
		return nil, err
	}
	// Create Task Group
	if gid, err := CreateTaskGroup(strconv.FormatInt(user.Id, 10), pid, bid); err != nil {
		return nil, err
	}
	// Create new scheduled task
	task := OneTimeTask{
		Gid:				gid,
        Name:               name,
		User:				user,
		Project:		project,
		Bot:				bot,
		Exec_time:	exec_time }
	// Insert into database
    // TODO update database -> Name
	if err := db.QueryRow("INSERT INTO onetime_tasks"+
		" (gid, name, exec_time)"+
    " VALUES ($1, $2, to_timestamp($3)) RETURNING id",
                          gid, name, exec_time.Unix()).
		Scan(&task.Id); err != nil {
    	fmt.Println("CreateUniqueTask: Error: "+err.Error())
			return nil, err
	}
	return &task, nil
}

//
// TODO document this
// NOTE DONE
//
func CreateInstantTask(user_token string, pid string, bid string) (*InstantTask, error) {
	// Check if pid, bid are integer values
	if _, err := strconv.ParseInt(pid, 10, 64); err != nil {
		return nil, err
	}
	if _, err := strconv.ParseInt(bid, 10, 64); err != nil {
		return nil, err
	}
	// Check whether the user is allowed to access the project
	if project, err := GetProject(pid, user_token); err != nil {
		return nil, err
	}
	// Retrieve user and bot information
	if user, err := GetUser(user_token); err != nil {
		return nil, err
	}
	if bot, err := GetBot(bid); err != nil {
		return nil, err
	}
	// Create Task Group
	if gid, err := CreateTaskGroup(strconv.FormatInt(user.Id, 10), pid, bid); err != nil {
		return nil, err
	}
	// Create new scheduled task
	task := InstantTask{
		Gid:				gid,
		User:				user,
		Project:		project,
		Bot:				bot }
	// Insert into database
	if err := db.QueryRow("INSERT INTO instant_tasks"+
		" (gid)"+
    " VALUES ($1) RETURNING id",
		 gid).
		Scan(&task.Id); err != nil {
    	fmt.Println("CreateInstantTask: Error: "+err.Error())
			return nil, err
	}
	return &task, nil
}

//
// TODO document this
// NOTE DONE
//
func CreateEventTask(user_token string, pid string, bid string, name string, status int64, event int64, hookId int64) (*EventTask, error) {
	// Check if pid, bid are integer values
	if _, err := strconv.ParseInt(pid, 10, 64); err != nil {
		return nil, err
	}
	if _, err := strconv.ParseInt(bid, 10, 64); err != nil {
		return nil, err
	}
	// Check whether the user is allowed to access the project
	if project, err := GetProject(pid, user_token); err != nil {
		return nil, err
	}
	// Retrieve user and bot information
	if user, err := GetUser(user_token); err != nil {
		return nil, err
	}
	if bot, err := GetBot(bid); err != nil {
		return nil, err
	}
	// Create Task Group
	if gid, err := CreateTaskGroup(strconv.FormatInt(user.Id, 10), pid, bid); err != nil {
		return nil, err
	}
	// Create new scheduled task
	task := EventTask{
		Gid:				gid,
		User:				user,
		Project:		project,
		Bot:				bot,
		Name:				name,
		Status:			status,
		Event:			event,
		HookId:			hookId	}
	// Insert into database
	if err := db.QueryRow("INSERT INTO _tasks"+
		" (name, gid, event, hook_id)"+
    " VALUES ($1, $2, $3, $4) RETURNING id",
		 name, gid, event, hookId).
		Scan(&task.Id); err != nil {
    	fmt.Println("CreateEventTask: Error: "+err.Error())
			return nil, err
	}
	return &task, nil
}

//
// TODO document this
// NOTE DONE
//
func CreateTask(gid string, user_token string) (*Task, error) {
	// declarations
	var worker_token string
	// Check if gtid is integer value
	if gidInt, err := strconv.ParseInt(gid, 10, 64); err != nil {
		return nil, err
	}
	//get Worker Token
	if err := db.QueryRow("SELECT users.worker_token FROM users WHERE token=$1", user_token).
	Scan(&worker_token); err != nil {
		return nil, err
	}
	// get Worker
	if worker, err := GetWorker(worker_token); err != nil {
		return nil, err
	}
	// timestamps
	var now time.Time = time.Now()
	var end pq.NullTime
	// Create new task
	task := Task{
		Gid:					gidInt,
		Start_time:   now,
		End_time:     end.Time,
		Status:     	Active,
		Exit_status:	-1,
		Output:      	""	}
	// Insert into database
	if err := db.QueryRow("INSERT INTO tasks"+
		" (gid, worker_token, start_time, end_time, status, exit_status, output)"+
		" VALUES ($1, $2, to_timestamp($3), to_timestamp($4), $5, $6, $7) RETURNING id",
		 gid, worker_token, &task.Start_time, &task.End_time, &task.Status, &task.Exit_status, &task.Output).
		Scan(&task.Id); err != nil {
		return nil, err
	}
	return &task, nil
}

//
// Tasks (SELECT)
//

//
// TODO document this
// NOTE DONE
//
func GetTask(tid string, user_token string) (*Task, error) {
	// declarations
	var gid 									sql.NullInt64
	var worker_token 					sql.NullString
	var start_time, end_time 	pq.NullTime
	var status 								sql.NullInt64
	var exit_status 					sql.NullInt64
	var output 								sql.NullString
	// check if tid is an integer value
  if tidInt, err := strconv.ParseInt(tid, 10, 64); err != nil{
      return nil, err
  }
	// initialize task
	task := Task{}
	// get task information
	if err := db.QueryRow("SELECT * FROM tasks WHERE id=$1", tidInt).
		Scan(&task.Id, &gid, &worker_token, &start_time, &end_time,
			&status, &exit_status, &output); err != nil {
			return nil, err
	}
	// set ScheduleTask
	if gid.Valid {
		task.Gid = gid.Int64
	}
	// set Worker
	if worker_token.Valid {
		if worker, err := GetWorker(worker_token.String); err != nil {
			return nil, err
		}
		task.Worker = worker
	}
	// set remaining fields
	if start_time.Valid {
		task.Start_time = start_time.Time
	}
	if end_time.Valid {
		task.End_time = end_time.Time
	}
	if status.Valid {
		task.Status = status.Int64
	}
	if exit_status.Valid {
		task.Exit_status = exit_status.Int64
	}
	if output.Valid {
		task.Output = output.String
	}
	return &task, nil
}

//
// TODO document this
// NOTE DONE
//
func GetTasks(gid string, user_token string) ([]*Task, error) {
	// declarations
	var tasks []*Task
	// check if gid is an integer value
  if gidInt, err := strconv.ParseInt(gid, 10, 64); err != nil{
    return nil, err
  }
	// get task id
	if rows, err := db.Query("SELECT parent.id FROM"+
  " (SELECT task_groups.id, task_groups.uid FROM tasks"+
  " INNER JOIN task_groups ON tasks.gid = task_groups.id) AS parent"+
	" INNER JOIN users ON parent.uid=users.id WHERE users.token=$1 AND parent.id=$2"+
	" ORDER BY parent.id", user_token, gidInt); err != nil {
		return nil, err
	}
	// fetch tasks
	defer rows.Close()
	for rows.Next() {
		var tid	int64
		if err := rows.Scan(&tid); err != nil {
			return nil, nil
		}
		if task, err := GetTask(strconv.FormatInt(tid, 10), user_token); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

//
// TODO document this
//
func GetHookId(tid int64) (int64, error){
	var hookId sql.NullInt64
	if err := db.QueryRow("SELECT hook_id FROM"+
	"	(SELECT task_groups.id FROM task_groups"+
	" INNER JOIN tasks ON tasks.gid=task_groups.id) AS parent"+
	"	INNER JOIN event_tasks ON event_tasks.gid=parent.id"+
	" WHERE tasks.id=$1", tid).
	Scan(&hookId); err == sql.ErrNoRows {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	if hookId.Valid {
		return hookId, nil
	}
	hErr := errors.New("GetHookId: hookId is not a valid int64")
	return 0, hErr
}

//
// Tasks (UPDATE)
//

//
// TODO document this
// NOTE DONE
//
func UpdateTaskStatus(tid int64, new_status int) error {
	var err error
	if new_status == Running {
		_, err = db.Exec("UPDATE tasks SET status=$1, start_time=now() WHERE id=$2",
			new_status, tid)
	} else if new_status == Canceled {
		_, err = db.Exec("UPDATE tasks SET status=$1, end_time=now() WHERE id=$2",
			new_status, tid)
	} else {
		_, err = db.Exec("UPDATE tasks SET status=$1 WHERE id=$2", new_status, tid)
	}
	return err
}

//
// TODO document this
// NOTE DONE
//
func UpdateEventTaskStatus(eid int64, new_status int) error {
		_, err := db.Exec("UPDATE event_tasks SET status=$1 WHERE id=$2", new_status, eid)
	return err
}

//
// TODO document this
// NOTE DONE
//
func UpdateTaskResult(tid int64, output string, exit_code int) error {
	new_status := Succeeded
	if exit_code != 0 {
		new_status = Failed
	}
	_, err := db.Exec("UPDATE tasks SET status=$1, end_time=now(), output=$2, "+
		"exit_status=$3 WHERE id=$4", new_status, output, exit_code, tid)
	return err
}

//
// TODO document this
// NOTE DONE
//
func UpdateScheduleTaskStatus(tid int64 , status int) error {
		_, err := db.Exec("UPDATE schedule_tasks SET status=$1 WHERE id=$2", status, tid)
    return err
}

//
// TODO document this
// NOTE DONE
//
func UpdateNextScheduleTime(tid int64, time time.Time) error {
	_, err := db.Exec("UPDATE scheduled_tasks SET next_run=$1 WHERE id=$2", scheduledTime, tid)
	return err
}

//
// TODO document this
//
func UpdateHookId(tid int64, hookId int64) error {
	_, err := db.Exec("UPDATE event_tasks SET hook_id=$1 WHERE id=$2", hookId, tid)
	return err
}

//
// Various ScheduleTask Functions
//

//
// TODO document this
//
func CreateTaskFromGroup(gid int64) (*Task, error) {
	var user_token sql.NullString
	if err := db.QueryRow("SELECT users.token FROM users"+
		" INNER JOIN task_groups ON task_groups.uid=users.id"+
		" WHERE task_groups.id=$1", parentId).
		Scan(&user_token); err != nil {
			return nil, nil
	}
	if user_token.Valid {
		return CreateTask(strconv.FormatInt(parentId, 10), user_token)
	}
	tErr := errors.New("CreateTaskFromGroup: user_token is not a valid string")
	return nil, tErr
}

//
// Tasks (TIMER)
// - GetRunningChildren
// - GetOverdueScheduleTasks
// - GetTimedOverTasks
// - GetPendingTask
//

//
// TODO document this
//
func GetRunningChildren(stid int64)([]*Task, error){
	var running []*Task

	if rows, err := db.Query("SELECT tasks.id , users.token FROM tasks"+
		" INNER JOIN scheduled_tasks ON scheduled_tasks.id=tasks.stid"+
		" INNER JOIN users ON users.id=scheduled_tasks.uid"+
		" WHERE scheduled_tasks.id=$1 AND tasks.status=$2", stid, Running); err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tid int64
		var user_token string
		if err := rows.Scan(&tid, &user_token); err != nil {
			return nil, nil
		}
		task, err := GetTask(strconv.FormatInt(tid, 10), user_token)
		if err != nil {
			return nil, err
		}
		running = append(running, task)
	}
	return running, nil
}

//
// TODO document this
//
func GetOverdueScheduleTasks(max_time time.Time) ([]*ScheduleTask, error){
	var scheduled_tasks []*ScheduleTask

	rows, err := db.Query("SELECT scheduled_tasks.id , scheduled_tasks.next_run, users.token"+
	" FROM scheduled_tasks INNER JOIN users ON users.id=scheduled_tasks.uid WHERE status=$1", Active)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var stid int64
		var next pq.NullTime
		var user_token string
		if err := rows.Scan(&stid, &next, &user_token); err != nil {
			return nil, nil
		}
		if next.Valid {
			if int64(time.Since(next.Time).Seconds()) <= int64(time.Since(max_time).Seconds()) {
				scheduled_task, err := GetScheduleTask(strconv.FormatInt(stid, 10), user_token, false)
				if err != nil {
					return nil, err
				}
				scheduled_tasks = append(scheduled_tasks, scheduled_task)
			}
		}
	}
	return scheduled_tasks, nil
}

//
// TODO document this
//
func GetTimedOverTasks(maxseconds int64) ([]int64, error) {
	var starttime pq.NullTime
	var tid 			int64
	var tasks 		[]int64

	// get all running tasks
	rows, err := db.Query("SELECT tasks.id , tasks.start_time FROM tasks"+
		" WHERE tasks.status=$1"+
		" ORDER BY tasks.start_time", Running)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// get starting time of running bots
	for rows.Next() {
		if err := rows.Scan(&tid, &starttime); err != nil {
			return nil, nil
		}
		// check if running time succeeded maximal time
		if starttime.Valid {
			runtime := int64(time.Since(starttime.Time).Seconds())
			if runtime >= maxseconds {
				// time is over - add to canceled tasks
				tasks = append(tasks, tid)
			}
		}
	}
	return tasks, nil
}

// Returns a pending task for the given user. If the user does not exist an
// error is returned and `(nil, nil)` if there is no pending task.
func GetPendingTask(uid int64, shared bool) (*Task, error) {
	//declarations
	rows, err := db.Query("SELECT tasks.id, tasks.uid, tasks.status, "+
		"users.token FROM tasks INNER JOIN users ON tasks.uid = users.id "+
		"WHERE tasks.uid = $1 AND tasks.status = 0", uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// fetch task
	for rows.Next() {
		var tid, uid, status, user_token string
		if err := rows.Scan(&tid, &uid, &status, &user_token); err != nil {
			return nil, nil
		}
		task, err := GetTask(tid, user_token)
		if err != nil {
			return nil, err
		}
		return task, nil
	}

	if shared {
		//declarations
		rows, err := db.Query("SELECT tasks.id, tasks.status, users.token " +
			"FROM tasks INNER JOIN users ON tasks.uid = users.id " +
			"WHERE tasks.status = 0")
		if err != nil {
			return nil, err
		}

		// fetch task
		defer rows.Close()
		for rows.Next() {
			var tid, status, user_token string
			if err := rows.Scan(&tid, &status, &user_token); err != nil {
				return nil, nil
			}
			task, err := GetTask(tid, user_token)
			if err != nil {
				return nil, err
			}
			return task, nil
		}
	}

	return nil, nil
}


// TODO document this
//
// func GetParentTask(childId int64)(*ScheduledTask, error){
//     var stid int64
// 		var token string
// 		err := db.QueryRow("SELECT scheduled_tasks.id FROM scheduled_tasks"+
// 		" INNER JOIN tasks ON tasks.stid=scheduled_tasks.id"+
// 		" WHERE tasks.id=$1", childId).Scan(&stid)
// 		if err != nil {
// 			return nil, err
// 		}
// 		err = db.QueryRow("SELECT users.token FROM users"+
// 		" INNER JOIN scheduled_tasks ON scheduled_tasks.uid=users.id"+
// 		" WHERE scheduled_tasks.id=$1", stid).Scan(&token)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return GetScheduledTask(strconv.FormatInt(stid, 10), token, false)
// }

// TODO document this
//
// func GetRunningScheduledTasks(token string) ([]*ScheduledTask, error) {
// 	var scheduled_tasks []*ScheduledTask
//
// 	rows, err := db.Query("SELECT scheduled_tasks.id FROM scheduled_tasks"+
// 		" INNER JOIN users ON users.id=scheduled_tasks.uid"+
// 		" WHERE users.token=$1 AND scheduled_tasks.status=$2", token, Active)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	defer rows.Close()
// 	for rows.Next() {
// 		var tid int64
// 		if err := rows.Scan(&tid); err != nil {
// 			return nil, nil
// 		}
// 		task, err := GetScheduledTask(strconv.FormatInt(tid, 10), token, false)
// 		if err != nil {
// 			return nil, err
// 		}
// 		scheduled_tasks = append(scheduled_tasks, task)
// 	}
// 	return scheduled_tasks, nil
// }

// TODO document this
//
// func GetHourlyTaskHours(tid int64) (int64, error) {
// 		var hours int64
// 		err := db.QueryRow("SELECT scale FROM hourly_tasks"+
// 		" INNER JOIN scheduled_tasks ON scheduled_tasks.sid=hourly_tasks.id"+
// 		" WHERE scheduled_tasks.id=$1", tid).Scan(&hours)
// 		return hours, err
// }

// TODO document this
//
// func GetMinimalNextTime() (time.Time, error) {
// 		var nextTime time.Time
//     err := db.QueryRow("SELECT MIN(next_run) FROM scheduled_tasks").Scan(&nextTime)
// 		return nextTime, err
// }

//
// TODO document this
//
// func GetAllTasks(token string) ([]*Task, error) {
// 	var tasks []*Task
// 	var tid 	int64
//
// 	rows, err := db.Query("SELECT tasks.id FROM tasks"+
// 		" INNER JOIN users ON tasks.uid=users.id WHERE users.token=$1"+
// 		" ORDER BY tasks.id", token)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	// fetch tasks
// 	defer rows.Close()
// 	for rows.Next() {
//
// 		if err := rows.Scan(&tid); err != nil {
// 			return nil, nil
// 		}
// 		task, err := GetTask(strconv.FormatInt(tid, 10), token)
// 		if err != nil {
// 			return nil, err
// 		}
// 		tasks = append(tasks, task)
// 	}
// 	return tasks, nil
// }
