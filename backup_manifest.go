package main

import (
	"time"
)

// TODO add JSON read/write functions
// And maybe "needs backup" function
// Validation of downloads?
// Should this include sanitized path?

type BackupManifest struct {
	Timestamp time.Time
	Entries   map[string]BackupManifestEntry
}

type BackupManifestEntry struct {
	Id             string
	Name           string
	Version        int64
	Owner          string
	ParentNames    []string
	ModifiedTime   time.Time
	DownloadedTime time.Time
}
