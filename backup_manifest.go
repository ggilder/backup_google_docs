package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

/*
TODO
- Add validation of downloads? (maybe)
*/

const manifestBasename = "manifest.json"

type BackupManifest struct {
	Timestamp time.Time
	Entries   map[string]BackupManifestEntry
}

type BackupManifestEntry struct {
	Id             string
	Name           string
	Version        int64
	Owner          string
	ParentNames    [][]string
	ModifiedTime   time.Time
	DownloadedTime time.Time
	DownloadPath   string
}

func NewBackupManifest() *BackupManifest {
	return &BackupManifest{Timestamp: time.Now(), Entries: make(map[string]BackupManifestEntry)}
}

func ReadBackupManifestFromDir(dir string) (*BackupManifest, error) {
	path := filepath.Join(dir, manifestBasename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return NewBackupManifest(), nil
	}

	jsonBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var m BackupManifest
	err = json.Unmarshal(jsonBytes, &m)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

func (m *BackupManifest) AddEntry(file *DriveFile, downloadPath string) {
	m.Entries[file.Id] = BackupManifestEntry{
		Id:             file.Id,
		Name:           file.Name,
		Version:        file.Version,
		Owner:          file.Owner,
		ParentNames:    file.ParentNames,
		ModifiedTime:   file.ModifiedTime,
		DownloadedTime: time.Now(),
		DownloadPath:   downloadPath,
	}
}

func (m *BackupManifest) CopyEntry(lastManifest *BackupManifest, file *DriveFile) {
	m.Entries[file.Id] = lastManifest.Entries[file.Id]
}

func (m *BackupManifest) AlreadyDownloaded(file *DriveFile) bool {
	if entry, ok := m.Entries[file.Id]; ok {
		if entry.Version == file.Version {
			return true
		}
	}
	return false
}

func (m *BackupManifest) Write(dir string) error {
	path := filepath.Join(dir, manifestBasename)
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path, jsonBytes, 0644)
	if err != nil {
		return err
	}
	return nil
}
