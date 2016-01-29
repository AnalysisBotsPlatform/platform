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
// Scheduled Tasks
//

//
// TODO document this
//
func GetScheduledTasks(token string) ([]*ScheduledTask, error) {
	//declarations
	var scheduled_tasks []*ScheduledTask

	rows, err := db.Query("SELECT scheduled_tasks.id FROM scheduled_tasks"+
		" INNER JOIN users ON scheduled_tasks.uid=users.id WHERE users.token=$1"+
		" ORDER BY scheduled_tasks.id", token)
	if err != nil {
		return nil, err
	}

	// fetch tasks
	defer rows.Close()
	for rows.Next() {
		var stid string
		if err := rows.Scan(&stid); err != nil {
			return nil, err
		}
		task, err := GetScheduledTask(stid, token, false)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return scheduled_tasks, nil
}

//
// TODO document this
//
func GetScheduledTask(stid string, token string, subtasks bool) (*ScheduledTask, error) {
	// declarations
	scheduled_task := ScheduledTask{}
	var name 					sql.NullString
	var pid, uid, bid sql.NullInt64
	var status 				sql.NullInt64
	var stype 				sql.NullInt64
	var event 				sql.NullInt64
	var next 					*time.Time

	// fetch task entry for tid
	if err := db.QueryRow("SELECT * FROM scheduled_tasks WHERE id=$1", stid).
		Scan(&scheduled_task.Id, &name, &uid, &pid, &bid, &status,
		&stype, &event, &next); err != nil {
		return nil, err
	}

	// fetch Tasks
	if subtasks {
		tasks, err := GetTasks(stid, token)
		if err != nil {
			return nil, err
		}
		scheduled_task.Tasks = tasks
	} else {
		scheduled_task.Tasks = nil
	}

	// fetch user and verify token
	user, err := getUser(strconv.FormatInt(uid, 10), token)
	if err != nil {
		return nil, err
	}
	scheduled_task.User = user

	// fetch project
	project, err := GetProject(strconv.FormatInt(pid, 10), token)
	if err != nil {
		return nil, err
	}
	scheduled_task.Project = project

	// fetch bot
	bot, err := GetBot(strconv.FormatInt(bid, 10))
	if err != nil {
		return nil, err
	}
	scheduled_task.Bot = bot

	if name.Valid {
		scheduled_task.Name = name.String
	}
	if status.Valid {
		scheduled_task.Status = status.Int64
	}
	if stype.Valid {
		scheduled_task.Type = stype.Int64
	}
	if event.Valid {
		scheduled_task.Event = event.Int64
	}
	if next.Valid {
		scheduled_task.Next = next.Time
	}

	return &scheduled_task, nil
}

//
// Tasks
//

//
// TODO document this
//
func GetTasks(stid int64, token string) ([]*Task, error) {
	var tasks []*Task

	rows, err := db.Query("SELECT tasks.id FROM tasks"+
		" INNER JOIN users ON tasks.uid=users.id WHERE users.token=$1 AND tasks.stid=$2"+
		" ORDER BY tasks.id", token, stid)
	if err != nil {
		return nil, err
	}

	// fetch tasks
	defer rows.Close()
	for rows.Next() {
		var tid string
		if err := rows.Scan(&tid); err != nil {
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

//
// TODO document this
//
func GetAllTasks(token string) ([]*Task, error) {
	var tasks []*Task

	rows, err := db.Query("SELECT tasks.id, users.token FROM tasks"+
		" INNER JOIN users ON tasks.uid=users.id WHERE users.token=$1"+
		" ORDER BY tasks.id", token)
	if err != nil {
		return nil, err
	}

	// fetch tasks
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

//
// TODO document this
//
func GetTask(tid string, token string) (*Task, error) {
	var task *Task
	var stid sql.NullInt64
	var worker_token sql.NullString
	var start_time, end_time pq.NullTime
	var status sql.NullInt64
	var exit_status sql.NullInt64
	var output sql.NullString

	if err := db.QueryRow("SELECT * FROM tasks WHERE id=$1", tid).
		Scan(&scheduled_task.Id, &stid, &worker_token, &start_time, &end_time,
			&status, &exit_status, &output); err != nil {
			return nil, err
	}

	// set ScheduledTask
	if stid.Valid {
		scheduled_task, err := GetScheduledTask(stidInt64, token, false)
		if err != nil {
			return nil, err
		}
		task.ScheduledTask = scheduled_task
	}

	// set Worker
	if worker_token.Valid {
		worker, err := GetWorker(worker_token.String)
		if err != nil {
			return nil, err
		}
		task.Worker = worker
	}

	// set remaining fields
	if start_time.Valid {
		task.Start_time = &start_time.Time
	}
	if end_time.Valid {
		task.End_time = &end_time.Time
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
	return task, nil
}

//
// TODO document this
//
func CreateNewScheduledTask(styp int64, name string, token string,
	 pid string, bid string, next *time.Time) (*ScheduledTask, error) {
	// Check whether the user is allowed to access the project
	project, err := GetProject(pid, token)
	if err != nil {
		return nil, err
	}

	// Retrieve user and bot information
	user, err := GetUser(token)
	if err != nil {
		return nil, err
	}
	bot, err := GetBot(bid)
	if err != nil {
		return nil, err
	}

	// Create new scheduled task
	scheduled_task := ScheduledTask{
		Name:					name,
		User:       	user,
		Project:    	project,
		Bot:        	bot,
		Status:     	Active,
		Type:      		styp,
		Next					next
	}

	// Insert into database
	if err := db.QueryRow("INSERT INTO scheduled_tasks"+
		" (name, uid, pid, bid, status, schedule_type, eid, next_run)"+
		" VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id ", name ,user.Id,
		project.Id, bot.Id, Active, schedule_type, eid, start_time).
		Scan(&scheduled_task.Id); err != nil {
		return nil, err
	}

	task, err := CreateNewTask(strconv.FormatInt(scheduled_task.Id, 10), token, next)
	if err != nil {
		return nil, err
	}
	var tasks []*Task
	scheduled_task.Tasks = append(tasks, task)

	return &scheduled_task, nil
}

type Task struct {
	Id          	int64
	ScheduledTask *ScheduledTask
	Worker      	*Worker
	Start_time  	*time.Time
	End_time    	*time.Time
	Status      	int64
	Exit_status 	int64
	Output      	string
}

//
// TODO document this
//
func CreateNewTask(stid string, token string, start_time *time.Time) (*Task, error) {
	var worker_token string

	scheduled_task, err := GetScheduledTask(stid, token, true)
	if err != nil {
		return nil, err
	}

	//get Worker Token
	err := db.QueryRow("SELECT users.worker_token FROM users WHERE token=$1", token).Scan(&worker_token)
	if err != nil {
		return nil, err
	}

	// get Worker
	worker, err := GetWorker(worker_token)
	if err != nil {
		return nil, err
	}

	// Create new task
	task := Task{
		ScheduledTask:	scheduled_task,
		Worker:    			worker,
		Start_time:     next,
		End_time:       time.NullTime,
		Status:     		Pending,
		Exit_status:		-1,
		Output:      		""
	}

	// Insert into database
	if err := db.QueryRow("INSERT INTO tasks"+
		" (stid, worker_token, start_time, end_time, status, exit_status, output)"+
		" VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id ", stid, worker_token,
		task.Start_time, task.End_time, task.Status, task.Exit_status, task.Output).
		Scan(&task.Id); err != nil {
		return nil, err
	}

	return &task, nil
}

// This function updates the tasks' status with the provided value
func UpdateTaskStatus(tid int64, new_status int64) {
	if new_status == Running {
		db.QueryRow("UPDATE tasks SET status=$1, start_time=now()::timestamp(0) WHERE id=$2",
			new_status, tid)
	} else if new_status == Cancelled {
		db.QueryRow("UPDATE tasks SET status=$1, end_time=now()::timestamp(0) WHERE id=$2",
			new_status, tid)
	} else {
		db.QueryRow("UPDATE tasks SET status=$1 WHERE id=$2", new_status, tid)
	}
}

// This function updates the tasks' result with the given output
func UpdateTaskResult(tid int64, output string, exit_code int) {
	new_status := Succeeded
	if exit_code != 0 {
		new_status = Failed
	}
	db.QueryRow("UPDATE tasks SET status=$1, end_time=now()::timestamp(0), output=$2, "+
		"exit_status=$3 WHERE id=$4", new_status, output, exit_code, tid)
}

// This function returns all tasks which succeeded the 'maxseconds' duration
func GetTimedOverTasks(maxseconds int64) ([]int64, error) {
	var starttime pq.NullTime
	var tid int64
	var tasks []int64

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
			return nil, err
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

	// fetch task
	defer rows.Close()
	for rows.Next() {
		var tid, uid, status, token string
		if err := rows.Scan(&tid, &uid, &status, &token); err != nil {
			return nil, err
		}
		task, err := GetTask(tid, token)
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
			var tid, status, token string
			if err := rows.Scan(&tid, &status, &token); err != nil {
				return nil, err
			}
			task, err := GetTask(tid, token)
			if err != nil {
				return nil, err
			}
			return task, nil
		}
	}

	return nil, nil
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
		"shared) VALUES ($1, $2, $3, now()::timestamp(0), $4, $5)", uid, token, name, false,
		shared)

	return token, nil
}

// Sets the given worker active, i.e. the `active` flag is set and the
// `last_contact` time is updated. If the worker does not exist an error is
// returned.
func SetWorkerActive(token string) error {
	db.QueryRow("UPDATE workers SET active=true, last_contact=now()::timestamp(0) "+
		"WHERE token=$1", token)

	return nil
}

// Sets the given worker inactive, i.e. the `active` flag is unset and the
// `last_contact` time is updated. If the worker does not exist an error is
// returned.
func SetWorkerInactive(token string) error {
	db.QueryRow("UPDATE workers SET active=false, last_contact=now()::timestamp(0) "+
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
// Schedules
//

// TODO document this
//
func SetHourlyTask(scale int64, date *time.Time) (int64, error) {
	var insertId int64
	err := db.QueryRow("UPDATE hourly_tasks SET scale=$1 , start_time=$2", scale, date).Scan(&insertId)
	return insertId, err
}

// TODO document this
//
func SetDailyTask(date *time.Time) (int64, error) {
	var insertId int64
	err := db.QueryRow("UPDATE daily_tasks SET start_time=$1", date).Scan(&insertId)
	return insertId, err
}

// TODO document this
//
func SetWeeklyTask(weekday int64, date *time.Time) (int64, error) {
	var insertId int64
	err := db.QueryRow("UPDATE weekly_tasks SET weekday=$1 , start_time=$2", weekday, date).Scan(&insertId)
	return insertId, err
}

// TODO document this
//
func SetSingleTask(date *time.Time) (int64, error) {
	var insertId int64
	err := db.QueryRow("UPDATE single_tasks SET start_time=$1", date).Scan(&insertId)
	return insertId, err
}

// TODO document this
//
func SetEventTask(event int64) (int64, error) {
	var insertId int64
	err := db.QueryRow("UPDATE event_tasks SET event_type=$1", event).Scan(&insertId)
	return insertId, err
}
