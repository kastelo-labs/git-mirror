package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/alecthomas/kingpin"
	"github.com/google/go-github/github"
	"github.com/xanzy/go-gitlab"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

func main() {
	gitlabURL := kingpin.Flag("gitlab-url", "GitLab instance base URL").Short('u').Default("https://kastelo.io").String()
	gitlabToken := kingpin.Flag("gitlab-token", "GitLab secret token").Short('t').Required().String()
	gitlabGroup := kingpin.Flag("gitlab-group", "GitLab group name").Short('g').Required().String()
	githubUser := kingpin.Flag("github-user", "GitHub user/organization name").Short('h').Required().String()
	kingpin.Parse()

	processMirror(*gitlabURL, *gitlabToken, *gitlabGroup, *githubUser)
}

func processMirror(gitlabURL string, gitlabToken, gitlabGroup, githubUser string) {
	repos, err := listGHRepos(githubUser)
	if err != nil {
		log.Fatalf("Listing repositories for %s: %v", githubUser, err)
	}

	gh := gitlab.NewClient(nil, gitlabToken)
	gh.SetBaseURL(gitlabURL + "/api/v4")

	for _, src := range repos {
		if src.GetFork() {
			continue
		}

		name := src.GetFullName()

		// Create a temporary repository location
		path := filepath.Join(os.TempDir(), githubUser, src.GetName()) + ".git"
		os.MkdirAll(filepath.Dir(path), 0700)

		// Clone or fetch the original repo
		var repo *git.Repository
		if _, err := os.Stat(path); os.IsNotExist(err) {
			url := src.GetGitURL()
			repo, err = cloneRepo(url, path)
		} else {
			repo, err = fetchRepo(path)
		}
		if err != nil {
			log.Printf("%s: fetching: %v", name, err)
			continue
		}

		// Make sure we have a remote pointing at the destination
		remoteURL := fmt.Sprintf("%s/%s/%s.git", gitlabURL, gitlabGroup, src.GetName())
		ensureRemote("gitlab", remoteURL, repo)

		// Push all refs
		if err := pushAllRefs("gitlab", gitlabToken, repo); err != nil {
			log.Printf("%s: pushing -> %s: %v", name, remoteURL, err)
			continue
		}

		// Check the project GitLab-side
		proj, _, err := gh.Projects.GetProject(gitlabGroup + "/" + src.GetName())
		if err != nil {
			log.Printf("%s: getting project: %v", name, err)
			continue
		}

		if proj.Description == "" {
			_, _, err := gh.Projects.EditProject(proj.ID, &gitlab.EditProjectOptions{
				Description: gitlab.String(fmt.Sprintf("Mirror of github.com/%s/%s", githubUser, src.GetName())),
				Visibility:  gitlab.Visibility(gitlab.PublicVisibility),
			})
			if err != nil && err != git.NoErrAlreadyUpToDate {
				log.Printf("%s: updating project: %v", name, err)
			}
		}
	}
}

func cloneRepo(url, path string) (*git.Repository, error) {
	return git.PlainClone(path, true, &git.CloneOptions{
		URL: url,
	})
}

func fetchRepo(path string) (*git.Repository, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	err = repo.Fetch(&git.FetchOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			"+refs/heads/*:refs/heads/*",
		},
		Tags: git.AllTags,
	})
	if err != git.NoErrAlreadyUpToDate && err != nil {
		return nil, err
	}
	return repo, nil
}

func listGHRepos(user string) ([]*github.Repository, error) {
	gh := github.NewClient(nil)
	var repos []*github.Repository
	opts := &github.RepositoryListOptions{}
	for {
		rs, resp, err := gh.Repositories.List(context.TODO(), user, opts)
		if err != nil {
			return nil, err
		}
		repos = append(repos, rs...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return repos, nil
}

func ensureRemote(name, url string, repo *git.Repository) {
	remotes, _ := repo.Remotes()
	exists := false
	for _, remote := range remotes {
		if remote.Config().Name == name {
			exists = true
			break
		}
	}
	if !exists {
		repo.CreateRemote(&config.RemoteConfig{
			Name: name,
			URLs: []string{url},
			Fetch: []config.RefSpec{
				config.RefSpec("+refs/heads/*:refs/remotes/" + name + "/*"),
			},
		})
	}
}

func pushAllRefs(remote, token string, repo *git.Repository) error {
	err := repo.Push(&git.PushOptions{
		RemoteName: remote,
		RefSpecs: []config.RefSpec{
			"+refs/heads/*:refs/heads/*",
			"+refs/tags/*:refs/tags/*",
		},
		Auth: &http.BasicAuth{
			Password: token,
		},
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}
	return nil
}
