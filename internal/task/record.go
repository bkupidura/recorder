package task

import (
	"context"
	"fmt"
	"log"
	"os"
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
	FileName string
	Prefix   string
	Date     string
}

type MultipleRecordResult struct {
	FileNames   []string
	Prefix      string
	DirName     string
	TotalLength int64
}

func (r *Record) Do(ctx context.Context, chResult chan interface{}) error {
	log.Printf("recording stream:%s; burst:%d; length:%d; cam_name:%s, prefix:%s", r.Stream, r.Burst, r.Length, r.CamName, r.Prefix)

	var wg sync.WaitGroup
	var parts []string

	startTime := time.Now()
	dirName := fmt.Sprintf("%s/%s/%s", ctx.Value("outputDir"), r.Prefix, startTime.Format(dateLayout))
	fileNamePrefix := fmt.Sprintf("%s-%s", startTime.Format(timeLayout), r.CamName)

	if err := os.MkdirAll(dirName, 0755); err != nil {
		log.Printf("unable to create %s: %v", dirName, err)
		return err
	}

	for i := int64(0); i < r.Burst; i++ {
		wg.Add(1)
		go func(r *Record) {
			defer wg.Done()

			fileName := fmt.Sprintf("%s/%s-%03d-%03d.mp4", dirName, fileNamePrefix, i+1, r.Burst)

			now := time.Now()
			if err := ffmpegRecord(r.Stream, fileName, ctx.Value("ffmpegInputArgs").(map[string]string), ctx.Value("ffmpegOutputArgs").(map[string]string), r.Length); err != nil {
				log.Printf("unable to record %s from stream: %v", fileName, err)
				return
			}
			log.Printf("recorded %s (took:%.2fs)", fileName, time.Since(now).Seconds())

			chResult <- &SingleRecordResult{
				FileName: fileName,
				Prefix:   r.Prefix,
				Date:     startTime.Format(dateLayout),
			}
			parts = append(parts, fileName)
		}(r)
		time.Sleep(time.Duration(r.Length-int64(burstOverlap)) * time.Second)
	}
	wg.Wait()
	chResult <- &MultipleRecordResult{
		FileNames:   parts,
		Prefix:      fileNamePrefix,
		DirName:     dirName,
		TotalLength: r.Burst * r.Length,
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
