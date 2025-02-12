package main

import (
	"html/template"
	"log"
	"net/http"
)

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

	tmpl, err := template.ParseFiles("templates/result.html")
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
