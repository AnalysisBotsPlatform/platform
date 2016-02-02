// Controller of the Analysis Bot Platform webservice.
package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/AnalysisBotsPlatform/platform/db"
	"github.com/AnalysisBotsPlatform/platform/utils"
	"github.com/AnalysisBotsPlatform/platform/worker"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

//
// Global state
//

// Application identification (used for GitHub)
const app_id_var = "CLIENT_ID"
const app_secret_var = "CLIENT_SECRET"

var client_id = os.Getenv(app_id_var)
var client_secret = os.Getenv(app_secret_var)

// Session support
const session_auth_var = "SESSION_AUTH"
const session_enc_var = "SESSION_ENC"

var session_store = sessions.NewCookieStore(
	[]byte(os.Getenv(session_auth_var)), []byte(os.Getenv(session_enc_var)))

// Database authentication
const db_host_var = "DB_HOST"
const db_user_var = "DB_USER"
const db_pass_var = "DB_PASS"
const db_name_var = "DB_NAME"

var db_host = os.Getenv(db_host_var)
var db_user = os.Getenv(db_user_var)
var db_pass = os.Getenv(db_pass_var)
var db_name = os.Getenv(db_name_var)

// File system cache
const cache_path_var = "CACHE_PATH"

var cache_path = os.Getenv(cache_path_var)

// Administrator
const admin_user_var = "ADMIN_USER"

var admin_user = os.Getenv(admin_user_var)

// Worker service
const worker_port_var = "WORKER_PORT"

var worker_port = os.Getenv(worker_port_var)

// Hostname (necessary for webhooks)
const controller_host_var = "CONTROLLER_HOST"

var controller_host = os.Getenv(controller_host_var)

// webhook path
const webhook_subpath = "webhook"


// Id regex
const id_regex = "0|[1-9][0-9]*"

// Type regex
const type_regex = "[0-5]"

// Time regex
const time_regex = "[0-9]+"

// Event regex
const event_regex = time_regex

// Day regex
const day_regex = "[0-6]"

// Template caching
const template_root = "tmpl"

var templates = template.Must(template.ParseGlob(
	fmt.Sprintf("%s/[^.]*", template_root),
))

//
// Application constants
//

// Interval in seconds for canceling timed over tasks
const time_check_interval = 10

// Number of character used to communicate with GitHub (secret message).
const state_size = 32

// Context settings
var error_counter = 0
var error_map = make(map[string]interface{})

// Webhooks

type WebhookConfig struct{
    url             string
    content_type    string
}

type Webhook struct{
    name        string
    active      bool
    events      []string
    config      WebhookConfig
}

//
// Entry point
//

// The `Start` function sets up the environment for the controller and calls the
// http ListenAndServe() function which actually starts the web service.
//
// In the beginning environment variables containing process relevant
// information are retrieved from the operating system. These variables are:
//
// - CLIENT_ID: The identifier retrieved from GitHub when registering this
// application as a GitHub application.
//
// - CLIENT_SECRET: The client/application secret retrieved from GitHub when
// registering this application as a GitHub application.
//
// - SESSION_AUTH: A random string authenticating this session uniquely among
// others.
//
// - SESSION_ENC: A random string used to encrypt the sessions.
//
// - DB_HOST: The host address where the postgresql database instance is
// running.
//
// - DB_USER: The database user (here a postgresql database is used, hence this
// is a postgres-user) owning the database specified by DB_NAME.
//
// - DB_PASS: The password of the database user (needed to access the database).
//
// - DB_NAME: Name of the database where all necessary tables are accessible.
//
// - CACHE_PATH: The file system path where the different components can store
// their data.
//
// - ADMIN_USER: GitHub user name of the system administrator. TODO: apply this
// to the database
//
// - WORKER_PORT: Port where all worker related communication takes place.
//
// In case some of the variables are missing a corresponding message is prompted
// to the standard output and the function terminates without any further
// action.
//
// If all environment variables were provided, the `OpenDB` function of the `db`
// package is called in order to establish a connection to the database.
//
// In case of another error the function terminates with a corresponding message
// and without any further actions.
//
// Next a channel `sigs` is created in order to listen for signals from the
// operating system, in particular for termination signals.
//
// Then a new goroutine listening on that channel is executed concurrently.
// Whenever something is received on the `sigs` channel the database connection
// is closed and the system exits with the status code 0.
//
// Finally, the `ListenAndServe` function of the http package is called in order
// to listen on port 8080 for incoming http requests. The router used to
// demultiplex paths and calling the respective handlers is created by the
// `initRoutes` call.
func Start() {
	// check environment
	_, id := os.LookupEnv(app_id_var)
	_, secret := os.LookupEnv(app_secret_var)
	_, auth := os.LookupEnv(session_auth_var)
	session_enc, enc := os.LookupEnv(session_enc_var)
	_, host := os.LookupEnv(db_host_var)
	_, user := os.LookupEnv(db_user_var)
	_, pass := os.LookupEnv(db_pass_var)
	_, name := os.LookupEnv(db_name_var)
	_, cache := os.LookupEnv(cache_path_var)
	_, admin := os.LookupEnv(admin_user_var)
	_, wport := os.LookupEnv(worker_port_var)
	if !id || !secret || !auth || !enc || !host || !user || !pass || !name ||
		!cache || !admin || !wport {
		fmt.Printf("Application settings missing!\n"+
			"Please set the %s, %s, %s, %s, %s, %s, %s, %s, %s, %s and %s "+
			"environment variables.\n", app_id_var, app_secret_var,
			session_auth_var, session_enc_var, db_host_var, db_user_var,
			db_pass_var, db_name_var, cache_path_var, admin_user_var,
			worker_port_var)
		return
	}

	// session encryption string has length of 32?
	if len(session_enc) != 32 {
		fmt.Println("The session encryption string has not a length of 32!")
		return
	}

	// initialize database connection
	fmt.Println("Controller start ...")
	if err := db.OpenDB(db_host, db_user, db_pass, db_name); err != nil {
		fmt.Println("Cannot connect to database.")
		fmt.Println(err)
		return
	}

	// initialize background worker
	if dir, err := filepath.Abs(cache_path); err != nil {
		fmt.Println(err)
		return
	} else {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			fmt.Println("Cache directory does not exist!")
			fmt.Println(err)
			return
		} else {
			if err := worker.Init(worker_port); err != nil {
				fmt.Println(err)
				return
			}
		}
	}

	// goroutine for cancelation of tasks
	ticker := time.NewTicker(time.Second * time_check_interval)
	go func() {
		for range ticker.C {
			worker.CancleTimedOverTasks()
		}
	}()

	// make sure database connection gets closed
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		ticker.Stop()
        worker.StopPeriodRunner()
		db.CloseDB()
		fmt.Println("... controller terminated")
		os.Exit(0)
	}()

	// listen on port 8080 to handle http requests
	if err := http.ListenAndServe(":8080", initRoutes()); err != nil {
		fmt.Println("Controller listen failed:")
		fmt.Println(err)
	}
}

//
// Helper functions
//

// Register all routes and their handlers.
func initRoutes() (rootRouter *mux.Router) {
	// declare routers
	rootRouter = mux.NewRouter()
	botsRouter := rootRouter.PathPrefix("/bots").Subrouter()
	projectsRouter := rootRouter.PathPrefix("/projects").Subrouter()
	tasksRouter := rootRouter.PathPrefix("/tasks").Subrouter()

	// register handlers for http requests

	// root
	rootRouter.HandleFunc("/", makeHandler(handleAuth)).Queries("code", "")
	rootRouter.HandleFunc("/", makeHandler(handleRoot))
	rootRouter.HandleFunc("/{file:.*\\.js}", makeHandler(handleJavaScripts))
	rootRouter.HandleFunc("/login", makeHandler(handleLogin))
	rootRouter.HandleFunc("/logout", makeHandler(handleLogout))
	// NOTE: Not implemented yet.
	rootRouter.HandleFunc("/user", makeHandler(makeTokenHandler(handleUser)))
    rootRouter.HandleFunc(fmt.Sprintf("/%s", webhook_subpath), handleWebhook)

	// bots
	botsRouter.HandleFunc("/", makeHandler(makeTokenHandler(handleBots)))
	botsRouter.HandleFunc("/new",
		makeHandler(makeTokenHandler(handleBotsNewForm))).Methods("GET")
	botsRouter.HandleFunc("/new",
		makeHandler(makeTokenHandler(handleBotsNewPost))).Methods("POST")
	botsRouter.HandleFunc(fmt.Sprintf("/{bid:%s}", id_regex),
		makeHandler(makeTokenHandler(handleBotsBid)))
	botsRouter.HandleFunc(fmt.Sprintf("/{bid:%s}/newtask", id_regex),
		makeHandler(makeTokenHandler(handleBotsBidNewtask)))
    botsRouter.HandleFunc(fmt.Sprintf("/{bid:%s}/{pid:%s}/0/{hours:%s}", id_regex, id_regex, time_regex),
		makeHandler(makeTokenHandler(handleTasksNewHourly)))
    botsRouter.HandleFunc(fmt.Sprintf("/{bid:%s}/{pid:%s}/1/{hour:%s}", id_regex, id_regex, time_regex),
		makeHandler(makeTokenHandler(handleTasksNewDaily)))
    botsRouter.HandleFunc(fmt.Sprintf("/{bid:%s}/{pid:%s}/2/{weekday:%s}/{hour:%s}", id_regex, id_regex, day_regex, time_regex),
		makeHandler(makeTokenHandler(handleTasksNewWeekly)))
    botsRouter.HandleFunc(fmt.Sprintf("/{bid:%s}/{pid:%s}/3/{hour:%s}", id_regex, id_regex, time_regex),
		makeHandler(makeTokenHandler(handleTasksNewOneTime)))
    botsRouter.HandleFunc(fmt.Sprintf("/{bid:%s}/{pid:%s}/4", id_regex, id_regex),
		makeHandler(makeTokenHandler(handleTasksNewInstant)))
    botsRouter.HandleFunc(fmt.Sprintf("/{bid:%s}/{pid:%s}/5/{event:%s}", id_regex, id_regex, event_regex),
		makeHandler(makeTokenHandler(handleTasksNewEventDriven)))


	// projects
	projectsRouter.HandleFunc("/",
		makeHandler(makeTokenHandler(handleProjects)))
	projectsRouter.HandleFunc(fmt.Sprintf("/{pid:%s}", id_regex),
		makeHandler(makeTokenHandler(handleProjectsPid)))
	projectsRouter.HandleFunc(fmt.Sprintf("/{pid:%s}/newtask", id_regex),
		makeHandler(makeTokenHandler(handleProjectsPidNewtask)))
    projectsRouter.HandleFunc(fmt.Sprintf("/{pid:%s}/{bid:%s}/0/{hours:%s}", id_regex, id_regex, time_regex),
		makeHandler(makeTokenHandler(handleTasksNewHourly)))
    projectsRouter.HandleFunc(fmt.Sprintf("/{pid:%s}/{bid:%s}/1/{hour:%s}", id_regex, id_regex, time_regex),
		makeHandler(makeTokenHandler(handleTasksNewDaily)))
    projectsRouter.HandleFunc(fmt.Sprintf("/{pid:%s}/{bid:%s}/2/{weekday:%s}/{hour:%s}", id_regex, id_regex, day_regex, time_regex),
		makeHandler(makeTokenHandler(handleTasksNewWeekly)))
    projectsRouter.HandleFunc(fmt.Sprintf("/{pid:%s}/{bid:%s}/3/{hour:%s}", id_regex, id_regex, time_regex),
		makeHandler(makeTokenHandler(handleTasksNewOneTime)))
    projectsRouter.HandleFunc(fmt.Sprintf("/{pid:%s}/{bid:%s}/4", id_regex, id_regex),
		makeHandler(makeTokenHandler(handleTasksNewInstant)))
    projectsRouter.HandleFunc(fmt.Sprintf("/{pid:%s}/{bid:%s}/5/{event:%s}", id_regex, id_regex, event_regex),
		makeHandler(makeTokenHandler(handleTasksNewEventDriven)))

	// tasks
	tasksRouter.HandleFunc("/", makeHandler(makeTokenHandler(handleTasks)))
	tasksRouter.HandleFunc(fmt.Sprintf("/{tid:%s}", id_regex),
		makeHandler(makeTokenHandler(handleTasksTid)))
	tasksRouter.HandleFunc(fmt.Sprintf("/{tid:%s}/cancel", id_regex),
		makeHandler(makeTokenHandler(handleTasksTidCancel)))

	return
}

// Function closure for `http.HandlerFunc`. Retrieves the session and path
// variables and then executes the given handler with this additional
// information.
func makeHandler(
	fn func(http.ResponseWriter, *http.Request, map[string]string,
		*sessions.Session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		session, err := session_store.Get(r, "analysis-bots")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fn(w, r, vars, session)
	}
}

// Function closure for `makeHandler`. In addition to `makeHandler` the token is
// read from the cookie.
func makeTokenHandler(
	fn func(http.ResponseWriter, *http.Request, map[string]string,
		*sessions.Session, string)) func(http.ResponseWriter, *http.Request,
	map[string]string, *sessions.Session) {

	return func(w http.ResponseWriter, r *http.Request, vars map[string]string,
		session *sessions.Session) {

		token, err := getTokenOrRedirect(w, r, session)
		if err != nil {
			return
		}
		fn(w, r, vars, session, token)
	}
}

// The function renders the given template `tmpl`. This is done by injecting
// `data` into the cached template. In case `tmpl` does not exist, an internal
// server error is triggered.
func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	err := templates.ExecuteTemplate(w,
		fmt.Sprintf("%s.html", tmpl), data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// The function sends a request `req_url` to GitHub. After receiving a
// successful response the result data in JSON format is decoded and returned.
// In case of an unexpected error, the error is returned.
func authGitHubRequest(w http.ResponseWriter, req_url string,
	token string) (interface{}, error) {
	// set up request parameters
	data := url.Values{}

	// set up request
	client := &http.Client{}
	req, _ := http.NewRequest("GET",
		fmt.Sprintf("https://api.github.com/%s", req_url),
		bytes.NewBufferString(data.Encode()))
	// req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("token %s", token))

	// do request
	response, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer response.Body.Close()

	// read response
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("Bad request!")
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	var resp_data interface{}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&resp_data); err != nil {
		return nil, errors.New("Decoding error!")
	}

	return resp_data, nil
}




// TODO document this
func authGitHubRequestPost(w http.ResponseWriter, req_url string,
	token string, payload []byte) (interface{}, error) {
	
	// set up request
	client := &http.Client{}
	req, _ := http.NewRequest("POST",
		fmt.Sprintf("https://api.github.com/%s", req_url),
		bytes.NewBuffer(payload))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("token %s", token))

	// do request
	response, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer response.Body.Close()

	// read response
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("Bad request!")
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	var resp_data interface{}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&resp_data); err != nil {
		return nil, errors.New("Decoding error!")
	}

	return resp_data, nil
}

// TODO document this
func authGitHubRequestDelete(w http.ResponseWriter, req_url string,
	token string) (interface{}, error) {
	
    data := url.Values{}
    
	// set up request
	client := &http.Client{}
	req, _ := http.NewRequest("DELETE",
		fmt.Sprintf("https://api.github.com/%s", req_url),
		bytes.NewBufferString(data.Encode()))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("token %s", token))

	// do request
	response, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer response.Body.Close()

	// read response
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("Bad request!")
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	var resp_data interface{}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&resp_data); err != nil {
		return nil, errors.New("Decoding error!")
	}

	return resp_data, nil
}

// Error handling routine. The user is redirected to the index page and an error
// message (stored in `error_map`) is displayed.
func handleError(w http.ResponseWriter, r *http.Request, err error) {
	error_counter++
	error_map[strconv.Itoa(error_counter)] = err
	http.Redirect(w, r, fmt.Sprintf("/?err=%d", error_counter),
		http.StatusFound)
}

// The function checks whether the session is valid. If this is the case the
// GitHub authentication token is returned. The token is stored in a cookie on
// the user's machine. Otherwise the function `handleError` is called and an
// error is returned.
func getTokenOrRedirect(w http.ResponseWriter, r *http.Request,
	session *sessions.Session) (string, error) {
	if token, ok := session.Values["token"].(string); ok {
		return token, nil
	} else {
		handleError(w, r, errors.New("User token not available!"))
	}
	return "", errors.New("User token not available!")
}

//
// Route handler
//

// The handler checks whether the session `token` is valid. If this is the case
// the function `renderTemplate` with the template `index` is called else with
// the template `login`. It might be that this page is requested with an error
// parameter. Then the corresponding error message is forwarded and thus
// displayed.
func handleRoot(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session) {
	if _, ok := session.Values["token"]; ok {
		if err := r.FormValue("err"); err != "" {
			renderTemplate(w, "index", error_map[err])
			delete(error_map, err)
		} else {
			renderTemplate(w, "index", nil)
		}
	} else {
		if err := r.FormValue("err"); err != "" {
			renderTemplate(w, "login", error_map[err])
			delete(error_map, err)
		} else {
			renderTemplate(w, "login", nil)
		}
	}
}

// The handler requests the personal access token and the user profile from
// GitHub to create a user in the database.
//
// After the user grants access to its personal data on GitHub, the application
// redirects to the "Authorization callback URL" from the application settings
// page on GitHub. This request will be handled by this handler.
//
// To ensure that the connection to GitHub is not hijacked, the validity of the
// session is checked by comparing the state variable previously sent to GitHub
// and the one received by an URL like the one below.
//
// NOTE: Fill in correct URL
// 	e.g.: http://analysis-bots-platform.com/?code=<code>&state=<s>
//
// The callback URL contains a variable code which will be used in the next
// step.
// To get the access token a http POST request is sent to the following URL:
//
// 	https://github.com/login/oauth/access_token
//
// The POST request contains the variables below:
// - client_id: The client_id from the application setting page on GitHub.
// - client_secret: The client_secret from the application setting page on
// GitHub.
// - code: The code received in the response.
// - state: The random string used in the login handler.
//
// Since the response is requested in the JSON format, by specifying the
// "Accept" value in the http header accordingly the data of interest is
// extracted by unmarshalling the body using a JSON parser. After retrieving the
// token it will be stored in the user's cookie. To get the user information,
// the function `authGitHubRequest` with the `req_url` "user" is called. If this
// fails the session is closed to enforce a redirection to the login page.
// Otherwise the database is updated and the user is redirected to the index
// page.
func handleAuth(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session) {
	if state, ok := session.Values["state"].(string); ok {
		if state != r.FormValue("state") {
			handleError(w, r, errors.New("GitHub connection was hijacked!"))
			return
		}

		// set up request parameters
		data := url.Values{}
		data.Add("client_id", client_id)
		data.Add("client_secret", client_secret)
		data.Add("code", r.FormValue("code"))
		data.Add("state", state)

		// set up request
		client := &http.Client{}
		req, _ := http.NewRequest("POST",
			"https://github.com/login/oauth/access_token",
			bytes.NewBufferString(data.Encode()))
		req.Header.Set("Accept", "application/json")
        
		// do request
		response, err := client.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		defer response.Body.Close()

		// read response
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		var resp_data map[string]interface{}
		if err := json.Unmarshal(body, &resp_data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
        
		// store access token and user information
		token := resp_data["access_token"].(string)
		session.Values["token"] = token
		user_resp, err := authGitHubRequest(w, "user", token)
		if err != nil {
			session.Options.MaxAge = -1
		} else {
			if err := db.UpdateUser(user_resp, token); err != nil {
				handleError(w, r, err)
				return
			}
		}
		session.Save(r, w)
		http.Redirect(w, r, "/", http.StatusFound)
	} else {
		handleError(w, r, errors.New("No state available!"))
	}
}

// Delivers JavaScript files.
func handleJavaScripts(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session) {
	err := templates.ExecuteTemplate(w, vars["file"], nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// The handler redirects the user to the GitHub login page to get a personal
// access token to access the user's GitHub data.
// Temporarily, the random string state is used as session identifier because
// the access token does not exist yet. This is necessary to ensure that the
// GitHub connection is not hijacked.
//
// The construction of the URL is as follows:
//
// https://github.com/login/oauth/authorize?client_id=<id>&scope="user, repo"&state=<st>
//
// - client_id: The client_id from the application settings page on GitHub.
// - scope: The scope determines which parts of the user's data the application
// is allowed to access. In this case "user" grants read and write access to the
// user's profile info and "repo" read and write access to code, commit
// statuses, collaborators, and deployment statuses for public and private
// repositories and organizations.
// - state: State is a random string to protect against cross-site request
// forgery attacks.
func handleLogin(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session) {
	state := utils.RandString(state_size)
	session.Values["state"] = state
	session.Save(r, w)

	http.Redirect(w, r,
		fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s"+
			"&scope=%s&state=%s", client_id, "user,repo", state),
		http.StatusFound)
}

// The handler invalidates the user's cookie and redirects to the login page.
func handleLogout(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session) {
	session.Options.MaxAge = -1
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)
}

// NOTE: Not implemented yet.
func handleUser(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	renderTemplate(w, "user", nil)
}

// The handler requests information about all Bots from the database. If an
// error occurs the `handleError` function is called else `renderTemplate` with
// the template "bots" and the retrieved data.
func handleBots(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	bots, err := db.GetBots()
	if err != nil {
		handleError(w, r, err)
	} else {
		renderTemplate(w, "bots", bots)
	}
}

// The handler displays the form for adding new Bots.
func handleBotsNewForm(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	renderTemplate(w, "bots-new", nil)
}

// The handler gets called after the user successfully submitted a new Bot via
// the corresponding form. It verifies that all fields are submitted and
// non-empty. Then a request to the Docker API is sent to check whether the
// Bot's image exists. If all requirements are met the Bot is added to the
// database and the user is redirected to the Bot overview page. Else an error
// message is displayed.
func handleBotsNewPost(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	path := r.FormValue("path")
	description := r.FormValue("description")
	tags := r.FormValue("tags")
	if path == "" || description == "" || tags == "" {
		handleError(w, r, errors.New("Not all input fields were filled in!"))
		return
	}
	resp, err := http.Get(
		fmt.Sprintf("https://index.docker.io/v1/repositories/%s/tags", path))
	if err != nil {
		handleError(w, r, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		handleError(w, r, errors.New("Docker Hub entry does not exist!"))
		return
	}
	if err := db.AddBot(path, description, tags); err != nil {
		handleError(w, r, err)
		return
	}
	http.Redirect(w, r, "/bots/", http.StatusFound)
}

// The handler requests detailed information about the Bot identified by its id.
// If an error occurs the `handleError` function is called else `renderTemplate`
// with the template "bots-bid" and the retrieved data.
func handleBotsBid(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	bot, err := db.GetBot(vars["bid"])
	if err != nil {
		handleError(w, r, err)
	} else {
		renderTemplate(w, "bots-bid", bot)
	}
}

// The handler requests detailed information about the Bot identified by its id.
// In addition it requests information about all available projects. In order to
// get up to date information, GitHub is contacted and the newest project data
// is fetched. If an error occurs the `handleError` function is called else
// `renderTemplate` with the template "bots-bid-newtask" and the retrieved data.
func handleBotsBidNewtask(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	bot, errBot := db.GetBot(vars["bid"])
	if errBot != nil {
		handleError(w, r, errBot)
		return
	}
	response, err := authGitHubRequest(w, "user/repos", token)
	if err != nil {
		session.Options.MaxAge = -1
		session.Save(r, w)
		handleError(w, r, errBot)
	} else {
		projects, errProjects := db.UpdateProjects(response, token)
		if errProjects != nil {
			handleError(w, r, errProjects)
			return
		}
		if errBot == nil && errProjects == nil {
			data := make(map[string]interface{}, 2)
			data["Bot"] = bot
			data["Projects"] = projects
			renderTemplate(w, "bots-bid-newtask", data)
		}
	}
}

// The handler calls the function `authGitHubRequest` with the URL "user/repos"
// to get the up to date information about the user's projects from GitHub. If
// this fails the session is closed and the user is redirected to the index
// page. Otherwise the database is updated and the `renderTemplate` function
// with the "projects" template and the data from the database is called.
func handleProjects(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	response, err := authGitHubRequest(w, "user/repos", token)
	if err != nil {
		session.Options.MaxAge = -1
		session.Save(r, w)
		handleError(w, r, err)
	} else {
		projects, err := db.UpdateProjects(response, token)
		if err != nil {
			handleError(w, r, err)
		} else {
			renderTemplate(w, "projects", projects)
		}
	}
}

// The handler requests detailed information about the project identified by its
// id. If an error occurs the `handleError` function is called else
// `renderTemplate` with the template "projects-pid" and the retrieved data.
func handleProjectsPid(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	project, err := db.GetProject(vars["pid"], token)
	if err != nil {
		handleError(w, r, err)
	} else {
		renderTemplate(w, "projects-pid", project)
	}
}

// The handler requests detailed information about the project identified by its
// id. In addition it requests information about all available Bots. If an error
// occurs the `handleError` function is called else `renderTemplate` with the
// template "projects-pid-newtask" and the retrieved data.
func handleProjectsPidNewtask(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	project, errProject := db.GetProject(vars["pid"], token)
	if errProject != nil {
		handleError(w, r, errProject)
		return
	}
	bots, errBots := db.GetBots()
	if errBots != nil {
		handleError(w, r, errBots)
		return
	}
	if errProject == nil && errBots == nil {
		data := make(map[string]interface{}, 2)
		data["Project"] = project
		data["Bots"] = bots
		renderTemplate(w, "projects-pid-newtask", data)
	}
}

// The handler requests information about all tasks ran by the user. If an error
// occurs the `handleError` function is called else `renderTemplate` with the
// template "tasks" and the retrieved data.
func handleTasks(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
    hErr := updateHooks(w, token)
    if(hErr != nil){
        handleError(w, r, hErr)
        return
    }
    
// TODO check this
	tasks, err := db.GetScheduledTasks(token)
	if err != nil {
		handleError(w, r, err)
	} else {
		renderTemplate(w, "tasks", tasks)
	}
}

// The handler requests detailed information about the task identified by its
// id. If an error occurs the `handleError` function is called else
// `renderTemplate` with the template "projects-pid" and the retrieved data.
func handleTasksTid(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
    
    hErr := updateHooks(w, token)
    if(hErr != nil){
        handleError(w, r, hErr)
        return
    }
    
    // TODO implement this

    
	task, err := db.GetTask(vars["tid"], token)
	if err != nil {
		handleError(w, r, err)
	} else {
		task.Output = template.HTMLEscapeString(task.Output)
		task.Output = strings.Replace(task.Output, "\n", "<br>", -1)
		output := template.HTML(task.Output)
		data := make(map[string]interface{}, 2)
		data["Task"] = task
		data["Output"] = output
		renderTemplate(w, "tasks-tid", data)
	}
}




// TODO document this
func handleTasksNewEventDriven(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string){
    
    hErr := updateHooks(w, token)
    if(hErr != nil){
        handleError(w, r, hErr)
        return
    }
   
    event, eErr := strconv.ParseInt(vars["event"], 10, 64)
    if(eErr != nil){
        handleError(w, r, eErr) 
        return
    }
    
    // TODO updated version
    schedTask, err := db.CreateNewScheduledTaskEventDriven(vars["name"], token,
                                            vars["pid"], vars["bid"], event)
    
    if err != nil{
        handleError(w, r, err)
    }else{
        
        project, projErr := db.GetProject(vars["pid"], token)
        if(projErr != nil){
            handleError(w, r, projErr)
        }else{
            reqUrl := fmt.Sprintf("/repos/%s/hooks", project.Name)
            
            var hookConfig = WebhookConfig{
                url : fmt.Sprintf("http://%s/%s/%d", controller_host, 
                                  webhook_subpath, schedTask.Id),
                content_type: "json" }
            
            var hook = Webhook{
                name: "web",
                active: true,
                events: []string{vars["event"]},
                config: hookConfig }
            
            payloadMarshalled, marshErr := json.Marshal(hook)
            if(marshErr != nil){
                handleError(w, r, marshErr)
            }else{
                authGitHubRequestPost(w, reqUrl, token, payloadMarshalled)
                http.Redirect(w, r, fmt.Sprintf("/tasks/%d", schedTask.Id),
                              http.StatusFound)
            }
            
        }
        
    }
}


func handleTasksNewHourly(w http.ResponseWriter, r *http.Request,
    vars map[string]string, session *sessions.Session, token string){
    
    currentTime := time.Now()
    
    hourPeriod := vars["hours"]    
    hourP, err := strconv.ParseInt(hourPeriod, 10, 64)
    if(err != nil){
        handleError(w, r, err)
        return
    }
    // TODO updated version
    schedTask, err := db.CreateNewScheduledTaskHourly(vars["name"], token,
                                        vars["pid"], vars["bid"], currentTime, hourP)
    if(err != nil){
        handleError(w, r, err)
        return
    }else{
        http.Redirect(w, r, fmt.Sprintf("/tasks/%d", schedTask.Id),
                      http.StatusFound)
    }
    
    worker.UpdatePeriodTimer()       
    
}

func handleTasksNewDaily(w http.ResponseWriter, r *http.Request,
    vars map[string]string, session *sessions.Session, token string){
     
    seconds, err := strconv.ParseInt(vars["hour"], 10, 64)
    if(err != nil){
        handleError(w, r, err)
    }else{
        currentTime := time.Now()
        scheduleTime := time.Unix(seconds, 0)
        scheduleTime = time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), scheduleTime.Hour(), scheduleTime.Minute(), scheduleTime.Second(), scheduleTime.Nanosecond(), currentTime.Location())
    
        // TODO updated version
        schedTask, err := db.CreateNewScheduledTaskDaily(vars["name"], token,
                                            vars["pid"], vars["bid"], scheduleTime)
        
        if(err != nil){
            handleError(w, r, err)
            return
        }else{
            http.Redirect(w, r, fmt.Sprintf("/tasks/%d", schedTask.Id),
                          http.StatusFound)
        }

        worker.UpdatePeriodTimer() 
    }
    
    
}

// TODO document this
func handleTasksNewWeekly(w http.ResponseWriter, r *http.Request,
    vars map[string]string, session *sessions.Session, token string){

    day, dayErr := strconv.ParseInt(vars["weekday"], 10, 64)
    if(dayErr != nil){
        handleError(w, r, dayErr)
    }else{
       seconds, err := strconv.ParseInt(vars["hour"], 10, 64)
        if(err != nil){
            handleError(w, r, err)
        }else{
            scheduleHour := time.Unix(seconds, 0)
            currentTime := time.Now();
            scheduleTime := time.Date(currentTime.Year(), currentTime.Month(),
                                      currentTime.Day(), scheduleHour.Hour(), scheduleHour.Minute(),
                                      scheduleHour.Second(), scheduleHour.Nanosecond(),currentTime.Location())
            scheduleTime = worker.ComputeDate(scheduleTime, int(day))
            
            // TODO updated version
            schedTask, err := db.CreateNewScheduledTaskWeekly(vars["name"], token,
                                                vars["pid"], vars["bid"], scheduleTime, day)

            if(err != nil){
                handleError(w, r, err)
            }else{
                http.Redirect(w, r, fmt.Sprintf("/tasks/%d", schedTask.Id),
                              http.StatusFound)
            }

            worker.UpdatePeriodTimer() 
        } 
    }
}




// TODO document this
func handleTasksNewOneTime(w http.ResponseWriter, r *http.Request,
                          vars map[string]string, session *sessions.Session, token string){
    
    
    seconds, err := strconv.ParseInt(vars["hour"], 10, 64)
    if(err != nil){
        handleError(w, r, err)
    }else{
        scheduleTime := time.Unix(seconds, 0)
         // TODO updated version
        schedTask, err := db.CreateNewScheduledTaskOneTime(vars["name"], token,
                                            vars["pid"], vars["bid"], scheduleTime)

        if(err != nil){
            handleError(w, r, err)
        }else{
            http.Redirect(w, r, fmt.Sprintf("/tasks/%d", schedTask.Id),
                          http.StatusFound)
        }

        worker.UpdatePeriodTimer()
    }
    
}

// TODO document this
func handleTasksNewInstant(w http.ResponseWriter, r *http.Request,
                          vars map[string]string, session *sessions.Session, token string){
    
    

    scheduleTime := time.Now()
     // TODO updated version
    schedTask, err := db.CreateNewScheduledTaskOneTime(vars["name"], token,
                                        vars["pid"], vars["bid"], scheduleTime)

    if(err != nil){
        handleError(w, r, err)
    }else{
        http.Redirect(w, r, fmt.Sprintf("/tasks/%d", schedTask.Id),
                      http.StatusFound)
    }

    worker.UpdatePeriodTimer()

    
}

// TODO document this
func handleWebhook(w http.ResponseWriter, r *http.Request){
    vars := mux.Vars(r)
    taskId := vars["tid"]
    
    body, err := ioutil.ReadAll(r.Body)
    if err != nil {
        // TODO err0r handling
    }
    
    type hook struct{
        Hook_id int64
    }
    
    var h hook
    err = json.Unmarshal(body, &h)
    if err != nil {
        // TODO error handling
        return
    }
    
    tid, iErr := strconv.ParseInt(taskId, 10, 64)
    if(iErr != nil ){
        // TODO error handling
        return
    }
    
    // sanity check
    hookId, tErr := db.GetHookId(tid)
    if(tErr != nil ){
        // TODO error handling
        return
    }
    if(hookId != h.Hook_id){
        // TODO error handling
        return
    }
    
    worker.CreateNewTask(tid)
}



// TODO document this
func handleTasksTidCancel(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
    
    task, pErr := db.GetScheduledTask(vars["tid"], token, false)
    if(pErr != nil){
        handleError(w, r, pErr)
        return
    }
    if(task.Type == db.Event){
        hookId, hErr := db.GetHookId(task.Id)
        if(hErr != nil){
            handleError(w, r, hErr)
            return
        }
        url := fmt.Sprintf("/repos/%s/hooks/%d", task.Project.Name, hookId)
        _, dErr := authGitHubRequestDelete(w, url, token)
        if(dErr != nil){
            handleError(w, r, dErr)
            return
        }
    }
    
    
    tid, err := strconv.ParseInt(vars["tid"], 10, 64)
    if(err != nil){
        handleError(w, r, err)
        return
    }
    
    children, cErr := db.GetRunningChildren(tid)
    if(cErr != nil){
        handleError(w, r, cErr)
        return
    }
    
    for _, task := range children{
        worker.Cancle(task.Id)        
    }
    
    
    
    db.UpdateScheduledTaskStatus(tid, db.Complete)    
	
    http.Redirect(w, r, "/tasks/", http.StatusFound)
	
}


// TODO document this
func updateHooks(w http.ResponseWriter, token string) (error){
    
    
    
    tasks, err := db.GetRunningScheduledTasks(token)
    if(err != nil){
        return err
    }
    
    for _,task := range tasks{
        hook_id, hErr := db.GetHookId(task.Id)
        if(hErr != nil){
        return hErr
    }
        url := fmt.Sprintf("/repos/%s/hooks/%d",task.Project.Name, hook_id)
        _, rErr := authGitHubRequest(w, url, token)
        if(rErr != nil){
            db.UpdateScheduledTaskStatus(task.Id, db.Complete)    
        }
    }
    
    return nil    
    
}
