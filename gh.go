// Package gh is a library for CLI Go applications to help interface with the gh CLI tool,
// and the GitHub API.
//
// Note that the examples in this package assume gh and git are installed. They do not run in
// the Go Playground used by pkg.go.dev.
package gh

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"

	iapi "github.com/cli/go-gh/internal/api"
	"github.com/cli/go-gh/internal/config"
	"github.com/cli/go-gh/internal/git"
	irepo "github.com/cli/go-gh/internal/repository"
	"github.com/cli/go-gh/internal/ssh"
	"github.com/cli/go-gh/pkg/api"
	repo "github.com/cli/go-gh/pkg/repository"
	"github.com/cli/safeexec"
)

// Exec gh command with provided arguments.
func Exec(args ...string) (stdOut, stdErr bytes.Buffer, err error) {
	path, err := path()
	if err != nil {
		err = fmt.Errorf("could not find gh executable in PATH. error: %w", err)
		return
	}
	return run(path, nil, args...)
}

func path() (string, error) {
	return safeexec.LookPath("gh")
}

func run(path string, env []string, args ...string) (stdOut, stdErr bytes.Buffer, err error) {
	cmd := exec.Command(path, args...)
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr
	if env != nil {
		cmd.Env = env
	}
	err = cmd.Run()
	if err != nil {
		err = fmt.Errorf("failed to run gh: %s. error: %w", stdErr.String(), err)
		return
	}
	return
}

// RESTClient builds a client to send requests to GitHub REST API endpoints.
// As part of the configuration a hostname, auth token, and default set of headers are resolved
// from the gh environment configuration. These behaviors can be overridden using the opts argument.
func RESTClient(opts *api.ClientOptions) (api.RESTClient, error) {
	var cfg config.Config
	var token string
	var err error
	if opts == nil {
		opts = &api.ClientOptions{}
	}
	if opts.Host == "" || opts.AuthToken == "" {
		cfg, err = config.Load()
		if err != nil {
			return nil, err
		}
	}
	if opts.Host == "" {
		opts.Host = cfg.Host()
	}
	if opts.AuthToken == "" {
		token, err = cfg.AuthToken(opts.Host)
		if err != nil {
			return nil, err
		}
		opts.AuthToken = token
	}
	return iapi.NewRESTClient(opts.Host, opts), nil
}

// GQLClient builds a client to send requests to GitHub GraphQL API endpoints.
// As part of the configuration a hostname, auth token, and default set of headers are resolved
// from the gh environment configuration. These behaviors can be overridden using the opts argument.
func GQLClient(opts *api.ClientOptions) (api.GQLClient, error) {
	var cfg config.Config
	var token string
	var err error
	if opts == nil {
		opts = &api.ClientOptions{}
	}
	if opts.Host == "" || opts.AuthToken == "" {
		cfg, err = config.Load()
		if err != nil {
			return nil, err
		}
	}
	if opts.Host == "" {
		opts.Host = cfg.Host()
	}
	if opts.AuthToken == "" {
		token, err = cfg.AuthToken(opts.Host)
		if err != nil {
			return nil, err
		}
		opts.AuthToken = token
	}
	return iapi.NewGQLClient(opts.Host, opts), nil
}

// CurrentRepository uses git remotes to determine the GitHub repository
// the current directory is tracking.
func CurrentRepository() (repo.Repository, error) {
	override := os.Getenv("GH_REPO")
	if override != "" {
		return repo.Parse(override)
	}

	remotes, err := git.Remotes()
	if err != nil {
		return nil, err
	}
	if len(remotes) == 0 {
		return nil, errors.New("unable to determine current repository, no git remotes configured for this repository")
	}

	sshConfig := ssh.ParseConfig()
	translateRemotes(remotes, sshConfig.Translator())

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	hosts := cfg.Hosts()

	filteredRemotes := remotes.FilterByHosts(hosts)
	if len(filteredRemotes) == 0 {
		return nil, errors.New("unable to determine current repository, none of the git remotes configured for this repository point to a known GitHub host")
	}

	r := filteredRemotes[0]
	return irepo.New(r.Host, r.Owner, r.Repo), nil
}

func translateRemotes(remotes git.RemoteSet, urlTranslate func(*url.URL) *url.URL) {
	for _, r := range remotes {
		if r.FetchURL != nil {
			r.FetchURL = urlTranslate(r.FetchURL)
		}
		if r.PushURL != nil {
			r.PushURL = urlTranslate(r.PushURL)
		}
	}
}
