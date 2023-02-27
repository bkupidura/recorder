package task

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

var (
	ffmpegConvertRatio = 2
	tmpDir             = "/tmp"
)

type Convert struct {
	Prefix         string
	RecordingDate  string
	FileNamePrefix string
	FilesPath      []string
	TotalLength    int64
}

func (r *Convert) Do(ctx context.Context, chResult chan interface{}) error {
	if len(r.FilesPath) == 0 {
		return nil
	}
	now := time.Now()

	// /data/prefix/26-02-2023
	dirPath := filepath.Join(ctx.Value("outputDir").(string), r.Prefix, r.RecordingDate)
	// 07:36:36.178-cam1-convert.mp4
	fileName := fmt.Sprintf("%s-convert.mp4", r.FileNamePrefix)
	// /data/prefix/26-02-2023/07:36:36.178-cam1-convert.mp4
	filePath := filepath.Join(dirPath, fileName)

	if err := ffmpegConvert(r.FilesPath, filePath, ctx.Value("ffmpegInputArgs").(map[string]string), ctx.Value("ffmpegOutputArgs").(map[string]string), r.TotalLength); err != nil {
		log.Printf("unable to convert %s: %v", filePath, err)
		return err
	}
	log.Printf("converted %s (length:%ds took:%.2fs)", filePath, int(r.TotalLength), time.Since(now).Seconds())
	return nil
}

func ffmpegConvert(inputFiles []string, outputFileName string, inputArgs map[string]string, outputArgs map[string]string, length int64) error {
	var parts []string
	for _, inputFile := range inputFiles {
		parts = append(parts, "file "+inputFile)
	}

	content := []byte(strings.Join(parts, "\n"))

	listFileName := filepath.Join(tmpDir, uuid.New().String())
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
