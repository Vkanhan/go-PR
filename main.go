package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

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

func init() {
	_ = godotenv.Load()
	UserName, Token = os.Getenv("GITHUB_USERNAME"), os.Getenv("GITHUB_TOKEN")
	if UserName == "" || Token == "" {
		log.Fatal("GitHub username or token is missing in the .env file")
	}
}

func getPRs() []PR {
	openPRs := getPRsByQuery("is:open")
	mergedPRs := getPRsByQuery("is:merged")
	prs := append(openPRs, mergedPRs...)

	// Cache to avoid duplicate API calls for repos.
	repoLogos := make(map[string]string)
	for i, pr := range prs {
		if logo, ok := repoLogos[pr.RepoName]; ok {
			prs[i].LogoURL = logo
		} else {
			logo, err := getRepoLogo(pr.RepoName)
			if err != nil {
				log.Printf("Error fetching logo for repo %s: %v", pr.RepoName, err)
				logo = "" 
			}
			repoLogos[pr.RepoName] = logo
			prs[i].LogoURL = logo
		}
	}
	return prs
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

func getPRsByQuery(queryQualifier string) []PR {
	var prs []PR
	page := 1
	perPage := 100 // Maximum allowed per page

	for {
		// query URL q=author:Githubusername+type:pr+is:open
		url := fmt.Sprintf("%s/search/issues?q=author:%s+type:pr+%s&sort=created&order=asc&per_page=%d&page=%d",
			GitHubAPIBase, UserName, queryQualifier, perPage, page)
			
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Fatalf("Error creating request: %v", err)
		}
		req.Header.Set("Authorization", "token "+Token)
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			log.Fatalf("Error fetching PRs: %v", err)
		}
		defer resp.Body.Close()

		var result GitHubPRResponse

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			log.Fatalf("Error decoding PR response: %v", err)
		}

		if len(result.Items) == 0 {
			break
		}

		for _, item := range result.Items {
			prs = append(prs, PR{
				Title:     item.Title,
				HTMLURL:   item.HTMLURL,
				RepoName:  extractRepoName(item.RepositoryURL),
				Number:    item.Number,
				CreatedAt: item.CreatedAt,
			})
		}

		if len(result.Items) < perPage {
			break
		}
		page++
	}
	return prs
}

// extractRepoName converts the repository URL to an owner/repo format.
func extractRepoName(repoURL string) string {
	return strings.TrimPrefix(repoURL, GitHubAPIBase+"/repos/")
}

func getRepoLogo(repoName string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s", GitHubAPIBase, repoName)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "token "+Token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get repo details, status: %d", resp.StatusCode)
	}

	var repoDetails struct {
		Owner struct {
			AvatarURL string `json:"avatar_url"`
		} `json:"owner"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repoDetails); err != nil {
		return "", err
	}
	return repoDetails.Owner.AvatarURL, nil
}

func getCommits(repo string, prNumber int) []Commit {
	url := fmt.Sprintf("%s/repos/%s/pulls/%d/commits", GitHubAPIBase, repo, prNumber)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Error creating request for commits: %v", err)
		return nil
	}
	req.Header.Set("Authorization", "token "+Token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error fetching commits for PR #%d in %s: %v", prNumber, repo, err)
		return nil
	}
	defer resp.Body.Close()

	var commits []Commit
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		log.Printf("Error decoding commits for PR #%d in %s: %v", prNumber, repo, err)
		return nil
	}

	var filtered []Commit
	for _, commit := range commits {
		// Filter out merge commits for clear commit message
		if strings.HasPrefix(commit.Commit.Message, "Merge") {
			continue
		}

		// Filter out for clear commit message
		lines := strings.Split(commit.Commit.Message, "\n")
		var newLines []string
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "Signed-off-by:") {
				continue
			}
			newLines = append(newLines, line)
		}
		commit.Commit.Message = strings.Join(newLines, "\n")
		filtered = append(filtered, commit)
	}
	return filtered
}

func handler(w http.ResponseWriter, r *http.Request) {
	prs := getPRs()
	if len(prs) == 0 {
		http.Error(w, "No matching PRs found.", http.StatusNotFound)
		return
	}

	var details []PRDetails
	for _, pr := range prs {
		commits := getCommits(pr.RepoName, pr.Number)
		details = append(details, PRDetails{
			PR:      pr,
			Commits: commits,
		})
	}

	tmpl, err := template.ParseFiles("result.html")
	if err != nil {
		http.Error(w, "Error loading template", http.StatusInternalServerError)
		log.Printf("Error loading template: %v", err)
		return
	}

	data := struct {
		PRs []PRDetails
	}{
		PRs: details,
	}

	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Error executing template: %v", err)
	}
}

func main() {
	http.HandleFunc("/", handler)
	log.Println("Server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
