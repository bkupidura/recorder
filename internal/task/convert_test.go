package task

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConvertDo(t *testing.T) {
	tests := []struct {
		inputCtxFunc   func() context.Context
		inputConvert   *Convert
		mockOsMkdirAll func(string, os.FileMode) error
		expectedErr    error
	}{
		{
			inputCtxFunc: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "outputDir", outputPath)
				ctx = context.WithValue(ctx, "ffmpegInputArgs", map[string]string{})
				ctx = context.WithValue(ctx, "ffmpegOutputArgs", map[string]string{})
				return ctx
			},
			inputConvert: &Convert{
				Prefix:         "prefix",
				RecordingDate:  "28-01-2023",
				FileNamePrefix: "23:40:27.876-cam1",
				FilesPath: []string{
					"a",
				},
				TotalLength: 10,
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
			inputConvert: &Convert{
				Prefix:         "prefix",
				RecordingDate:  "28-01-2023",
				FileNamePrefix: "23:40:27.876-cam1",
				FilesPath: []string{
					"a",
				},
				TotalLength: 10,
			},
			expectedErr: fmt.Errorf("exit status 183"),
		},
		{
			inputCtxFunc: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "outputDir", outputPath)
				ctx = context.WithValue(ctx, "ffmpegInputArgs", map[string]string{})
				ctx = context.WithValue(ctx, "ffmpegOutputArgs", map[string]string{})
				return ctx
			},
			inputConvert: &Convert{
				Prefix:         "prefix",
				RecordingDate:  "28-01-2023",
				FileNamePrefix: "23:40:27.876-cam1",
				FilesPath:      []string{},
				TotalLength:    0,
			},
		},
		{
			inputCtxFunc: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "outputDir", outputPath)
				ctx = context.WithValue(ctx, "ffmpegInputArgs", map[string]string{"f": "concat", "safe": "0"})
				ctx = context.WithValue(ctx, "ffmpegOutputArgs", map[string]string{"c:a": "copy", "c:v": "copy", "preset": "veryfast"})
				return ctx
			},
			inputConvert: &Convert{
				Prefix:         "prefix",
				RecordingDate:  "28-01-2023",
				FileNamePrefix: "23:40:27.876-cam1",
				FilesPath: []string{
					filepath.Join(outputPath, "test_recording.mp4"),
					filepath.Join(outputPath, "test_recording.mp4"),
				},
				TotalLength: 10,
			},
		},
	}

	os.RemoveAll(outputPath)
	err := os.Mkdir(outputPath, os.ModePerm)
	require.Nil(t, err)
	defer os.RemoveAll(outputPath)

	chResult := make(chan interface{}, 10)

	for _, test := range tests {
		createTestVideo(filepath.Join(outputPath, "test_recording.mp4"))

		osMkdirAll = os.MkdirAll
		if test.mockOsMkdirAll != nil {
			osMkdirAll = test.mockOsMkdirAll
			defer func() {
				osMkdirAll = os.MkdirAll
			}()
		}

		ctx := test.inputCtxFunc()

		err := test.inputConvert.Do(ctx, chResult)
		if test.expectedErr != nil {
			require.Equal(t, test.expectedErr.Error(), err.Error())
		} else {
			require.Nil(t, err)
		}
	}
}

func TestFFMEGConvert(t *testing.T) {
	tests := []struct {
		inputFFMPEGInputArgs  map[string]string
		inputFFMPEGOutputArgs map[string]string
		inputFFMPEGInputFiles []string
		mockOsWriteFile       func(string, []byte, os.FileMode) error
		expectedErr           error
	}{
		{
			inputFFMPEGInputArgs:  map[string]string{},
			inputFFMPEGOutputArgs: map[string]string{},
			mockOsWriteFile: func(string, []byte, os.FileMode) error {
				return fmt.Errorf("mock error")
			},
			expectedErr: fmt.Errorf("mock error"),
		},
		{
			inputFFMPEGInputArgs:  map[string]string{"f": "concat", "safe": "0"},
			inputFFMPEGOutputArgs: map[string]string{"c:a": "copy", "c:v": "copy", "preset": "veryfast"},
			inputFFMPEGInputFiles: []string{
				"a",
				"b",
			},
			expectedErr: fmt.Errorf("exit status 254"),
		},
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
			inputFFMPEGInputArgs:  map[string]string{"f": "concat", "safe": "0"},
			inputFFMPEGOutputArgs: map[string]string{"c:a": "copy", "c:v": "copy", "preset": "veryfast"},
			inputFFMPEGInputFiles: []string{
				filepath.Join(outputPath, "test_recording.mp4"),
				filepath.Join(outputPath, "test_recording.mp4"),
			},
		},
	}

	os.RemoveAll(outputPath)
	err := os.Mkdir(outputPath, os.ModePerm)
	require.Nil(t, err)
	defer os.RemoveAll(outputPath)

	err = createTestVideo(filepath.Join(outputPath, "test_recording.mp4"))
	require.Nil(t, err)

	for _, test := range tests {
		osWriteFile = os.WriteFile
		if test.mockOsWriteFile != nil {
			osWriteFile = test.mockOsWriteFile
			defer func() {
				osWriteFile = os.WriteFile
			}()
		}
		err = ffmpegConvert(test.inputFFMPEGInputFiles, filepath.Join(outputPath, "test_output.mp4"), test.inputFFMPEGInputArgs, test.inputFFMPEGOutputArgs, 5)
		if test.expectedErr != nil {
			require.Equal(t, test.expectedErr.Error(), err.Error())
		}
	}
}
