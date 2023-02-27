package task

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

var (
	dateLayout        = "02-01-2006"
	timeLayout        = "15:04:05.000"
	ffmpegRecordRatio = 2
	burstOverlap      = 2
)

type Record struct {
	Stream  string
	Prefix  string
	CamName string
	Length  int64
	Burst   int64
}

type SingleRecordResult struct {
	RecordRootDir  string
	Prefix         string
	RecordingDate  string
	FileName       string
	FilePath       string
	FileNamePrefix string
}

type MultipleRecordResult struct {
	RecordRootDir  string
	Prefix         string
	RecordingDate  string
	FilesPath      []string
	FileNamePrefix string
	TotalLength    int64
}

func (r *Record) Do(ctx context.Context, chResult chan interface{}) error {
	log.Printf("recording stream:%s; burst:%d; length:%d; cam_name:%s, prefix:%s", r.Stream, r.Burst, r.Length, r.CamName, r.Prefix)

	var wg sync.WaitGroup
	var parts []string

	startTime := time.Now()
	// /data/prefix/20-02-2023
	dirPath := filepath.Join(ctx.Value("outputDir").(string), r.Prefix, startTime.Format(dateLayout))
	// 07:36:36.178-cam_nam
	fileNamePrefix := fmt.Sprintf("%s-%s", startTime.Format(timeLayout), r.CamName)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		log.Printf("unable to create %s: %v", dirPath, err)
		return err
	}

	for i := int64(0); i < r.Burst; i++ {
		wg.Add(1)
		go func(r *Record) {
			defer wg.Done()

			// 07:36:36.178-cam_nam-001-003.mp4
			fileName := fmt.Sprintf("%s-%03d-%03d.mp4", fileNamePrefix, i+1, r.Burst)
			// /data/prefix/20-02-2023/07:36:36.178-cam_nam-001-003.mp4
			filePath := filepath.Join(dirPath, fileName)

			now := time.Now()
			if err := ffmpegRecord(r.Stream, filePath, ctx.Value("ffmpegInputArgs").(map[string]string), ctx.Value("ffmpegOutputArgs").(map[string]string), r.Length); err != nil {
				log.Printf("unable to record %s from stream: %v", fileName, err)
				return
			}
			log.Printf("recorded %s (took:%.2fs)", fileName, time.Since(now).Seconds())

			chResult <- &SingleRecordResult{
				RecordRootDir:  ctx.Value("outputDir").(string),
				Prefix:         r.Prefix,
				RecordingDate:  startTime.Format(dateLayout),
				FileName:       fileName,
				FilePath:       filePath,
				FileNamePrefix: fileNamePrefix,
			}
			parts = append(parts, filePath)
		}(r)
		time.Sleep(time.Duration(r.Length-int64(burstOverlap)) * time.Second)
	}
	wg.Wait()
	chResult <- &MultipleRecordResult{
		RecordRootDir:  ctx.Value("outputDir").(string),
		Prefix:         r.Prefix,
		RecordingDate:  startTime.Format(dateLayout),
		FilesPath:      parts,
		FileNamePrefix: fileNamePrefix,
		TotalLength:    r.Burst * r.Length,
	}

	return nil
}

func ffmpegRecord(stream, outputFile string, inputArgs map[string]string, outputArgs map[string]string, length int64) error {
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
		WithTimeout(time.Duration(length*int64(ffmpegRecordRatio)) * time.Second).
		Run()

	if err != nil {
		defer os.Remove(outputFile)
		return err
	}
	return nil
}
