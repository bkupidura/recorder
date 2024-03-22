package task

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	gssh "github.com/gliderlabs/ssh"
	"github.com/pkg/sftp"
	"github.com/stretchr/testify/require"
)

var (
	sshServerAddr = "127.0.0.1:2222"
)

func TestUploadRetry(t *testing.T) {
	past10s := time.Now().Add(-10 * time.Second)

	timeNow = func() time.Time {
		date, _ := time.Parse("2006-01-02", "2023-01-28")
		return date
	}
	defer func() {
		timeNow = time.Now
	}()

	tests := []struct {
		inputUpload    *Upload
		inputCtxFunc   func() context.Context
		inputOnlyRetry bool
		expectedResult *UploadResult
	}{
		{
			inputCtxFunc: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "maxError", 30)
				return ctx
			},
			inputUpload: &Upload{
				Prefix:        "prefix",
				RecordingDate: "28-01-2023",
				FileName:      "23:40:27.876-cam1-001-003.mp4",
				FilePath:      filepath.Join(outputPath, "test_recording.mp4"),
				NoError:       30,
				LastError:     past10s,
			},
			inputOnlyRetry: true,
			expectedResult: &UploadResult{
				Prefix:        "prefix",
				RecordingDate: "28-01-2023",
				FileName:      "23:40:27.876-cam1-001-003.mp4",
				FilePath:      filepath.Join(outputPath, "test_recording.mp4"),
				NoError:       30,
				LastError:     past10s,
			},
		},
		{
			inputCtxFunc: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "maxError", 30)
				return ctx
			},
			inputUpload: &Upload{
				Prefix:        "prefix",
				RecordingDate: "28-01-2023",
				FileName:      "23:40:27.876-cam1-001-003.mp4",
				FilePath:      filepath.Join(outputPath, "test_recording.mp4"),
				NoError:       10,
				LastError:     past10s,
			},
			inputOnlyRetry: true,
			expectedResult: &UploadResult{
				Prefix:        "prefix",
				RecordingDate: "28-01-2023",
				FileName:      "23:40:27.876-cam1-001-003.mp4",
				FilePath:      filepath.Join(outputPath, "test_recording.mp4"),
				NoError:       10,
				LastError:     past10s,
			},
		},
		{
			inputCtxFunc: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "maxError", 30)
				return ctx
			},
			inputUpload: &Upload{
				Prefix:        "prefix",
				RecordingDate: "28-01-2023",
				FileName:      "23:40:27.876-cam1-001-003.mp4",
				FilePath:      filepath.Join(outputPath, "test_recording.mp4"),
				NoError:       10,
				LastError:     past10s,
			},
			inputOnlyRetry: false,
			expectedResult: &UploadResult{
				Prefix:        "prefix",
				RecordingDate: "28-01-2023",
				FileName:      "23:40:27.876-cam1-001-003.mp4",
				FilePath:      filepath.Join(outputPath, "test_recording.mp4"),
				NoError:       11,
				LastError:     timeNow(),
			},
		},
		{
			inputCtxFunc: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "maxError", 30)
				return ctx
			},
			inputUpload: &Upload{
				Prefix:        "prefix",
				RecordingDate: "28-01-2023",
				FileName:      "23:40:27.876-cam1-001-003.mp4",
				FilePath:      filepath.Join(outputPath, "test_recording.mp4"),
				NoError:       30,
				LastError:     past10s,
			},
			inputOnlyRetry: false,
			expectedResult: &UploadResult{
				Prefix:        "prefix",
				RecordingDate: "28-01-2023",
				FileName:      "23:40:27.876-cam1-001-003.mp4",
				FilePath:      filepath.Join(outputPath, "test_recording.mp4"),
				NoError:       30,
				LastError:     past10s,
			},
		},
	}

	resultCh := make(chan interface{}, 3)

	for _, test := range tests {
		ctx := test.inputCtxFunc()

		test.inputUpload.retry(ctx, resultCh, test.inputOnlyRetry)

		require.Equal(t, 1, len(resultCh))
		result := <-resultCh
		require.Equal(t, test.expectedResult, result)
	}
}

func TestUploadDo(t *testing.T) {
	now := time.Now()
	timeNow = func() time.Time {
		date, _ := time.Parse("2006-01-02", "2023-01-28")
		return date
	}
	defer func() {
		timeNow = time.Now
	}()

	tests := []struct {
		inputCtxFunc     func() context.Context
		inputChResult    chan interface{}
		inputUpload      *Upload
		inputSFTPHandler *TestSftpHandler
		expectedResults  []interface{}
		expectedErr      error
	}{
		{
			inputCtxFunc: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "sshKey", "key")
				ctx = context.WithValue(ctx, "sshUser", "recorder")
				ctx = context.WithValue(ctx, "sshServer", sshServerAddr)
				ctx = context.WithValue(ctx, "maxError", 30)
				ctx = context.WithValue(ctx, "timeout", 5)
				return ctx
			},
			inputChResult: make(chan interface{}, 3),
			inputUpload: &Upload{
				Prefix:        "prefix",
				RecordingDate: "28-01-2023",
				FileName:      "23:40:27.876-cam1-001-003.mp4",
				FilePath:      filepath.Join(outputPath, "test_recording.mp4"),
				NoError:       30,
				LastError:     now,
			},
			inputSFTPHandler: &TestSftpHandler{},
			expectedResults: []interface{}{
				&UploadResult{
					Prefix:        "prefix",
					RecordingDate: "28-01-2023",
					FileName:      "23:40:27.876-cam1-001-003.mp4",
					FilePath:      filepath.Join(outputPath, "test_recording.mp4"),
					NoError:       30,
					LastError:     now,
				},
			},
		},
		{
			inputCtxFunc: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "sshKey", "key")
				ctx = context.WithValue(ctx, "sshUser", "recorder")
				ctx = context.WithValue(ctx, "sshServer", sshServerAddr)
				ctx = context.WithValue(ctx, "maxError", 30)
				ctx = context.WithValue(ctx, "timeout", 5)
				return ctx
			},
			inputChResult: make(chan interface{}, 3),
			inputUpload: &Upload{
				Prefix:        "prefix",
				RecordingDate: "28-01-2023",
				FileName:      "23:40:27.876-cam1-001-003.mp4",
				FilePath:      filepath.Join(outputPath, "test_recording.mp4"),
			},
			inputSFTPHandler: &TestSftpHandler{},
			expectedResults: []interface{}{
				&UploadResult{
					Prefix:        "prefix",
					RecordingDate: "28-01-2023",
					FileName:      "23:40:27.876-cam1-001-003.mp4",
					FilePath:      filepath.Join(outputPath, "test_recording.mp4"),
					NoError:       1,
					LastError:     timeNow(),
				},
			},
			expectedErr: fmt.Errorf("open key: no such file or directory"),
		},
		{
			inputCtxFunc: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "sshKey", filepath.Join(outputPath, "id_rsa"))
				ctx = context.WithValue(ctx, "sshUser", "recorder")
				ctx = context.WithValue(ctx, "sshServer", "127.0.0.1:2223")
				ctx = context.WithValue(ctx, "maxError", 30)
				ctx = context.WithValue(ctx, "timeout", 5)
				return ctx
			},
			inputChResult: make(chan interface{}, 3),
			inputUpload: &Upload{
				Prefix:        "prefix",
				RecordingDate: "28-01-2023",
				FileName:      "23:40:27.876-cam1-001-003.mp4",
				FilePath:      filepath.Join(outputPath, "test_recording.mp4"),
				NoError:       10,
			},
			inputSFTPHandler: &TestSftpHandler{},
			expectedResults: []interface{}{
				&UploadResult{
					Prefix:        "prefix",
					RecordingDate: "28-01-2023",
					FileName:      "23:40:27.876-cam1-001-003.mp4",
					FilePath:      filepath.Join(outputPath, "test_recording.mp4"),
					NoError:       11,
					LastError:     timeNow(),
				},
			},
			expectedErr: fmt.Errorf("dial tcp 127.0.0.1:2223: connect: connection refused"),
		},
		{
			inputCtxFunc: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "sshKey", filepath.Join(outputPath, "id_rsa"))
				ctx = context.WithValue(ctx, "sshUser", "recorder")
				ctx = context.WithValue(ctx, "sshServer", sshServerAddr)
				ctx = context.WithValue(ctx, "maxError", 30)
				ctx = context.WithValue(ctx, "timeout", 5)
				return ctx
			},
			inputChResult: make(chan interface{}, 3),
			inputUpload: &Upload{
				Prefix:        "prefix",
				RecordingDate: "28-01-2023",
				FileName:      "23:40:27.876-cam1-001-003.mp4",
				FilePath:      filepath.Join(outputPath, "test_recording.mp4"),
				NoError:       11,
			},
			inputSFTPHandler: nil,
			expectedResults: []interface{}{
				&UploadResult{
					Prefix:        "prefix",
					RecordingDate: "28-01-2023",
					FileName:      "23:40:27.876-cam1-001-003.mp4",
					FilePath:      filepath.Join(outputPath, "test_recording.mp4"),
					NoError:       12,
					LastError:     timeNow(),
				},
			},
			expectedErr: fmt.Errorf("ssh: subsystem request failed"),
		},
		{
			inputCtxFunc: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, "sshKey", filepath.Join(outputPath, "id_rsa"))
				ctx = context.WithValue(ctx, "sshUser", "recorder")
				ctx = context.WithValue(ctx, "sshServer", sshServerAddr)
				ctx = context.WithValue(ctx, "maxError", 30)
				ctx = context.WithValue(ctx, "timeout", 5)
				return ctx
			},
			inputChResult: make(chan interface{}, 3),
			inputUpload: &Upload{
				Prefix:        "prefix",
				RecordingDate: "28-01-2023",
				FileName:      "23:40:27.876-cam1-001-003.mp4",
				FilePath:      filepath.Join(outputPath, "test_recording.mp4"),
				NoError:       11,
			},
			inputSFTPHandler: &TestSftpHandler{},
		},
	}

	os.RemoveAll(outputPath)
	err := os.Mkdir(outputPath, os.ModePerm)
	require.Nil(t, err)
	defer os.RemoveAll(outputPath)

	err = createTestVideo(filepath.Join(outputPath, "test_recording.mp4"))
	require.Nil(t, err)

	err = createFakeSSHKey(filepath.Join(outputPath, "id_rsa"))
	require.Nil(t, err)

	for _, test := range tests {

		sshServer := fakeSSHServer(test.inputSFTPHandler)
		go sshServer.ListenAndServe()
		time.Sleep(10 * time.Millisecond)

		ctx := test.inputCtxFunc()

		err := test.inputUpload.Do(ctx, test.inputChResult)

		sshServer.Close()
		time.Sleep(10 * time.Millisecond)

		if test.expectedErr != nil {
			require.Equal(t, test.expectedErr.Error(), err.Error())
		} else {
			require.Nil(t, err)
		}

		require.Equal(t, len(test.expectedResults), len(test.inputChResult))

		for _, expectedResult := range test.expectedResults {
			result := <-test.inputChResult
			require.Equal(t, expectedResult, result)
		}
	}
}

func TestReadSSHAuthKey(t *testing.T) {
	tests := []struct {
		inputKeyName string
		expectedErr  string
	}{
		{
			inputKeyName: "missing",
			expectedErr:  "open missing: no such file or directory",
		},
		{
			inputKeyName: filepath.Join(outputPath, "empty"),
			expectedErr:  "ssh: no key found",
		},
		{
			inputKeyName: filepath.Join(outputPath, "id_rsa"),
		},
	}

	os.RemoveAll(outputPath)
	err := os.Mkdir(outputPath, os.ModePerm)
	require.Nil(t, err)
	defer os.RemoveAll(outputPath)

	err = createFakeSSHKey(filepath.Join(outputPath, "id_rsa"))
	require.Nil(t, err)

	f, err := os.Create(filepath.Join(outputPath, "empty"))
	require.Nil(t, err)
	f.Close()

	for _, test := range tests {
		_, err := readSSHAuthKey(test.inputKeyName)
		if test.expectedErr != "" {
			require.Equal(t, test.expectedErr, err.Error())
		} else {
			require.Nil(t, err)
		}
	}
}

func TestSftpUpload(t *testing.T) {
	tests := []struct {
		inputSFTPHandler *TestSftpHandler
		inputRemoteDir   string
		inputRemoteFile  string
		inputLocalFile   string
		mockIoCopy       func(io.Writer, io.Reader) (int64, error)
		expectedErr      string
	}{
		{
			inputSFTPHandler: nil,
			expectedErr:      "ssh: subsystem request failed",
		},
		{
			inputSFTPHandler: &TestSftpHandler{},
			inputRemoteDir:   "/non_existing",
			expectedErr:      "sftp: \"mkdir /non_existing: read-only file system\" (SSH_FX_FAILURE)",
		},
		{
			inputSFTPHandler: &TestSftpHandler{},
			inputRemoteDir:   outputPath,
			inputRemoteFile:  "fake/file",
			expectedErr:      "file does not exist",
		},
		{
			inputSFTPHandler: &TestSftpHandler{},
			inputRemoteDir:   outputPath,
			inputRemoteFile:  "file",
			inputLocalFile:   "missing",
			expectedErr:      "open missing: no such file or directory",
		},
		{
			inputSFTPHandler: &TestSftpHandler{},
			inputRemoteDir:   outputPath,
			inputRemoteFile:  "file",
			inputLocalFile:   filepath.Join(outputPath, "test_recording.mp4"),
			mockIoCopy: func(io.Writer, io.Reader) (int64, error) {
				return 0, fmt.Errorf("copy mock error")
			},
			expectedErr: "copy mock error",
		},
		{
			inputSFTPHandler: &TestSftpHandler{},
			inputRemoteDir:   outputPath,
			inputRemoteFile:  "file",
			inputLocalFile:   filepath.Join(outputPath, "test_recording.mp4"),
		},
	}

	os.RemoveAll(outputPath)
	err := os.Mkdir(outputPath, os.ModePerm)
	require.Nil(t, err)
	defer os.RemoveAll(outputPath)

	err = createTestVideo(filepath.Join(outputPath, "test_recording.mp4"))
	require.Nil(t, err)

	for _, test := range tests {
		if test.mockIoCopy != nil {
			ioCopy = test.mockIoCopy
		}
		sshServer := fakeSSHServer(test.inputSFTPHandler)

		go sshServer.ListenAndServe()
		time.Sleep(10 * time.Millisecond)

		sshConfig := &ssh.ClientConfig{
			User:            "test",
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         time.Duration(1) * time.Second,
		}
		sshClient, err := ssh.Dial("tcp", sshServerAddr, sshConfig)
		require.Nil(t, err)

		err = sftpUpload(sshClient, test.inputLocalFile, test.inputRemoteDir, test.inputRemoteFile)

		ioCopy = io.Copy
		sshServer.Close()
		time.Sleep(10 * time.Millisecond)

		if test.expectedErr != "" {
			require.Equal(t, test.expectedErr, err.Error())
		} else {
			require.Nil(t, err)
		}
	}
}

func createFakeSSHKey(outputFile string) error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return err
	}

	privateKeyFile, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer privateKeyFile.Close()

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return err
	}
	return nil
}

func fakeSSHServer(sftpHandler *TestSftpHandler) *gssh.Server {
	if sftpHandler != nil {
		return &gssh.Server{
			Addr: sshServerAddr,
			SubsystemHandlers: map[string]gssh.SubsystemHandler{
				"sftp": sftpHandler.Handler,
			},
		}
	} else {
		return &gssh.Server{
			Addr: sshServerAddr,
		}
	}
}

type TestSftpHandler struct {
	EveryNRequestShouldFail int
	requestNumber           int
}

func (h *TestSftpHandler) Handler(sess gssh.Session) {
	requestShouldFail := false
	if h.EveryNRequestShouldFail > 0 {
		if h.requestNumber%h.EveryNRequestShouldFail == 0 {
			requestShouldFail = true
		}
	}
	h.requestNumber++

	if requestShouldFail {
		return
	}
	debugStream := io.Discard
	serverOptions := []sftp.ServerOption{
		sftp.WithDebug(debugStream),
		sftp.WithServerWorkingDirectory(outputPath),
	}
	server, err := sftp.NewServer(
		sess,
		serverOptions...,
	)
	if err != nil {
		log.Panicf("sftp server init error: %s", err)
		return
	}
	if err := server.Serve(); err == io.EOF {
		server.Close()
	} else if err != nil {
		log.Panicf("sftp server serve error: %s", err)
	}
}
