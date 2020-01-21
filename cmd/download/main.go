package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"

	"bitbucket.org/smartystreets/satisfy/cmd"
	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/core"
	"bitbucket.org/smartystreets/satisfy/shell"
)

func main() {
	config := parseConfig()

	decoder := json.NewDecoder(os.Stdin)
	var listing cmd.DependencyListing
	err := decoder.Decode(&listing)
	if err != nil {
		log.Fatal(err)
	}

	err = listing.Validate()
	if err != nil {
		log.Fatal(err)
	}

	working, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	disk := shell.NewDiskFileSystem(working)
	client := shell.NewGoogleCloudStorageClient(cmd.NewHTTPClient(), config.GoogleCredentials, http.StatusOK)
	installer := core.NewPackageInstaller(core.NewRetryClient(client, config.MaxRetry), disk)
	integrity := core.NewCompoundIntegrityCheck(
		core.NewFileListingIntegrityChecker(disk),
		core.NewFileContentIntegrityCheck(md5.New(), disk, config.Verify),
	)

	app := NewApp(listing, installer, integrity)
	app.Run()
}

type App struct {
	listing   cmd.DependencyListing
	installer *core.PackageInstaller
	integrity contracts.IntegrityCheck
	waiter    *sync.WaitGroup
	results   chan error
}

func NewApp(listing cmd.DependencyListing, installer *core.PackageInstaller, integrity contracts.IntegrityCheck) *App {
	waiter := new(sync.WaitGroup)
	waiter.Add(len(listing.Dependencies))
	results := make(chan error)
	return &App{listing: listing, installer: installer, integrity: integrity, waiter: waiter, results: results}
}

func (this *App) Run() {
	for _, dependency := range this.listing.Dependencies {
		go this.install(dependency)
	}
	go this.awaitCompletion()
	var failed int
	for err := range this.results {
		failed++
		log.Println("[WARN]", err)
	}
	os.Exit(failed)
}

func (this *App) awaitCompletion() {
	this.waiter.Wait()
	close(this.results)
}

func (this *App) install(dependency cmd.Dependency) {
	defer this.waiter.Done()
	manifest, err := loadManifest(dependency)
	if err == nil && manifest.Version == dependency.Version && this.integrity.Verify(manifest) == nil {
		return
	}
	installation := contracts.InstallationRequest{LocalPath: dependency.LocalDirectory}

	installation.RemoteAddress = dependency.ComposeRemoteAddress(cmd.RemoteManifestFilename)
	manifest, err = this.installer.InstallManifest(installation)
	if err != nil {
		this.results <- fmt.Errorf("failed to install manifest for %s: %v", dependency.Name, err)
		return
	}

	installation.RemoteAddress = dependency.ComposeRemoteAddress(cmd.RemoteArchiveFilename)
	err = this.installer.InstallPackage(manifest, installation)
	if err != nil {
		this.results <- fmt.Errorf("[WARN] Failed to install package for %s: %v", dependency.Name, err)
	}
}

func loadManifest(dependency cmd.Dependency) (manifest contracts.Manifest, err error) {
	path := core.ComposeManifestPath(dependency.LocalDirectory, dependency.Name)

	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		return manifest, errNotInstalled
	}

	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return manifest, err
	}

	err = json.Unmarshal(raw, &manifest)
	if err != nil {
		return manifest, err
	}

	return manifest, nil
}

var (
	errNotInstalled = errors.New("package not yet installed")
)
