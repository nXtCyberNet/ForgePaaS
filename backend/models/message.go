package models

type Create struct {
	GitRepo string `json:"gitrepo"`
	UserId  string `json:"userid"`
}

type Delete struct {
	DepId string `json:"depid"`
}

type Job struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
