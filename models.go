package main 

type PR struct {
	Title     string `json:"title"`
	HTMLURL   string `json:"html_url"`
	RepoName  string `json:"repo_name,omitempty"`
	Number    int    `json:"number"`
	CreatedAt string `json:"created_at"`
	LogoURL   string `json:"logo_url,omitempty"`
}

type Commit struct {
	Commit struct {
		Message string `json:"message"`
	} `json:"commit"`
}

type PRDetails struct {
	PR
	Commits []Commit
}

type GitHubPRResponse struct {
	Items []struct {
		Title         string `json:"title"`
		HTMLURL       string `json:"html_url"`
		RepositoryURL string `json:"repository_url"`
		Number        int    `json:"number"`
		CreatedAt     string `json:"created_at"`
	} `json:"items"`
}