package main

import (
	"strconv"

	"github.com/google/go-github/github"
)

// Check if an error is a rate limit error
func (p PRMirror) isRatelimit(err error) bool {
	if _, ok := err.(*github.RateLimitError); ok {
		// TODO: Maybe add some context here
		log.Error("The github.com rate limit has been hit")
		return true
	}
	return false
}

// GetRepoEvents returns a list a list of RepoEvents
func (p PRMirror) GetRepoEvents() ([]*github.Event, int64, error) {
	var allEvents []*github.Event
	var pollInterval = int64(0)

	opt := &github.ListOptions{
		PerPage: 100,
	}

	for {
		log.Debugf("Getting RepoEvents Page %d\n", opt.Page)

		events, resp, err := p.GitHubClient.Activity.ListRepositoryEvents(*p.Context, p.Configuration.UpstreamOwner, p.Configuration.UpstreamRepo, opt)
		if err != nil {
			log.Errorf("Error while listing repository events. %s", err.Error())
			return nil, 60, err
		}

		allEvents = append(allEvents, events...)
		if resp.NextPage == 0 {
			pollInterval, err = strconv.ParseInt(resp.Response.Header.Get("X-Poll-Interval"), 10, 64)
			if err != nil {
				panic(err)
			}
			break
		}
		opt.Page = resp.NextPage
	}

	return allEvents, pollInterval, nil
}

// GetDownstreamRepoEvents returns a list a list of RepoEvents, but for downstream
func (p PRMirror) GetDownstreamRepoEvents() ([]*github.Event, int64, error) {
	var allEvents []*github.Event
	var pollInterval = int64(0)

	opt := &github.ListOptions{
		PerPage: 100,
	}

	for {
		log.Debugf("Getting Downstream RepoEvents Page %d\n", opt.Page)

		events, resp, err := p.GitHubClient.Activity.ListRepositoryEvents(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamRepo, opt)
		if err != nil {
			log.Errorf("Error while listing repository events. %s", err.Error())
			return nil, 60, err
		}

		allEvents = append(allEvents, events...)
		if resp.NextPage == 0 {
			pollInterval, err = strconv.ParseInt(resp.Response.Header.Get("X-Poll-Interval"), 10, 64)
			if err != nil {
				panic(err)
			}
			break
		}
		opt.Page = resp.NextPage
	}

	return allEvents, pollInterval, nil
}

// CreateLabel creates a new label
func (p PRMirror) CreateLabel(labelText string, labelColour string) bool {
	label := github.Label{
		Name:  &labelText,
		Color: &labelColour,
	}

	_, _, err := p.GitHubClient.Issues.CreateLabel(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamRepo, &label)
	if err != nil {
		log.Errorf("Error while creating a label - %s", err.Error())
		return false
	}

	return true
}

// AddLabels adds label/s to a pull request
func (p PRMirror) AddLabels(id int, labels []string) bool {
	_, _, err := p.GitHubClient.Issues.AddLabelsToIssue(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamRepo, id, labels)
	if err != nil {
		log.Errorf("Error while adding a label on issue#:%d - %s", id, err.Error())
		return false
	}

	return true
}

// RemoveLabel Removes a label from a pull request
func (p PRMirror) RemoveLabel(id int, labels string) bool {
	_, err := p.GitHubClient.Issues.RemoveLabelForIssue(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamRepo, id, labels)
	if err != nil {
		log.Errorf("Error while removing a label on issue#:%d - %s", id, err.Error())
		return false
	}

	return true
}

// AddComment adds a comment to a pull request
func (p PRMirror) AddComment(id int, comment string) bool {
	issueComment := github.IssueComment{}
	issueComment.Body = &comment

	_, _, err := p.GitHubClient.Issues.CreateComment(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamRepo, id, &issueComment)
	if err != nil {
		log.Errorf("Error while adding a comment to issue#:%d - %s", id, err.Error())
		return false
	}

	return true
}
