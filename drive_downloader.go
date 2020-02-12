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

type ExportType struct {
	ExportMIMEType      string
	ExportFileExtension string
}

var exportableMIMETypes = map[string]ExportType{
	"application/vnd.google-apps.spreadsheet": {
		ExportMIMEType:      "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		ExportFileExtension: ".xlsx",
	},
	"application/vnd.google-apps.document": {
		ExportMIMEType:      "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		ExportFileExtension: ".docx",
	},
	"application/vnd.google-apps.presentation": {
		ExportMIMEType:      "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		ExportFileExtension: ".pptx",
	},
	"application/vnd.google-apps.form": {
		ExportMIMEType:      "application/zip",
		ExportFileExtension: ".zip",
	},
}

type DriveFile struct {
	Id           string
	Name         string
	Version      int64
	Owner        string
	ParentNames  []string
	ModifiedTime time.Time
}

type DriveDownloader struct {
	service                *drive.Service
	DestinationPath        string
	supportedMIMETypeQuery string
	cachedNames            map[string][]string
}

func NewDriveDownloader(service *drive.Service, destinationPath string) *DriveDownloader {
	inst := &DriveDownloader{
		service:         service,
		DestinationPath: destinationPath,
	}
	mimeTypes := make([]string, 0, len(exportableMIMETypes))
	for k := range exportableMIMETypes {
		mimeTypes = append(mimeTypes, k)
	}
	inst.supportedMIMETypeQuery = "mimeType='" + strings.Join(mimeTypes, "' or mimeType='") + "'"

	root, err := service.Files.Get("root").Do()
	if err != nil {
		panic(err)
	}
	inst.cachedNames = map[string][]string{root.Id: {"Drive"}}

	return inst
}

func (d *DriveDownloader) ListExportableFiles() ([]*drive.File, error) {
	scannedFiles := 0
	nextPageToken := ""
	driveFiles := []*drive.File{}

	for {
		result, err := d.listAll(nextPageToken)
		if err != nil {
			return nil, err
		}

		nextPageToken = result.NextPageToken
		driveFiles = append(driveFiles, result.Files...)
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
			Q("trashed != true and " + d.supportedMIMETypeQuery).
			Do()
		return err
	}, apiRetries, time.Second*1)
	return
}

func (d *DriveDownloader) DownloadFile(file *drive.File) (string, error) {
	exportMIMEType := exportableMIMETypes[file.MimeType].ExportMIMEType
	exportFileExtension := exportableMIMETypes[file.MimeType].ExportFileExtension
	// TODO generate path with directory structure (sanitized)
	destinationPath := filepath.Join(d.DestinationPath, file.Name+exportFileExtension)

	contentResponse, err := d.service.Files.Export(file.Id, exportMIMEType).Download()
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

	return destinationPath, nil
}

func (d *DriveDownloader) TransformDriveFile(file *drive.File) (*DriveFile, error) {
	modTime, _ := time.Parse(time.RFC3339, file.ModifiedTime)
	owner := "unknown"
	if len(file.Owners) == 1 {
		if file.Owners[0].Me {
			owner = "me"
		} else {
			owner = file.Owners[0].EmailAddress
		}
	}
	if len(file.Parents) > 1 {
		return nil, fmt.Errorf("Multiple parents for file '%s'", file.Name)
	}
	parentNames := []string{"Unorganized"}
	var err error
	if len(file.Parents) == 1 {
		parentNames, err = d.GetParentNames(file.Parents[0])
		if err != nil {
			return nil, err
		}
	}

	return &DriveFile{
		Id:           file.Id,
		Name:         file.Name,
		Version:      file.Version,
		Owner:        owner,
		ParentNames:  parentNames,
		ModifiedTime: modTime,
	}, nil
}

func (d *DriveDownloader) GetParentNames(id string) ([]string, error) {
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

	parentNames, err := d.GetParentNames(file.Parents[0])
	if err != nil {
		return nil, err
	}

	names := append(parentNames, file.Name)
	d.cachedNames[id] = names
	return names, nil
}
