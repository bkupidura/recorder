package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

func getConfig() (*viper.Viper, error) {
	config := viper.New()

	config.SetConfigName("config")
	config.SetConfigType("yaml")

	config.AddConfigPath(".")
	config.AddConfigPath("/config")

	replacer := strings.NewReplacer(".", "_")
	config.SetEnvKeyReplacer(replacer)
	config.SetEnvPrefix("recorder")
	config.AutomaticEnv()

	config.SetDefault("record.workers", 4)
	config.SetDefault("record.input_args", map[string]interface{}{})
	config.SetDefault("record.output_args", map[string]interface{}{"c:a": "aac", "c:v": "copy"})

	config.SetDefault("ssh.user", "recorder")
	config.SetDefault("ssh.key", "/config/id_rsa")
	config.SetDefault("upload.workers", 4)
	config.SetDefault("upload.timeout", 60)
	config.SetDefault("upload.max_errors", 30)

	config.SetDefault("convert.workers", 0)
	config.SetDefault("convert.input_args", map[string]interface{}{"f": "concat", "safe": "0"})
	config.SetDefault("convert.output_args", map[string]interface{}{"c:a": "copy", "c:v": "h264", "preset": "veryfast"})

	config.SetDefault("output.path", "/data")

	if err := config.ReadInConfig(); err != nil {
		log.Printf("unable to read config file, starting with defaults: %s", err)
	}

	requiredArgs := []string{"ssh.server"}

	for _, argName := range requiredArgs {
		if config.Get(argName) == nil {
			return nil, fmt.Errorf("missing required config key: %s", argName)
		}
	}

	if config.GetInt64("record.workers") < 1 {
		return nil, fmt.Errorf("config record.workers should bigger than 0")
	}

	return config, nil
}
