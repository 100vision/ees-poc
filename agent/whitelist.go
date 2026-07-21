package main

import (
	"encoding/json"
	"fmt"
	"os"

	"ees-demo/common/constants"
	"ees-demo/common/log"
	"ees-demo/common/types"
)

// whitelistEntry represents a single allowed program in the whitelist.
type whitelistEntry struct {
	SHA256      string `json:"SHA256"`
	Publisher   string `json:"Publisher"`
	Description string `json:"Description"`
	Enabled     bool   `json:"Enabled"`
}

// whitelistData is the top-level structure of whitelist.json.
type whitelistData struct {
	Entries []whitelistEntry `json:"entries"`
}

// whitelist holds the loaded allow-list and provides matching logic.
type whitelist struct {
	entries []whitelistEntry
	logger  *log.Logger
}

// loadWhitelist reads and parses the whitelist JSON file.
func loadWhitelist(path string, logger *log.Logger) (*whitelist, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read whitelist: %w", err)
	}

	var wl whitelistData
	if err := json.Unmarshal(data, &wl); err != nil {
		return nil, fmt.Errorf("parse whitelist: %w", err)
	}

	if logger != nil {
		logger.Info("Whitelist loaded: %d entries", len(wl.Entries))
	}

	return &whitelist{entries: wl.Entries, logger: logger}, nil
}

// match checks if a file's SHA256 and Publisher match any whitelist entry.
// Returns the matching entry and a boolean. Matching rules:
//   - SHA256 is compared first (exact match)
//   - If SHA256 doesn't match, Publisher is compared (exact match)
//   - Both checks must pass for a match (unless a field is empty in the whitelist)
func (wl *whitelist) match(v *verifyResult) *whitelistEntry {
	for _, entry := range wl.entries {
		if !entry.Enabled {
			continue
		}

		shaMatch := false
		pubMatch := false

		// SHA256 check: if whitelist has a hash, it must match
		if entry.SHA256 == "" || entry.SHA256 == v.SHA256 {
			shaMatch = true
		}

		// Publisher check: if whitelist has a publisher, it must match
		if entry.Publisher == "" || entry.Publisher == v.Publisher {
			pubMatch = true
		}

		if shaMatch && pubMatch {
			return &entry
		}
	}
	return nil
}

// decide processes verification results against the whitelist and returns
// an Allow/Deny response with a descriptive message.
func (wl *whitelist) decide(v *verifyResult) *types.Response {
	if wl == nil || wl.entries == nil {
		return &types.Response{
			Result:  constants.ResultError,
			Message: "Whitelist not loaded",
		}
	}

	entry := wl.match(v)

	if wl.logger != nil {
		if entry != nil {
			wl.logger.Info("Allow: %s (%s)", entry.Description, entry.Publisher)
		} else {
			wl.logger.Info("Deny: no matching whitelist entry")
		}
	}

	if entry != nil {
		return &types.Response{
			Result:  constants.ResultAllow,
			Message: fmt.Sprintf("Elevation Successful — %s", entry.Description),
		}
	}

	return &types.Response{
		Result:  constants.ResultDeny,
		Message: "Application Not Approved — no matching whitelist entry",
	}
}
