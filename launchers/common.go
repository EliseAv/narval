package launchers

import (
	"archive/tar"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

type Server interface {
	Prepare() error
	Start() error
	NumPlayers() int
	GetLinesChannel() chan ParsedLine
	SendCommand(ParsedLine) error
}

type ParsedLine struct {
	Raw     string
	Event   EventKind
	Author  User
	Message string
}

type EventKind byte

const (
	EventTalk EventKind = iota
	EventReady
	EventSaved
	EventStop
	EventJoin
	EventLeave
)

type User string

func stdinPassThrough(destination io.WriteCloser) {
	buffer := []byte{1}
	numBytes, _ := os.Stdin.Read(buffer)
	for numBytes > 0 {
		_, _ = destination.Write(buffer)
		numBytes, _ = os.Stdin.Read(buffer)
	}
	_ = os.Stdin.Close()
}

func unTar(decompressedReader io.Reader, pathPrefix string) error {
	madeDir := map[string]bool{}
	unpacked := tar.NewReader(decompressedReader)
	header, err := unpacked.Next()
	for ; err == nil; header, err = unpacked.Next() {
		if header == nil { // no one knows why this happens
			continue
		}
		relativePath := filepath.FromSlash(header.Name)
		path := filepath.Join(pathPrefix, relativePath)
		info := header.FileInfo()
		mode := info.Mode()
		switch {
		case mode.IsRegular():
			log.Printf("Extracting %s", path)
			dir := filepath.Dir(path)
			if !madeDir[dir] {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					return err
				}
				madeDir[dir] = true
			}
			writeFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode.Perm())
			if err != nil {
				return err
			}
			numBytesWritten, err := io.Copy(writeFile, unpacked)
			if closeErr := writeFile.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
			if err != nil {
				return fmt.Errorf("error writing to %s: %v", path, err)
			}
			if numBytesWritten != header.Size {
				return fmt.Errorf("only wrote %d bytes to %s; expected %d", numBytesWritten, path, header.Size)
			}
		case mode.IsDir():
			_ = os.MkdirAll(path, 0755)
			madeDir[path] = true
		}
	}
	if err == io.EOF {
		log.Print("Done!")
		return nil
	}
	return err
}

func CloseDontCare(closer io.Closer) {
	_ = closer.Close()
}
