package record

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"recorder/convert"
	"recorder/ffmpeg"
	"recorder/upload"

	"github.com/asaskevich/EventBus"
	"github.com/spf13/viper"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

var (
	bus EventBus.Bus
)

func newMqttClient(server, user, password, topic string) (MQTT.Client, error) {
	connOpts := MQTT.NewClientOptions().
		AddBroker(server).
		SetCleanSession(true).
		SetKeepAlive(30 * time.Second).
		SetPingTimeout(5 * time.Second).
		SetMaxReconnectInterval(3 * time.Second)

	if user != "" {
		connOpts.SetUsername(user)
		if password != "" {
			connOpts.SetPassword(password)
		}
	}

	connOpts.SetWill(fmt.Sprintf("%s/available", topic), "offline", 1, true)

	tlsConfig := &tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert}
	connOpts.SetTLSConfig(tlsConfig)

	onMessage := func(client MQTT.Client, message MQTT.Message) {
		r, err := newRecordMsg(message)
		if err != nil {
			log.Printf("unable to create new record message from '%s': %v", message.Payload(), err)
			return
		}
		bus.Publish("recorder:record", r)
	}

	connOpts.OnConnect = func(client MQTT.Client) {
		log.Printf("connected to mqtt %s", server)
		if token := client.Subscribe(topic, byte(2), onMessage); token.Wait() && token.Error() != nil {
			log.Panicf("unable to subscribe to topic: %v", token.Error())
		}
		if token := client.Publish(fmt.Sprintf("%s/available", topic), 1, true, "online"); token.Wait() && token.Error() != nil {
			log.Panicf("unable to publish availability message: %v", token.Error())
		}
	}

	client := MQTT.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return client, nil
}

type recordMsg struct {
	Stream  string `json:"stream"`
	CamName string `json:"cam_name"`
	Prefix  string `json:"prefix"`
	Length  int64  `json:"length"`
	Burst   int64  `json:"burst"`
}

func (m *recordMsg) record(outputDir string, burstOverlap int64, uploadEnabled bool) (*convert.ConvertMsg, error) {
	log.Printf("recording stream:%s; burst:%d; length:%d; cam_name:%s, prefix:%s", m.Stream, m.Burst, m.Length, m.CamName, m.Prefix)

	var wg sync.WaitGroup
	var parts []string

	startTime := time.Now()

	dateLayout := "02-01-2006"
	timeLayout := "15:04:05.000"

	dirName := fmt.Sprintf("%s/%s/%s", outputDir, m.Prefix, startTime.Format(dateLayout))

	if err := os.MkdirAll(dirName, 0755); err != nil {
		bus.Publish("metrics:recorder_error", 1, "record")
		return nil, errors.New(fmt.Sprintf("unable to create %s: %v", dirName, err))
	}

	fileNamePrefix := fmt.Sprintf("%s-%s", startTime.Format(timeLayout), m.CamName)

	for i := int64(0); i < m.Burst; i++ {
		wg.Add(1)
		go func(m *recordMsg) {
			defer wg.Done()

			fileName := fmt.Sprintf("%s/%s-%03d-%03d.mp4", dirName, fileNamePrefix, i+1, m.Burst)

			now := time.Now()
			if err := ffmpeg.StartRecording(m.Stream, fileName, m.Length); err != nil {
				log.Printf("unable to record %s from stream: %v", fileName, err)
				bus.Publish("metrics:recorder_error", 1, "record")
				return
			}

			log.Printf("recorded %s (took:%.2fs)", fileName, time.Since(now).Seconds())

			if uploadEnabled {
				bus.Publish("uploader:upload", upload.NewUploadMsg(fileName))
			}

			parts = append(parts, fileName)
		}(m)
		time.Sleep(time.Duration(m.Length-burstOverlap) * time.Second)
	}
	wg.Wait()
	return convert.NewConvertMsg(dirName, fileNamePrefix, parts, m.Burst*m.Length), nil
}

func newRecordMsg(mqttMessage MQTT.Message) (*recordMsg, error) {
	r := &recordMsg{}
	if err := json.Unmarshal(mqttMessage.Payload(), r); err != nil {
		return nil, err
	}

	if r.Burst == 0 {
		r.Burst = 1
	}
	if r.Length < 5 {
		r.Length = 5
	}
	if r.Prefix == "" {
		r.Prefix = "unknown"
	}
	if r.CamName == "" {
		r.CamName = "unknown"
	}

	return r, nil
}

type recorder struct {
	outputDir      string
	burstOverlap   int64
	workers        int64
	runningWorkers int64
	uploadEnabled  bool
	convertEnabled bool
	mqttClient     MQTT.Client
	mtx            *sync.Mutex
}

func (r *recorder) dispatch(msg *recordMsg) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	for r.runningWorkers >= r.workers {
		time.Sleep(500)
	}

	atomic.AddInt64(&r.runningWorkers, 1)
	bus.Publish("metrics:recorder_worker", &r.runningWorkers, "recorder")

	go func(msg *recordMsg) {
		defer atomic.AddInt64(&r.runningWorkers, -1)
		defer bus.Publish("metrics:recorder_worker", &r.runningWorkers, "recorder")

		cm, err := msg.record(r.outputDir, r.burstOverlap, r.uploadEnabled)
		if err != nil {
			log.Print(err)
		}
		if r.convertEnabled {
			bus.Publish("recorder:convert", cm)
		}
		return
	}(msg)
}

func (r *recorder) IsConnected() bool {
	return r.mqttClient.IsConnected()
}

func NewRecorder(c *viper.Viper, evbus EventBus.Bus) (*recorder, error) {
	mqttClient, err := newMqttClient(c.GetString("mqtt.server"), c.GetString("mqtt.user"), c.GetString("mqtt.password"), c.GetString("mqtt.topic"))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to create mqtt client: %v", err))
	}

	bus = evbus

	r := &recorder{
		outputDir:      c.GetString("output.path"),
		burstOverlap:   c.GetInt64("record.burst_overlap"),
		workers:        c.GetInt64("record.workers"),
		convertEnabled: c.GetInt64("convert.workers") != 0,
		uploadEnabled:  c.GetInt64("upload.workers") != 0,
		mqttClient:     mqttClient,
		mtx:            &sync.Mutex{},
	}

	if err := bus.SubscribeAsync("recorder:record", r.dispatch, true); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to subscribe: %v", err))
	}

	return r, nil
}
