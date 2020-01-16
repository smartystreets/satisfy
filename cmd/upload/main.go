package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"hash"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"bitbucket.org/smartystreets/satisfy/archive"
	"bitbucket.org/smartystreets/satisfy/contracts"
	"bitbucket.org/smartystreets/satisfy/core"
	"bitbucket.org/smartystreets/satisfy/fs"
	"bitbucket.org/smartystreets/satisfy/remote"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	NewApp(parseConfig()).Run()
	log.Println("OK")
}

type App struct {
	config     Config
	file       *os.File
	hasher     hash.Hash
	compressor io.WriteCloser
	builder    *core.PackageBuilder
	manifest   contracts.Manifest
	client     contracts.RemoteStorage
}

func NewApp(config Config) *App {
	return &App{config: config}
}

func (this *App) Run() {
	this.buildRemoteStorageClient()

	if this.uploadedPreviously() {
		log.Println("[INFO] Package manifest already present on remote storage. You can go about your business. Move along.")
		return
	}

	log.Println("Building the archive...")

	this.buildArchiveAndManifestContents()
	this.completeManifest()

	log.Println("Manifest:", this.dumpManifest())

	log.Println("Uploading the archive...")

	this.upload(this.buildArchiveUploadRequest())
	this.closeArchiveFile()
	this.deleteLocalArchiveFile()

	log.Println("Uploading the manifest...")

	this.upload(this.buildManifestUploadRequest())
}

func (this *App) uploadedPreviously() bool {
	if this.config.ForceUpload {
		return false
	}
	return this.remoteManifestExists()
}

func (this *App) remoteManifestExists() bool {
	request := contracts.DownloadRequest{
		Bucket:   this.config.RemoteBucket,
		Resource: this.config.composeRemotePath(remoteManifestFilename),
	}
	reader, err := this.client.Download(request)
	if err != nil {
		return false
	}
	_ = reader.Close()
	return true
}

func (this *App) buildArchiveUploadRequest() contracts.UploadRequest {
	this.openArchiveFile()
	return contracts.UploadRequest{
		Bucket:      this.config.RemoteBucket,
		Resource:    this.config.composeRemotePath(remoteArchiveFilename),
		Body:        NewFileWrapper(this.file),
		Size:        int64(this.manifest.Archive.Size),
		ContentType: "application/zstd",
		Checksum:    this.manifest.Archive.MD5Checksum,
	}
}

func (this *App) buildArchiveAndManifestContents() {
	var err error
	this.file, err = ioutil.TempFile("", "")
	if err != nil {
		log.Fatal(err)
	}
	this.hasher = md5.New()
	writer := io.MultiWriter(this.hasher, this.file)
	this.InitializeCompressor(writer)

	this.builder = core.NewPackageBuilder(
		fs.NewDiskFileSystem(this.config.SourceDirectory),
		archive.NewTarArchiveWriter(this.compressor),
		md5.New(),
	)

	err = this.builder.Build()
	if err != nil {
		log.Fatal(err)
	}

	err = this.compressor.Close()
	if err != nil {
		log.Fatal(err)
	}

	this.closeArchiveFile()
}

func (this *App) InitializeCompressor(writer io.Writer) {
	factory, found := compression[this.config.CompressionAlgorithm]
	if !found {
		log.Fatalln("Unsupported compression algorithm:", this.config.CompressionAlgorithm)
	}
	this.compressor = factory(writer, this.config.CompressionLevel)
}

func (this *App) buildManifestUploadRequest() contracts.UploadRequest {
	buffer := this.writeManifestToBuffer()
	return contracts.UploadRequest{
		Bucket:      this.config.RemoteBucket,
		Resource:    this.config.composeRemotePath(remoteManifestFilename),
		Body:        bytes.NewReader(buffer.Bytes()),
		Size:        int64(buffer.Len()),
		ContentType: "application/json",
		Checksum:    this.hasher.Sum(nil),
	}
}

func (this *App) buildRemoteStorageClient() {
	client := &http.Client{Timeout: time.Minute}
	gcsClient := remote.NewGoogleCloudStorageClient(client, this.config.GoogleCredentials)
	this.client = remote.NewRetryClient(gcsClient, this.config.MaxRetry)
}

func (this *App) completeManifest() {
	fileInfo, err := os.Stat(this.file.Name())
	if err != nil {
		log.Fatal(err)
	}
	this.manifest = contracts.Manifest{
		Name:    this.config.PackageName,
		Version: this.config.PackageVersion,
		Archive: contracts.Archive{
			Filename:             filepath.Base(this.config.composeRemotePath(remoteArchiveFilename)),
			Size:                 uint64(fileInfo.Size()),
			MD5Checksum:          this.hasher.Sum(nil),
			Contents:             this.builder.Contents(),
			CompressionAlgorithm: this.config.CompressionAlgorithm,
		},
	}
}

func (this *App) closeArchiveFile() {
	err := this.file.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func (this *App) deleteLocalArchiveFile() {
	err := os.Remove(this.file.Name())
	if err != nil {
		log.Fatal(err)
	}
}

func (this *App) openArchiveFile() {
	var err error
	this.file, err = os.Open(this.file.Name())
	if err != nil {
		log.Fatal(err)
	}
}

func (this *App) upload(request contracts.UploadRequest) {
	err := this.client.Upload(request)
	if err != nil {
		log.Fatal(err)
	}
}

func (this *App) writeManifestToBuffer() *bytes.Buffer {
	buffer := new(bytes.Buffer)
	this.hasher.Reset()
	writer := io.MultiWriter(buffer, this.hasher)
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(this.manifest)
	return buffer
}

func (this *App) dumpManifest() string {
	raw, err := json.MarshalIndent(this.manifest, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	return string(raw)
}
