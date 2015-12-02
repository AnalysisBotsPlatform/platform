package html_handler

import(
    "net/http"
    "log"
    "io/ioutil"
    "html/template"
    "db"
	"strings"
)

const CookieName string = "analysis_bot_cookie"

const UserPagePath string = "../../web-interface_copy/index.html"
const ProjectsPagePath string = "../../web-interface_copy/repositories.html"

//TODO implement
func HandleError(w http.ResponseWriter, req *http.Request, err error){

}

//TODO implement
func HandleLoginRequest(w http.ResponseWriter, req *http.Request){
    c := new(http.Cookie)
    c.Name = CookieName
    c.Value = "13"
    http.SetCookie(w, c)
}


// Project related handlers
func HandleProjects(w http.ResponseWriter, req *http.Request){
    url := req.URL.String()
    
    userCookie, err := retrieveCookie(w, req)
    if(err != nil){
        return
    }
    
    splittedURL := strings.Split(url, "/")
    numURLSubPaths := len(splittedURL)
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



func HandleUserRequest(w http.ResponseWriter, req *http.Request){
    userCookie, err := retrieveCookie(w, req)
    if(err != nil){
        return
    }

    // retrieve user from DB
    user, err := db.GetUserById(userCookie)
    if err != nil{
        log.Println("handleUserRequest: ", err)
        HandleError(w, req, err)
        return
    }
    
    // fill html Template
    fillTemplate(w, req, UserPagePath, user)
    
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













