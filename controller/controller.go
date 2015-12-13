package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/AnalysisBotsPlatform/platform/db"
	"github.com/AnalysisBotsPlatform/platform/utils"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
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

// Id regex
const id_regex = "0|[1-9][0-9]*"

// Template caching
const template_root = "tmpl"

var templates = template.Must(template.ParseGlob(
	fmt.Sprintf("%s/*.html", template_root),
))

// App constants
const state_size = 32

//
// entry point
//

//Parameters:
//--
//Returns:
//--
//
//The Start() function sets up the environment for the controller and calls the http ListenAndServe() function which actually starts the webservice.
//
//In the begining the environment variables listed below and containing process relevant information are beeing retreived from the operating system:
//- CLIENT_ID: The identifier retrieved from GitHub when registering this application as a GitHub application.
//- CLIENT_SECRET: The client secret retrieved from GitHub when registering this application as a GitHub application.
//- SESSION_AUTH: A random string authenticating this session uniquely among others.
//- SESSION_ENC: A random string used to encrypt the session.
//- DB_USER: The database user (here a postgresql database is used, hence this is a postgres-user) owning the database "analysisbots".
//- DB_PASS: The password of the databse user (needed to acces the database).
//
//In case some of the variables are missing a corresponding message is beeing prompted to the standard output and the function terminates without any further action.
//If all environment variables were provided the OpenDB() function of the db package is being calles in order to establish a connection to the database. In case of an error again the function terminates with a corresponding message and without any further actions.
//Next a channel "sigs" is beeing created in order to listen for signals from the operating system in particular for termination signals.
//Then a new goroutine listening on that channel is beeing executed concurrently, which whenever it receives something on the "sigs" channel closes the database connection and exits the system with the status code 0.
//
//Finally the ListenAndServe() function of the http package is beeing called in order to listen on port 8080 for incomming http requests. The router used to demultiplex paths and calling the respective handlers is created by the initRoutes() call.
func Start() {
	// check environment
	_, id := os.LookupEnv(app_id_var)
	_, secret := os.LookupEnv(app_secret_var)
	_, auth := os.LookupEnv(session_auth_var)
	_, enc := os.LookupEnv(session_enc_var)
	_, user := os.LookupEnv(db_user_var)
	_, pass := os.LookupEnv(db_pass_var)
	if !id || !secret || !auth || !enc || !user || !pass {
		fmt.Printf("Application settings missing!\n"+
			"Please set the %s, %s, %s, %s, %s and %s environment variables.\n",
			app_id_var, app_secret_var, session_auth_var, session_enc_var,
			db_user_var, db_pass_var)
		return
	}

	// initialize database connection
	fmt.Println("Controller start ...")
	if err := db.OpenDB(db_user, db_pass); err != nil {
		fmt.Println("Cannot connect to database.")
		fmt.Println(err)
		return
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
	rootRouter.HandleFunc("/login", makeHandler(handleLogin))
	rootRouter.HandleFunc("/logout", makeHandler(handleLogout))
	rootRouter.HandleFunc("/user", makeHandler(makeTokenHandler(handleUser)))

	// bots
	botsRouter.HandleFunc("/", makeHandler(makeTokenHandler(handleBots)))
	botsRouter.HandleFunc(fmt.Sprintf("/{bid:%s}", id_regex),
		makeHandler(makeTokenHandler(handleBotsBid)))

	// projects
	projectsRouter.HandleFunc("/",
		makeHandler(makeTokenHandler(handleProjects)))
	projectsRouter.HandleFunc(fmt.Sprintf("/{pid:%s}", id_regex),
		makeHandler(makeTokenHandler(handleProjectsPid)))
	projectsRouter.HandleFunc(
		fmt.Sprintf("/{pid:%s}/{bid:%s}", id_regex, id_regex),
		makeHandler(makeTokenHandler(handleProjectsPidBid)))

	// tasks
	tasksRouter.HandleFunc("/", makeHandler(makeTokenHandler(handleTasks)))
	tasksRouter.HandleFunc(fmt.Sprintf("/{tid:%s}", id_regex),
		makeHandler(makeTokenHandler(handleTasksTid)))
	tasksRouter.HandleFunc(fmt.Sprintf("/{tid:%s}/result", id_regex),
		makeHandler(makeTokenHandler(handleTasksTidResult)))

	return
}


//Parameters:
//-fn: the function to wrap
//
//Returns:
//A that performs the retrieval of the session and path-variables and then executing the given handler.
//The makeHandler() function takes a function with signature (http.ResponseWriter, *http.Request, map[string]string, *sessions.Session) and returns a http.HandlerFunc with the signature w http.ResponseWriter, r *http.Request), which is mandatory for being registered as a request handler of the gorilla mux. 
//Besides a http.ResponseWriter and a *http.Request the actual some of the handlers called for http requests are in need of a map[string]string containing information of the path that triggers that handler and a pointer to a session *sessions.Session. Since the retrieval of the session and the variables containing information about the composition of the url that triggers the execution of a handler is the same for every handler, makeHandler() creates and returns a "wrapper function" that retrieves the session as well as the variables and then executes the given handler with the retrieved information.
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


//Parameters:
//-fn: The given handler to be wrapped.
//
//Returns:
//A function retrieving the token and then executing the handler.
//
//The makeTokenHandler() function takes a function of signature (http.ResponseWriter, *http.Request, map[string]string, *sessions.Session, string) and returns a function of signature (http.ResponseWriter, *http.Request, map[string]string, *sessions.Session), which in turn can be passed to makeHandler() in order to make the given handler usable as a http.HandlerFunc.
//Most of the handlers beeing triggered by a certain path are in need of the authentification token (stored in a cookie/session) in order to operate in the desired way (retrieving information from the database, from GitHub, etc.).
//Since the retrieval of the token from the session for all these handlers is the same, makeTokenHandler() "wraps" the given handler in a function that retrieves the token corresponding to the given session (or takes the corresponding action in case there is no such token available) and then executes the given handler.
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

// TODO document this
func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	err := templates.ExecuteTemplate(w,
		fmt.Sprintf("%s.html", tmpl), data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// TODO document this
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
func handleError(w http.ResponseWriter, r *http.Request, err error) {
	fmt.Println(err)
	http.Redirect(w, r, "/", http.StatusFound)
}

// TODO document this
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

// TODO document this
func handleRoot(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session) {
	if _, ok := session.Values["token"]; ok {
		renderTemplate(w, "index", nil)
	} else {
		renderTemplate(w, "login", nil)
	}
}

// TODO document this
func handleAuth(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session) {
	if state, ok := session.Values["state"].(string); ok {
		if state != r.FormValue("state") {
			handleError(w, r, errors.New("GitHub connection was hijacked!"))
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
			}
		}
		session.Save(r, w)
		http.Redirect(w, r, "/", http.StatusFound)
	} else {
		handleError(w, r, errors.New("No state available!"))
	}
}

// TODO document this
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
// TODO template missing
func handleUser(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	renderTemplate(w, "user", nil)
}

// TODO document this
func handleBots(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	if bots, err := db.GetBots(); err != nil {
		handleError(w, r, err)
	} else {
		renderTemplate(w, "bots", bots)
	}
}

// TODO document this
func handleBotsBid(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	if bot, err := db.GetBot(vars["bid"]); err != nil {
		handleError(w, r, err)
	} else {
		renderTemplate(w, "bots-bid", bot)
	}
}

// TODO document this
func handleProjects(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	response, err := authGitHubRequest(w, "user/repos", token)
	if err != nil {
		session.Options.MaxAge = -1
		session.Save(r, w)
		http.Redirect(w, r, "/", http.StatusFound)
	} else {
		if projects, err := db.UpdateProjects(response, token); err != nil {
			handleError(w, r, err)
		} else {
			renderTemplate(w, "projects", projects)
		}
	}
}

// TODO document this
// TODO template incomplete, i.e. selection for bots missing
// TODO add all available bots to payload
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
// TODO template missing
func handleProjectsPidBid(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	renderTemplate(w, "projects-pid-bid", nil)
}

// TODO document this
func handleTasks(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	if tasks, err := db.GetTasks(token); err != nil {
		handleError(w, r, err)
	} else {
		renderTemplate(w, "tasks", tasks)
	}
}

// TODO document this
// TODO template missing
func handleTasksTid(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	if task, err := db.GetTask(vars["tid"], token); err != nil {
		handleError(w, r, err)
	} else {
		renderTemplate(w, "tasks-tid", task)
	}
}

// TODO document this
// TODO template incomplete
func handleTasksTidResult(w http.ResponseWriter, r *http.Request,
	vars map[string]string, session *sessions.Session, token string) {
	if task, err := db.GetTask(vars["tid"], token); err != nil {
		handleError(w, r, err)
	} else {
		renderTemplate(w, "tasks-tid-result", task)
	}
}
