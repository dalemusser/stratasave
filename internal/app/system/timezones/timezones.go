package timezones

import (
	"embed"
	"encoding/json"
	"sort"
	"sync"
)

//go:embed timezonedata/timezones.json
var FS embed.FS

type Zone struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Region string `json:"region,omitempty"`
}

type ZoneGroup struct {
	Region string
	Zones  []Zone
}

var (
	loadOnce sync.Once
	zones    []Zone
	byID     map[string]Zone
	loadErr  error

	groupsOnce sync.Once
	groups     []ZoneGroup
	groupsErr  error
)

func load() {
	loadOnce.Do(func() {
		data, err := FS.ReadFile("timezonedata/timezones.json")
		if err != nil {
			loadErr = err
			return
		}

		var list []Zone
		if err := json.Unmarshal(data, &list); err != nil {
			loadErr = err
			return
		}

		zones = list
		byID = make(map[string]Zone, len(list))
		for _, z := range list {
			byID[z.ID] = z
		}
	})
}

// Load is optional: you can call it at startup if you want to fail fast.
// It returns any error encountered loading the embedded JSON.
func Load() error {
	load()
	return loadErr
}

// All returns the curated list of zones in a stable order.
func All() ([]Zone, error) {
	load()
	if loadErr != nil {
		return nil, loadErr
	}
	return zones, nil
}

// Label returns the human-friendly label for an ID, or the ID itself if not found.
func Label(id string) string {
	load()
	if loadErr != nil {
		return id
	}
	if z, ok := byID[id]; ok && z.Label != "" {
		return z.Label
	}
	return id
}

// Valid reports whether the given ID exists in the curated list.
func Valid(id string) bool {
	load()
	if loadErr != nil {
		return false
	}
	_, ok := byID[id]
	return ok
}

func buildGroups() {
	groupsOnce.Do(func() {
		// Ensure base zones are loaded
		if err := Load(); err != nil {
			groupsErr = err
			return
		}

		byRegion := make(map[string][]Zone)
		for _, z := range zones {
			region := z.Region
			if region == "" {
				region = "Other"
			}
			byRegion[region] = append(byRegion[region], z)
		}

		// Build a slice of ZoneGroup from the map
		out := make([]ZoneGroup, 0, len(byRegion))
		for region, zs := range byRegion {
			// Optionally sort zs by label for stable ordering
			sort.SliceStable(zs, func(i, j int) bool {
				return zs[i].Label < zs[j].Label
			})
			out = append(out, ZoneGroup{
				Region: region,
				Zones:  zs,
			})
		}

		// Optionally sort groups by Region name
		sort.SliceStable(out, func(i, j int) bool {
			return out[i].Region < out[j].Region
		})

		groups = out
	})
}

// Groups returns the curated zones grouped by region. The groups and their
// contents are built lazily and cached for reuse.
func Groups() ([]ZoneGroup, error) {
	buildGroups()
	if groupsErr != nil {
		return nil, groupsErr
	}
	return groups, nil
}
