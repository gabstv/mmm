package mtime

import (
	"sync"
	"time"
)

var (
	locMu    sync.RWMutex
	locByID  []*time.Location
	idByName map[string]uint16
)

func init() {
	idByName = make(map[string]uint16, 64)
	// UTC = 0, Local = 1
	locByID = append(locByID, time.UTC)
	idByName[time.UTC.String()] = 0
	locByID = append(locByID, time.Local)
	idByName[time.Local.String()] = 1
}

const maxLocations = 1<<16 - 1 // 65535

func registerLocation(loc *time.Location) uint16 {
	if loc == nil {
		return 0 // treat nil as UTC
	}
	name := loc.String()

	locMu.RLock()
	id, ok := idByName[name]
	locMu.RUnlock()
	if ok {
		return id
	}

	locMu.Lock()
	defer locMu.Unlock()
	// Double-check after acquiring write lock.
	if id, ok := idByName[name]; ok {
		return id
	}
	if len(locByID) > maxLocations {
		panic("mtime: timezone registry overflow (>65535 unique locations)")
	}
	id = uint16(len(locByID))
	locByID = append(locByID, loc)
	idByName[name] = id
	return id
}

func resolveLocation(id uint16) *time.Location {
	locMu.RLock()
	defer locMu.RUnlock()
	if int(id) >= len(locByID) {
		return time.UTC
	}
	return locByID[id]
}
