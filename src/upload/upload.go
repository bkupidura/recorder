package upload

import (
	"log"
	"os"
	"sync"
	"time"

	"github.com/bramvdbogaerde/go-scp/auth"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"

	scp "github.com/bramvdbogaerde/go-scp"
)

var (
	metricUploadErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "recorder_uploader_errors_total",
			Help: "Uploader total errors",
		}, []string{"service"},
	)
)

type UploadMsg struct {
	FileName  string
	NoError   int64
	LastError time.Time
}

func (m *UploadMsg) upload(client scp.Client) error {
	err := client.Connect()
	if err != nil {
		return err
	}
	defer client.Close()

	f, err := os.Open(m.FileName)
	if err != nil {
		return err
	}
	defer f.Close()

	err = client.CopyFromFile(*f, m.FileName, "0655")
	if err != nil {
		return err
	}

	return nil
}

func NewUploadMsg(fileName string) *UploadMsg {
	return &UploadMsg{FileName: fileName}
}

type uploader struct {
	scpClients []scp.Client
	queue      *chan *UploadMsg
	maxError   int64
}

func (u *uploader) Start() {
	log.Printf("starting uploader workers")

	var wg sync.WaitGroup

	for _, scpClient := range u.scpClients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				m, ok := <-*u.queue
				if !ok {
					log.Panicf("reading from uploader queue error", ok)
				}

				if time.Since(m.LastError) < time.Duration(m.NoError)*time.Second*2 {
					time.Sleep(time.Second * 2)
					*u.queue <- m
					continue
				}

				now := time.Now()
				if err := m.upload(scpClient); err != nil {
					log.Printf("unable to upload %s (errors: %d): %v", m.FileName, m.NoError, err)
					metricUploadErrors.WithLabelValues("upload").Inc()
					m.NoError++
					if m.NoError < u.maxError {
						m.LastError = time.Now()
						*u.queue <- m
					}
					continue
				}

				log.Printf("uploaded %s (errors:%d; took:%.2fs)", m.FileName, m.NoError, time.Since(now).Seconds())
			}
			log.Panic("uploader worker loop finished, this never should happend")
		}()
	}
	wg.Wait()
}

func NewUploader(c *viper.Viper, uploaderQueue *chan *UploadMsg) (*uploader, error) {
	clientConfig, err := auth.PrivateKey(c.GetString("ssh.user"), c.GetString("ssh.key"), ssh.InsecureIgnoreHostKey())
	if err != nil {
		return nil, err
	}

	prometheus.MustRegister(metricUploadErrors)
	scpClients := make([]scp.Client, c.GetInt64("upload.workers"))

	for i := int64(0); i < c.GetInt64("upload.workers"); i++ {
		scpClient := scp.NewClientWithTimeout(c.GetString("ssh.server"), &clientConfig, time.Duration(c.GetInt("upload.timeout"))*time.Second)
		scpClients[i] = scpClient
	}

	return &uploader{
		scpClients: scpClients,
		queue:      uploaderQueue,
		maxError:   c.GetInt64("upload.max_errors"),
	}, nil
}
