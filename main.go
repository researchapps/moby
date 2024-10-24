package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
)

// This should be run in the .devcontainer environment, and following the instructions
// here: https://github.com/moby/moby/blob/master/docs/contributing/set-up-dev-env.md
// to build and start the daemon:

// In one terminal (this builds and starts the daemon)
// hack/make.sh binary install-binary run

// In another terminal:
// make interactive
// This will work
// ./bin/docker-build -t test -f Dockerfile.test .

// This will fail
// ./bin/docker-build -t fail -f Dockerfile.fail .

// But with interactive -i, it will work!
// ./bin/docker-build -i -t works -f Dockerfile.fail .

// copyFile copies a file from a source to a destination
func copyFile(src, dest string) {
	sourceFile, err := os.Open(src)
	if err != nil {
		panic(err)
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dest)
	if err != nil {
		panic(err)
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		panic(err)
	}
}

// GetImageIDFromBody reads the image ID from the build response body.
func GetImageIDFromBody(body io.Reader) string {
	var (
		jm  jsonmessage.JSONMessage
		br  types.BuildResult
		dec = json.NewDecoder(body)
	)
	for {
		err := dec.Decode(&jm)
		if err == io.EOF {
			break
		}
		if jm.Aux == nil {
			continue
		}
		json.Unmarshal(*jm.Aux, &br)
		break
	}
	io.Copy(io.Discard, body)
	return br.ID
}

func main() {

	target := flag.String("t", "", "Build target")
	dockerfile := flag.String("f", "Dockerfile", "Dockerfile path")
	interactiveDebug := flag.Bool("i", false, "interactive debug build")

	flag.Parse()
	args := flag.Args()

	if *target == "" {
		log.Panicf("Please enter a -t target to build")
	}
	buildContext := "."
	if len(args) > 0 {
		buildContext = args[0]
	}
	fmt.Println("🦎 Dinosaur debug builder:")
	fmt.Println("  interactive debug:", *interactiveDebug)
	fmt.Println("         dockerfile:", *dockerfile)
	fmt.Println("            context:", buildContext)
	fmt.Println("             target:", *target)

	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}
	defer apiClient.Close()

	// Create temporary directory for reader (context)
	// We will copy dockerfile there
	tmp, err := os.MkdirTemp("", "docker-dinosaur-build")
	if err != nil {
		log.Fatalf("could not create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmp)

	copyFile(*dockerfile, filepath.Join(tmp, "Dockerfile"))
	reader, err := archive.TarWithOptions(tmp, &archive.TarOptions{})
	if err != nil {
		log.Fatalf("could not create tar: %v", err)
	}

	resp, err := apiClient.ImageBuild(
		context.Background(),
		reader,
		types.ImageBuildOptions{
			Remove:      true,
			ForceRemove: true,
			Dockerfile:  "Dockerfile",
			Tags:        []string{*target},
			Interactive: *interactiveDebug,
		},
	)
	if err != nil {
		log.Fatalf("could not build image: %v", err)
	}

	if resp.Body != nil {
		defer resp.Body.Close()
	}
	img := GetImageIDFromBody(resp.Body)
	if img == "" {
		fmt.Println("😭 Sorry, that image build failed.")
	} else {
		fmt.Println(img)
	}
}
