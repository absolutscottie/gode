package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gode/internal/app"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var host = "192.168.4.106:8080"
var modelName = "Qwen3.6-35B-A3B-UD-IQ3_XXS.gguf"

var logger zerolog.Logger

func main() {
	dirFlag := flag.String("dir", "", "Directory to recursively scan and provide to the LLM")
	flag.Parse()

	// read project files, if needed
	var err error
	var fileList []string
	if *dirFlag != "" {
		fileList, err = readDir(*dirFlag)
		if err != nil {
			fmt.Printf("failed to read project directory: %s\n", err)
			os.Exit(1)
		}
	}

	// open log file
	var logFile *os.File
	logFile, err = openLogFile()
	if err != nil {
		panic(err)
	}
	defer logFile.Close()

	// configure logging
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: logFile})
	log.Logger = log.Logger.With().Timestamp().Logger()
	logger = log.Logger

	// run program
	a := app.New(host, modelName, fileList, logger)
	if _, err := a.Run(); err != nil {
		log.Error().Msgf("Error starting program: %s", err)
	}
}

func openLogFile() (*os.File, error) {
	file, err := os.OpenFile(
		"app.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0664,
	)

	return file, err
}

func readDir(dirFlag string) ([]string, error) {
	dir, err := filepath.Abs(dirFlag)
	if err != nil {
		log.Error().Err(err).Msg("failed to resolve absolute path")
		return nil, err
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Error().Str("dir", dir).Msg("directory does not exist")
		return nil, err
	}

	fileList, err := readAllFiles(dir)
	if err != nil {
		log.Error().Err(err).Msg("failed to read directory")
		return nil, err
	}

	return fileList, nil
}

// readAllFiles recursively reads all files in the given directory and returns their relative paths.
func readAllFiles(root string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Skip hidden files and directories
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		// Store relative path from root
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		files = append(files, rel)
		return nil
	})

	return files, err
}
