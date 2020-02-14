package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/go-homedir"

	"google.golang.org/api/drive/v3"
)

/*
TODO
- Handle deletions since last backup
- Handle file name collisions somehow
- Parallelize - downloading can start while still listing from API - maybe make page size smaller so it can start sooner
- Improve feedback during downloading
- Output summary at end of number downloaded, skipped, deleted

Maybe:
- Copy modification times to downloaded files?
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
	err = backup(destinationDir, srv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func backup(destinationDir string, srv *drive.Service) error {
	fmt.Printf("Backing up Google docs to %s\n", destinationDir)

	downloader := NewDriveDownloader(srv, destinationDir)

	lastManifest, err := ReadBackupManifestFromDir(destinationDir)
	if err != nil {
		return err
	}
	if len(lastManifest.Entries) > 0 {
		fmt.Printf("Last backup %s\n", lastManifest.Timestamp)
	}

	manifest, err := downloader.DownloadExportableFiles(lastManifest)
	if err != nil {
		return err
	}

	err = manifest.Write(destinationDir)
	if err != nil {
		return err
	}

	return nil
}
