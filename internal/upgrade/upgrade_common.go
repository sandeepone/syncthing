// Copyright (C) 2014 The Syncthing Authors.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option)
// any later version.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License for
// more details.
//
// You should have received a copy of the GNU General Public License along
// with this program. If not, see <http://www.gnu.org/licenses/>.

// Package upgrade downloads and compares releases, and upgrades the running binary.
package upgrade

import (
	"errors"
	"strconv"
	"strings"

	"github.com/calmh/osext"
)

type Release struct {
	Tag        string  `json:"tag_name"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

type Asset struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

var (
	ErrVersionUpToDate    = errors.New("current version is up to date")
	ErrVersionUnknown     = errors.New("couldn't fetch release information")
	ErrUpgradeUnsupported = errors.New("upgrade unsupported")
	ErrUpgradeInProgress  = errors.New("upgrade already in progress")
	upgradeUnlocked       = make(chan bool, 1)
)

func init() {
	upgradeUnlocked <- true
}

// A wrapper around actual implementations
func To(rel Release) error {
	select {
	case <-upgradeUnlocked:
		path, err := osext.Executable()
		if err != nil {
			upgradeUnlocked <- true
			return err
		}
		err = upgradeTo(path, rel)
		// If we've failed to upgrade, unlock so that another attempt could be made
		if err != nil {
			upgradeUnlocked <- true
		}
		return err
	default:
		return ErrUpgradeInProgress
	}
}

// A wrapper around actual implementations
func ToURL(url string) error {
	select {
	case <-upgradeUnlocked:
		path, err := osext.Executable()
		if err != nil {
			upgradeUnlocked <- true
			return err
		}
		err = upgradeToURL(path, url)
		// If we've failed to upgrade, unlock so that another attempt could be made
		if err != nil {
			upgradeUnlocked <- true
		}
		return err
	default:
		return ErrUpgradeInProgress
	}
}

// Returns 1 if a>b, -1 if a<b and 0 if they are equal
func CompareVersions(a, b string) int {
	arel, apre := versionParts(a)
	brel, bpre := versionParts(b)

	minlen := len(arel)
	if l := len(brel); l < minlen {
		minlen = l
	}

	// First compare major-minor-patch versions
	for i := 0; i < minlen; i++ {
		if arel[i] < brel[i] {
			return -1
		}
		if arel[i] > brel[i] {
			return 1
		}
	}

	// Longer version is newer, when the preceding parts are equal
	if len(arel) < len(brel) {
		return -1
	}
	if len(arel) > len(brel) {
		return 1
	}

	// Prerelease versions are older, if the versions are the same
	if len(apre) == 0 && len(bpre) > 0 {
		return 1
	}
	if len(apre) > 0 && len(bpre) == 0 {
		return -1
	}

	minlen = len(apre)
	if l := len(bpre); l < minlen {
		minlen = l
	}

	// Compare prerelease strings
	for i := 0; i < minlen; i++ {
		switch av := apre[i].(type) {
		case int:
			switch bv := bpre[i].(type) {
			case int:
				if av < bv {
					return -1
				}
				if av > bv {
					return 1
				}
			case string:
				return -1
			}
		case string:
			switch bv := bpre[i].(type) {
			case int:
				return 1
			case string:
				if av < bv {
					return -1
				}
				if av > bv {
					return 1
				}
			}
		}
	}

	// If all else is equal, longer prerelease string is newer
	if len(apre) < len(bpre) {
		return -1
	}
	if len(apre) > len(bpre) {
		return 1
	}

	// Looks like they're actually the same
	return 0
}

// Split a version into parts.
// "1.2.3-beta.2" -> []int{1, 2, 3}, []interface{}{"beta", 2}
func versionParts(v string) ([]int, []interface{}) {
	if strings.HasPrefix(v, "v") || strings.HasPrefix(v, "V") {
		// Strip initial 'v' or 'V' prefix if present.
		v = v[1:]
	}
	parts := strings.SplitN(v, "+", 2)
	parts = strings.SplitN(parts[0], "-", 2)
	fields := strings.Split(parts[0], ".")

	release := make([]int, len(fields))
	for i, s := range fields {
		v, _ := strconv.Atoi(s)
		release[i] = v
	}

	var prerelease []interface{}
	if len(parts) > 1 {
		fields = strings.Split(parts[1], ".")
		prerelease = make([]interface{}, len(fields))
		for i, s := range fields {
			v, err := strconv.Atoi(s)
			if err == nil {
				prerelease[i] = v
			} else {
				prerelease[i] = s
			}
		}
	}

	return release, prerelease
}
