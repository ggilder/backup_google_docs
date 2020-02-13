package main

import (
	"path/filepath"
	"regexp"
	"time"
)

type DownloadType struct {
	DownloadMimeType      string
	DownloadFileExtension string
}

var exportableMimeTypes = map[string]DownloadType{
	"application/vnd.google-apps.spreadsheet": {
		DownloadMimeType:      "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		DownloadFileExtension: ".xlsx",
	},
	"application/vnd.google-apps.document": {
		DownloadMimeType:      "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		DownloadFileExtension: ".docx",
	},
	"application/vnd.google-apps.presentation": {
		DownloadMimeType:      "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		DownloadFileExtension: ".pptx",
	},
	"application/vnd.google-apps.form": {
		DownloadMimeType:      "application/zip",
		DownloadFileExtension: ".zip",
	},
}

type DriveFile struct {
	Id           string
	Name         string
	Version      int64
	Owner        string
	ParentNames  [][]string
	ModifiedTime time.Time
	MimeType     string
}

func (df *DriveFile) SanitizedDownloadPath() string {
	parents := df.ParentNames[0]
	path := ""
	for _, name := range parents {
		path = filepath.Join(path, sanitizePart(name))
	}
	path = filepath.Join(path, sanitizePart(df.Name)+df.DownloadFileExtension())
	return path
}

func (df *DriveFile) DownloadMimeType() string {
	return exportableMimeTypes[df.MimeType].DownloadMimeType
}

func (df *DriveFile) DownloadFileExtension() string {
	return exportableMimeTypes[df.MimeType].DownloadFileExtension
}

var illegalPathChars = regexp.MustCompile(`[/\\:\0]`)

func sanitizePart(part string) string {
	if part == "." {
		return "_"
	} else if part == ".." {
		return "__"
	}
	return illegalPathChars.ReplaceAllString(part, "_")
}
