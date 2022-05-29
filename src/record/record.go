package record

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"recorder/ffmpeg"
	"recorder/upload"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

var (
	recorderQueue      = make(chan *recorderMsg, 1024)
	metricRecordErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "recorder_recorder_errors_total",
			Help: "Recorder total errors",
		}, []string{"service"},
	)
)

type recorderMsg struct {
	Stream  string `json:"stream"`
	CamName string `json:"cam_name"`
	Prefix  string `json:"prefix"`
	Length  int64  `json:"length"`
	Burst   int64  `json:"burst"`
}

func (m *recorderMsg) record(outputDir string, convert bool, burstOverlap int64, uploaderQueue *chan *upload.UploadMsg) ([]string, string, string) {
	log.Printf("recording stream:%s; burst:%d; length:%d; cam_name:%s, prefix:%s", m.Stream, m.Burst, m.Length, m.CamName, m.Prefix)

	var wg sync.WaitGroup
	var parts []string

	startTime := time.Now()

	dateLayout := "02-01-2006"
	timeLayout := "15:04:05.000"

	dirName := fmt.Sprintf("%s/%s/%s", outputDir, m.Prefix, startTime.Format(dateLayout))

	if err := os.MkdirAll(dirName, 0755); err != nil {
		log.Printf("unable to create %s: %v", dirName, err)
		metricRecordErrors.WithLabelValues("record").Inc()
		return parts, "", ""
	}

	fileNamePrefix := fmt.Sprintf("%s-%s", startTime.Format(timeLayout), m.CamName)

	for i := int64(0); i < m.Burst; i++ {
		wg.Add(1)
		go func(m *recorderMsg) {
			defer wg.Done()

			fileName := fmt.Sprintf("%s/%s-%03d-%03d.mp4", dirName, fileNamePrefix, i+1, m.Burst)

			now := time.Now()
			if err := ffmpeg.StartRecording(m.Stream, fileName, m.Length); err != nil {
				log.Printf("unable to record %s from stream: %v", fileName, err)
				metricRecordErrors.WithLabelValues("record").Inc()
				return
			}

			log.Printf("recorded %s (took:%.2fs)", fileName, time.Since(now).Seconds())

			*uploaderQueue <- upload.NewUploadMsg(fileName)

			if convert == true {
				parts = append(parts, fileName)
			}

		}(m)
		time.Sleep(time.Duration(m.Length-burstOverlap) * time.Second)
	}
	wg.Wait()
	return parts, dirName, fileNamePrefix
}

func (m *recorderMsg) convert(parts []string, outputDir, fileNamePrefix string, inputArgs, outputArgs map[string]string) {
	if len(parts) == 0 {
		return
	}

	now := time.Now()

	recordFileName := fmt.Sprintf("%s/%s-full.mp4", outputDir, fileNamePrefix)

	if err := ffmpeg.ConvertRecording(parts, recordFileName, inputArgs, outputArgs, m.Burst*m.Length); err != nil {
		log.Printf("unable to convert %s recording: %v", recordFileName, err)
		metricRecordErrors.WithLabelValues("convert").Inc()
		return
	}

	log.Printf("converted %s (length:%ds took:%.2fs)", recordFileName, int(m.Burst*m.Length), time.Since(now).Seconds())
}

func (m *recorderMsg) populate(mqttMessage MQTT.Message) error {
	if err := json.Unmarshal(mqttMessage.Payload(), &m); err != nil {
		log.Printf("unable to unmarshal message '%s': %v", mqttMessage.Payload(), err)
		return err
	}

	if m.Burst == 0 {
		m.Burst = 1
	}
	if m.Length < 5 {
		m.Length = 5
	}
	if m.Prefix == "" {
		m.Prefix = "unknown"
	}
	if m.CamName == "" {
		m.CamName = "unknown"
	}

	return nil
}

type recorder struct {
	outputDir         string
	burstOverlap      int64
	workers           int64
	convert           bool
	convertInputArgs  map[string]string
	convertOutputArgs map[string]string
	uploaderQueue     *chan *upload.UploadMsg
	mqttClient        MQTT.Client
}

func (r *recorder) Start() {
	log.Printf("starting recorder workers")
	var wg sync.WaitGroup
	for i := int64(0); i < r.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				m, ok := <-recorderQueue
				if !ok {
					log.Panicf("reading from recorder queue error: %v", ok)
				}

				parts, outputDir, fileNamePrefix := m.record(r.outputDir, r.convert, r.burstOverlap, r.uploaderQueue)
				m.convert(parts, outputDir, fileNamePrefix, r.convertInputArgs, r.convertOutputArgs)
			}
			log.Panic("recorder worker loop finished, this never should happend")
		}()
	}
	wg.Wait()
}

func (r *recorder) IsConnected() bool {
	return r.mqttClient.IsConnected()
}

func NewRecorder(c *viper.Viper, uploaderQueue *chan *upload.UploadMsg) (*recorder, error) {
	mqttClient, err := newMqttClient(c.GetString("mqtt.server"), c.GetString("mqtt.user"), c.GetString("mqtt.password"), c.GetString("mqtt.topic"))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("unable to create mqtt client: %v", err))
	}

	prometheus.MustRegister(metricRecordErrors)

	return &recorder{
		outputDir:         c.GetString("output.path"),
		burstOverlap:      c.GetInt64("record.burst_overlap"),
		workers:           c.GetInt64("record.workers"),
		convert:           c.GetBool("convert.enabled"),
		convertInputArgs:  c.GetStringMapString("convert.input_args"),
		convertOutputArgs: c.GetStringMapString("convert.output_args"),
		mqttClient:        mqttClient,
		uploaderQueue:     uploaderQueue,
	}, nil
}

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
		m := &recorderMsg{}
		if err := m.populate(message); err != nil {
			return
		}
		recorderQueue <- m
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
