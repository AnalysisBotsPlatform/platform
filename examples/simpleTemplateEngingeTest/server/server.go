package main

import (
    "fmt"
    "io"
    "net/http"
    "html/template"
    "io/ioutil"
)

type Activity struct{
    Identifier string
    Hours int
}

type Person struct {
	Name       string
	Sirname    string
    Hobbies    []Activity
}

func ParseTemp(w http.ResponseWriter){
    a1 := Activity{"Playing the guitar", 2}
    a2 := Activity{"Photography", 1}
    a3 := Activity{"Windsurfing", 0}
    a4 := Activity{"Climbing", 4}
    
    p := Person{"Merlin", "Köhler", []Activity{a1, a2, a3, a4}}
    /*adjust this for your needs:*/
    b, err := ioutil.ReadFile("/home/merlin/workspace_go/serverTest/html/index.html")
    if err != nil {
        panic(err)
	}
    s := string(b)
    
    tmpl, err := template.New("page").Parse(s)
    if err != nil { panic(err) }
    err = tmpl.Execute(w, p)
    if err != nil { panic(err) }
    

}

func ReqHandlerHello(w http.ResponseWriter, req *http.Request){
    fmt.Printf("Handler called\n")
    ParseTemp(w)
//    io.WriteString(w, "Hello ;)")
}


func ReqHandlerCSS(w http.ResponseWriter, req *http.Request){
		    /*adjust this for your needs:*/
        b, err := ioutil.ReadFile("/home/merlin/workspace_go/serverTest/html/style.css")
    if err != nil {
        panic(err)
	}
    s := string(b)
    io.WriteString(w, s)
}

func main(){
    fmt.Println("Start listening...")
    http.HandleFunc("/test", ReqHandlerHello)
    http.HandleFunc("/style.css", ReqHandlerCSS)
    err := http.ListenAndServe(":8080", nil)
    if err != nil{
        
        fmt.Printf("Failed: \n",err)
    }
        
    fmt.Printf("Termiated\n")
}
