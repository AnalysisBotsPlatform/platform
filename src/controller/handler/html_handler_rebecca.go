func MatchTasksRequest(w http.ResponseWriter, req *http.Request){
    
    userCookie, err := retrieveCookie(w, req)
    if(err != nil){
        return
    }
    
    splitURL = strings.Split(req.URL.String(), "/")
    lenghtSplitURL := len(splitURL)
    if lenghtSplitURL {
        HandleTasksRequest(w, req, userCookie)
    }
    else{
        if lenghtSplitURL {
            HandleTasksIdRequest(w, req , splitURL[1])
        }
        else{
            HandleTasksResultRequest(w, req , splitURL[1])
        }
    }
}



const TasksPagePath string = "../../web-interface_copy/" //!!!!!!!!!!!!

func HandleTasksRequest(w http.ResponseWriter, req *http.Request, uid string){
    
    // retrieve user from DB
    tasks [] , err := db.GetTasksByUid(uid)
    if err != nil{
        log.Println("handleUTasksRequest: ", err)
        HandleError(w, req, err)
        return
    }
    
    // fill html Template
    fillTemplate(w, req, TasksPagePath, tasks)
}


const OneTaskPagePath string = "../../web-interface_copy/" //!!!!!!!!!!!!
func HandleTasksIdRequest(w http.ResponseWriter, req *http.Request, taskId string){
    
    task , err := db.GetTaskById(taskId)
    if err != nil{
        log.Println("handleUTasksRequest: ", err)
        HandleError(w, req, err)
        return
    }
    
    // fill html Template
    fillTemplate(w, req, OneTaskPagePath, task)
}


const TaskResultPagePath string = "../../web-interface_copy/" //!!!!!!!!!!!!
func HandleTasksResultRequest(w http.ResponseWriter, req *http.Request, taskId string){
    
    result , err := db.GetTaskResultById(taskId)
    if err != nil{
        log.Println("handleUTasksRequest: ", err)
        HandleError(w, req, err)
        return
    }
    
    // fill html Template
    fillTemplate(w, req, TaskResultPagePath, result)
}






func MatchBotsRequest(w http.ResponseWriter, req *http.Request){
    
    userCookie, err := retrieveCookie(w, req)
    if(err != nil){
        return
    }
    
    splitURL = strings.Split(req.URL.String(), "/")
    lenghtSplitURL := len(splitURL)
    if lenghtSplitURL {
        HandleTasksRequest(w, req, userCookie)
    }
    else{
        HandleTasksIdRequest(w, req , splitURL[1])

    }
}


const BotsPagePath string = "../../web-interface_copy/" //!!!!!!!!!!!!

func HandleBotsRequest(w http.ResponseWriter, req *http.Request){
    
    // retrieve user from DB
    bots [] , err := db.GetAllBots()
    if err != nil{
        log.Println("handleUTasksRequest: ", err)
        HandleError(w, req, err)
        return
    }
    
    // fill html Template
    fillTemplate(w, req, BotsPagePath, bots)
}


const OneBotsPagePath string = "../../web-interface_copy/" //!!!!!!!!!!!!

func HandleBotsIdRequest(w http.ResponseWriter, req *http.Request, bid string){
    
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