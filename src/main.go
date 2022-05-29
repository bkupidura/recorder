package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"recorder/convert"
	"recorder/metric"
	"recorder/mqtt"
	"recorder/record"
	"recorder/upload"

	"github.com/alexliesenfeld/health"
	"github.com/asaskevich/EventBus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
)

func getConfig() (*viper.Viper, error) {
	config := viper.New()

	config.SetConfigName("config")
	config.SetConfigType("yaml")

	config.AddConfigPath("/config")

	replacer := strings.NewReplacer(".", "_")
	config.SetEnvKeyReplacer(replacer)
	config.AutomaticEnv()

	config.SetDefault("mqtt.topic", "recorder")
	config.SetDefault("ssh.user", "recorder")
	config.SetDefault("ssh.key", "/config/id_rsa")
	config.SetDefault("output.path", "/data")
	config.SetDefault("record.burst_overlap", 2)
	config.SetDefault("record.workers", 4)
	config.SetDefault("convert.workers", 0)
	config.SetDefault("convert.input_args", map[string]string{"f": "concat", "vaapi_device": "/dev/dri/renderD128", "hwaccel": "vaapi", "safe": "0"})
	config.SetDefault("convert.output_args", map[string]string{"c:a": "copy", "c:v": "h264_vaapi", "preset": "veryfast", "vf": "format=nv12|vaapi,hwupload"})
	config.SetDefault("upload.workers", 4)
	config.SetDefault("upload.timeout", 60)
	config.SetDefault("upload.max_errors", 30)

	if err := config.ReadInConfig(); err != nil {
		return nil, err
	}

	requiredArgs := []string{"mqtt.server", "ssh.server"}

	for _, argName := range requiredArgs {
		if config.Get(argName) == nil {
			return nil, errors.New(fmt.Sprintf("missing required config key: %s", argName))
		}
	}

	if config.GetInt64("record.workers") < 1 {
		return nil, errors.New(fmt.Sprintf("config record.workers should bebiger than 0"))
	}

	return config, nil
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	config, err := getConfig()
	if err != nil {
		log.Panicf("unable to read config: %v", err)
	}

	bus := EventBus.New()

    if err := metric.Register(bus); err != nil {
        log.Panicf("unable to register prometheus metrics: %v", err)
    }

	mqttClient, err := mqtt.New(config, bus)
	if err != nil {
		log.Panicf("unable to create mqtt client: %v", err)
	}

	if _, err := record.New(config, bus); err != nil {
		log.Panicf("unable to create recorder: %v", err)
	}

	if config.GetInt64("convert.workers") > 0 {
		if _, err := convert.New(config, bus); err != nil {
			log.Panicf("unable to create converter: %v", err)
		}
	}

	if config.GetInt64("upload.workers") > 0 {
		if _, err := upload.New(config, bus); err != nil {
			log.Panicf("unable to create uploader: %v", err)
		}
	}

	checker := health.NewChecker(
		health.WithCacheDuration(1*time.Second),
		health.WithTimeout(10*time.Second),
		health.WithCheck(health.Check{
			Name:    "mqtt",
			Timeout: 2 * time.Second,
			Check: func(ctx context.Context) error {
				if mqttClient.IsConnected() != true {
					return fmt.Errorf("mqtt not connected")
				}
				return nil
			},
		}),
	)

	recordings := http.FileServer(http.Dir(config.GetString("output.path")))

	http.Handle("/healthz", health.NewHandler(checker))
	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/", http.RedirectHandler("/recordings/", http.StatusMovedPermanently))
	http.Handle("/recordings/", http.StripPrefix("/recordings/", recordings))

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Panicf("http server error: %v", err)
	}
}
