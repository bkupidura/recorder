package record

import (
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

type recordMsg struct {
	Stream  string `json:"stream"`
	CamName string `json:"cam_name"`
	Prefix  string `json:"prefix"`
	Length  int64  `json:"length"`
	Burst   int64  `json:"burst"`
}

func (m *recordMsg) record(outputDir string, burstOverlap int64, inputArgs, outputArgs map[string]string, timeoutRatio int64, uploadEnabled bool) (*convert.ConvertMsg, error) {
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
			if err := ffmpeg.StartRecording(m.Stream, fileName, inputArgs, outputArgs, m.Length, timeoutRatio); err != nil {
				log.Printf("unable to record %s from stream: %v", fileName, err)
				bus.Publish("metrics:recorder_error", 1, "record")
				return
			}

			log.Printf("recorded %s (took:%.2fs)", fileName, time.Since(now).Seconds())

			if uploadEnabled {
				bus.Publish("uploader:upload", upload.NewMsg(fileName))
			}

			parts = append(parts, fileName)
		}(m)
		time.Sleep(time.Duration(m.Length-burstOverlap) * time.Second)
	}
	wg.Wait()
	return convert.NewMsg(dirName, fileNamePrefix, parts, m.Burst*m.Length), nil
}

func NewMsg(mqttMessage MQTT.Message) (*recordMsg, error) {
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
	inputArgs      map[string]string
	outputArgs     map[string]string
	timeoutRatio   int64
	uploadEnabled  bool
	convertEnabled bool
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
		defer bus.Publish("metrics:recorder_worker", &r.runningWorkers, "recorder")
		defer atomic.AddInt64(&r.runningWorkers, -1)

		cm, err := msg.record(r.outputDir, r.burstOverlap, r.inputArgs, r.outputArgs, r.timeoutRatio, r.uploadEnabled)
		if err != nil {
			log.Print(err)
		}
		if r.convertEnabled {
			bus.Publish("recorder:convert", cm)
		}
		return
	}(msg)
}

func New(c *viper.Viper, evbus EventBus.Bus) (*recorder, error) {
	bus = evbus

	r := &recorder{
		outputDir:      c.GetString("output.path"),
		burstOverlap:   c.GetInt64("record.burst_overlap"),
		workers:        c.GetInt64("record.workers"),
		inputArgs:      c.GetStringMapString("record.input_args"),
		outputArgs:     c.GetStringMapString("record.output_args"),
		timeoutRatio:   c.GetInt64("record.timeout_ratio"),
		convertEnabled: c.GetInt64("convert.workers") != 0,
		uploadEnabled:  c.GetInt64("upload.workers") != 0,
		mtx:            &sync.Mutex{},
	}

	if err := bus.SubscribeAsync("recorder:record", r.dispatch, true); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to subscribe: %v", err))
	}

	bus.Publish("metrics:recorder_error", 0, "record")

	return r, nil
}
