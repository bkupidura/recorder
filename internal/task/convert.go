package task

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

var (
	ffmpegConvertRatio = 2
)

type Convert struct {
	FileNames []string
	Prefix    string
	DirName   string
	Length    int64
}

func (r *Convert) Do(ctx context.Context, chResult chan interface{}) error {
	if len(r.FileNames) == 0 {
		return nil
	}
	now := time.Now()
	outputFileName := fmt.Sprintf("%s/%s-convert.mp4", r.DirName, r.Prefix)
	if err := ffmpegConvert(r.FileNames, outputFileName, ctx.Value("ffmpegInputArgs").(map[string]string), ctx.Value("ffmpegOutputArgs").(map[string]string), r.Length); err != nil {
		log.Printf("unable to convert %s: %v", outputFileName, err)
		return err
	}
	log.Printf("converted %s (length:%ds took:%.2fs)", outputFileName, int(r.Length), time.Since(now).Seconds())
	return nil
}

func ffmpegConvert(inputFiles []string, outputFileName string, inputArgs map[string]string, outputArgs map[string]string, length int64) error {
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
		Output(outputFileName, outputKwArgs).
		WithTimeout(time.Duration(length*int64(ffmpegConvertRatio)) * time.Second).
		Run()

	if err != nil {
		defer os.Remove(outputFileName)
		return err
	}

	return nil
}
