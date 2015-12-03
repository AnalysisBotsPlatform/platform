package html_handler

import(
    "net/http"
    "net/url"
    "log"
    "io/ioutil"
    "html/template"
    "strings"
    "regexp"
    "db"
    "datatypes"
//    "encoding/json"
//    "bytes"
//    "strconv"
)

const client_id string = "4049f0af2d782f297291"
const client_secret = "5d0a06326015f5b50af0b5e57e4934217a21d156"
const GitHubResponseURL string = "/github/"
const AppPrefix string = "http://analysis-bots.ddns.org:8080"
const GitHubAuthentification string = "https://github.com/login/oauth/authorize?client_id="+client_id//+"&redirect_uri="+AppPrefix+GitHubResponseURL+"&state=HalloGithub"
const GitHubPostAuthentification string = "https://github.com/login/oauth/access_token"

const CookieName string = "analysis_bot_cookie"

const UserPagePath string = "../../web-interface_copy/index.html"
const ProjectsPagePath string = "../../web-interface_copy/repositories.html"
const OneBotsPagePath string = "../../web-interface_copy/bots.html"
const BotsPagePath string = "../../web-interface_copy/bots.html" 
const TasksPagePath string = "../../web-interface_copy/bots-actions.html" 
const OneTaskPagePath string = "../../web-interface_copy/bots-actions.html" 
const TaskResultPagePath string = "../../web-interface_copy/bots-actions.html"

const WebInterfacePath string = "../../web-interface_copy/"


type GitHubResponse struct{
    token_type string
    scope string
    access_token string
    
}

//TODO implement
func HandleError(w http.ResponseWriter, req *http.Request, err error){

}

//TODO implement
func HandleLoginRequest(w http.ResponseWriter, req *http.Request){
    log.Println("Login Handler")
    http.Redirect(w, req, GitHubAuthentification, http.StatusFound)

//    resp, err := http.NewRequest("GET", GitHubAuthentification, nil)
//        
//    log.Println(resp)
//    if(err != nil){
//        log.Println("handleUserRequest: ", err)
//        HandleError(w, req, err)
//        return
//    }
}

func HandleGitHubResponse(w http.ResponseWriter, req * http.Request){
    log.Println("Received Github response", req.URL.String())
    
    
    
    responseCode := req.URL.Query().Get("code")
    data := url.Values{"client_id": {client_id}, "client_secret": {client_secret}, "code":{responseCode}}
    
/*    client := &http.Client{}
    r, _ := http.NewRequest("POST", GitHubPostAuthentification,bytes.NewBufferString(data.Encode()))
    r.Header.Add("Accept:","application/json")
    r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
    r.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))
    
    resp, _ := client.Do(r)    
    
    decoder := json.NewDecoder(resp.Body)
    
    body, _ := ioutil.ReadAll(resp.Body)
    
    var responseStruct GitHubResponse
    log.Println(string(body))
    decoder.Decode(&responseStruct)
    
    log.Println(string(body))
    log.Println(responseStruct.access_token)*/
    
    resp, err := http.PostForm(GitHubPostAuthentification, data)
    
    if(err != nil){
        log.Println("handleUserRequest: ", err)
        HandleError(w, req, err)
        return
    }
    body, err := ioutil.ReadAll(resp.Body)
    if(err != nil){
        log.Println("handleUserRequest: ", err)
        HandleError(w, req, err)
        return
    }
    
    bodyString := string(body)
    log.Println(bodyString)
    
    re := regexp.MustCompile("&scope=.*")
    bodyString = re.ReplaceAllString(bodyString, "")
    
    re = regexp.MustCompile("access_token=")
    bodyString = re.ReplaceAllString(bodyString, "")
    
    log.Println(bodyString)
    
    c := new(http.Cookie)
    c.Name = CookieName
    c.Value = bodyString
    http.SetCookie(w, c)
    
    HandleUserRequest(w, req)
}


// Project related handlers
func HandleProjects(w http.ResponseWriter, req *http.Request){
    url := req.URL.String()
    url = strings.TrimPrefix(url, "/")
    url = strings.TrimSuffix(url, "/")
    
    log.Println("handle projects: ",url)
    
    userCookie, err := retrieveCookie(w, req)
    if(err != nil){
        return
    }
    sanityCheckCookie(userCookie)
    
    if(! (strings.HasSuffix(url, ".css") || strings.HasSuffix(url, ".js"))){
        splittedURL := strings.Split(url, "/")
        numURLSubPaths := len(splittedURL)
        log.Println("num: ", numURLSubPaths)
        switch (numURLSubPaths){
            case 1:
            handleProjects(userCookie, w, req)
            return
            case 2:
            handleProject(userCookie, splittedURL[1], w, req)
            return
            case 3:
            handleProjectBotAttach(userCookie, splittedURL[1], splittedURL[2], w, req)
            return
        }    
    }else{
        handleCSSJSRequests(w, req)
    }
}


func handleProjects(userCookie string, w http.ResponseWriter, req *http.Request){
    
    projects, err := db.GetProjectsByUid(userCookie)
    if(err != nil){
        log.Println("handleUserRequest: ", err)
        HandleError(w, req, err)
        return
    }
    
    fillTemplate(w, req, ProjectsPagePath, projects)
    
}

func handleProject(userCookie string, pId string, w http.ResponseWriter, req *http.Request){
    log.Println("handle single Project")
    project, err := db.GetProjectByUidPid(userCookie, pId);
    if(err != nil){
        log.Println("handleUserRequest: ", err)
        HandleError(w, req, err)
        return
    }
    
    bots, err := db.GetAttachedBotsByUidPid(userCookie, pId)
    if(err != nil){
        log.Println("handleUserRequest: ", err)
        HandleError(w, req, err)
        return
    }
    
    uiProject := new(datatypes.UIProject)
    uiProject.Project = project
    uiProject.Bots = bots
    
    fillTemplate(w, req, ProjectsPagePath, uiProject)
    
}

func handleProjectBotAttach(userCookie string, pId string, bId string, w http.ResponseWriter, req *http.Request){
    
    db.AttachBotToUsersProject(userCookie, pId, bId)
    handleProject(userCookie, pId, w, req)
}


// User related handlers
func HandleUserRequest(w http.ResponseWriter, req *http.Request){
    userCookie, err := retrieveCookie(w, req)
    if(err != nil){
        return
    }
    
    sanityCheckCookie(userCookie)
    
    url := req.URL.String()
    
    
    if(! (strings.HasSuffix(url, ".css") || strings.HasSuffix(url, ".js"))){

        // retrieve user from DB
        user, err := db.GetUserById(userCookie)
        if err != nil{
            log.Println("handleUserRequest: ", err)
            HandleError(w, req, err)
            return
        }

        // fill html Template
        fillTemplate(w, req, UserPagePath, user)
        
    }else{
        handleCSSJSRequests(w, req)
    }
    
}



// Bot related handlers
func HandleBotsRequest(w http.ResponseWriter, req *http.Request){
    
    userCookie, err := retrieveCookie(w, req)
    if(err != nil){
        return
    }
    
    sanityCheckCookie(userCookie)
    
    url := req.URL.String()
    url = strings.TrimPrefix(url, "/")
    url = strings.TrimSuffix(url, "/")
    
    if(! (strings.HasSuffix(url, ".css") || strings.HasSuffix(url, ".js"))){
        splitURL := strings.Split(url , "/")
        lenghtSplitURL := len(splitURL)

        switch lenghtSplitURL{
        case 1:
            handleBotsRequest(w, req)
            return
        case 2:
            handleBotsIdRequest(w, req , splitURL[1])
            return
        }
    }else{
        handleCSSJSRequests(w, req)
    }
    
}

func handleBotsRequest(w http.ResponseWriter, req *http.Request){
    
    // retrieve user from DB
    bots, err := db.GetAllBots()
    if err != nil{
        log.Println("handleUTasksRequest: ", err)
        HandleError(w, req, err)
        return
    }
    
    // fill html Template
    fillTemplate(w, req, BotsPagePath, bots)
}

func handleBotsIdRequest(w http.ResponseWriter, req *http.Request, bid string){
    
    // retrieve user from DB
    bot , err := db.GetBotById(bid)
    if err != nil{
        log.Println("handleUTasksRequest: ", err)
        HandleError(w, req, err)
        return
    }
    
    // fill html Template
    fillTemplate(w, req, OneBotsPagePath, bot)
}


// Task related handlersPath
func HandleTasksRequest(w http.ResponseWriter, req *http.Request){
    
    userCookie, err := retrieveCookie(w, req)
    if(err != nil){
        return
    }
    
    sanityCheckCookie(userCookie)
    
    url := req.URL.String()
    url = strings.TrimPrefix(url, "/")
    url = strings.TrimSuffix(url, "/")
    
    if(! (strings.HasSuffix(url, ".css") || strings.HasSuffix(url, ".js"))){

        splitURL := strings.Split(url, "/")
        lenghtSplitURL := len(splitURL)

        switch lenghtSplitURL{

        case 1:
            handleTasksRequest(w, req, userCookie)
            return
        case 2:
            handleTasksIdRequest(w, req , splitURL[1])
            return
        case 3:
            handleTasksResultRequest(w, req , splitURL[1])
            return
        }
    }else{
        handleCSSJSRequests(w, req)
    }
    
}



func handleTasksRequest(w http.ResponseWriter, req *http.Request, uid string){
    
    // retrieve user from DB
    tasks, err := db.GetTasksByUid(uid)
    if err != nil{
        log.Println("handleUTasksRequest: ", err)
        HandleError(w, req, err)
        return
    }
    
    // fill html Template
    fillTemplate(w, req, TasksPagePath, tasks)
}

func handleTasksIdRequest(w http.ResponseWriter, req *http.Request, taskId string){
    
    task , err := db.GetTaskById(taskId)
    if err != nil{
        log.Println("handleUTasksRequest: ", err)
        HandleError(w, req, err)
        return
    }
    
    // fill html Template
    fillTemplate(w, req, OneTaskPagePath, task)
}



func handleTasksResultRequest(w http.ResponseWriter, req *http.Request, taskId string){
    
    task , err := db.GetTaskById(taskId)
    if err != nil{
        log.Println("handleUTasksRequest: ", err)
        HandleError(w, req, err)
        return
    }
    
    // fill html Template
    fillTemplate(w, req, TaskResultPagePath, task)
}



func handleCSSJSRequests(w http.ResponseWriter, req *http.Request){
    url := req.URL.String()
    var newURL string
    if(strings.Contains(url, "bower_components")){
        re := regexp.MustCompile(".*bower_components")    
        newURL = WebInterfacePath + re.ReplaceAllString(url, "bower_components")
    }else{
        re := regexp.MustCompile(".*dist")    
        newURL = WebInterfacePath + re.ReplaceAllString(url, "dist")
    }
    
    http.ServeFile(w, req, newURL)
}


//TODO
func sanityCheckCookie(userCookie string){
    return
}

func retrieveCookie(w http.ResponseWriter, req *http.Request) (cookieValue string, err error){
    cookie, err := req.Cookie(CookieName)
    // no session is running
    if err == http.ErrNoCookie{
        HandleLoginRequest(w, req)
        return 
    }
    if err != nil{
        log.Println("retrieveCookie: ", err)
        HandleError(w, req, err)
        return
    }
    
    cookieValue = cookie.Value
    return
}


func fillTemplate(w http.ResponseWriter, req *http.Request, templateFileName string, fillObject interface{}){
    // open html templateme
    b, err := ioutil.ReadFile(templateFileName)
    if err != nil{
        log.Println("fillTemplate: ", err)
        HandleError(w, req, err)
        return
    }
    
    templateString := string(b)
    
    tmpl, err := template.New("page").Parse(templateString)
    if err != nil {
        log.Println("fillTemplate: ", err)
        HandleError(w, req, err)
        return
    }
    err = tmpl.Execute(w, fillObject)
    if err != nil {
        log.Println("fillTemplate: ", err)
        HandleError(w, req, err)
        return
    }
    
}
