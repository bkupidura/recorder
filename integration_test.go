package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"recorder/internal/api"

	"github.com/gliderlabs/ssh"
	"github.com/pkg/sftp"
	"github.com/stretchr/testify/require"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

var (
	// This directory will be removed after tests!
	outputPath        = "/tmp/recorder_tests"
	testVideoFileName = fmt.Sprintf("%s/recorder_test.mp4", outputPath)
	sshKey            = fmt.Sprintf("%s/id_rsa", outputPath)
	sshServerAddr     = "127.0.0.1:2222"
	httpBaseURL       = fmt.Sprintf("http://localhost:%d", api.HTTPPort)
)

func TestRecorder(t *testing.T) {
	os.Setenv("RECORDER_SSH_SERVER", sshServerAddr)
	os.Setenv("RECORDER_SSH_KEY", sshKey)
	os.Setenv("RECORDER_OUTPUT_PATH", outputPath)
	os.Setenv("RECORDER_CONVERT_WORKERS", "1")
	for _, k := range []string{"RECORDER_SSH_SERVER", "RECORDER_SSH_KEY", "RECORDER_OUTPUT_PATH", "RECORDER_CONVERT_WORKERS"} {
		defer os.Unsetenv(k)
	}

	err := os.Mkdir(outputPath, os.ModePerm)
	defer os.RemoveAll(outputPath)
	require.Nil(t, err)

	err = os.Mkdir(filepath.Join(outputPath, "data"), os.ModePerm)
	require.Nil(t, err)

	err = createFakeSSHKey(sshKey)
	require.Nil(t, err)

	err = createTestVideo(testVideoFileName)
	require.Nil(t, err)

	sftpHandler := &TestSftpHandler{EveryNRequestShouldFail: 10}
	sshServer := fakeSSHServer(sftpHandler.Handler)
	go sshServer.ListenAndServe()

	go main()

	time.Sleep(100 * time.Millisecond)

	jsonBody := []byte(fmt.Sprintf(`{"stream": "%s", "burst": 3, "length": 5, "prefix": "test", "cam_name": "cam1"}`, testVideoFileName))
	bodyReader := bytes.NewReader(jsonBody)
	requestURL := fmt.Sprintf("%s/api/record", httpBaseURL)
	req, err := http.NewRequest(http.MethodPost, requestURL, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	require.Nil(t, err)

	res, err := http.DefaultClient.Do(req)
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)

	// Lets wait till first recording is completed.
	time.Sleep(6 * time.Second)

	now := time.Now()
	nowDate := now.Format("02-01-2006")
	for _, endpoint := range []string{"ready", "healthz", "metrics", "recordings/", "recordings/test/", fmt.Sprintf("recordings/test/%s/", nowDate)} {
		res, err = http.Get(fmt.Sprintf("%s/%s", httpBaseURL, endpoint))
		require.Equal(t, http.StatusOK, res.StatusCode)
	}

	// All recordings + convert should be done already.
	time.Sleep(8 * time.Second)

	// Check "local" recording files.
	recordingFiles, err := ioutil.ReadDir(filepath.Join(outputPath, "test", nowDate))
	require.Nil(t, err)
	require.Equal(t, 4, len(recordingFiles))

	for _, recordingFile := range recordingFiles {
		recognizedRecording := false
		for _, allowedSuffix := range []string{"cam1-001-003.mp4", "cam1-002-003.mp4", "cam1-003-003.mp4", "cam1-convert.mp4"} {
			if strings.HasSuffix(recordingFile.Name(), allowedSuffix) {
				recognizedRecording = true
			}
		}
		require.True(t, recognizedRecording)
	}

	// Check "remote" recording files.
	recordingFiles, err = ioutil.ReadDir(filepath.Join(outputPath, "data", "test", nowDate))
	require.Nil(t, err)
	require.Equal(t, 3, len(recordingFiles))

	for _, recordingFile := range recordingFiles {
		recognizedRecording := false
		for _, allowedSuffix := range []string{"cam1-001-003.mp4", "cam1-002-003.mp4", "cam1-003-003.mp4"} {
			if strings.HasSuffix(recordingFile.Name(), allowedSuffix) {
				recognizedRecording = true
			}
		}
		require.True(t, recognizedRecording)
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

func createTestVideo(outputFile string) error {
	err := ffmpeg.Input("testsrc=duration=5:size=qcif:rate=10", ffmpeg.KwArgs{"f": "lavfi"}).
		Output(outputFile).
		Run()
	return err
}

func fakeSSHServer(sftpHandler func(ssh.Session)) *ssh.Server {
	s := &ssh.Server{
		Addr: sshServerAddr,
		SubsystemHandlers: map[string]ssh.SubsystemHandler{
			"sftp": sftpHandler,
		},
	}
	return s
}

type TestSftpHandler struct {
	EveryNRequestShouldFail int
	requestNumber           int
}

func (h *TestSftpHandler) Handler(sess ssh.Session) {
	requestShouldFail := false
	if h.EveryNRequestShouldFail > 0 {
		if h.requestNumber%h.EveryNRequestShouldFail == 0 {
			log.Printf("this sftp should fail")
			requestShouldFail = true
		}
	}
	h.requestNumber++
	if requestShouldFail {
		return
	}
	debugStream := ioutil.Discard
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
