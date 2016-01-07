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
)

//
// global state
//

// Application identification
const app_id_var = "CLIENT_ID"
const app_secret_var = "CLIENT_SECRET"

var client_id = os.Getenv(app_id_var)
var client_secret = os.Getenv(app_secret_var)

// Session support
const session_auth_var = "SESSION_AUTH"
const session_enc_var = "SESSION_ENC"

var session_store = sessions.NewCookieStore(
	[]byte(os.Getenv(session_auth_var)), []byte(os.Getenv(session_enc_var)))

// Database authentification
const db_user_var = "DB_USER"
const db_pass_var = "DB_PASS"

var db_user = os.Getenv(db_user_var)
var db_pass = os.Getenv(db_pass_var)

// Filesystem cache
const cache_path_var = "CACHE_PATH"

var cache_path = os.Getenv(cache_path_var)

// Id regex
const id_regex = "0|[1-9][0-9]*"

// Template caching
const template_root = "tmpl"

var templates = template.Must(template.ParseGlob(
	fmt.Sprintf("%s/[^.]*", template_root),
))

// App constants
const state_size = 32

// Context settings
var error_counter = 0
var error_map = make(map[string]interface{})

//
// entry point
//

// Parameters:
// --
//
// Returns:
// --
//
// The Start() function sets up the environment for the controller and calls the
// http ListenAndServe() function which actually starts the webservice.
//
// In the begining the environment variables listed below and containing process
// relevant information are beeing retreived from the operating system:
//
// - CLIENT_ID: The identifier retrieved from GitHub when registering this
// application as a GitHub application.
//
// - CLIENT_SECRET: The client secret retrieved from GitHub when registering
// this application as a GitHub application.
//
// - SESSION_AUTH: A random string authenticating this session uniquely among
// others.
//
// - SESSION_ENC: A random string used to encrypt the session.
//
// - DB_USER: The database user (here a postgresql database is used, hence this
// is a postgres-user) owning the database "analysisbots".
//
// - DB_PASS: The password of the databse user (needed to acces the database).
//
// - CACHE_PATH: The filesystem path where the different components can store
// their data.
//
// In case some of the variables are missing a corresponding message is beeing
// prompted to the standard output and the function terminates without any
// further action.
// If all environment variables were provided the OpenDB() function of the db
// package is being calles in order to establish a connection to the database.
// In case of an error again the function terminates with a corresponding
// message and without any further actions.
// Next a channel "sigs" is beeing created in order to listen for signals from
// the operating system in particular for termination signals.
// Then a new goroutine listening on that channel is beeing executed
// concurrently, which whenever it receives something on the "sigs" channel
// closes the database connection and exits the system with the status code 0.
//
// Finally the ListenAndServe() function of the http package is beeing called in
// order to listen on port 8080 for incomming http requests. The router used to
// demultiplex paths and calling the respective handlers is created by the
// initRoutes() call.
func Start() {
	// check environment
	_, id := os.LookupEnv(app_id_var)
	_, secret := os.LookupEnv(app_secret_var)
	_, auth := os.LookupEnv(session_auth_var)
	_, enc := os.LookupEnv(session_enc_var)
	_, user := os.LookupEnv(db_user_var)
	_, pass := os.LookupEnv(db_pass_var)
	_, cache := os.LookupEnv(cache_path_var)
	if !id || !secret || !auth || !enc || !user || !pass || !cache {
		fmt.Printf("Application settings missing!\n"+
			"Please set the %s, %s, %s, %s, %s, %s and %s environment variables.\n",
			app_id_var, app_secret_var, session_auth_var, session_enc_var,
			db_user_var, db_pass_var, cache_path_var)
		return
	}

	// initialize database connection
	fmt.Println("Controller start ...")
	if err := db.OpenDB(db_user, db_pass); err != nil {
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
			worker.Init(dir)
		}
	}

	// make sure database connection gets closed
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		db.CloseDB()
		fmt.Println("... controller termiated")
		os.Exit(0)
	}()

	// listen on port 8080 to handle http requests
	if err := http.ListenAndServe(":8080", initRoutes()); err != nil {
		fmt.Println("Controller listen failed:")
		fmt.Println(err)
	}
}

//
// helper functions
//

// TODO document this
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
	rootRouter.HandleFunc("/user", makeHandler(makeTokenHandler(handleUser)))

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
		makeHandler(makeTokenHandler(handleTasksNew)))

	// projects
	projectsRouter.HandleFunc("/",
		makeHandler(makeTokenHandler(handleProjects)))
	projectsRouter.HandleFunc(fmt.Sprintf("/{pid:%s}", id_regex),
		makeHandler(makeTokenHandler(handleProjectsPid)))
	projectsRouter.HandleFunc(fmt.Sprintf("/{pid:%s}/newtask", id_regex),
		makeHandler(makeTokenHandler(handleProjectsPidNewtask)))
	projectsRouter.HandleFunc(
		fmt.Sprintf("/{pid:%s}/{bid:%s}", id_regex, id_regex),
		makeHandler(makeTokenHandler(handleTasksNew)))

	// tasks
	tasksRouter.HandleFunc("/", makeHandler(makeTokenHandler(handleTasks)))
	tasksRouter.HandleFunc(fmt.Sprintf("/{tid:%s}", id_regex),
		makeHandler(makeTokenHandler(handleTasksTid)))
	tasksRouter.HandleFunc(fmt.Sprintf("/{tid:%s}/cancel", id_regex),
		makeHandler(makeTokenHandler(handleTasksTidCancel)))

	return
}

// Parameters:
// - fn: the function to wrap
//
// Returns:
// A that performs the retrieval of the session and path-variables and then
// executing the given handler.
// The makeHandler() function takes a function with signature
// (http.ResponseWriter, *http.Request, map[string]string, *sessions.Session)
// and returns a http.HandlerFunc with the signature w http.ResponseWriter, r
// *http.Request), which is mandatory for being registered as a request handler
// of the gorilla mux.
// Besides a http.ResponseWriter and a *http.Request the actual some of the
// handlers called for http requests are in need of a map[string]string
// containing information of the path that triggers that handler and a pointer
// to a session *sessions.Session. Since the retrieval of the session and the
// variables containing information about the composition of the url that
// triggers the execution of a handler is the same for every handler,
// makeHandler() creates and returns a "wrapper function" that retrieves the
// session as well as the variables and then executes the given handler with the
// retrieved information.
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

// Parameters:
// - fn: The given handler to be wrapped.
//
// Returns:
// A function retrieving the token and then executing the handler.
//
// The makeTokenHandler() function takes a function of signature
// (http.ResponseWriter, *http.Request, map[string]string, *sessions.Session,
// string) and returns a function of signature (http.ResponseWriter,
// *http.Request, map[string]string, *sessions.Session), which in turn can be
// passed to makeHandler() in order to make the given handler usable as a
// http.HandlerFunc.
// Most of the handlers beeing triggered by a certain path are in need of the
// authentification token (stored in a cookie/session) in order to operate in
// the desired way (retrieving information from the database, from GitHub,
// etc.).
// Since the retrieval of the token from the session for all these handlers is
// the same, makeTokenHandler() "wraps" the given handler in a function that
// retrieves the token corresponding to the given session (or takes the
// corresponding action in case there is no such token available) and then
// executes the given handler.
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

// Parameters:
// w http.ResponseWriter,
// tmpl string: The name of the template,
// data interface{}: A struct containing the data to be inserted in the
// template.
//
// The function executes the template tmpl with the data. If the execution fails
// is responses to request with an error message explaining the reason of fail.
func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	err := templates.ExecuteTemplate(w,
		fmt.Sprintf("%s.html", tmpl), data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Parameters:
// w http.ResponseWriter,
// req_url string: URL indicating the desired information to retrieve from
// GitHub,
// token string: personal access token
//
// Return values
// interface{}: If the request was successful an struct representing the
// received and decoded data otherwise nil.
// error: If the request was successful nil otherwise an error indicating the
// reason of fail.
//
// The function sends an https GET request with the req_url to GitHub. After
// receiving an successful repose the body of the https request containing the
// requested data in json format is decoded and returned.
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

// Parameters:
// w http.ResponseWriter,
// r *http.Request,
// err error
//
// The handler prints the error to the standard output and redirects the user to
// the root page.
func handleError(w http.ResponseWriter, r *http.Request, err error) {
	error_counter++
	error_map[strconv.Itoa(error_counter)] = err
	http.Redirect(w, r, fmt.Sprintf("/?err=%d", error_counter),
		http.StatusFound)
}

// Parameters:
// w http.ResponseWriter,
// r *http.Request,
// session *sessions.Session
//
// Return values:
// string If the session is valid the GitHub authentication token otherwise an
// empty string.
// error If the session is valid nil otherwise an error indicating that no valid
// session exists.
//
// The function checks, if the session is valid. If this is the case the GitHub
// authentication token is returned, otherwise the function handleError is
// called and an error is returned.
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
// route handler
//

// Parameters:
// w http.ResponseWriter,
// r *http.Request,
// vars map[string]string,
// session *sessions.Session
//
// The handler checks if the session "token" is valid. Is this the case the
// function renderTemplate with the template "index" is called otherwise with
// the template "login".
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

// Parameters:
// w http.ResponseWriter,
// r *http.Request,
// vars map[string]string: map of route variables containing assignments of
// regexp variables occurring in the matching path,
// session *sessions.Session: ,
// token string: personal access token
//
// The handler requests the personal access token and the user profile from
// GitHub to create a user in the database and redirects the user to the root
// page.
//
// After the user granted the application access to its data GitHub redirects to
// the "Authorization callback URL" from the application setting page on GitHub.
// This request will be handled by this handler.
//
// To ensure that the connection to GitHub is not hijacked the validity of the
// session is checked by comparing the state variable previously sent to GitHub
// and the one received by a URL like the one below.
//
// 	e.g.: http://analysis-bots-platform.com/?code=<code>&state=<s>
//
// The callback URL contains a variable code which will be used in the next
// step.
// To get the access token a http POST request is sent to the following URL:
//
// 	https://github.com/login/oauth/access_token
//
// Containing the variables below:
// client_id: The client_id from the application setting page on GitHub.
// client_secret: The client_secret from the application setting page on GitHub.
// code: The code received in the response.
// state: The random string used in the login handler.
//
// Since the response is requested in the json format by specifying the the
// accept value in the http header accordingly the data of interest is being
// extracted by unmarshalling the body using a json parser. After retrieving the
// token it will replace the state as the session identifier. To get the user
// information the function authGitHubRequest with the req_url "user" is called.
// If this fails the session is closed to enforce a redirecting to the login
// page. Otherwise the database is updated and the user is being redirected to
// the root page.
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

// TODO document this
func handleJavaScripts(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session) {
	err := templates.ExecuteTemplate(w, vars["file"], nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

//Parameters:
//w http.ResponseWriter,
//r *http.Request,
//vars map[string]string,
//session *sessions.Session
//
//The Handler redirects the user to the GitHub login page to get a personal
//access token to access the users GitHub data.
//Temporarily the random string state is used as session identifier because the
//access token does not exists yet. This is necessary to ensure that the GitHub
//connection can not be hijacked.
//
//The construction of the URL is as follows:
//
//https://github.com/login/oauth/authorize?client_id=<id>&scope="user, repo"&state=<st>
//
//client_id: The client_id from the application setting page on GitHub.
//
//scope: The scope determines which parts of the users data our application is
//allowed to access. In this case "user" grants read and write access to the
//users profile info and "repo" read and write access to code, commit statuses,
//collaborators, and deployment statuses for public and private repositories and
//organizations.
//
//state: State is a random string to protect against cross-site request forgery
//attacks.
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

// TODO document this
func handleLogout(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session) {
	session.Options.MaxAge = -1
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)
}

// TODO document this
func handleUser(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	renderTemplate(w, "user", nil)
}

//Parameters:
//w http.ResponseWriter,
//r *http.Request,
//vars map[string]string: map of route variables containing assignments of
//regexp variables occurring in the matching path,
//session *sessions.Session: ,
//token string: personal access token
//
//The handler requests information about all bots from the database. If an error
//occurs the handleError function is called otherwise renderTemplate with the
//template "bots" and the  retrieved data.
func handleBots(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	bots, err := db.GetBots()
	if err != nil {
		handleError(w, r, err)
	} else {
		renderTemplate(w, "bots", bots)
	}
}

// TODO document this
func handleBotsNewForm(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	renderTemplate(w, "bots-new", nil)
}

// TODO document this
func handleBotsNewPost(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	path := r.FormValue("path")
	description := r.FormValue("description")
	tags := r.FormValue("tags")
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

//Parameters:
//w http.ResponseWriter,
//r *http.Request,
//vars map[string]string: map of route variables containing assignments of
//regexp variables occurring in the matching path,
//session *sessions.Session: ,
//token string: personal access token
//
//The handler requests detailed information about the bot identified by bid in
//the rout variables from the database. If an error occurs the handleError
//function is called otherwise renderTemplate with the template "bots-bid" and
//the  retrieved data.
func handleBotsBid(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	bot, err := db.GetBot(vars["bid"])
	if err != nil {
		handleError(w, r, err)
	} else {
		renderTemplate(w, "bots-bid", bot)
	}
}

// TODO document this
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

//Parameters:
//w http.ResponseWriter,
//r *http.Request,
//vars map[string]string: map of route variables containing assignments of
//regexp variables occurring in the matching path,
//session *sessions.Session: ,
//token string: personal access token
//
//The handler calls the function authGitHubRequest with the URL "user/repos" to
//get the up to date information about the user's projects. If This fails the
//session is closed and the user redirected to the root page. Otherwise the
//database is updated and the renderTemplate function with the "projects"
//template and the data from the database is called.
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

//Parameters:
//w http.ResponseWriter,
//r *http.Request,
//vars map[string]string: map of route variables containing assignments of
//regexp variables occurring in the matching path,
//session *sessions.Session: ,
//token string: personal access token
//
//The handler requests detailed information about the project identified by pid
//in the rout variables from the database. If an error occurs the handleError
//function is called otherwise renderTemplate with the template "projects-pid"
//and the  retrieved data.
func handleProjectsPid(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	project, err := db.GetProject(vars["pid"], token)
	if err != nil {
		handleError(w, r, err)
	} else {
		renderTemplate(w, "projects-pid", project)
	}
}

// TODO document this
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

//Parameters:
//w http.ResponseWriter,
//r *http.Request,
//vars map[string]string: map of route variables containing assignments of
//regexp variables occurring in the matching path,
//session *sessions.Session: ,
//token string: personal access token
//
//The handler requests information about all tasks of the user identified by his
//token from the database. If an error occurs the handleError function is called
//otherwise renderTemplate with the template "tasks" and the  retrieved data.
func handleTasks(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	tasks, err := db.GetTasks(token)
	if err != nil {
		handleError(w, r, err)
	} else {
		renderTemplate(w, "tasks", tasks)
	}
}

//Parameters:
//w http.ResponseWriter,
//r *http.Request,
//vars map[string]string: map of route variables containing assignments of
//regexp variables occurring in the matching path,
//session *sessions.Session: ,
//token string: personal access token
//
//The handler requests detailed information about the task identified, by tid in
//the rout variables, of the user identified by the token from the database. If
//an error occurs the handleError function is called otherwise renderTemplate
//with the template "projects-pid" and the  retrieved data.
func handleTasksTid(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
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
func handleTasksNew(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	tid, err := worker.CreateNewTask(token, vars["pid"], vars["bid"])
	if err != nil {
		handleError(w, r, err)
	} else {
		http.Redirect(w, r, fmt.Sprintf("/tasks/%d", tid), http.StatusFound)
	}
}

// TODO document this
func handleTasksTidCancel(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	err := worker.Cancle(vars["tid"])
	if err != nil {
		handleError(w, r, err)
	} else {
		http.Redirect(w, r, "/tasks/", http.StatusFound)
	}
}
