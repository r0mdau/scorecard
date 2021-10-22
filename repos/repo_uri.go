// Copyright 2020 Security Scorecard Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package repos defines a generic repository.
package repos

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	sce "github.com/ossf/scorecard/v3/errors"
)

var (
	// ErrorUnsupportedhost indicates the repo's host is unsupported.
	ErrorUnsupportedhost = errors.New("unsupported host")
	// ErrorInvalidGithubURL indicates the repo's GitHub URL is not in the proper format.
	ErrorInvalidGithubURL = errors.New("invalid GitHub repo URL")
	// ErrorInvalidGithubUsername indicates the repo's GitHub Username is not in the proper format.
	ErrorInvalidGithubUsername = errors.New("invalid GitHub repo Username")
	// ErrorInvalidURL indicates the repo's full GitHub URL was not passed.
	ErrorInvalidURL = errors.New("invalid repo flag")
	// errInvalidRepoType indicates the repo's type is invalid.
	errInvalidRepoType = errors.New("invalid repo type")
)

// RepoURI represents the URI for a repo.
//nolint:govet
type RepoURI struct {
	repoType RepoType
	localDir repoLocalDir
	url      repoURL
	metadata []string
}

type repoLocalDir struct {
	path string
}

type repoURL struct {
	host, owner, repo string
}

// RepoType is the type of a file.
type RepoType int

const (
	// RepoTypeURL is for URLs.
	RepoTypeURL RepoType = iota
	// RepoTypeLocalDir is for source code in directories.
	RepoTypeLocalDir
)

func (r repoLocalDir) Equal(o repoLocalDir) bool {
	return r.path == o.path
}

func (r repoURL) Equal(o repoURL) bool {
	return r.host == o.host &&
		r.owner == o.owner &&
		r.repo == o.repo
}

// NewFromURL creates a RepoURI from URL.
func NewFromURL(u string) (*RepoURI, error) {
	r := &RepoURI{
		repoType: RepoTypeURL,
	}

	if err := r.SetURL(u); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return r, nil
}

// NewFromLocalDirectory creates a RepoURI as a local directory.
func NewFromLocalDirectory(path string) *RepoURI {
	return &RepoURI{
		localDir: repoLocalDir{
			path: path,
		},
		repoType: RepoTypeLocalDir,
	}
}

// SetMetadata sets metadata.
func (r *RepoURI) SetMetadata(m []string) error {
	r.metadata = m
	return nil
}

// AppendMetadata appends metadata.
func (r *RepoURI) AppendMetadata(m ...string) error {
	r.metadata = append(r.metadata, m...)
	return nil
}

// SetURL sets the URL.
func (r *RepoURI) SetURL(u string) error {
	if r.repoType != RepoTypeURL {
		return fmt.Errorf("%w", errInvalidRepoType)
	}
	if err := r.Set(u); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

// Equal checks objects for equality.
func (r *RepoURI) Equal(o *RepoURI) bool {
	return cmp.Equal(r.localDir, o.localDir) &&
		cmp.Equal(r.url, o.url) &&
		cmp.Equal(r.repoType, o.repoType) &&
		cmp.Equal(r.metadata, o.metadata, cmpopts.SortSlices(func(x, y string) bool { return x < y }))
}

// Type method is needed so that this struct can be used as cmd flag.
func (r *RepoURI) Type() string {
	return "repo"
}

// RepoType gives the type of URI.
func (r *RepoURI) RepoType() RepoType {
	return r.repoType
}

// Path retusn the path for a local directory.
func (r *RepoURI) Path() string {
	return r.localDir.path
}

// URL returns a valid url for Repo struct.
func (r *RepoURI) URL() string {
	return fmt.Sprintf("%s/%s/%s", r.url.host, r.url.owner, r.url.repo)
}

// Metadata returns a valid url for Repo struct.
func (r *RepoURI) Metadata() []string {
	return r.metadata
}

// String returns a string representation of Repo struct.
func (r *RepoURI) String() string {
	return fmt.Sprintf("%s-%s-%s", r.url.host, r.url.owner, r.url.repo)
}

// setV4 for the v4 version.
func (r *RepoURI) setV4(s string) error {
	const httpsPrefix = "https://"
	const filePrefix = "file://"

	// Validate the URI and scheme.
	if !strings.HasPrefix(s, filePrefix) &&
		!strings.HasPrefix(s, httpsPrefix) {
		return sce.WithMessage(sce.ErrScorecardInternal, fmt.Sprintf("invalid URI: %v", s))
	}

	u, e := url.Parse(s)
	if e != nil {
		return sce.WithMessage(sce.ErrScorecardInternal, fmt.Sprintf("url.Parse: %v", e))
	}

	switch {
	case strings.HasPrefix(s, httpsPrefix):
		const splitLen = 2
		split := strings.SplitN(strings.Trim(u.Path, "/"), "/", splitLen)
		if len(split) != splitLen {
			return sce.WithMessage(ErrorInvalidURL, fmt.Sprintf("%v. Expected full repository url", s))
		}
		r.url.host, r.url.owner, r.url.repo = u.Host, split[0], split[1]
	case strings.HasPrefix(s, filePrefix):
		r.localDir.path = s[len(filePrefix):]
		r.repoType = RepoTypeLocalDir
	default:
		break
	}

	return nil
}

func (r *RepoURI) set(s string) error {
	var t string

	const two = 2
	const three = 3

	c := strings.Split(s, "/")

	switch l := len(c); {
	// This will takes care of repo/owner format.
	// By default it will use github.com
	case l == two:
		t = "github.com/" + c[0] + "/" + c[1]
	case l >= three:
		t = s
	}

	// Allow skipping scheme for ease-of-use, default to https.
	if !strings.Contains(t, "://") {
		t = "https://" + t
	}

	u, e := url.Parse(t)
	if e != nil {
		return sce.WithMessage(sce.ErrScorecardInternal, fmt.Sprintf("url.Parse: %v", e))
	}

	const splitLen = 2
	split := strings.SplitN(strings.Trim(u.Path, "/"), "/", splitLen)
	if len(split) != splitLen {
		return sce.WithMessage(ErrorInvalidURL, fmt.Sprintf("%v. Exepted full repository url", s))
	}

	r.url.host, r.url.owner, r.url.repo = u.Host, split[0], split[1]
	return nil
}

// Set parses a URI string into Repo struct.
func (r *RepoURI) Set(s string) error {
	var v4 bool
	_, v4 = os.LookupEnv("SCORECARD_V4")
	if v4 {
		return r.setV4(s)
	}

	return r.set(s)
}

// IsValidGitHubURL checks whether Repo represents a valid GitHub repo and returns errors otherwise.
func (r *RepoURI) IsValidGitHubURL() error {
	switch r.url.host {
	case "github.com":
		// Username may only contain alphanumeric characters or single hyphens, cannot begin or end with a hyphen and max length of 39.
		match, err := regexp.MatchString("^[a-zA-Z0-9][-a-zA-Z0-9]{0,37}[a-zA-Z0-9]$", r.url.owner)
		if !match || strings.Contains(r.url.owner, "--") || err != nil {
			return sce.WithMessage(ErrorInvalidGithubUsername, r.url.owner)
		}
	default:
		return sce.WithMessage(ErrorUnsupportedhost, r.url.host)
	}

	if strings.TrimSpace(r.url.owner) == "" || strings.TrimSpace(r.url.repo) == "" {
		return sce.WithMessage(ErrorInvalidGithubURL,
			fmt.Sprintf("%v. Expected the full repository url", r.URL()))
	}
	return nil
}
