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

const (
	// Number of allowed accesses during `api_restriction_interval`
	api_restriction_count = 5000
	// Time interval used to count the number of API accesses
	api_restriction_interval = "1 hour"
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
	var dummy string

	// update user information
	db.QueryRow("UPDATE users SET username=$1, realname=$2, email=$3, token=$4"+
		" WHERE gh_id=$5", user.User_name, user.Real_name, user.Email,
		user.Token, user.GH_Id).Scan(&dummy)
}

// This function creates an user by storing the information
// provided by the User struct in the database
func createUser(user *User) {
	var dummy string

	// create user
	db.QueryRow("INSERT INTO users (gh_id, username, realname, email, token, "+
		"worker_token, admin) VALUES ($1, $2, $3, $4, $5, $6, $7)", user.GH_Id,
		user.User_name, user.Real_name, user.Email, user.Token,
		user.Worker_token, user.Admin).Scan(&dummy)
}

// Fetch all user specific statistics from the database.
func GetUserStatistics(token string) (*User_statistics, error) {
	// declarations
	stats := User_statistics{}

	// fetch statistics
	if err := db.QueryRow("SELECT "+
		"(SELECT count(*) FROM users INNER JOIN members "+
		"ON users.id = members.uid WHERE users.token = $1), "+
		"(SELECT count(DISTINCT group_tasks.bid) FROM users "+
		"INNER JOIN group_tasks ON users.id = group_tasks.uid "+
		"WHERE users.token = $1), "+
		"(SELECT count(*) FROM users "+
		"INNER JOIN group_tasks ON users.id = group_tasks.uid "+
		"INNER JOIN tasks ON group_tasks.id = tasks.gid "+
		"WHERE users.token = $1 AND (tasks.status IN ($2, $3, $4))), "+
		"(SELECT count(*) FROM users "+
		"INNER JOIN group_tasks ON users.id = group_tasks.uid "+
		"INNER JOIN tasks ON group_tasks.id = tasks.gid "+
		"WHERE users.token = $1)", token, Pending, Scheduled,
		Running).Scan(&stats.GH_projects, &stats.Bots_used,
		&stats.Tasks_unfinished, &stats.Tasks_total); err != nil {
		return nil, err
	}

	return &stats, nil
}

// Fetch all user specific API statistics from the database.
func GetAPIStatistics(token string) (*API_statistics, error) {
	// declarations
	stats := API_statistics{
		Interval: api_restriction_interval,
	}
	var last_access pq.NullTime

	// fetch statistics
	if err := db.QueryRow(fmt.Sprintf("SELECT "+
		"(SELECT max(time) FROM users INNER JOIN api_accesses "+
		"ON users.id = api_accesses.uid WHERE users.token = $1), "+
		"(SELECT $2 - count(*) FROM users INNER JOIN api_accesses "+
		"ON users.id = api_accesses.uid WHERE users.token = $1 "+
		"AND api_accesses.time > now () - interval '%s')",
		api_restriction_interval), token, api_restriction_count).
		Scan(&last_access, &stats.Remaining_accesses); err != nil {
		if err != sql.ErrNoRows {
			return nil, err
		}
	}

	if last_access.Valid {
		stats.Was_accessed = true
		stats.Last_access = last_access.Time
	}

	return &stats, nil
}

// Verifies that the given token is a valid API token. Returns false if it is
// not a valid token or the number of API accesses exceeds the value specified
// by `api_restriction_count` within `api_restriction_interval`.
// If the provided API token is valid a new entry is created in the
// "api_accesses" relation.
func IsValidAPIToken(token string) bool {
	var dummy string

	if err := db.QueryRow("SELECT token FROM api_tokens WHERE token = $1",
		token).Scan(&dummy); err != nil {
		return false
	}

	var accesses int
	if err := db.QueryRow(fmt.Sprintf("SELECT count(*) FROM api_accesses "+
		"WHERE uid = (SELECT uid FROM api_tokens WHERE token = $1) "+
		"AND time > now () - interval '%s'", api_restriction_interval), token).
		Scan(&accesses); err != nil {
		return false
	}

	if accesses < api_restriction_count {
		db.QueryRow("INSERT INTO api_accesses (uid, time) "+
			"VALUES ((SELECT uid FROM api_tokens WHERE token = $1), now())",
			token).Scan(&dummy)
		return true
	} else {
		return false
	}
}

// Retrieves the user's GitHub application token using one of his/her API
// tokens.
func GetUserTokenFromAPIToken(token string) (string, error) {
	var result string

	// get user token
	if err := db.QueryRow("SELECT users.token, api_tokens.token FROM users "+
		"INNER JOIN api_tokens ON users.id = api_tokens.uid "+
		"WHERE api_tokens.token = $1", token).
		Scan(&result, &token); err != nil {
		return "", err
	}

	return result, nil
}

// Retrieves all of the user's API access tokens.
func GetAPITokens(user_token string) ([]*API_token, error) {
	// declarations
	var tokens []*API_token
	rows, err := db.Query("SELECT at.* FROM api_tokens AS at INNER JOIN users "+
		"ON at.uid = users.id WHERE users.token = $1", user_token)
	if err != nil {
		return nil, err
	}

	// fetch API tokens
	defer rows.Close()
	for rows.Next() {
		token := API_token{}

		err := rows.Scan(&token.Token, &token.Uid, &token.Name)
		if err != nil {
			return nil, err
		}

		tokens = append(tokens, &token)
	}

	return tokens, nil
}

// Generate a new API token for the given user and insert it into the database.
func AddAPIToken(user_token, name string) error {
	// declarations
	api_token := nonExistingRandString(Token_length,
		"SELECT 42 FROM api_tokens WHERE token = $1")
	var dummy string

	if err := db.QueryRow("INSERT INTO api_tokens (token, uid, name) "+
		"VALUES ($1, (SELECT id FROM users WHERE token = $2), $3)", api_token,
		user_token, name).Scan(&dummy); err != nil {
		if err != sql.ErrNoRows {
			return err
		}
	}

	return nil
}

// Delete the given API token for the given user from the database.
func DeleteAPIToken(user_token, api_token string) error {
	//declarations
	token := API_token{}

	if err := db.QueryRow("DELETE FROM api_tokens WHERE token = $1 "+
		"AND uid = (SELECT id FROM users WHERE token = $2) RETURNING *",
		api_token, user_token).Scan(&token.Token, &token.Uid,
		&token.Name); err != nil {
		return err
	}

	return nil
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
			var dummy string
			db.QueryRow("DELETE FROM members WHERE uid=$1 AND pid=$2", uid,
				pid).Scan(&dummy)
			var count int64 = -1
			if err := db.QueryRow("SELECT count(*) FROM members WHERE pid=$1",
				pid).Scan(&count); err == nil && count == 0 {
				db.QueryRow("DELETE FROM projects WHERE id=$1", pid).
					Scan(&dummy)
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
		var dummy string
		db.QueryRow("INSERT INTO members VALUES ($1, $2)", uid, project.Id).
			Scan(&dummy)
	}

	return nil
}

// This function updates the Project referenced by the users' id in the database
// with the information given by the Project structure
func updateProject(project *Project, uid int64) error {
	var dummy string

	// update project information
	db.QueryRow("UPDATE projects SET name=$1, clone_url=$2 WHERE gh_id=$3",
		project.Name, project.Clone_url, project.GH_Id).Scan(&dummy)

	if err := fillProject(project, uid); err != nil {
		return err
	}

	return nil
}

// This function inserts a new Project into the database with the information
// given by the Project structure and sets a reference to the user specified by
// his id
func createProject(project *Project, uid int64) error {
	var dummy string

	// create project
	db.QueryRow("INSERT INTO projects (gh_id, name, clone_url)"+
		" VALUES ($1, $2, $3)", project.GH_Id, project.Name, project.Clone_url).
		Scan(&dummy)

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
func AddBot(path, description, tags string) (string, error) {
	// check whether bot exists already
	err := db.QueryRow("SELECT id FROM bots WHERE name=$1", path).Scan(&path)
	if err == nil {
		return "", errors.New("Bot already exists!")
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
		" VALUES ($1, $2, $3, $4) RETURNING id", path, description,
		buffer.String(), path).
		Scan(&result); err != nil && err != sql.ErrNoRows {
		return "", err
	}

	return result, nil
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

		if err := rows.Scan(&bot.Id, &bot.Name, &description, &tags,
			&fs_path); err != nil {
			return nil, err
		}

		if description.Valid {
			bot.Description = description.String
		}
		if tags.Valid {
			bot.Tags = strings.Split(strings.Replace(
				tags.String[1:len(tags.String)-1], "\"", "", -1), ",")
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
		bot.Tags = strings.Split(strings.Replace(
			tags.String[1:len(tags.String)-1], "\"", "", -1), ",")
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
	var dummy string

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
		shared).Scan(&dummy)

	return token, nil
}

// Sets the given worker active, i.e. the `active` flag is set and the
// `last_contact` time is updated. If the worker does not exist an error is
// returned.
func SetWorkerActive(token string) error {
	var dummy string
	return db.QueryRow("UPDATE workers SET active=true, last_contact=now() "+
		"WHERE token=$1 RETURNING 42", token).Scan(&dummy)
}

// Sets the given worker inactive, i.e. the `active` flag is unset and the
// `last_contact` time is updated. If the worker does not exist an error is
// returned.
func SetWorkerInactive(token string) error {
	var dummy string
	return db.QueryRow("UPDATE workers SET active=false, last_contact=now() "+
		"WHERE token=$1 RETURNING 42", token).Scan(&dummy)
}

// Returns the worker that corresponds to the given token. In case the token is
// invalid an error is returned.
func GetWorker(token string) (*Worker, error) {
	// declarations
	worker := Worker{}
	var dummy string

	// update last contact
	db.QueryRow("UPDATE workers SET last_contact = now() WHERE token = $1",
		token).Scan(&dummy)

	// get worker
	if err := db.QueryRow("SELECT * FROM workers WHERE token = $1", token).
		Scan(&worker.Id, &worker.Uid, &worker.Token, &worker.Name,
		&worker.Last_contact, &worker.Active, &worker.Shared); err != nil {
		return nil, err
	}

	return &worker, nil
}

// Retrieves all of the user's workers.
func GetWorkers(token string) ([]*Worker, error) {
	// declarations
	var workers []*Worker
	rows, err := db.Query("SELECT * FROM workers "+
		"WHERE uid = (SELECT id FROM users WHERE token = $1)", token)
	if err != nil {
		return nil, err
	}

	// fetch workers
	defer rows.Close()
	for rows.Next() {
		worker := Worker{}

		err := rows.Scan(&worker.Id, &worker.Uid, &worker.Token, &worker.Name,
			&worker.Last_contact, &worker.Active, &worker.Shared)
		if err != nil {
			return nil, err
		}

		workers = append(workers, &worker)
	}

	return workers, nil
}

// Delete the given worker for the given user from the database.
func DeleteWorker(user_token, worker_token string) error {
	//declarations
	worker := Worker{}

	if err := db.QueryRow("DELETE FROM workers WHERE token = $1 "+
		"AND uid = (SELECT id FROM users WHERE token = $2) RETURNING *",
		worker_token, user_token).Scan(&worker.Id, &worker.Uid, &worker.Token,
		&worker.Name, &worker.Last_contact, &worker.Active,
		&worker.Shared); err != nil {
		return err
	}

	return nil
}

//
// Tasks
//

// This function returns the latest `size` tasks from the database specified by
// the users' token
func GetLatestTasks(token string, size int) ([]*Task, error) {
	//declarations
	var tasks []*Task
	rows, err := db.Query("SELECT tasks.id FROM tasks"+
		" INNER JOIN group_tasks ON tasks.gid=group_tasks.id "+
		" INNER JOIN users ON group_tasks.uid=users.id WHERE users.token=$1"+
		" ORDER BY tasks.id DESC LIMIT $2", token, size)
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

// TODO document this
func getGroupTask(gid int64) (*group_task, error) {
	// declarations
	gt := group_task{}
	var uid, pid, bid int64
	user := User{}

	if err := db.QueryRow("SELECT * FROM group_tasks WHERE id = $1", gid).
		Scan(&gt.id, &uid, &pid, &bid); err != nil {
		return nil, err
	}

	// get user
	var user_name, real_name, email sql.NullString

	// fetch user
	if err := db.QueryRow("SELECT * FROM users WHERE id=$1", uid).
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

	gt.user = &user
	gt.project, _ = GetProject(strconv.FormatInt(pid, 10), user.Token)
	gt.bot, _ = GetBot(strconv.FormatInt(bid, 10))

	return &gt, nil
}

// TODO documentation
func GetTask(tid, user_token string) (*Task, error) {
	// declarations
	var start_time, end_time pq.NullTime
	var exit_status sql.NullInt64
	var output sql.NullString

	// initialize Task
	task := Task{}
	// get task information
	if err := db.QueryRow("SELECT * FROM tasks WHERE tasks.id=$1", tid).
		Scan(&task.Id, &task.Gid, &start_time, &end_time, &task.Status,
		&exit_status, &output, &task.Patch); err != nil {
		return nil, err
	}
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

	group_task, _ := getGroupTask(task.Gid)
	task.User = group_task.user
	task.Project = group_task.project
	task.Bot = group_task.bot

	return &task, nil
}

//
// TODO documentation
//
func GetPendingTask(uid int64, shared bool) (*Task, error) {
	//declarations
	rows, err := db.Query("SELECT tasks.id, users.token FROM tasks "+
		"INNER JOIN group_tasks ON tasks.gid = group_tasks.id "+
		"INNER JOIN users ON group_tasks.uid = users.id "+
		"WHERE group_tasks.uid = $1 AND tasks.status = $2", uid, Pending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// fetch task
	for rows.Next() {
		var tid, user_token string
		if err := rows.Scan(&tid, &user_token); err != nil {
			return nil, err
		}

		task, err := GetTask(tid, user_token)
		if err != nil {
			return nil, err
		}
		return task, nil
	}

	if shared {
		//declarations
		rows, err := db.Query("SELECT tasks.id, users.token FROM tasks "+
			"INNER JOIN group_tasks ON tasks.gid = group_tasks.id "+
			"INNER JOIN users ON group_tasks.uid = users.id "+
			"WHERE tasks.status = $1", Pending)
		if err != nil {
			return nil, err
		}

		// fetch task
		defer rows.Close()
		for rows.Next() {
			var tid, user_token string
			if err := rows.Scan(&tid, &user_token); err != nil {
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

//
// TODO documentation
//
func CreateNewChildTask(gtid int64) (*Task, error) {
	group_task, _ := getGroupTask(gtid)

	// create task
	task := Task{
		Gid:         gtid,
		User:        group_task.user,
		Project:     group_task.project,
		Bot:         group_task.bot,
		Exit_status: -1,
	}

	// insert into db
	if err := db.QueryRow("INSERT INTO tasks (gid, status, patch)"+
		" VALUES ($1, $2, '') RETURNING id", gtid, Pending).
		Scan(&task.Id); err != nil {
		return nil, err
	}

	return &task, nil
}

// This function updates the tasks' status with the provided value.
// Do not call this function with Succeeded or Failed as values for new_status.
func UpdateTaskStatus(tid int64, new_status int64) {
	var dummy string

	if new_status == Running {
		db.QueryRow("UPDATE tasks SET status=$1, start_time=now() WHERE id=$2",
			new_status, tid).Scan(&dummy)
	} else if new_status == Canceled {
		db.QueryRow("UPDATE tasks SET status=$1, end_time=now() WHERE id=$2",
			new_status, tid).Scan(&dummy)
	} else {
		db.QueryRow("UPDATE tasks SET status=$1 WHERE id=$2", new_status, tid).
			Scan(&dummy)
	}
}

// This function updates the tasks' result with the given output and returns a
// non-existing file name if requested.
func UpdateTaskResult(tid int64, output string, exit_code int,
	gen_file_name bool) string {
	new_status := Succeeded
	if exit_code != 0 {
		new_status = Failed
	}

	var file_name, dummy string

	if gen_file_name {
		file_name = nonExistingRandString(Token_length,
			"SELECT 42 FROM tasks WHERE patch = $1 || '.patch'") + ".patch"
	}

	db.QueryRow("UPDATE tasks SET status=$1, end_time=now(), output=$2, "+
		"exit_status=$3, patch=$4 WHERE id=$5", new_status, output, exit_code,
		file_name, tid).Scan(&dummy)

	return file_name
}

// Returns the file name for the given patch file. Fails if the user does not
// have access to the file.
func GetPatchFileName(token, patch_file string) (string, error) {
	// declarations
	var file_name string

	// fetch file name
	if err := db.QueryRow("SELECT patch FROM tasks "+
		"WHERE uid = (SELECT id FROM users WHERE token = $1) AND patch = $2",
		token, patch_file).Scan(&file_name); err != nil {
		return "", err
	}

	return file_name, nil
}

//
// TODO documentation
//
func GetChildTasks(gtid int64) ([]*Task, error) {
	var tasks []*Task

	rows, err := db.Query("SELECT tasks.id, users.token FROM tasks "+
		"INNER JOIN group_tasks ON tasks.gid = group_tasks.id "+
		"INNER JOIN users ON group_tasks.uid = users.id "+
		"WHERE group_tasks.id = $1", gtid)
	if err != nil {
		if err == sql.ErrNoRows {
			return tasks, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tid, user_token string
		if err := rows.Scan(&tid, &user_token); err != nil {
			return nil, err
		}
		task, err := GetTask(tid, user_token)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, err
}

//
// TODO documentation
//
func GetActiveChildren(gtid int64) ([]*Task, error) {
	var tasks []*Task

	rows, err := db.Query("SELECT tasks.id, users.token FROM tasks "+
		"INNER JOIN group_tasks ON tasks.gid = group_tasks.id "+
		"INNER JOIN users ON group_tasks.uid = users.id "+
		"WHERE group_tasks.id = $1 AND tasks.status IN ($2, $3, $4)", gtid,
		Pending, Scheduled, Running)
	if err != nil {
		if err == sql.ErrNoRows {
			return tasks, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tid, user_token string
		if err := rows.Scan(&tid, &user_token); err != nil {
			return nil, err
		}
		task, err := GetTask(tid, user_token)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, err
}

//
// TODO documentation
//
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

//########################################################

// ScheduledTask
//########################################################
//
// TODO documentation
//
func CreateScheduledTask(token string, pid string, bid string, name string,
	next time.Time, cron_exp string) (*ScheduledTask, error) {
	var gid int64
	if err := db.QueryRow("WITH row AS ("+
		"INSERT INTO group_tasks (uid, pid, bid) VALUES ("+
		"(SELECT id FROM users WHERE token = $1), $2, $3) RETURNING id"+
		")"+
		"INSERT INTO schedule_tasks (id, name, status, next, cron) "+
		"VALUES ((SELECT id FROM row), $4, $5, $6, $7) RETURNING id", token,
		pid, bid, name, Active, next, cron_exp).Scan(&gid); err != nil {
		return nil, err
	}
	return GetScheduledTask(gid)
}

//
// TODO documentation
//
func GetScheduledTask(stid int64) (*ScheduledTask, error) {
	var next pq.NullTime
	task := ScheduledTask{}

	if err := db.QueryRow("SELECT * FROM schedule_tasks WHERE id=$1", stid).
		Scan(&task.Id, &task.Name, &task.Status, &next,
		&task.Cron); err != nil {
		return nil, err
	}

	group_task, err := getGroupTask(task.Id)
	if err != nil {
		return nil, err
	}

	task.User = group_task.user
	task.Project = group_task.project
	task.Bot = group_task.bot

	if next.Valid {
		task.Next = next.Time
	}

	return &task, nil
}

//
// TODO documentation
//
func GetScheduledTasks(token string) ([]*ScheduledTaskInstances, error) {
	var tasks []*ScheduledTaskInstances

	rows, err := db.Query("SELECT group_tasks.id FROM schedule_tasks "+
		"NATURAL JOIN group_tasks "+
		"WHERE group_tasks.uid=(SELECT id FROM users WHERE token = $1)", token)
	if err != nil {
		if err == sql.ErrNoRows {
			return tasks, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tid int64
		if err := rows.Scan(&tid); err != nil {
			return nil, err
		}
		task, err := GetScheduledTask(tid)
		if err != nil {
			return nil, err
		}
		child_tasks, err := GetChildTasks(tid)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, &ScheduledTaskInstances{
			Task:        task,
			Child_tasks: child_tasks,
		})
	}
	return tasks, nil
}

//
// TODO documentation
//
func UpdateScheduledTaskStatus(stid int64, status int) error {
	var dummy string
	if err := db.QueryRow("UPDATE schedule_tasks SET status=$1 WHERE id=$2 "+
		"RETURNING id", status, stid).Scan(&dummy); err != nil {
		return err
	}
	return nil
}

//
// TODO documentation
//
func UpdateNextScheduleTime(stid int64, next time.Time) error {
	var dummy string
	if err := db.QueryRow("UPDATE schedule_tasks SET next=$1 WHERE id=$2 "+
		"RETURNING id", next, stid).Scan(&dummy); err != nil {
		return err
	}
	return nil
}

//
// TODO documentation
//
func GetScheduledTaskIdsWithStatus(status int) ([]int64, error) {
	var ids []int64

	rows, err := db.Query("SELECT group_tasks.id FROM group_tasks "+
		"NATURAL JOIN schedule_tasks WHERE status = $1", status)
	if err != nil {
		if err == sql.ErrNoRows {
			return ids, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tid int64
		if err := rows.Scan(&tid); err != nil {
			return nil, err
		}

		ids = append(ids, tid)
	}

	return ids, nil
}

//########################################################

// OneTimeTask
//########################################################
//
// TODO documentation
//
func CreateOneTimeTask(token string, pid string, bid string, name string,
	exec_time time.Time) (*OneTimeTask, error) {
	var gid int64
	if err := db.QueryRow("WITH row AS ("+
		"INSERT INTO group_tasks (uid, pid, bid) VALUES ("+
		"(SELECT id FROM users WHERE token = $1), $2, $3) RETURNING id"+
		")"+
		"INSERT INTO onetime_tasks (id, name, status, exec_time) "+
		"VALUES ((SELECT id FROM row), $4, $5, $6) RETURNING id", token, pid,
		bid, name, Active, exec_time).Scan(&gid); err != nil {
		return nil, err
	}
	return GetOneTimeTask(gid)
}

//
// TODO documentation
//
func GetOneTimeTask(otid int64) (*OneTimeTask, error) {
	task := OneTimeTask{}

	if err := db.QueryRow("SELECT * FROM onetime_tasks WHERE id=$1", otid).
		Scan(&task.Id, &task.Name, &task.Status, &task.Exec_time); err != nil {
		return nil, err
	}

	group_task, err := getGroupTask(task.Id)
	if err != nil {
		return nil, err
	}

	task.User = group_task.user
	task.Project = group_task.project
	task.Bot = group_task.bot

	return &task, nil
}

//
// TODO documentation
//
func GetOneTimeTasks(token string) ([]*OneTimeTaskInstances, error) {
	var tasks []*OneTimeTaskInstances

	rows, err := db.Query("SELECT group_tasks.id FROM onetime_tasks "+
		"NATURAL JOIN group_tasks "+
		"WHERE group_tasks.uid=(SELECT id FROM users WHERE token = $1)", token)
	if err != nil {
		if err == sql.ErrNoRows {
			return tasks, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tid int64
		if err := rows.Scan(&tid); err != nil {
			return nil, err
		}
		task, err := GetOneTimeTask(tid)
		if err != nil {
			return nil, err
		}
		child_tasks, err := GetChildTasks(tid)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, &OneTimeTaskInstances{
			Task:        task,
			Child_tasks: child_tasks,
		})
	}
	return tasks, nil
}

// NOTE DONE
func UpdateOneTimeTaskStatus(otid int64, status int) error {
	var dummy string
	if err := db.QueryRow("UPDATE onetime_tasks SET status=$1 WHERE id=$2 "+
		"RETURNING id", status, otid).Scan(&dummy); err != nil {
		return err
	}
	return nil
}

//
// TODO documentation
//
func GetOneTimeTaskIdsWithStatus(status int) ([]int64, error) {
	var ids []int64

	rows, err := db.Query("SELECT group_tasks.id FROM group_tasks "+
		"NATURAL JOIN onetime_tasks WHERE status = $1", status)
	if err != nil {
		if err == sql.ErrNoRows {
			return ids, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tid int64
		if err := rows.Scan(&tid); err != nil {
			return nil, err
		}

		ids = append(ids, tid)
	}

	return ids, nil
}

//########################################################

// InstantTask
//########################################################
//
// TODO documentation
//
func CreateNewInstantTask(token string, pid string,
	bid string) (*InstantTask, error) {
	var gid int64
	if err := db.QueryRow("SELECT instant_tasks.id FROM group_tasks "+
		"NATURAL JOIN instant_tasks "+
		"WHERE uid = (SELECT id FROM users WHERE token = $1) AND pid = $2 "+
		"AND bid = $3", token, pid, bid).Scan(&gid); err != nil {
		if err == sql.ErrNoRows {
			if err := db.QueryRow("WITH row AS ("+
				"INSERT INTO group_tasks (uid, pid, bid) VALUES ("+
				"(SELECT id FROM users WHERE token = $1), $2, $3) RETURNING id"+
				") "+
				"INSERT INTO instant_tasks SELECT id FROM row RETURNING id",
				token, pid, bid).Scan(&gid); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return GetInstantTask(gid)
}

//
// TODO documentation
//
func GetInstantTask(itid int64) (*InstantTask, error) {
	task := InstantTask{}

	if err := db.QueryRow("SELECT * FROM instant_tasks WHERE id=$1", itid).
		Scan(&task.Id); err != nil {
		return nil, err
	}

	group_task, err := getGroupTask(task.Id)
	if err != nil {
		return nil, err
	}

	task.User = group_task.user
	task.Project = group_task.project
	task.Bot = group_task.bot

	return &task, nil
}

//
// TODO documentation
//
func GetInstantTasks(token string) ([]*InstantTaskInstances, error) {
	var tasks []*InstantTaskInstances

	rows, err := db.Query("SELECT group_tasks.id FROM instant_tasks "+
		"NATURAL JOIN group_tasks "+
		"WHERE group_tasks.uid=(SELECT id FROM users WHERE token = $1)", token)
	if err != nil {
		if err == sql.ErrNoRows {
			return tasks, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tid int64
		if err := rows.Scan(&tid); err != nil {
			return nil, err
		}
		task, err := GetInstantTask(tid)
		if err != nil {
			return nil, err
		}
		child_tasks, err := GetChildTasks(tid)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, &InstantTaskInstances{
			Task:        task,
			Child_tasks: child_tasks,
		})
	}
	return tasks, nil
}

//########################################################

// EventTask
//########################################################
//
// TODO documentation
func CreateNewEventTask(token string, pid string, bid string, name string,
	event int64) (*EventTask, error) {
	var gid int64
	if err := db.QueryRow("WITH row AS ("+
		"INSERT INTO group_tasks (uid, pid, bid) VALUES ("+
		"(SELECT id FROM users WHERE token = $1), $2, $3) RETURNING id"+
		")"+
		"INSERT INTO event_tasks (id, name, status, event) "+
		"VALUES ((SELECT id FROM row), $4, $5, $6) RETURNING id", token, pid,
		bid, name, Active, event).Scan(&gid); err != nil {
		return nil, err
	}
	return GetEventTask(gid)
}

//
// TODO documentation
//
func GetEventTasks(token string) ([]*EventTaskInstances, error) {
	var tasks []*EventTaskInstances

	rows, err := db.Query("SELECT group_tasks.id FROM event_tasks "+
		"NATURAL JOIN group_tasks "+
		"WHERE group_tasks.uid=(SELECT id FROM users WHERE token = $1)", token)
	if err != nil {
		if err == sql.ErrNoRows {
			return tasks, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tid int64
		if err := rows.Scan(&tid); err != nil {
			return nil, err
		}
		task, err := GetEventTask(tid)
		if err != nil {
			return nil, err
		}
		child_tasks, err := GetChildTasks(tid)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, &EventTaskInstances{
			Task:        task,
			Child_tasks: child_tasks,
		})
	}
	return tasks, nil
}

//
// TODO documentation
//
func UpdateEventTaskStatus(etid int64, status int) error {
	var dummy string
	if err := db.QueryRow("UPDATE event_tasks SET status=$1 WHERE id=$2 "+
		"RETURNING id", status, etid).Scan(&dummy); err != nil {
		return err
	}
	return nil
}

// TODO
func GetActiveEventTasks(token string) ([]*EventTask, error) {
	var tasks []*EventTask

	rows, err := db.Query("SELECT group_tasks.id FROM event_tasks "+
		"NATURAL JOIN group_tasks "+
		"WHERE group_tasks.uid = (SELECT id FROM users WHERE token = $1) "+
		"AND event_tasks.status = $2", token, Active)
	if err != nil {
		if err == sql.ErrNoRows {
			return tasks, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tid int64
		if err := rows.Scan(&tid); err != nil {
			return nil, err
		}
		task, err := GetEventTask(tid)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

//
// TODO documentation
//
func GetEventTask(etid int64) (*EventTask, error) {
	var hook_id sql.NullInt64
	task := EventTask{}

	if err := db.QueryRow("SELECT * FROM event_tasks WHERE id=$1", etid).
		Scan(&task.Id, &task.Name, &task.Status, &task.Event,
		&hook_id); err != nil {
		return nil, err
	}

	if hook_id.Valid {
		task.HookId = hook_id.Int64
	}

	group_task, err := getGroupTask(task.Id)
	if err != nil {
		return nil, err
	}

	task.User = group_task.user
	task.Project = group_task.project
	task.Bot = group_task.bot

	return &task, nil
}

//
// TODO documentation
//
// func GetHookId(etid int64) (int64, error) {
// 	var hookId int64
// 	if err := db.QueryRow("SELECT hook_id FROM event_tasks WHERE id=$1", etid).Scan(&hookId); err != nil {
// 		return 0, err
// 	}
// 	return hookId, nil
// }

func SetHookId(etid int64, hook_id int64) error {
	var dummy string
	if err := db.QueryRow("UPDATE event_tasks SET hook_id=$1 WHERE id=$2 "+
		"RETURNING id", hook_id, etid).Scan(&dummy); err != nil {
		return err
	}
	return nil
}

// TODO document this
func CancelTaskGroup(tid string) (interface{}, error) {
	var task_type int

	if err := db.QueryRow("WITH s AS ( "+
		"UPDATE schedule_tasks SET status = $2 WHERE id = $1 RETURNING 1 "+
		"), e AS ( "+
		"UPDATE event_tasks SET status = $2 WHERE id = $1 RETURNING 2 "+
		") "+
		"UPDATE onetime_tasks SET status = $2 WHERE id = $1 RETURNING 3 ", tid,
		Complete).Scan(&task_type); err != nil {
		if sql.ErrNoRows != err {
			return nil, err
		}
		task_type = 4
	}

	gid, _ := strconv.ParseInt(tid, 10, 64)

	switch {
	case task_type == 1:
		return GetScheduledTask(gid)
	case task_type == 2:
		return GetEventTask(gid)
	case task_type == 3:
		return GetOneTimeTask(gid)
	case task_type == 4:
		return GetInstantTask(gid)
	}

	return nil, fmt.Errorf("Ups! This should not happen!")
}
