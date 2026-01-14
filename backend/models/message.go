package models

type Create struct {
	GitRepo string `json:"gitrepo"`
	DepId   string `json:"DepId"`
	AppName string `json:"appName"`
}

type Delete struct {
	UserID  string `json:"userId"`
	AppName string `json:"appname"`
	Force   bool   `json:"force"`
}

type Job struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type QueueResult struct {
	Queue  string
	Create *Create
	Delete *Delete
}
