package main

import (
	"archive/zip"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Bowery/gopackages/aws"
	"github.com/Bowery/gopackages/config"
)

var cmds = map[string]func(...string) error{"zip": zipsCmd, "aws": awsCmd}

func init() {
	// Let insecure ssl go through, s3 has trouble routing the certificate if bucket contains periods.
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: util <zip|aws> [arguments]")
		os.Exit(1)
	}

	cmd, ok := cmds[os.Args[1]]
	if !ok {
		fmt.Fprintln(os.Stderr, "Cmd", os.Args[1], "not found.")
		os.Exit(1)
	}

	err := cmd(os.Args[2:]...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// zipsCmd takes a direcory and writes the contents to a destination.
func zipsCmd(args ...string) error {
	if len(args) < 2 {
		return errors.New("Usage: util zip <source\\dir> <output>")
	}

	output, err := os.Create(args[1])
	if err != nil {
		return err
	}
	defer output.Close()
	zipWriter := zip.NewWriter(output)
	defer zipWriter.Close()

	// Walk the tree and copy files to the zip writer.
	return filepath.Walk(args[0], func(path string, info os.FileInfo, err error) error {
		if err != nil || args[0] == path || info.IsDir() {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Get the relative path, and convert separators to /.
		relPath, err := filepath.Rel(args[0], path)
		if err != nil {
			return err
		}
		relPath = strings.Replace(relPath, string(filepath.Separator), "/", -1)
		header.Name = relPath

		partWriter, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		source, err := os.Open(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(partWriter, source)
		if err != nil {
			return err
		}

		return source.Close()
	})
}

// awsCmd takes a path and uploads its contents to aws, if a directory is given
// the directories children are uploaded in parallel.
func awsCmd(args ...string) error {
	var (
		done        = make(chan error, 1)
		contentType = "application/octet-stream"
		perm        = "public-read"
		wg          sync.WaitGroup
	)
	if len(args) < 2 {
		return errors.New("Usage: util aws <bucket> <path>")
	}
	bucket := args[0]

	client, err := aws.NewClient(config.S3AccessKey, config.S3SecretKey)
	if err != nil {
		return err
	}

	stat, err := os.Stat(args[1])
	if err != nil {
		return err
	}

	// If the path isn't a directory just upload it.
	if !stat.IsDir() {
		fmt.Println("Uploading", args[1], "to", filepath.Base(args[1]))
		return client.PutFile(bucket, filepath.Base(args[1]), args[1], contentType, perm)
	}

	dir, err := os.Open(args[1])
	if err != nil {
		return err
	}
	defer dir.Close()

	stats, err := dir.Readdir(0)
	if err != nil {
		return err
	}

	// Loop stats and start uploads in parallel.
	for _, stat := range stats {
		wg.Add(1)

		go func(info os.FileInfo) {
			defer wg.Done()
			path := filepath.Join(args[1], info.Name())

			err := client.PutFile(bucket, info.Name(), path, contentType, perm)
			if err != nil {
				done <- err
			}
			fmt.Println("Uploaded", path, "to", info.Name())
		}(stat)
	}

	// Wait and then signal done.
	go func() {
		wg.Wait()
		done <- nil
	}()

	return <-done
}
