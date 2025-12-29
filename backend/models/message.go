package models

type Create struct {
	GitRepo string `json:"gitrepo"`
	DepId   string `json:"DepId"`
	AppName string `json:"appName"`
}

type Delete struct {
	DepId string `json:"depid"`
}

type Job struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
