package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/go-homedir"
)

/*
TODO
- Load manifest from backup dir if it exists
- Get file listing
- Compare to manifest to decide what needs to be downloaded (based on version)
- Download all needed - creating sanitized folder structure
- Update and save manifest

- Maybe figure out if map files can be exported? .kmz files
- Version backup in git possibly - think about total size of repo
*/

func main() {
	homeDir, err := homedir.Dir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Please set $HOME to a readable path!")
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	configDir := filepath.Join(homeDir, ".backup_google_docs")
	srv, err := NewDriveService(filepath.Join(configDir, "credentials.json"), filepath.Join(configDir, "token.json"))

	var opts struct {
		Verbose     bool   `short:"v" long:"verbose" description:"Show verbose debug information"`
		Destination string `short:"d" long:"destination" description:"Local directory to place backup files in" default:"" required:"true"`
	}

	args, err := flags.Parse(&opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if len(args) > 0 {
		fmt.Fprintln(os.Stderr, "Extra arguments provided! Did you mean to use `--destination`?")
		os.Exit(1)
	}

	destinationDir, _ := filepath.Abs(opts.Destination)
	downloader := NewDriveDownloader(srv, destinationDir)

	fmt.Printf("Backing up Google docs to %s\n", destinationDir)
	files, err := downloader.ListExportableFiles()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	// TEMP debugging info
	for i := 0; i < 25; i++ {
		file := files[i]

		converted, err := downloader.TransformDriveFile(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		fmt.Printf("%+v\n", converted)
	}

	path, err := downloader.DownloadFile(files[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Printf("Downloaded to %s\n", path)
}
