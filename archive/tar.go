package archive

import (
	"archive/tar"
	"io"
	"log"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type TarArchiveWriter struct {
	*tar.Writer
}

func NewTarArchiveWriter(writer io.Writer) *TarArchiveWriter {
	return &TarArchiveWriter{Writer: tar.NewWriter(writer)}
}

func (this *TarArchiveWriter) WriteHeader(header contracts.ArchiveHeader) {
	tarHeader := &tar.Header{
		Name:    header.Name,
		Size:    header.Size,
		ModTime: header.ModTime,
		Mode:    0644,
	}
	err := this.Writer.WriteHeader(tarHeader)
	if err != nil {
		log.Panic(err)
	}
}
