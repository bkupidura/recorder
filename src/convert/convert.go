package convert

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"recorder/ffmpeg"

	"github.com/asaskevich/EventBus"
	"github.com/spf13/viper"
)

var (
	bus EventBus.Bus
)

type ConvertMsg struct {
	outputFilePrefix string
	outputDir        string
	parts            []string
	length           int64
}

func (m *ConvertMsg) convert(inputArgs, outputArgs map[string]string) error {
	if len(m.parts) == 0 {
		return nil
	}

	now := time.Now()

	recordFileName := fmt.Sprintf("%s/%s-full.mp4", m.outputDir, m.outputFilePrefix)

	if err := ffmpeg.ConvertRecording(m.parts, recordFileName, inputArgs, outputArgs, m.length); err != nil {
		bus.Publish("metrics:recorder_error", 1, "convert")
		return errors.New(fmt.Sprintf("unable to convert %s recording: %v", recordFileName, err))
	}

	log.Printf("converted %s (length:%ds took:%.2fs)", recordFileName, int(m.length), time.Since(now).Seconds())

	return nil
}

func NewConvertMsg(outputDir, outputFilePrefix string, parts []string, length int64) *ConvertMsg {
	return &ConvertMsg{
		outputFilePrefix: outputFilePrefix,
		outputDir:        outputDir,
		parts:            parts,
		length:           length,
	}
}

type converter struct {
	workers           int64
	runningWorkers    int64
	convertInputArgs  map[string]string
	convertOutputArgs map[string]string
	mtx               *sync.Mutex
}

func (c *converter) dispatch(msg *ConvertMsg) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	for c.runningWorkers >= c.workers {
		time.Sleep(500)
	}

	atomic.AddInt64(&c.runningWorkers, 1)
	bus.Publish("metrics:recorder_worker", &c.runningWorkers, "converter")

	go func(msg *ConvertMsg) {
		defer atomic.AddInt64(&c.runningWorkers, -1)
		defer bus.Publish("metrics:recorder_worker", &c.runningWorkers, "converter")

		if err := msg.convert(c.convertInputArgs, c.convertOutputArgs); err != nil {
			log.Print(err)
		}
		return
	}(msg)
}

func NewConverter(c *viper.Viper, evbus EventBus.Bus) (*converter, error) {
	bus = evbus

	r := &converter{
		workers:           c.GetInt64("convert.workers"),
		convertInputArgs:  c.GetStringMapString("convert.input_args"),
		convertOutputArgs: c.GetStringMapString("convert.output_args"),
		mtx:               &sync.Mutex{},
	}

	if err := bus.SubscribeAsync("recorder:convert", r.dispatch, true); err != nil {
		return nil, errors.New(fmt.Sprintf("unable to subscribe: %v", err))
	}

	return r, nil
}
