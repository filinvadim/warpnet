package warpnet

import (
	"embed"
	"errors"
	"fmt"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
)

// NOTE this file always must be in a root dir of project

/* #nosec */
//go:embed server/frontend/dist
var static embed.FS

func GetStaticFS() fs.FS {
	return static
}

//go:embed config.yml
var configFile []byte

func GetConfigFile() []byte {
	return configFile
}

func CloneStatic() error {
	repoURL := "https://github.com/filinvadim/warpnet-frontend"
	cloneDir := "./server/frontend"

	_ = os.Mkdir(cloneDir, 0750)

	pass := os.Getenv("GITHUB_TOKEN")
	if pass == "" {
		log.Fatal("GITHUB_TOKEN environment variable not set")
		return nil
	}

	fmt.Println("github token found")

	repo, err := git.PlainClone(cloneDir, false, &git.CloneOptions{
		URL:      repoURL,
		Progress: io.Discard,
		Auth: &http.BasicAuth{
			Username: "git", // required
			Password: pass,
		},
	})
	defer func() {
		_ = os.RemoveAll(path.Join(cloneDir, ".git"))
	}()
	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
		repo, err := git.PlainOpen(cloneDir)
		if err != nil {
			log.Fatalf("failed to open existing repo: %v", err)
			return err
		}
		worktree, err := repo.Worktree()
		if err != nil {
			log.Fatalf("failed to get worktree: %v", err)
			return err
		}
		err = worktree.Pull(&git.PullOptions{
			RemoteName: "origin",
			Auth: &http.BasicAuth{
				Username: "git", // required
				Password: pass,
			},
			Progress: io.Discard,
		})
		if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			log.Fatalf("Failed to pull latest changes: %v", err)
			return err
		}
		head, _ := repo.Head()
		log.Println("static repository updated successfully", head.Hash().String())
		return nil
	}
	if err != nil {
		log.Fatalf("failed to clone static repository: %v", err)
	}

	head, _ := repo.Head()
	log.Println("static repository cloned successfully", head.Hash().String())
	return nil
}
