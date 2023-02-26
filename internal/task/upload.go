package task

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var (
	errorBackoffSecond = 2
	sftpDirectory      = "data"
)

type Upload struct {
	FileName  string
	NoError   int
	LastError time.Time
}

type UploadResult struct {
	FileName  string
	NoError   int
	LastError time.Time
}

func (r *Upload) retry(ctx context.Context, chResult chan interface{}, onlyRetry bool) {
	result := &UploadResult{
		FileName:  r.FileName,
		NoError:   r.NoError,
		LastError: r.LastError,
	}
	if !onlyRetry && r.NoError < ctx.Value("maxError").(int)-1 {
		result.NoError++
		result.LastError = time.Now()
	}

	chResult <- result
}

func (r *Upload) Do(ctx context.Context, chResult chan interface{}) error {
	if time.Since(r.LastError) < time.Duration(r.NoError)*time.Duration(errorBackoffSecond)*time.Second {
		time.Sleep(2 * time.Second)
		r.retry(ctx, chResult, true)
		return nil
	}

	sshKey, err := readSSHAuthKey(ctx.Value("sshKey").(string))
	if err != nil {
		log.Printf("unable to read ssh private key: %v", err)
		r.retry(ctx, chResult, false)
		return err
	}

	sshConfig := &ssh.ClientConfig{
		User: ctx.Value("sshUser").(string),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(sshKey),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(ctx.Value("timeout").(int)) * time.Second,
	}
	sshClient, err := ssh.Dial("tcp", ctx.Value("sshServer").(string), sshConfig)
	if err != nil {
		log.Printf("unable to connect to ssh server: %v", err)
		r.retry(ctx, chResult, false)
		return err
	}
	defer sshClient.Close()

	now := time.Now()
	if err := sftpUpload(sshClient, r.FileName); err != nil {
		log.Printf("unable to upload %s: %v", r.FileName, err)
		r.retry(ctx, chResult, false)
		return err
	}
	log.Printf("uploaded %s (errors:%d; took:%.2fs)", r.FileName, r.NoError, time.Since(now).Seconds())

	return nil
}

func readSSHAuthKey(keyName string) (ssh.Signer, error) {
	key, err := os.ReadFile(keyName)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}
	return signer, nil
}

func sftpUpload(sshClient *ssh.Client, fileName string) error {
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	dstFileName := filepath.Base(fileName)
	dstFile, err := sftpClient.Create(filepath.Join(sftpDirectory, dstFileName))
	if err != nil {
		return err
	}
	defer dstFile.Close()

	srcFile, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	return nil
}
