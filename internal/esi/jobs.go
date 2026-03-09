package esi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Job represents an active or ready research/copying job from ESI.
// Only jobs with status "active" or "ready" and a known research activity are returned.
type Job struct {
	JobID       int64
	BlueprintID int64
	InstallerID int64
	Activity    string // "me_research" | "te_research" | "copying"
	Status      string // "active" | "ready"
	StartDate   time.Time
	EndDate     time.Time
}

// activityNames maps ESI activity_id values to internal activity strings.
// Only research and copying activities are supported; manufacturing and others are skipped.
//
// EVE Online activity IDs:
//
//	1  = Manufacturing
//	3  = Researching Time Efficiency  → "te_research"
//	4  = Researching Material Efficiency → "me_research"
//	5  = Copying                      → "copying"
//	8  = Invention
//	11 = Reactions
var activityNames = map[int]string{
	3: "te_research",
	4: "me_research",
	5: "copying",
}

// esiJobItem is the raw JSON shape returned by ESI.
type esiJobItem struct {
	JobID       int64     `json:"job_id"`
	BlueprintID int64     `json:"blueprint_id"`
	InstallerID int64     `json:"installer_id"`
	ActivityID  int       `json:"activity_id"`
	Status      string    `json:"status"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
}

// GetCharacterJobs fetches active and ready research/copying jobs for characterID.
func (c *httpClient) GetCharacterJobs(ctx context.Context, characterID int64, token string) ([]Job, time.Time, error) {
	url := fmt.Sprintf("%s/characters/%d/industry/jobs", c.baseURL, characterID)
	return c.fetchAllJobs(ctx, url, token)
}

// GetCorporationJobs fetches active and ready research/copying jobs for corporationID.
// token must belong to a character with director roles in the corporation.
func (c *httpClient) GetCorporationJobs(ctx context.Context, corporationID int64, token string) ([]Job, time.Time, error) {
	url := fmt.Sprintf("%s/corporations/%d/industry/jobs", c.baseURL, corporationID)
	return c.fetchAllJobs(ctx, url, token)
}

// fetchAllJobs fetches all pages of jobs from url and returns filtered results.
// cacheUntil is taken from the first page response.
func (c *httpClient) fetchAllJobs(ctx context.Context, url, token string) ([]Job, time.Time, error) {
	body, headers, cacheUntil, err := c.doWithHeader(ctx, url, token)
	if err != nil {
		return nil, cacheUntil, err
	}
	var allRaw []esiJobItem
	if err := json.Unmarshal(body, &allRaw); err != nil {
		return nil, cacheUntil, fmt.Errorf("parsing jobs response: %w", err)
	}
	totalPages := parseXPages(headers.Get("X-Pages"))
	for page := 2; page <= totalPages; page++ {
		pageURL := fmt.Sprintf("%s?page=%d", url, page)
		pageBody, _, _, pageErr := c.doWithHeader(ctx, pageURL, token)
		if pageErr != nil {
			return nil, cacheUntil, pageErr
		}
		var pageItems []esiJobItem
		if err := json.Unmarshal(pageBody, &pageItems); err != nil {
			return nil, cacheUntil, fmt.Errorf("parsing jobs page %d: %w", page, err)
		}
		allRaw = append(allRaw, pageItems...)
	}
	return filterJobs(allRaw), cacheUntil, nil
}

func filterJobs(raw []esiJobItem) []Job {
	jobs := make([]Job, 0, len(raw))
	for _, item := range raw {
		// Filter: only active and ready statuses.
		if item.Status != "active" && item.Status != "ready" {
			continue
		}
		// Filter: only research and copying activities; skip manufacturing etc.
		activity, ok := activityNames[item.ActivityID]
		if !ok {
			continue
		}
		jobs = append(jobs, Job{
			JobID:       item.JobID,
			BlueprintID: item.BlueprintID,
			InstallerID: item.InstallerID,
			Activity:    activity,
			Status:      item.Status,
			StartDate:   item.StartDate,
			EndDate:     item.EndDate,
		})
	}
	return jobs
}
