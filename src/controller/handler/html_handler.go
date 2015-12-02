package html_handler

import(
    "net/http"
    "log"
    "io/ioutil"
    "html/template"
    "db"
)

const CookieName string = "analysis_bot_cookie"

const UserPagePath string = "../../web-interface_copy/index.html"

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

func HandleUserRequest(w http.ResponseWriter, req *http.Request){
    userCookie, err := retrieveCookie(w, req)
    if(err != nil){
        return
    }

    // retrieve user from DB
    user := new(datatypes.User)
    user.User_name = "Nik"
    user.Real_name = "Nikolaus"
    err = db.GetUserById(userCookie, user)
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













