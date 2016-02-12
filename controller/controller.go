// Controller of the Analysis Bot Platform webservice.
package controller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/AnalysisBotsPlatform/platform/db"
	"github.com/AnalysisBotsPlatform/platform/utils"
	"github.com/AnalysisBotsPlatform/platform/worker"
	"github.com/gorhill/cronexpr"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
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

// Application access
const application_host_var = "APP_HOST"
const application_port_var = "APP_PORT"
const application_subdirectory_var = "APP_SUBDIR"

var application_host = os.Getenv(application_host_var)
var application_port = os.Getenv(application_port_var)
var application_subdirectory = os.Getenv(application_subdirectory_var)

// Worker service
const worker_port_var = "WORKER_PORT"

var worker_port = os.Getenv(worker_port_var)

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

// Bot name regex
const bot_name_regex = "[a-z]+[a-z,0-9]*"

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
var error_guard *sync.RWMutex = &sync.RWMutex{}

// Webhooks

type WebhookConfig struct {
	Url          string `json:"url"`
	Content_type string `json:"content_type"`
}

type Webhook struct {
	Name   string        `json:"name"`
	Active bool          `json:"active"`
	Events []string      `json:"events"`
	Config WebhookConfig `json:"config"`
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
// - ADMIN_USER: GitHub user name of the system administrator. NOTE Not
// implemented yet.
//
// - WORKER_PORT: Port where all worker related communication takes place.
//
// - APP_HOST: Host where the application is accessible.
//
// - APP_PORT: Port where the application is accessible.
//
// - APP_SUBDIR: Subdirectory where the application is reachable.
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
// to listen on port APP_PORT for incoming http requests. The router used to
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
	_, aport := os.LookupEnv(application_port_var)
	_, subdir := os.LookupEnv(application_subdirectory_var)
	_, hname := os.LookupEnv(application_host_var)
	if !id || !secret || !auth || !enc || !host || !user || !pass || !name ||
		!cache || !admin || !wport || !aport || !subdir || !hname {
		fmt.Printf("Application settings missing!\n"+
			"Please set the %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, "+
			"%s and %s environment variables.\n", app_id_var, app_secret_var,
			session_auth_var, session_enc_var, db_host_var, db_user_var,
			db_pass_var, db_name_var, cache_path_var, admin_user_var,
			worker_port_var, application_port_var, application_subdirectory_var,
			application_host_var)
		return
	}

	// session encryption string has length of 32?
	if len(session_enc) != 32 {
		fmt.Println("The session encryption string has not a length of 32!")
		return
	}

	// ensure that application_subdirectory starts and ends with /
	if application_subdirectory[0] != '/' ||
		application_subdirectory[len(application_subdirectory)-1] != '/' {
		fmt.Println("The applications subdirectory should start and end with /")
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
			fmt.Printf("Create cache directory %s\n", dir)
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Println("Cache directory cannot be created!")
				fmt.Println(err)
				return
			}
		}
		if err := worker.Init(worker_port, cache_path); err != nil {
			fmt.Println(err)
			return
		}
	}

	// goroutine for cancelation of tasks
	ticker := time.NewTicker(time.Second * time_check_interval)
	go func() {
		for range ticker.C {
			worker.CancelTimedOverTasks()
		}
	}()

	// make sure database connection gets closed
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		ticker.Stop()
		worker.StopPeriodRunners()
		db.CloseDB()
		fmt.Println("... controller terminated")
		os.Exit(0)
	}()

	// listen on port APP_PORT to handle http requests
	if err := http.ListenAndServe(fmt.Sprintf(":%s", application_port),
		initRoutes()); err != nil {
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
	botsRouter := rootRouter.PathPrefix(fmt.Sprintf("%sbots",
		application_subdirectory)).Subrouter()
	projectsRouter := rootRouter.PathPrefix(fmt.Sprintf("%sprojects",
		application_subdirectory)).Subrouter()
	tasksRouter := rootRouter.PathPrefix(fmt.Sprintf("%stasks",
		application_subdirectory)).Subrouter()
	apiRouter := rootRouter.PathPrefix(fmt.Sprintf("%sapi",
		application_subdirectory)).Subrouter()

	// register handlers for http requests

	// root
	rootRouter.HandleFunc(fmt.Sprintf("%s", application_subdirectory),
		makeHandler(handleAuth)).Queries("code", "")
	rootRouter.HandleFunc(fmt.Sprintf("%s", application_subdirectory),
		makeHandler(handleRoot))
	rootRouter.HandleFunc(fmt.Sprintf("%s{file:.*\\.js}",
		application_subdirectory), makeHandler(handleJavaScripts))
	rootRouter.HandleFunc(fmt.Sprintf("%slogin", application_subdirectory),
		makeHandler(handleLogin))
	rootRouter.HandleFunc(fmt.Sprintf("%slogout", application_subdirectory),
		makeHandler(handleLogout))
	rootRouter.HandleFunc(fmt.Sprintf("%suser", application_subdirectory),
		makeHandler(makeTokenHandler(handleUser)))
	rootRouter.HandleFunc(fmt.Sprintf("%suser/api_token",
		application_subdirectory),
		makeHandler(makeTokenHandler(handleUserNewAPIToken))).Methods("POST")
	rootRouter.HandleFunc(fmt.Sprintf("%suser/api_token/revoke",
		application_subdirectory),
		makeHandler(makeTokenHandler(handleUserRevokeAPIToken)))
	rootRouter.HandleFunc(fmt.Sprintf("%suser/worker/deregister",
		application_subdirectory),
		makeHandler(makeTokenHandler(handleUserDegegisterWorker)))
	rootRouter.HandleFunc(fmt.Sprintf("%scache/patches/{patch:.*\\.patch}",
		application_subdirectory),
		makeHandler(makeTokenHandler(handlePatchDownload)))
	rootRouter.HandleFunc(fmt.Sprintf("%snewpullrequest/{tid:.%s}",
		application_subdirectory, id_regex),
		makeHandler(makeTokenHandler(handlePullRequestNew)))
	rootRouter.HandleFunc(fmt.Sprintf("%s%s/{tid:%s}", application_subdirectory,
		webhook_subpath, id_regex), handleWebhook)

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
	botsRouter.HandleFunc(fmt.Sprintf("/{bid:%s}/{pid:%s}", id_regex, id_regex),
		makeHandler(makeTokenHandler(handleTasksNewScheduled))).
		Queries("cron", "")
	botsRouter.HandleFunc(fmt.Sprintf("/{bid:%s}/{pid:%s}", id_regex, id_regex),
		makeHandler(makeTokenHandler(handleTasksNewOneTime))).
		Queries("time", "")
	botsRouter.HandleFunc(fmt.Sprintf("/{bid:%s}/{pid:%s}", id_regex, id_regex),
		makeHandler(makeTokenHandler(handleTasksNewEventDriven))).
		Queries("type", "")
	botsRouter.HandleFunc(fmt.Sprintf("/{bid:%s}/{pid:%s}", id_regex, id_regex),
		makeHandler(makeTokenHandler(handleTasksNewInstant)))

	// projects
	projectsRouter.HandleFunc("/",
		makeHandler(makeTokenHandler(handleProjects)))
	projectsRouter.HandleFunc(fmt.Sprintf("/{pid:%s}", id_regex),
		makeHandler(makeTokenHandler(handleProjectsPid)))
	projectsRouter.HandleFunc(fmt.Sprintf("/{pid:%s}/newtask", id_regex),
		makeHandler(makeTokenHandler(handleProjectsPidNewtask)))
	projectsRouter.HandleFunc(
		fmt.Sprintf("/{pid:%s}/{bid:%s}", id_regex, id_regex),
		makeHandler(makeTokenHandler(handleTasksNewScheduled))).
		Queries("cron", "")
	projectsRouter.HandleFunc(
		fmt.Sprintf("/{pid:%s}/{bid:%s}", id_regex, id_regex),
		makeHandler(makeTokenHandler(handleTasksNewOneTime))).
		Queries("time", "")
	projectsRouter.HandleFunc(
		fmt.Sprintf("/{pid:%s}/{bid:%s}", id_regex, id_regex),
		makeHandler(makeTokenHandler(handleTasksNewEventDriven))).
		Queries("type", "")
	projectsRouter.HandleFunc(
		fmt.Sprintf("/{pid:%s}/{bid:%s}", id_regex, id_regex),
		makeHandler(makeTokenHandler(handleTasksNewInstant)))

	// tasks
	tasksRouter.HandleFunc("/", makeHandler(makeTokenHandler(handleTasks)))
	tasksRouter.HandleFunc(fmt.Sprintf("/{tid:%s}", id_regex),
		makeHandler(makeTokenHandler(handleTasksTid)))
	tasksRouter.HandleFunc(fmt.Sprintf("/{tid:%s}/cancel", id_regex),
		makeHandler(makeTokenHandler(handleTasksTidCancel)))
	tasksRouter.HandleFunc(fmt.Sprintf("/{tid:%s}/cancel_group", id_regex),
		makeHandler(makeTokenHandler(handleTasksTidCancelGroup)))

	// API
	apiRouter.HandleFunc("/bot", makeAPIHandler(handleAPIPostBot)).
		Methods("POST")
	apiRouter.HandleFunc("/bots", makeAPIHandler(handleAPIGetBots)).
		Methods("GET")
	apiRouter.HandleFunc("/projects", makeAPIHandler(handleAPIGetProjects)).
		Methods("GET")
	apiRouter.HandleFunc("/task", makeAPIHandler(handleAPIGetTask)).
		Methods("GET")
	apiRouter.HandleFunc("/task", makeAPIHandler(handleAPIPostTask)).
		Methods("POST")
	apiRouter.HandleFunc("/task", makeAPIHandler(handleAPIDeleteTask)).
		Methods("DELETE")
	apiRouter.HandleFunc("/tasks", makeAPIHandler(handleAPIGetTasks)).
		Methods("GET")

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

// Function closure for `http.HandlerFunc`. In addition to `http.HandlerFunc`
// the API token is read from the "Authentication" header field and verified.
func makeAPIHandler(
	fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authentication")
		if !db.IsValidAPIToken(token) {
			http.Error(w, "Unautherized access or number of accesses exceeded "+
				"allowed amount!", http.StatusNotFound)
			return
		}
		fn(w, r, token)
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
// Also supports pagination
func authGitHubRequest(method, req_url string, token string,
	payload map[string]interface{}, header map[string]string,
	expected_status int) (interface{}, error) {
	// set up request payload
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// set up request
	client := &http.Client{}
	req, _ := http.NewRequest(method,
		fmt.Sprintf("https://api.github.com/%s", req_url),
		bytes.NewBuffer(data))
	req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	for key, value := range header {
		req.Header.Set(key, value)
	}

	// do request
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// read response
	if response.StatusCode != expected_status {
		return nil, errors.New("Bad request!")
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	var resp_data interface{}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&resp_data); err != nil {
		return nil, errors.New("Decoding error!")
	}

	url, err := getNextUrl("next", response.Header.Get("Link"))
	if err != nil {
		return resp_data, nil
	}

	var resp_data_slice []interface{}

	for _, value := range resp_data.([]interface{}) {
		resp_data_slice = append(resp_data_slice, value)
	}

	for {
		req, _ := http.NewRequest(method, url, nil)
		req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
		for key, value := range header {
			req.Header.Set(key, value)
		}

		// do request
		response, err = client.Do(req)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()

		// read response
		if response.StatusCode != expected_status {
			return nil, errors.New("Bad request!")
		}
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
		var data interface{}
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber()
		if err := dec.Decode(&data); err != nil {
			return nil, errors.New("Decoding error!")
		}
		for _, value := range data.([]interface{}) {
			resp_data_slice = append(resp_data_slice, value)
		}

		url, err = getNextUrl("next", response.Header.Get("Link"))
		if err != nil {
			break
		}
	}

	return resp_data_slice, nil

}

// Extracts the URL from a list of strings from the "Link" field in a
// HTTP-Header specified by "specifier"var
// e.g. for
// <https://api.github.com/search/code?q=addClass+user%3Amozilla&page=2>;
// rel="next",
// <https://api.github.com/search/code?q=addClass+user%3Amozilla&page=34>;
// rel="last"
// as a link and the spcifier "next" this returns:
// https://api.github.com/search/code?q=addClass+user%3Amozilla&page=2
// If no match is being found it returns an error.
func getNextUrl(specifier, link string) (string, error) {
	regexpNext := regexp.MustCompile(fmt.Sprintf("<.*>;.*rel=\"%s\"", specifier))

	matches := regexpNext.FindAllString(link, -1)
	if len(matches) == 0 {
		return "", errors.New("No match found.")
	}

	url := matches[0]

	regexpUrl := regexp.MustCompile("<.*>")
	matches = regexpUrl.FindAllString(url, -1)
	if len(matches) == 0 {
		return "", errors.New("No match found.")
	}

	url = matches[0]

	url = strings.TrimPrefix(url, "<")
	url = strings.TrimSuffix(url, ">")
	return url, nil
}

// Error handling routine. The user is redirected to the index page and an error
// message (stored in `error_map`) is displayed.
func handleError(w http.ResponseWriter, r *http.Request, err error) {
	error_guard.Lock()
	defer error_guard.Unlock()

	error_counter++
	error_map[strconv.Itoa(error_counter)] = err
	http.Redirect(w, r, fmt.Sprintf("%s?err=%d", application_subdirectory,
		error_counter), http.StatusFound)
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

	data := make(map[string]interface{})
	data["Subdir"] = application_subdirectory

	error_guard.Lock()
	defer error_guard.Unlock()
	if err := r.FormValue("err"); err != "" {
		if _, ok := error_map[err]; ok {
			data["Error"] = error_map[err]
			delete(error_map, err)
		} else {
			data["Error"] = "Unknown error!"
		}
	}

	if token, ok := session.Values["token"]; ok {
		user_stats, err := db.GetUserStatistics(token.(string))
		if err != nil {
			// TODO error handling
			fmt.Println(err)
			return
		}

		tasks, err := db.GetLatestTasks(token.(string), 10)
		if err != nil {
			// TODO error handling
			fmt.Println(err)
			return
		}

		data["User_statistics"] = user_stats
		data["Latest_tasks_size"] = 10
		data["Latest_tasks"] = tasks

		renderTemplate(w, "index", data)
	} else {
		renderTemplate(w, "login", data)
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

		// verify user has granted the requested permissions
		scopes := resp_data["scope"].(string)
		perm := make(map[string]bool)
		for _, scope := range strings.Split(scopes, ",") {
			perm[scope] = true
		}
		_, ok_user := perm["user"]
		_, ok_repo := perm["repo"]
		_, ok_hooks := perm["admin:repo_hook"]
		if !ok_user || !ok_repo || !ok_hooks {
			handleError(w, r, fmt.Errorf("No permissions!"))
			return
		}

		// store access token and user information
		token := resp_data["access_token"].(string)
		session.Values["token"] = token
		user_resp, err := authGitHubRequest("GET", "user", token,
			make(map[string]interface{}), make(map[string]string),
			http.StatusOK)
		if err != nil {
			session.Options.MaxAge = -1
		} else {
			if err := db.UpdateUser(user_resp, token); err != nil {
				handleError(w, r, err)
				return
			}
		}
		session.Save(r, w)
		http.Redirect(w, r, application_subdirectory, http.StatusFound)
	} else {
		handleError(w, r, errors.New("No state available!"))
	}
}

// Delivers JavaScript files.
func handleJavaScripts(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session) {
	file_path := fmt.Sprintf("tmpl/%s", vars["file"])
	in, err := ioutil.ReadFile(file_path)
	if err != nil {
		handleError(w, r, err)
	} else {
		w.Write(in)
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
// TODO document scopes
// TODO verify scopes
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
			"&scope=%s&state=%s", client_id, "user,repo,admin:repo_hook",
			state), http.StatusFound)
}

// The handler invalidates the user's cookie and redirects to the login page.
func handleLogout(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session) {
	session.Options.MaxAge = -1
	session.Save(r, w)
	http.Redirect(w, r, application_subdirectory, http.StatusFound)
}

// The handler fetches all user specific data and statistics to pass them to
// `renderTemplate` with "user" as template.
func handleUser(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	user, err := db.GetUser(token)
	if err != nil {
		handleError(w, r, err)
		return
	}

	user_stats, err := db.GetUserStatistics(token)
	if err != nil {
		handleError(w, r, err)
		return
	}

	api_stats, err := db.GetAPIStatistics(token)
	if err != nil {
		handleError(w, r, err)
		return
	}

	api_tokens, err := db.GetAPITokens(token)
	if err != nil {
		handleError(w, r, err)
		return
	}

	workers, err := db.GetWorkers(token)
	if err != nil {
		handleError(w, r, err)
		return
	}

	data := make(map[string]interface{})
	data["User"] = user
	data["User_statistics"] = user_stats
	data["API_statistics"] = api_stats
	data["API_tokens"] = api_tokens
	data["Workers"] = workers
	data["Subdir"] = application_subdirectory
	data["Host"] = application_host
	data["Port"] = worker_port
	renderTemplate(w, "user", data)
}

// The handler creates a new API token named `name` for the user and redirects
// to the user page.
func handleUserNewAPIToken(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	name := r.FormValue("name")
	if name == "" {
		handleError(w, r, errors.New("Not all input fields were filled in!"))
		return
	}

	if err := db.AddAPIToken(token, name); err != nil {
		handleError(w, r, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("%suser", application_subdirectory),
		http.StatusFound)
}

// The handler invalidates the specified API token for the user and redirects to
// the user page.
func handleUserRevokeAPIToken(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	api_token := r.FormValue("token")
	if api_token == "" {
		handleError(w, r, errors.New("No API token specified!"))
		return
	}

	if err := db.DeleteAPIToken(token, api_token); err != nil {
		handleError(w, r, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("%suser", application_subdirectory),
		http.StatusFound)
}

// The handler invalidates the specified worker for the user and redirects to
// the user page.
func handleUserDegegisterWorker(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	worker_token := r.FormValue("token")
	if worker_token == "" {
		handleError(w, r, errors.New("No Worker token specified!"))
		return
	}

	worker.DeleteWorker(worker_token)
	if err := db.DeleteWorker(token, worker_token); err != nil {
		handleError(w, r, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("%suser", application_subdirectory),
		http.StatusFound)
}

// The handler verifies that the logged in user has access to the requested
// patch file and if he has the file content is sent back.
func handlePatchDownload(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	file_name, err := db.GetPatchFileName(token, vars["patch"])
	if err != nil {
		http.Error(w, fmt.Sprint(err), http.StatusNotFound)
	} else {
		file_path := fmt.Sprintf("%s/%s", worker.GetPatchPath(), file_name)
		in, err := ioutil.ReadFile(file_path)
		if err != nil {
			handleError(w, r, err)
		} else {
			w.Write(in)
		}
	}
}

// The handler applies the Git patch to the project if applicable. This involves
// the following steps:
// - Verify that the user is allowed to perform this action.
// - Request the current commit ID the master branch of the project references.
// - Create a new branch pointing the this commit ID.
// - Pull the new branch and apply the patch.
// - Upload the changes.
// - Create a pull request on GitHub.
func handlePullRequestNew(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	// verify user has access to requested task
	task, err := db.GetTask(vars["tid"], token)
	if err != nil {
		handleError(w, r, err)
		return
	}

	// request master branch information
	ref_response, err := authGitHubRequest("GET",
		fmt.Sprintf("repos/%s/git/refs/heads/master", task.Project.Name), token,
		make(map[string]interface{}), make(map[string]string), http.StatusOK)
	if err != nil {
		handleError(w, r, err)
		return
	}
	master := ref_response.(map[string]interface{})
	object := master["object"].(map[string]interface{})
	sha := object["sha"].(string)

	// create new branch to put the changes on
	branch_name := fmt.Sprintf("analysisbots_task_%d", task.Id)
	new_ref_payload := make(map[string]interface{})
	new_ref_payload["ref"] = fmt.Sprintf("refs/heads/%s", branch_name)
	new_ref_payload["sha"] = sha
	new_ref_response, err := authGitHubRequest("POST",
		fmt.Sprintf("repos/%s/git/refs", task.Project.Name), token,
		new_ref_payload, make(map[string]string), http.StatusCreated)
	if err != nil {
		handleError(w, r, err)
		return
	}
	new_ref := new_ref_response.(map[string]interface{})
	new_ref_object := new_ref["object"].(map[string]interface{})
	new_ref_sha := new_ref_object["sha"].(string)
	if sha != new_ref_sha {
		handleError(w, r, fmt.Errorf("New sha value does not match old one!"))
		return
	}

	// commit the changes
	if err := worker.CommitPatch(task, branch_name); err != nil {
		handleError(w, r, err)
		return
	}

	// create pull request
	pullreq_payload := make(map[string]interface{})
	pullreq_payload["title"] = fmt.Sprintf("[AUTO] Analysis Bots Action #%d",
		task.Id)
	pullreq_payload["head"] = branch_name
	pullreq_payload["base"] = "master"
	pullreq_payload["body"] = "Please pull this in!"
	_, err = authGitHubRequest("POST",
		fmt.Sprintf("repos/%s/pulls", task.Project.Name), token,
		pullreq_payload, make(map[string]string), http.StatusCreated)
	if err != nil {
		handleError(w, r, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("%stasks/%d", application_subdirectory,
		task.Id), http.StatusFound)
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
		data := make(map[string]interface{})
		data["Bots"] = bots
		data["Subdir"] = application_subdirectory
		renderTemplate(w, "bots", data)
	}
}

// The handler displays the form for adding new Bots.
func handleBotsNewForm(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	data := make(map[string]interface{})
	data["Subdir"] = application_subdirectory
	renderTemplate(w, "bots-new", data)
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
	if _, err := db.AddBot(path, description, tags); err != nil {
		handleError(w, r, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("%sbots/", application_subdirectory),
		http.StatusFound)
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
		data := make(map[string]interface{})
		data["Bot"] = bot
		data["Subdir"] = application_subdirectory
		renderTemplate(w, "bots-bid", data)
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
	response, err := authGitHubRequest("GET", "user/repos", token,
		make(map[string]interface{}), make(map[string]string), http.StatusOK)
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
			data := make(map[string]interface{})
			data["Bot"] = bot
			data["Projects"] = projects
			data["Subdir"] = application_subdirectory
			data["Events"] = db.Event_names
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
	response, err := authGitHubRequest("GET", "user/repos", token,
		make(map[string]interface{}), make(map[string]string), http.StatusOK)
	if err != nil {
		session.Options.MaxAge = -1
		session.Save(r, w)
		handleError(w, r, err)
	} else {
		projects, err := db.UpdateProjects(response, token)
		if err != nil {
			handleError(w, r, err)
		} else {
			data := make(map[string]interface{})
			data["Projects"] = projects
			data["Subdir"] = application_subdirectory
			renderTemplate(w, "projects", data)
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
		data := make(map[string]interface{})
		data["Project"] = project
		data["Subdir"] = application_subdirectory
		renderTemplate(w, "projects-pid", data)
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
		data := make(map[string]interface{})
		data["Project"] = project
		data["Bots"] = bots
		data["Subdir"] = application_subdirectory
		data["Events"] = db.Event_names
		renderTemplate(w, "projects-pid-newtask", data)
	}
}

// The handler requests information about all tasks ran by the user. If an error
// occurs the `handleError` function is called else `renderTemplate` with the
// template "tasks" and the retrieved data.
func handleTasks(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	err := updateHooks(token)
	if err != nil {
		handleError(w, r, err)
		return
	}

	scheduled, err := db.GetScheduledTasks(token)
	if err != nil {
		handleError(w, r, err)
		return
	}
	event, err := db.GetEventTasks(token)
	if err != nil {
		handleError(w, r, err)
		return
	}
	instant, err := db.GetInstantTasks(token)
	if err != nil {
		handleError(w, r, err)
		return
	}
	one_time, err := db.GetOneTimeTasks(token)
	if err != nil {
		handleError(w, r, err)
		return
	}

	task_groups := make(map[string]interface{})
	task_groups["Scheduled"] = scheduled
	task_groups["Event"] = event
	task_groups["Instant"] = instant
	task_groups["OneTime"] = one_time

	data := make(map[string]interface{})
	data["TaskGroups"] = task_groups
	data["Subdir"] = application_subdirectory
	renderTemplate(w, "tasks", data)
}

// The handler requests detailed information about the task identified by its
// id. If an error occurs the `handleError` function is called else
// `renderTemplate` with the template "projects-pid" and the retrieved data.
func handleTasksTid(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	task, err := db.GetTask(vars["tid"], token)
	if err != nil {
		handleError(w, r, err)
	} else {
		task.Output = template.HTMLEscapeString(task.Output)
		task.Output = strings.Replace(task.Output, "\n", "<br>", -1)
		output := template.HTML(task.Output)
		data := make(map[string]interface{})
		data["Task"] = task
		data["Output"] = output
		data["Subdir"] = application_subdirectory
		renderTemplate(w, "tasks-tid", data)
	}
}

// The handler creates a new event triggered task by using the query arguments
// 'name' and 'event'. After creating a new event task instance a new web hook
// on GitHub is created. (How this is done you can lookup here:
// https://developer.github.com/v3/repos/hooks/ ) The id of the hook is
// retrieved from the response and added to the event task. In the end the users
// is redirected to the overview page of the tasks. In case the creation of a
// hook failed the status of the task is updated to complete and the
// errorhandler is called.
func handleTasksNewEventDriven(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {

	err := updateHooks(token)
	if err != nil {
		handleError(w, r, err)
		return
	}

	event, err := strconv.ParseInt(r.FormValue("type"), 10, 64)
	if err != nil {
		handleError(w, r, err)
		return
	}

	task, err := db.CreateNewEventTask(token, vars["pid"], vars["bid"],
		r.FormValue("name"), event)
	if err != nil {
		handleError(w, r, err)
		return
	}

	reqUrl := fmt.Sprintf("repos/%s/hooks", task.Project.Name)
	payload := make(map[string]interface{})
	payload["name"] = "web"
	payload["active"] = true
	payload["events"] = [...]string{task.EventString()}
	config := make(map[string]interface{})
	config["url"] = fmt.Sprintf("http://%s%s%s/%d", application_host,
		application_subdirectory, webhook_subpath, task.Id)
	config["content_type"] = "json"
	payload["config"] = config

	header := make(map[string]string)
	header["secret"] = task.Token

	hookResp, err := authGitHubRequest("POST", reqUrl, token, payload, header,
		http.StatusCreated)
	if err != nil {
		handleError(w, r, err)
		// TODO proper error handling
		db.UpdateEventTaskStatus(task.Id, db.Complete)
		return
	}

	json_hid := hookResp.(map[string]interface{})["id"].(json.Number)
	hid, _ := json_hid.Int64()

	err = db.SetHookId(task.Id, hid)
	if err != nil {
		handleError(w, r, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("%stasks/", application_subdirectory),
		http.StatusFound)
}

// The handler creates a new scheduled task by using the query arguments
// 'name' and 'cron'. The 'cron' argument is a a unix cron expression
// (https://en.wikipedia.org/wiki/Cron) to identify the schedule times. First
// the next time satisfying the cron expression is calculated (corresponds to
// the next execution time). Then a a new instance of scheduled task is created
// and a go routine for scheduling the the task is started. In the end the
// users is redirected to the overview page of the tasks. In case of an error
// the errorhandler is called.
func handleTasksNewScheduled(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {

	cron_str := strings.Replace(r.FormValue("cron"), "_", " ", -1)
	nextTime := cronexpr.MustParse(cron_str).Next(time.Now())
	if nextTime.IsZero() {
		handleError(w, r,
			fmt.Errorf("The cron expression <%s> could not have been parsed.",
				cron_str))
		return
	}

	scheduledTask, err := db.CreateScheduledTask(token, vars["pid"],
		vars["bid"], r.FormValue("name"), nextTime, cron_str)
	if err != nil {
		handleError(w, r, err)
		return
	}

	worker.RunScheduledTask(scheduledTask.Id)
	http.Redirect(w, r, fmt.Sprintf("%stasks/", application_subdirectory),
		http.StatusFound)
}

// The handler creates a new one time task by using the query arguments 'name'
// and 'time'. The 'time' argument is passed in unix time and is the time stamp
// the task should be executed. If it is the past the task is executed
// immediately. After converting the time stamp from unix time to a go time
// type a new instance of a one time task is created. To schedule the task a go
// routine in the worker is started. In the end the users is redirected to the
// overview page of the tasks. In case of an error the errorhandler is called.
func handleTasksNewOneTime(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {

	seconds, err := strconv.ParseInt(r.FormValue("time"), 10, 64)
	if err != nil {
		handleError(w, r, err)
		return
	}

	scheduleTime := time.Unix(seconds/1000, 0)
	oneTimeTask, err := db.CreateOneTimeTask(token, vars["pid"],
		vars["bid"], r.FormValue("name"), scheduleTime)
	if err != nil {
		handleError(w, r, err)
		return
	}

	worker.RunOneTimeTask(oneTimeTask.Id)
	http.Redirect(w, r, fmt.Sprintf("%stasks/", application_subdirectory),
		http.StatusFound)
}

// The handler creates a new instant task. After creating a new instance of
// instant task the execution is immediately initiate by calling CreateNewTask.
// In the end the users is redirected to the overview page of the tasks. In
// case of an error the errorhandler is called.
func handleTasksNewInstant(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	instantTask, err := db.CreateNewInstantTask(token, vars["pid"], vars["bid"])

	if err != nil {
		handleError(w, r, err)
		return
	}
	tid, err := worker.CreateNewTask(instantTask.Id)
	if err != nil {
		handleError(w, r, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("%stasks/%d", application_subdirectory,
		tid), http.StatusFound)
}

// The handler handles the requests from GitHub to the call back url specified
// during the creation of a hook. The url '.../webhook/id' ends with the id
// of the associated event task to identify the request. After checking the
// validity of the request CreateNewTask is called to initiate the execution of
// the event task.
func handleWebhook(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	taskId := vars["tid"]

	key, err := db.GetSecret(taskId)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unknown event task id!")
		return
	}
	mac := hmac.New(sha1.New, key)
	body, _ := ioutil.ReadAll(r.Body)
	mac.Write(body)
	expectedMAC := mac.Sum(nil)
	if !hmac.Equal([]byte(r.Header.Get("X-Hub-Signature")), expectedMAC) {
		fmt.Fprintf(os.Stderr, "Unknown secret!")
		return
	}

	tid, iErr := strconv.ParseInt(taskId, 10, 64)
	if iErr != nil {
		// TODO error handling
		return
	}

	worker.CreateNewTask(tid)
}

// The handler attempts to cancel the specified task. If this fails the
// `handleError` function is called else the user is redirected to the Bot
// status overview page.
func handleTasksTidCancel(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	tid, _ := strconv.ParseInt(vars["tid"], 10, 64)
	worker.Cancel(tid)

	http.Redirect(w, r, fmt.Sprintf("%stasks/", application_subdirectory),
		http.StatusFound)
}

// The handler attempts to cancel the whole group task. Depending on the type
// of the task the corresponding cancel function is called. In case of a event
// task also the corresponding hook is deleted. (How to delete a webhook you
// can lookup here: https://developer.github.com/v3/repos/hooks/) In the end
// the users is redirected to the overview page of the tasks. In case of an
// error the errorhandler is called.
func handleTasksTidCancelGroup(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	task, err := db.CancelTaskGroup(vars["tid"])
	if err != nil {
		handleError(w, r, errors.New(
			"The task id was not known and thus could not have been canceled."))
		return
	}

	switch task.(type) {
	case db.ScheduledTask:
		err = worker.CancelScheduledTask(task.(db.ScheduledTask).Id)
	case db.EventTask:
		eventTask := task.(db.EventTask)
		err = worker.CancelEventTask(eventTask.Id)
		url := fmt.Sprintf("repos/%s/hooks/%d", eventTask.Project.Name,
			eventTask.HookId)
		if _, err := authGitHubRequest("DELETE", url, token,
			make(map[string]interface{}), make(map[string]string),
			http.StatusNoContent); err != nil {
			handleError(w, r, err)
			return
		}
	case db.OneTimeTask:
		err = worker.CancelOneTimeTask(task.(db.OneTimeTask).Id)
	case db.InstantTask:
		err = worker.CancelInstantTask(task.(db.InstantTask).Id)
	}
	if err != nil {
		handleError(w, r, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("%stasks/", application_subdirectory),
		http.StatusFound)
}

// Because a user is also able to delete the webhooks manually the status of
// the database needs to be updated. For every active event task it is checked
// if the corresponding hook still exists. If it is not the case the task is
// set to completed.
func updateHooks(token string) error {
	tasks, err := db.GetActiveEventTasks(token)
	if err != nil {
		return err
	}

	for _, task := range tasks {
		url := fmt.Sprintf("repos/%s/hooks/%d", task.Project.Name, task.HookId)
		_, rErr := authGitHubRequest("GET", url, token,
			make(map[string]interface{}), make(map[string]string),
			http.StatusOK)
		if rErr != nil {
			db.UpdateScheduledTaskStatus(task.Id, db.Complete)
		}
	}

	return nil

}

//
// API
//

// Validates the user's input and adds a new Bot to the database. After
// successful insertion the newly created Bot is retrieved again, marshaled as
// JSON object and sent back.
func handleAPIPostBot(w http.ResponseWriter, r *http.Request, token string) {
	path := r.FormValue("path")
	description := r.FormValue("description")
	tags := r.FormValue("tags")
	if path == "" || description == "" || tags == "" {
		http.Error(w, "Invalid input for Bot creation!", http.StatusNotFound)
		return
	}
	resp, err := http.Get(
		fmt.Sprintf("https://index.docker.io/v1/repositories/%s/tags", path))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Docker Hub entry does not exist!", http.StatusNotFound)
		return
	}
	if bid, err := db.AddBot(path, description, tags); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	} else {
		bot, err := db.GetBot(bid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			js, err := json.Marshal(bot)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			} else {
				w.Header().Set("Content-Type", "application/json")
				w.Write(js)
			}
		}
	}
}

// Retrieves all Bots from the database and marshals them as JSON object.
func handleAPIGetBots(w http.ResponseWriter, r *http.Request, token string) {
	bots, err := db.GetBots()
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
	} else {
		js, err := json.Marshal(bots)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(js)
		}
	}
}

// Retrieves all Projects of the user from the database and marshals them as
// JSON object. Before doing so, the project information is synchronized with
// GitHub.
func handleAPIGetProjects(w http.ResponseWriter, r *http.Request,
	token string) {
	user_token, err := db.GetUserTokenFromAPIToken(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
	}
	response, err := authGitHubRequest("GET", "user/repos", user_token,
		make(map[string]interface{}), make(map[string]string), http.StatusOK)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
	} else {
		projects, err := db.UpdateProjects(response, user_token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			js, err := json.Marshal(projects)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			} else {
				w.Header().Set("Content-Type", "application/json")
				w.Write(js)
			}
		}
	}
}

// Retrieves a Task (specified by the "tid" GET parameter) of the user from the
// database and marshals it as JSON object.
func handleAPIGetTask(w http.ResponseWriter, r *http.Request, token string) {
	user_token, err := db.GetUserTokenFromAPIToken(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
	}
	task, err := db.GetTask(r.FormValue("tid"), user_token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
	} else {
		js, err := json.Marshal(task)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(js)
		}
	}
}

// Validates the user's input and adds a new Task to the database. After
// successful insertion the newly created Task is retrieved again, marshaled as
// JSON object and sent back.
func handleAPIPostTask(w http.ResponseWriter, r *http.Request, token string) {
	user_token, err := db.GetUserTokenFromAPIToken(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
	}
	pid := r.FormValue("pid")
	bid := r.FormValue("bid")
	instantTask, err := db.CreateNewInstantTask(user_token, pid, bid)

	tid, err := worker.CreateNewTask(instantTask.Id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
	} else {
		task, err := db.GetTask(strconv.FormatInt(tid, 10), user_token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			js, err := json.Marshal(task)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			} else {
				w.Header().Set("Content-Type", "application/json")
				w.Write(js)
			}
		}
	}
}

// Cancels the specified task, if it is pending or running.
func handleAPIDeleteTask(w http.ResponseWriter, r *http.Request, token string) {
	tid, _ := strconv.ParseInt(r.FormValue("tid"), 10, 64)
	worker.Cancel(tid)
}

// Retrieves all Tasks of the user from the database and marshals them as JSON
// object.
func handleAPIGetTasks(w http.ResponseWriter, r *http.Request, token string) {
	user_token, err := db.GetUserTokenFromAPIToken(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
	}

	scheduled, err := db.GetScheduledTasks(user_token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	event, err := db.GetEventTasks(user_token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	instant, err := db.GetInstantTasks(user_token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	one_time, err := db.GetOneTimeTasks(user_token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	task_groups := make(map[string]interface{})
	task_groups["Scheduled"] = scheduled
	task_groups["Event"] = event
	task_groups["Instant"] = instant
	task_groups["OneTime"] = one_time

	js, err := json.Marshal(task_groups)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(js)
	}
}
