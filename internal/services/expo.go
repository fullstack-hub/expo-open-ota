// expo.go used to be the api.expo.dev GraphQL client (see the upstream
// expo-open-ota project). This build replaces every function body with a
// fully self-hosted equivalent, keeping every exported name and signature so
// no call site changes:
//
//   - publisher auth   : per-app publishTokens (opaque secrets) in config
//   - channel mapping  : per-app channels map in config, fallback channel==branch
//   - branch registry  : none — branches are bucket prefixes created on upload
//   - usernames        : the appId itself (feeds the upload-token JWT sub)
//
// Going back to the EAS-backed integration = git revert of this change.
package services

import (
	"crypto/subtle"
	"errors"
	"expo-open-ota/config"
	"expo-open-ota/internal/types"
	"expo-open-ota/internal/version"
	"fmt"
	"sort"
)

type ExpoUserAccount struct {
	Id       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type ExpoChannelMapping struct {
	Id         string `json:"id"`
	BranchName string `json:"branchName"`
}

type ExpoBranchMapping struct {
	BranchName  string  `json:"branchName"`
	BranchId    string  `json:"branchId"`
	ChannelName *string `json:"channelName"`
}

type ExpoChannel struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	BranchId string `json:"branchId"`
}

// ValidateExpoAuth authenticates publish calls (upload/rollback/republish and
// the dashboard Use-Expo-Auth relay) against the app's publishTokens. The
// check is per-app, so a token for app A can never publish to app B.
//
// Tokens are opaque to the server: the incoming token is compared verbatim
// against the configured entries, so how a token was generated (random,
// shared secret, derived) is entirely the operator's choice. Auth strength
// is the token's entropy — EXPO_APPS_JSON already carries the code-signing
// key config, so it is treated as a secret of that grade regardless.
func ValidateExpoAuth(appId string, expoAuth types.ExpoAuth) (*ExpoUserAccount, error) {
	if expoAuth.SessionSecret != nil {
		return nil, errors.New("expo-session auth is not supported; use a publish token")
	}
	if expoAuth.Token == nil || *expoAuth.Token == "" {
		return nil, errors.New("no publish token provided")
	}
	app, err := config.GetAppConfig(appId)
	if err != nil {
		return nil, err
	}
	for _, candidate := range app.PublishTokens {
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(*expoAuth.Token)) == 1 {
			return &ExpoUserAccount{Id: appId, Username: appId}, nil
		}
	}
	return nil, errors.New("invalid publish token")
}

// FetchSelfExpoUsername feeds the upload-token JWT sub claim in localBucket;
// returning the appId keeps issuance and validation consistent without any
// external lookup.
func FetchSelfExpoUsername(appId string) string {
	return appId
}

// ComputeChannelMappingCacheKey is kept because the dashboard handler deletes
// this key after mapping updates. Mapping resolution itself no longer uses the
// cache (config lives in memory), so the delete is a harmless no-op.
func ComputeChannelMappingCacheKey(appId, channelName string) string {
	return fmt.Sprintf("channelMapping:%s:%s:%s", version.Version, appId, channelName)
}

// FetchExpoChannelMapping resolves a client channel to a branch: the app's
// channels map first, then the EAS convention fallback channel==branch.
func FetchExpoChannelMapping(appId, channelName string) (*ExpoChannelMapping, error) {
	if channelName == "" {
		return nil, errors.New("no channel name provided")
	}
	app, err := config.GetAppConfig(appId)
	if err != nil {
		return nil, err
	}
	branch := app.Channels[channelName]
	if branch == "" {
		branch = channelName
	}
	return &ExpoChannelMapping{Id: channelName, BranchName: branch}, nil
}

func FetchExpoChannels(appId string) ([]ExpoChannel, error) {
	app, err := config.GetAppConfig(appId)
	if err != nil {
		return nil, err
	}
	channels := make([]ExpoChannel, 0, len(app.Channels))
	for name, branch := range app.Channels {
		channels = append(channels, ExpoChannel{Id: name, Name: name, BranchId: branch})
	}
	sort.Slice(channels, func(i, j int) bool { return channels[i].Name < channels[j].Name })
	return channels, nil
}

// FetchExpoBranches returns nothing: its only caller is branch.UpsertBranch,
// which then calls CreateBranch (a no-op) — branches are implicit bucket
// prefixes created on first upload. The dashboard lists branches straight
// from the bucket (GetBranchesHandler), not through this function.
func FetchExpoBranches(appId string) ([]string, error) {
	if _, err := config.GetAppConfig(appId); err != nil {
		return nil, err
	}
	return nil, nil
}

// FetchExpoBranchesMapping reports which branches are pointed at by a
// configured channel. Branch ids ARE branch names here. The dashboard merges
// this with the bucket's branch list, so branches without a channel still
// show up (with null branchId/releaseChannel).
func FetchExpoBranchesMapping(appId string) ([]ExpoBranchMapping, error) {
	app, err := config.GetAppConfig(appId)
	if err != nil {
		return nil, err
	}
	branchToChannels := make(map[string][]string)
	for channelName, branchName := range app.Channels {
		branchToChannels[branchName] = append(branchToChannels[branchName], channelName)
	}
	branches := make([]string, 0, len(branchToChannels))
	for branch := range branchToChannels {
		branches = append(branches, branch)
	}
	sort.Strings(branches)
	var result []ExpoBranchMapping
	for _, branch := range branches {
		channelNames := branchToChannels[branch]
		sort.Strings(channelNames)
		for _, channelName := range channelNames {
			cn := channelName
			result = append(result, ExpoBranchMapping{BranchName: branch, BranchId: branch, ChannelName: &cn})
		}
	}
	return result, nil
}

// CreateBranch is a no-op: there is no external registry to sync.
func CreateBranch(appId, branch string) error {
	return nil
}

// UpdateChannelBranchMapping rejects dashboard edits: the mapping is part of
// EXPO_APPS_JSON, so changing it is a config change + redeploy, not an API
// call. Returning an error surfaces a clear 500 in the dashboard instead of
// silently dropping the edit.
func UpdateChannelBranchMapping(appId, channelName, branchId string) error {
	return errors.New("channel mapping is managed via EXPO_APPS_JSON (channels map); update the config and redeploy")
}
