package controller

import(
    "fmt"
    "net/http"
    "controller/handler"
)

const rootPagePattern string = "/"
const userPagePattern string = "/user/"
const loginPagePattern string = "/login/"
const botsPagePattern string = "/bots/"
const projectsPagePattern string = "/projects/"
const tasksPagePattern string = "/tasks/"



func main(){
    fmt.Println("Controller start...")
    
    // register handlers for http requests
    http.HandleFunc(html_handler.GitHubResponseURL, html_handler.HandleGitHubResponse)
    http.HandleFunc(rootPagePattern, html_handler.HandleUserRequest)
    http.HandleFunc(userPagePattern, html_handler.HandleUserRequest)
    http.HandleFunc(loginPagePattern, html_handler.HandleLoginRequest)
    http.HandleFunc(botsPagePattern, html_handler.HandleBotsRequest)
    http.HandleFunc(projectsPagePattern, html_handler.HandleProjects)
    http.HandleFunc(tasksPagePattern, html_handler.HandleTasksRequest)
    
    http.HandleFunc("/test/", test)

    // listen on port 8080 to handle http requests
    err := http.ListenAndServe(":8082", nil)
    if err != nil{
        fmt.Printf("Controller listen failed: \n", err)
    }
        
    fmt.Printf("Termiated\n")
}

func test(w http.ResponseWriter, req *http.Request){
    fmt.Println(req.URL.Query().Get("hallo"))
    fmt.Println(req)
}
