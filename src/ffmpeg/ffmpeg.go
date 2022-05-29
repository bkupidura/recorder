package ffmpeg

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

func StartRecording(stream, outputFile string, inputArgs map[string]string, outputArgs map[string]string, length int64, timeoutRatio int64) error {
	inputKwArgs := ffmpeg.KwArgs{}
	outputKwArgs := ffmpeg.KwArgs{"t": length}

	for k, v := range inputArgs {
		inputKwArgs[k] = v
	}

	for k, v := range outputArgs {
		outputKwArgs[k] = v
	}

	err := ffmpeg.Input(stream, inputKwArgs).
		Output(outputFile, outputKwArgs).
		WithTimeout(time.Duration(length*timeoutRatio) * time.Second).
		Run()
	if err != nil {
		defer os.Remove(outputFile)
		return err
	}
	return nil
}

func ConvertRecording(inputFiles []string, outputFile string, inputArgs map[string]string, outputArgs map[string]string, totalLength, timeoutRatio int64) error {
	var parts []string
	for _, inputFile := range inputFiles {
		parts = append(parts, "file "+inputFile)
	}

	content := []byte(strings.Join(parts, "\n"))

	listFileName := fmt.Sprintf("/tmp/" + uuid.New().String())
	if err := ioutil.WriteFile(listFileName, content, 0644); err != nil {
		log.Printf("unable to prepare concat list: %v\n", err)
		return err
	}
	defer os.Remove(listFileName)

	inputKwArgs := ffmpeg.KwArgs{}
	outputKwArgs := ffmpeg.KwArgs{}

	for k, v := range inputArgs {
		inputKwArgs[k] = v
	}

	for k, v := range outputArgs {
		outputKwArgs[k] = v
	}

	err := ffmpeg.Input(listFileName, inputKwArgs).
		Output(outputFile, outputKwArgs).
		WithTimeout(time.Duration(totalLength*timeoutRatio) * time.Second).
		Run()

	if err != nil {
		defer os.Remove(outputFile)
		return err
	}

	return nil
}
