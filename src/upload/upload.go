package upload

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asaskevich/EventBus"
	"github.com/bramvdbogaerde/go-scp/auth"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"

	scp "github.com/bramvdbogaerde/go-scp"
)

var (
	bus EventBus.Bus
)

type uploadMsg struct {
	fileName  string
	noError   int64
	lastError time.Time
}

func (m *uploadMsg) upload(client scp.Client) error {
	now := time.Now()
	if err := client.Connect(); err != nil {
		return fmt.Errorf("unable to connect to ssh: %v", err)
	}
	defer client.Close()

	f, err := os.Open(m.fileName)
	if err != nil {
		return fmt.Errorf("unable to open %s: %v", m.fileName, err)
	}
	defer f.Close()

	if err := client.CopyFromFile(*f, m.fileName, "0655"); err != nil {
		return fmt.Errorf("unable to copy file over ssh: %v", err)
	}

	log.Printf("uploaded %s (errors:%d; took:%.2fs)", m.fileName, m.noError, time.Since(now).Seconds())

	return nil
}

func (m *uploadMsg) error(maxErrors int64) error {
	m.noError++
	m.lastError = time.Now()
	if m.noError > maxErrors {
		return errors.New("to many upload errors, giving up")
	}
	return nil
}

func (m *uploadMsg) shouldWait() bool {
	if time.Since(m.lastError) < time.Duration(m.noError)*time.Second*2 {
		return true
	}
	return false
}

func NewMsg(fileName string) *uploadMsg {
	return &uploadMsg{fileName: fileName}
}

type uploader struct {
	scpClients     []scp.Client
	maxError       int64
	runningWorkers int64
	mtx            *sync.Mutex
}

func (u *uploader) dispatch(msg *uploadMsg) {
	u.mtx.Lock()
	defer u.mtx.Unlock()

	for u.runningWorkers >= int64(len(u.scpClients)) {
		time.Sleep(500)
	}

	atomic.AddInt64(&u.runningWorkers, 1)
	bus.Publish("metrics:recorder_worker", &u.runningWorkers, "uploader")

	go func(msg *uploadMsg) {
		defer atomic.AddInt64(&u.runningWorkers, -1)
		defer bus.Publish("metrics:recorder_worker", &u.runningWorkers, "uploader")

		if msg.shouldWait() {
			time.Sleep(time.Second * 2)
			bus.Publish("uploader:upload", msg)
			return
		}

		if err := msg.upload(u.scpClients[u.runningWorkers-1]); err != nil {
			log.Print(err)
			bus.Publish("metrics:recorder_error", 1, "upload")

			if err := msg.error(u.maxError); err == nil {
				bus.Publish("uploader:upload", msg)
			}
			return
		}
	}(msg)
}

func New(c *viper.Viper, evbus EventBus.Bus) (*uploader, error) {
	clientConfig, err := auth.PrivateKey(c.GetString("ssh.user"), c.GetString("ssh.key"), ssh.InsecureIgnoreHostKey())
	if err != nil {
		return nil, err
	}

	scpClients := make([]scp.Client, c.GetInt64("upload.workers"))

	for i := int64(0); i < c.GetInt64("upload.workers"); i++ {
		scpClient := scp.NewClientWithTimeout(c.GetString("ssh.server"), &clientConfig, time.Duration(c.GetInt("upload.timeout"))*time.Second)
		scpClients[i] = scpClient
	}

	bus = evbus

	u := &uploader{
		scpClients: scpClients,
		maxError:   c.GetInt64("upload.max_errors"),
		mtx:        &sync.Mutex{},
	}

	if err := bus.SubscribeAsync("uploader:upload", u.dispatch, true); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to subscribe: %v", err))
	}

	return u, nil
}
