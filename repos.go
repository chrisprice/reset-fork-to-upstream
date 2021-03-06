package main

import (
	"github.com/google/go-github/github"
)

type Repo struct {
	Name  string
	Owner string
	URL   string
}

func ListRepos(client *github.Client) ([]Repo, error) {
	repos, _, err := client.Repositories.List("", &github.RepositoryListOptions{
		ListOptions: github.ListOptions{
			PerPage: 100},
		Type: "all",
	})

	if err != nil {
		return nil, err
	}

	result := make([]Repo, 0, len(repos))
	for _, repo := range repos {
		if repo.Fork == nil || !*repo.Fork {
			continue
		}
		push := repo.Permissions == nil || (*repo.Permissions)["push"]
		if !push {
			continue
		}
		result = append(result, Repo{
			Name:  *repo.Name,
			Owner: *repo.Owner.Login,
			URL:   *repo.HTMLURL})
	}

	return result, nil
}
