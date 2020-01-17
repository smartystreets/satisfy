package main

import (
	"log"
	"net/http"
	"time"

	"bitbucket.org/smartystreets/satisfy/cmd"
	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/remote"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	NewApp(cmd.ParseConfig()).Run()
}

type App struct {
	config cmd.Config
	client contracts.RemoteStorage
}

func NewApp(config cmd.Config) *App {
	return &App{config: config}
}

func (this *App) Run() {
	// TODO: public contract, e.g. exit code 0 + output vs exit 1 + output
	if this.uploadedPreviously(cmd.RemoteManifestFilename) {
		log.Fatal("[INFO] Package manifest already present on remote storage. You can go about your business. Move along.")
	}
}

func (this *App) uploadedPreviously(path string) bool {
	this.buildRemoteStorageClient()

	request := contracts.DownloadRequest{
		Bucket:   this.config.RemoteBucket,
		Resource: this.config.ComposeRemotePath(path),
	}
	_, err := this.client.Download(request)
	return err != nil
}

func (this *App) buildRemoteStorageClient() {
	// TODO: using a clean http.Client and Transport
	client := &http.Client{Timeout: time.Minute}
	gcsClient := remote.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials, http.StatusNotFound)
	this.client = remote.NewRetryClient(gcsClient, this.config.MaxRetry)
}
