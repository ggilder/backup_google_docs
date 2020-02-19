package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/api/drive/v3"

	"github.com/rafaeljesus/retry-go"
)

const apiRetries int = 10

type DriveDownloader struct {
	service                *drive.Service
	DestinationPath        string
	supportedMimeTypeQuery string
	cachedNames            map[string][]string
}

func NewDriveDownloader(service *drive.Service, destinationPath string) *DriveDownloader {
	inst := &DriveDownloader{
		service:         service,
		DestinationPath: destinationPath,
	}
	mimeTypes := make([]string, 0, len(exportableMimeTypes))
	for k := range exportableMimeTypes {
		mimeTypes = append(mimeTypes, k)
	}
	inst.supportedMimeTypeQuery = "mimeType='" + strings.Join(mimeTypes, "' or mimeType='") + "'"

	root, err := service.Files.Get("root").Do()
	if err != nil {
		panic(err)
	}
	inst.cachedNames = map[string][]string{root.Id: {"Drive"}}

	return inst
}

func (d *DriveDownloader) DownloadExportableFiles(lastManifest *BackupManifest) (*BackupManifest, error) {
	files, err := d.ListExportableFiles()
	if err != nil {
		return nil, err
	}

	manifest := NewBackupManifest()
	for _, file := range files {
		filePath := file.SanitizedDownloadPath()
		// TODO feedback channel here
		if lastManifest.AlreadyDownloaded(file) {
			fmt.Printf("Skipping %s - unchanged from last backup\n", filePath)
			manifest.CopyEntry(lastManifest, file)
			continue
		}
		// Download/export
		fmt.Printf("Downloading %s\n", filePath)
		relPath, err := d.DownloadFile(file)
		if err != nil {
			return manifest, err
		}
		manifest.AddEntry(file, relPath)
	}
	return manifest, nil
}

func (d *DriveDownloader) ListExportableFiles() ([]*DriveFile, error) {
	scannedFiles := 0
	nextPageToken := ""
	driveFiles := []*DriveFile{}

	for {
		result, err := d.listAll(nextPageToken)
		if err != nil {
			return nil, err
		}

		nextPageToken = result.NextPageToken
		for _, file := range result.Files {
			converted, err := d.transformDriveFile(file)
			if err != nil {
				return nil, err
			}
			driveFiles = append(driveFiles, converted)
		}
		scannedFiles += len(result.Files)
		// TODO feedback channel
		fmt.Fprintf(os.Stderr, "Listing %d files\r", scannedFiles)

		if nextPageToken == "" {
			break
		}
	}

	// TODO send feedback via a channel
	fmt.Printf("\n\nScanned %d files.\n", len(driveFiles))
	return driveFiles, nil
}

func (d *DriveDownloader) listAll(nextPageToken string) (result *drive.FileList, err error) {
	err = retry.Do(func() error {
		result, err = d.service.Files.List().
			PageToken(nextPageToken).
			PageSize(1000).
			Fields("nextPageToken, files(id, name, parents, owners, trashed, version, mimeType, modifiedTime)").
			Q("trashed != true and " + d.supportedMimeTypeQuery).
			Do()
		return err
	}, apiRetries, time.Second*1)
	return
}

func (d *DriveDownloader) DownloadFile(file *DriveFile) (string, error) {
	downloadMimeType := file.DownloadMimeType()
	destinationPathRel := file.SanitizedDownloadPath()
	destinationPath := filepath.Join(d.DestinationPath, destinationPathRel)

	err := os.MkdirAll(filepath.Dir(destinationPath), 0755)
	if err != nil {
		return "", err
	}

	contentResponse, err := d.service.Files.Export(file.Id, downloadMimeType).Download()
	if err != nil {
		return "", err
	}
	defer contentResponse.Body.Close()

	body, err := ioutil.ReadAll(contentResponse.Body)
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile(destinationPath, body, 0644)
	if err != nil {
		return "", err
	}

	return destinationPathRel, nil
}

func (d *DriveDownloader) transformDriveFile(file *drive.File) (*DriveFile, error) {
	modTime, _ := time.Parse(time.RFC3339, file.ModifiedTime)
	owner := "unknown"
	if len(file.Owners) == 1 {
		if file.Owners[0].Me {
			owner = "me"
		} else {
			owner = file.Owners[0].EmailAddress
		}
	}
	parentNames := [][]string{{"Unorganized"}}
	if len(file.Parents) > 0 {
		parentNames = [][]string{}
		for _, parent := range file.Parents {
			names, err := d.getParentNames(parent)
			if err != nil {
				return nil, err
			}
			parentNames = append(parentNames, names)
		}
	}

	return &DriveFile{
		Id:           file.Id,
		Name:         file.Name,
		Version:      file.Version,
		Owner:        owner,
		ParentNames:  parentNames,
		ModifiedTime: modTime,
		MimeType:     file.MimeType,
	}, nil
}

func (d *DriveDownloader) getParentNames(id string) ([]string, error) {
	if cached, ok := d.cachedNames[id]; ok {
		return cached, nil
	}

	file, err := d.service.Files.Get(id).Fields("id, name, parents, trashed").Do()
	if err != nil {
		return nil, err
	}
	if len(file.Parents) > 1 {
		return nil, fmt.Errorf("Multiple parents for file '%s'", file.Name)
	}

	if len(file.Parents) == 0 {
		return []string{"Unorganized", file.Name}, nil
	}

	parentNames, err := d.getParentNames(file.Parents[0])
	if err != nil {
		return nil, err
	}

	names := append(parentNames, file.Name)
	d.cachedNames[id] = names
	return names, nil
}
