package controller

import(
    "fmt"
    "net/http"
    "controller/handler"
)

const userPagePattern string = "/user"
const loginPagePattern string = "/login"

func main(){
    fmt.Println("Controller start...")
    
    // register handlers for http requests
    http.HandleFunc(userPagePattern, html_handler.HandleUserRequest)
    http.HandleFunc(loginPagePattern, html_handler.HandleLoginRequest)
    
    // listen on port 8080 to handle http requests
    err := http.ListenAndServe(":8080", nil)
    if err != nil{
        fmt.Printf("Controller listen failed: \n", err)
    }
        
    fmt.Printf("Termiated\n")
}
