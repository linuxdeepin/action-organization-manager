package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v43/github"
	"golang.org/x/sync/errgroup"
)

func main() {
	var configFile string
	var appID, installationID int64
	flag.StringVar(&configFile, "f", "config.yaml", "config file")
	flag.Int64Var(&appID, "app_id", 0, "*github app id")
	flag.Int64Var(&installationID, "installation_id", 0, "*github installation id")
	flag.Parse()
	if appID == 0 || installationID == 0 {
		flag.PrintDefaults()
		return
	}

	config, err := ParseConfigFile(configFile)
	if err != nil {
		log.Fatal(err)
	}
	privateKey := []byte(os.Getenv("PRIVATE_KEY"))
	itr, err := ghinstallation.New(http.DefaultTransport, appID, installationID, []byte(privateKey))
	if err != nil {
		log.Fatal(err)
	}
	client := github.NewClient(&http.Client{Transport: itr})
	err = run(context.Background(), client, config)
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, client *github.Client, config *Config) error {
	opt := github.RepositoryListByOrgOptions{}
	for {
		ownerName := config.Organization
		repos, resp, err := client.Repositories.ListByOrg(context.Background(), ownerName, &opt)
		if err != nil {
			log.Fatal(err)
		}
		for _, repo := range repos {
			repoName := repo.GetName()
			for _, setting := range config.Settings {
				for _, repoRegexp := range setting.Repositories {
					match, err := regexp.MatchString(repoRegexp, repoName)
					if err != nil {
						return fmt.Errorf("%s match %s failed: %w", repoRegexp, repoName, err)
					}
					if !match {
						continue
					}
					log.Println(repoRegexp, "match to", repo.GetFullName())
					eg, ctx := errgroup.WithContext(ctx)
					eg.Go(func() error {
						return featuresSync(ctx, client, repo.GetFullName(), setting.Features)
					})
					listBranchOpt := github.ListOptions{}
					for {
						branches, resp, err := client.Repositories.ListBranches(context.Background(), ownerName, repoName, &listBranchOpt)
						if err != nil {
							return fmt.Errorf("list branch: %w", err)
						}
						for i := range branches {
							branch := branches[i].GetName()
							for branchRegexp := range setting.Branches {
								match, err := regexp.MatchString(branchRegexp, branch)
								if err != nil {
									return fmt.Errorf("%s match %s failed: %w", branchRegexp, branch, err)
								}
								if !match {
									continue
								}
								log.Println("\t", branchRegexp, "match to", branchRegexp)
								eg.Go(func() error {
									return branchesSync(ctx, client, ownerName, repoName, branch, setting.Branches[branchRegexp])
								})
								break
							}
						}
						if resp.NextPage == 0 {
							break
						}
						listBranchOpt.Page = resp.NextPage
					}
					err = eg.Wait()
					if err != nil {
						return err
					}
				}
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage

	}

	return nil
}

func featuresSync(ctx context.Context, client *github.Client, repo string, features Features) error {
	var r github.Repository
	if features.Issues.Enable != nil {
		r.HasIssues = features.Issues.Enable
	}
	if features.Projects.Enable != nil {
		r.HasProjects = features.Projects.Enable
	}
	if features.Wiki.Enable != nil {
		r.HasWiki = features.Wiki.Enable
	}
	r.AllowMergeCommit = features.AllowMergeCommit.Enable
	r.AllowRebaseMerge = features.AllowRebaseMerge.Enable
	r.AllowSquashMerge = features.AllowSquashMerge.Enable
	owner, repo := split(repo)
	_, _, err := client.Repositories.Edit(ctx, owner, repo, &r)
	if err != nil {
		return fmt.Errorf("edit repo: %w", err)
	}
	return nil
}
func branchesSync(ctx context.Context, client *github.Client, owner, repo string, branch string, setting Branches) error {
	var req github.ProtectionRequest

	if setting.EnforceAdmins != nil {
		req.EnforceAdmins = *setting.EnforceAdmins
	}
	if setting.DismissStaleReviews != nil {
		if req.RequiredPullRequestReviews == nil {
			req.RequiredPullRequestReviews = &github.PullRequestReviewsEnforcementRequest{}
		}
		req.RequiredPullRequestReviews.DismissStaleReviews = *setting.DismissStaleReviews
	}
	if setting.RequiredApprovingReviewCount != nil {
		if req.RequiredPullRequestReviews == nil {
			req.RequiredPullRequestReviews = &github.PullRequestReviewsEnforcementRequest{}
		}
		req.RequiredPullRequestReviews.RequiredApprovingReviewCount = *setting.RequiredApprovingReviewCount
	}

	if setting.RequiredStatusChecks.Strict != nil {
		if req.RequiredStatusChecks == nil {
			req.RequiredStatusChecks = &github.RequiredStatusChecks{Contexts: []string{}}
		}
		req.RequiredStatusChecks.Strict = *setting.RequiredStatusChecks.Strict
	}
	if setting.RequiredStatusChecks.Content != nil {
		if req.RequiredStatusChecks == nil {
			req.RequiredStatusChecks = &github.RequiredStatusChecks{}
		}
		req.RequiredStatusChecks.Contexts = setting.RequiredStatusChecks.Content
	}
	if setting.AllowForcePushes != nil {
		req.AllowForcePushes = setting.AllowForcePushes
	}
	if setting.AllowDeletions != nil {
		req.AllowDeletions = setting.AllowDeletions
	}
	_, _, err := client.Repositories.UpdateBranchProtection(ctx, owner, repo, branch, &req)
	if err != nil {
		return fmt.Errorf("update branch protection: %w", err)
	}
	return nil
}

func split(repo string) (string, string) {
	arr := strings.SplitN(repo, "/", 3)
	return arr[0], arr[1]
}
