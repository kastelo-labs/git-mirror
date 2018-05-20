package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/kingpin"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
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
	old := time.Now().Add(-2 * 365 * 24 * time.Hour)

	for _, src := range repos {
		name := src.GetFullName()

		// Check the project GitLab-side
		proj, _, err := gh.Projects.GetProject(gitlabGroup + "/" + src.GetName())
		if err != nil {
			log.Printf("%s: getting project: %v", name, err)
			continue
		}

		err = pullPushRepo(githubUser, src.GetName(), src.GetGitURL(), gitlabURL, gitlabGroup, gitlabToken)
		if err != nil && proj.Archived {
			// Try to unarchive the project so we can push to it
			gh.Projects.UnarchiveProject(proj.ID)
			proj.Archived = false
			err = pullPushRepo(githubUser, src.GetName(), src.GetGitURL(), gitlabURL, gitlabGroup, gitlabToken)
		}
		if err != nil {
			log.Printf("%s: push/pull: %v", name, err)
			continue
		}

		if !strings.HasPrefix(proj.Description, src.GetDescription()) {
			_, _, err := gh.Projects.EditProject(proj.ID, &gitlab.EditProjectOptions{
				Description: gitlab.String(fmt.Sprintf("%s (mirror of github.com/%s/%s)", src.GetDescription(), githubUser, src.GetName())),
				Visibility:  gitlab.Visibility(gitlab.PublicVisibility),
			})
			if err != nil && err != git.NoErrAlreadyUpToDate {
				log.Printf("%s: updating project: %v", name, err)
			}
		} else if proj.Description == "" && src.GetDescription() == "" {
			_, _, err := gh.Projects.EditProject(proj.ID, &gitlab.EditProjectOptions{
				Description: gitlab.String(fmt.Sprintf("Mirror of github.com/%s/%s", githubUser, src.GetName())),
				Visibility:  gitlab.Visibility(gitlab.PublicVisibility),
			})
			if err != nil && err != git.NoErrAlreadyUpToDate {
				log.Printf("%s: updating project: %v", name, err)
			}
		}

		archived := src.GetArchived() || src.GetFork() && src.GetUpdatedAt().Before(old)
		if archived && !proj.Archived {
			_, _, err := gh.Projects.ArchiveProject(proj.ID)
			if err != nil && err != git.NoErrAlreadyUpToDate {
				log.Printf("%s: archiving project: %v", name, err)
			}
		}
	}
}

func pullPushRepo(githubUser, githubRepo, githubURL, gitlabURL, gitlabGroup, gitlabToken string) error {
	// Create a temporary repository location
	path := filepath.Join(os.TempDir(), githubUser, githubRepo) + ".git"
	os.MkdirAll(filepath.Dir(path), 0700)

	// Clone or fetch the original repo
	var repo *git.Repository
	var err error
	if _, err = os.Stat(path); os.IsNotExist(err) {
		repo, err = cloneRepo(githubURL, path)
	} else {
		repo, err = fetchRepo(path)
	}
	if err != nil {
		return errors.Wrap(err, "fetching")
	}

	// Make sure we have a remote pointing at the destination
	remoteURL := fmt.Sprintf("%s/%s/%s.git", gitlabURL, gitlabGroup, githubRepo)
	ensureRemote("gitlab", remoteURL, repo)

	// Push all refs
	if err := pushAllRefs("gitlab", gitlabToken, repo); err != nil {
		return errors.Wrap(err, "pushing")
	}
	return nil
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
