package task

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

var (
	// This directory will be removed after tests!
	outputPath = "/tmp/recorder_tests"
)

func TestRecordDo(t *testing.T) {
	tests := []struct {
		inputCtxFunc          func() context.Context
		inputChResult         chan interface{}
		inputRecord           *Record
		inputFuncBeforeRecord func()
		mockOsMkdirAll        func(string, os.FileMode) error
		expectedErr           error
		expectedResults       []interface{}
	}{
		{
			inputCtxFunc: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "outputDir", outputPath)
				ctx = context.WithValue(ctx, "ffmpegInputArgs", map[string]string{})
				ctx = context.WithValue(ctx, "ffmpegOutputArgs", map[string]string{})
				return ctx
			},
			inputChResult: make(chan interface{}, 1),
			inputRecord: &Record{
				Stream:  "stream",
				Prefix:  "prefix",
				CamName: "camName",
				Length:  5,
				Burst:   1,
			},
			mockOsMkdirAll: func(string, os.FileMode) error {
				return fmt.Errorf("mock error")
			},
			expectedErr: fmt.Errorf("mock error"),
		},
		{
			inputCtxFunc: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "outputDir", outputPath)
				ctx = context.WithValue(ctx, "ffmpegInputArgs", map[string]string{})
				ctx = context.WithValue(ctx, "ffmpegOutputArgs", map[string]string{})
				return ctx
			},
			inputChResult: make(chan interface{}, 1),
			inputRecord: &Record{
				Stream:  "missing_stream",
				Prefix:  "prefix",
				CamName: "camName",
				Length:  2,
				Burst:   1,
			},
			expectedErr: fmt.Errorf("unable to record all bursts"),
		},
		{
			inputCtxFunc: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "outputDir", outputPath)
				ctx = context.WithValue(ctx, "ffmpegInputArgs", map[string]string{})
				ctx = context.WithValue(ctx, "ffmpegOutputArgs", map[string]string{})
				return ctx
			},
			inputChResult: make(chan interface{}, 3),
			inputRecord: &Record{
				Stream:  filepath.Join(outputPath, "test_recording.mp4"),
				Prefix:  "prefix",
				CamName: "camName",
				Length:  3,
				Burst:   1,
			},
			expectedResults: []interface{}{
				&SingleRecordResult{
					RecordRootDir:  "/tmp/recorder_tests",
					Prefix:         "prefix",
					RecordingDate:  "20-01-2023",
					FileName:       "01:02:03.000-camName-001-001.mp4",
					FilePath:       "/tmp/recorder_tests/prefix/20-01-2023/01:02:03.000-camName-001-001.mp4",
					FileNamePrefix: "01:02:03.000-camName",
				},
				&MultipleRecordResult{
					RecordRootDir: "/tmp/recorder_tests",
					Prefix:        "prefix",
					RecordingDate: "20-01-2023",
					FilesPath: []string{
						"/tmp/recorder_tests/prefix/20-01-2023/01:02:03.000-camName-001-001.mp4",
					},
					FileNamePrefix: "01:02:03.000-camName",
					TotalLength:    3,
				},
			},
		},
		{
			inputCtxFunc: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "outputDir", outputPath)
				ctx = context.WithValue(ctx, "ffmpegInputArgs", map[string]string{})
				ctx = context.WithValue(ctx, "ffmpegOutputArgs", map[string]string{})
				return ctx
			},
			inputChResult: make(chan interface{}, 5),
			inputRecord: &Record{
				Stream:  filepath.Join(outputPath, "test_recording.mp4"),
				Prefix:  "prefix",
				CamName: "camName",
				Length:  3,
				Burst:   2,
			},
			expectedResults: []interface{}{
				&SingleRecordResult{
					RecordRootDir:  "/tmp/recorder_tests",
					Prefix:         "prefix",
					RecordingDate:  "20-01-2023",
					FileName:       "01:02:03.000-camName-001-002.mp4",
					FilePath:       "/tmp/recorder_tests/prefix/20-01-2023/01:02:03.000-camName-001-002.mp4",
					FileNamePrefix: "01:02:03.000-camName",
				},
				&SingleRecordResult{
					RecordRootDir:  "/tmp/recorder_tests",
					Prefix:         "prefix",
					RecordingDate:  "20-01-2023",
					FileName:       "01:02:03.000-camName-002-002.mp4",
					FilePath:       "/tmp/recorder_tests/prefix/20-01-2023/01:02:03.000-camName-002-002.mp4",
					FileNamePrefix: "01:02:03.000-camName",
				},
				&MultipleRecordResult{
					RecordRootDir: "/tmp/recorder_tests",
					Prefix:        "prefix",
					RecordingDate: "20-01-2023",
					FilesPath: []string{
						"/tmp/recorder_tests/prefix/20-01-2023/01:02:03.000-camName-001-002.mp4",
						"/tmp/recorder_tests/prefix/20-01-2023/01:02:03.000-camName-002-002.mp4",
					},
					FileNamePrefix: "01:02:03.000-camName",
					TotalLength:    6,
				},
			},
		},
		{
			inputCtxFunc: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "outputDir", outputPath)
				ctx = context.WithValue(ctx, "ffmpegInputArgs", map[string]string{})
				ctx = context.WithValue(ctx, "ffmpegOutputArgs", map[string]string{})
				return ctx
			},
			inputChResult: make(chan interface{}, 5),
			inputRecord: &Record{
				Stream:  filepath.Join(outputPath, "test_recording.mp4"),
				Prefix:  "prefix",
				CamName: "camName",
				Length:  5,
				Burst:   3,
			},
			inputFuncBeforeRecord: func() {
				go func() {
					time.Sleep(3 * time.Second)
					os.Remove(filepath.Join(outputPath, "test_recording.mp4"))
				}()
			},
			expectedResults: []interface{}{
				&SingleRecordResult{
					RecordRootDir:  "/tmp/recorder_tests",
					Prefix:         "prefix",
					RecordingDate:  "20-01-2023",
					FileName:       "01:02:03.000-camName-001-003.mp4",
					FilePath:       "/tmp/recorder_tests/prefix/20-01-2023/01:02:03.000-camName-001-003.mp4",
					FileNamePrefix: "01:02:03.000-camName",
				},
				&MultipleRecordResult{
					RecordRootDir: "/tmp/recorder_tests",
					Prefix:        "prefix",
					RecordingDate: "20-01-2023",
					FilesPath: []string{
						"/tmp/recorder_tests/prefix/20-01-2023/01:02:03.000-camName-001-003.mp4",
					},
					FileNamePrefix: "01:02:03.000-camName",
					TotalLength:    5,
				},
			},
			expectedErr: fmt.Errorf("unable to record all bursts"),
		},
	}
	timeNow = func() time.Time {
		return time.Date(2023, time.January, 20, 1, 2, 3, 4, time.UTC)
	}
	defer func() {
		timeNow = time.Now
	}()

	os.RemoveAll(outputPath)
	err := os.Mkdir(outputPath, os.ModePerm)
	require.Nil(t, err)
	defer os.RemoveAll(outputPath)

	for _, test := range tests {
		createTestVideo(filepath.Join(outputPath, "test_recording.mp4"))

		osMkdirAll = os.MkdirAll
		if test.mockOsMkdirAll != nil {
			osMkdirAll = test.mockOsMkdirAll
			defer func() {
				osMkdirAll = os.MkdirAll
			}()
		}

		if test.inputFuncBeforeRecord != nil {
			test.inputFuncBeforeRecord()
		}

		ctx := test.inputCtxFunc()

		err := test.inputRecord.Do(ctx, test.inputChResult)
		require.Equal(t, test.expectedErr, err)
		require.Equal(t, len(test.expectedResults), len(test.inputChResult))

		for _, expectedResult := range test.expectedResults {
			result := <-test.inputChResult
			require.Equal(t, expectedResult, result)
		}
	}
}

func TestFFMEGRecord(t *testing.T) {
	tests := []struct {
		inputFFMPEGInputArgs  map[string]string
		inputFFMPEGOutputArgs map[string]string
		expectedErr           error
	}{
		{
			inputFFMPEGInputArgs:  map[string]string{"abc": "test"},
			inputFFMPEGOutputArgs: map[string]string{},
			expectedErr:           fmt.Errorf("exit status 8"),
		},
		{
			inputFFMPEGInputArgs:  map[string]string{},
			inputFFMPEGOutputArgs: map[string]string{"abc": "test"},
			expectedErr:           fmt.Errorf("exit status 8"),
		},
		{
			inputFFMPEGInputArgs:  map[string]string{},
			inputFFMPEGOutputArgs: map[string]string{},
		},
	}

	os.RemoveAll(outputPath)
	err := os.Mkdir(outputPath, os.ModePerm)
	require.Nil(t, err)
	defer os.RemoveAll(outputPath)

	err = createTestVideo(filepath.Join(outputPath, "test_recording.mp4"))
	require.Nil(t, err)

	for _, test := range tests {
		err = ffmpegRecord(filepath.Join(outputPath, "test_recording.mp4"), filepath.Join(outputPath, "test_output.mp4"), test.inputFFMPEGInputArgs, test.inputFFMPEGOutputArgs, 2)
		if test.expectedErr != nil {
			require.Equal(t, test.expectedErr.Error(), err.Error())
		}
	}
}

func createTestVideo(outputFile string) error {
	err := ffmpeg.Input("testsrc=duration=5:size=qcif:rate=10", ffmpeg.KwArgs{"f": "lavfi"}).
		Output(outputFile).
		Run()
	return err
}
