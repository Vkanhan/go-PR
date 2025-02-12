package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

const GitHubAPIBase = "https://api.github.com"

var (
	UserName string
	Token    string
)

type PR struct {
	Title     string `json:"title"`
	HTMLURL   string `json:"html_url"`
	RepoName  string
	Number    int    `json:"number"`
	CreatedAt string `json:"created_at"`
}

type Commit struct {
	Commit struct {
		Message string `json:"message"`
	} `json:"commit"`
}

type PRDetails struct {
	PR      PR
	Commits []Commit
}

func init() {
	_ = godotenv.Load()
	UserName, Token = os.Getenv("GITHUB_USERNAME"), os.Getenv("GITHUB_TOKEN")
	if UserName == "" || Token == "" {
		log.Fatal("GitHub username or token is missing in the .env file")
	}
}

type GitHubPRResponse struct {
	Items []GitHubPRItem `json:"items"`
}

type GitHubPRItem struct {
	Title         string `json:"title"`
	HTMLURL       string `json:"html_url"`
	RepositoryURL string `json:"repository_url"`
	Number        int    `json:"number"`
	CreatedAt     string `json:"created_at"`
}

func getPRs() []PR {
	url := fmt.Sprintf("%s/search/issues?q=author:%s+type:pr&per_page=10", GitHubAPIBase, UserName)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error fetching PRs: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var result GitHubPRResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Error decoding PR response: %v", err)
		return nil
	}

	prs := make([]PR, len(result.Items))
	for i, item := range result.Items {
		prs[i] = PR{
			Title:     item.Title,
			HTMLURL:   item.HTMLURL,
			RepoName:  extractRepoName(item.RepositoryURL),
			Number:    item.Number,
			CreatedAt: item.CreatedAt,
		}
	}
	return prs
}

func extractRepoName(repoURL string) string {
	return strings.TrimPrefix(repoURL, GitHubAPIBase+"/repos/")
}

func getCommits(repo string, prNumber int) []Commit {
	url := fmt.Sprintf("%s/repos/%s/pulls/%d/commits", GitHubAPIBase, repo, prNumber)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error fetching commits for PR #%d in %s: %v", prNumber, repo, err)
		return nil
	}
	defer resp.Body.Close()

	var commits []Commit
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		log.Printf("Error decoding commits for PR #%d in %s: %v", prNumber, repo, err)
		return []Commit{}
	}
	return commits
}

func handler(w http.ResponseWriter, r *http.Request) {
	prs := getPRs()
	if len(prs) == 0 {
		http.Error(w, "No PRs found.", http.StatusNotFound)
		return
	}

	details := make([]PRDetails, len(prs))
	for i, pr := range prs {
		details[i] = PRDetails{
			PR:      pr,
			Commits: getCommits(pr.RepoName, pr.Number),
		}
	}

	tmpl, err := template.ParseFiles("result.html")
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Error loading HTML template: %v", err)
		return
	}

	_ = tmpl.Execute(w, map[string]any{"UserName": UserName, "PRs": details})
}

func main() {
	http.HandleFunc("/", handler)
	fmt.Printf("Server running on http://localhost:8080 as %s\n", UserName)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
