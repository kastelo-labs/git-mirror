package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

func pullPushRepo(srcURL, srcUser, srcToken, dstURL, dstUser, dstToken string, verbose bool) error {
	// Create a temporary repository location
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(srcURL)))[:8]
	path := filepath.Join(os.TempDir(), id) + ".git"
	if verbose {
		log.Println("Local repo path is", path)
	}

	// Clone or fetch the original repo
	var repo *git.Repository
	var err error
	if _, err = os.Stat(path); os.IsNotExist(err) {
		repo, err = cloneRepo(path, srcURL, srcUser, srcToken, verbose)
	} else {
		repo, err = fetchRepo(path, srcUser, srcToken, verbose)
	}
	if err != nil {
		return err
	}

	// Make sure we have a remote pointing at the destination
	if err := ensureRemote(repo, "dest", dstURL, verbose); err != nil {
		return err
	}

	// Push all refs
	if err := pushAllRefs(repo, "dest", dstUser, dstToken, verbose); err != nil {
		return err
	}
	return nil
}

func cloneRepo(path, url, user, token string, verbose bool) (*git.Repository, error) {
	if verbose {
		log.Println("Cloning", url, "...")
	}
	opts := &git.CloneOptions{URL: url}
	if user != "" || token != "" {
		opts.Auth = &http.BasicAuth{
			Username: user,
			Password: token,
		}
	}
	repo, err := git.PlainClone(path, true, opts)
	if err != nil {
		return nil, fmt.Errorf("git clone: %w", err)
	}
	return repo, nil
}

func fetchRepo(path, user, token string, verbose bool) (*git.Repository, error) {
	if verbose {
		log.Println("Fetching ...")
	}
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}
	opts := &git.FetchOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			"+refs/heads/*:refs/heads/*",
		},
		Tags: git.AllTags,
	}
	if user != "" || token != "" {
		opts.Auth = &http.BasicAuth{
			Username: user,
			Password: token,
		}
	}
	err = repo.Fetch(opts)
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil, fmt.Errorf("git fetch: %w", err)
	}
	return repo, nil
}

func ensureRemote(repo *git.Repository, name, url string, verbose bool) error {
	remotes, _ := repo.Remotes()
	exists := false
	for _, remote := range remotes {
		cfg := remote.Config()
		if cfg.Name == name {
			if len(cfg.URLs) != 1 || cfg.URLs[0] != url {
				if verbose {
					log.Printf("Remote %q has different URL, removing", name)
				}
				if err := repo.DeleteRemote(name); err != nil {
					return fmt.Errorf("delete remote: %w", err)
				}
				break
			}
			exists = true
			break
		}
	}
	if !exists {
		if verbose {
			log.Printf("Adding remote %q for URL %s", name, url)
		}
		_, err := repo.CreateRemote(&config.RemoteConfig{
			Name: name,
			URLs: []string{url},
			Fetch: []config.RefSpec{
				config.RefSpec("+refs/heads/*:refs/remotes/" + name + "/*"),
			},
		})
		if err != nil {
			return fmt.Errorf("create remote: %w", err)
		}
	}
	return nil
}

func pushAllRefs(repo *git.Repository, remote, user, token string, verbose bool) error {
	if verbose {
		log.Println("Pushing ...")
	}
	opts := &git.PushOptions{
		RemoteName: remote,
		RefSpecs: []config.RefSpec{
			"+refs/heads/*:refs/heads/*",
			"+refs/tags/*:refs/tags/*",
		},
	}
	if user != "" || token != "" {
		opts.Auth = &http.BasicAuth{
			Username: user,
			Password: token,
		}
	}
	err := repo.Push(opts)
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}
