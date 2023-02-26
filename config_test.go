package main

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestGetConfig(t *testing.T) {
	tests := []struct {
		inputEnv           map[string]string
		expectedErr        error
		expectedConfigFunc func() *viper.Viper
	}{
		{
			expectedErr: errors.New("missing required config key: ssh.server"),
		},
		{
			inputEnv: map[string]string{"RECORDER_SSH_SERVER": "1.2.3.4:22"},
			expectedConfigFunc: func() *viper.Viper {
				c := viper.New()
				d := []byte(`
                record:
                  workers: 4
                  output_args:
                    "c:a": "aac"
                    "c:v": "copy"
                ssh:
                  user: recorder
                  key: /config/id_rsa
                upload:
                  workers: 4
                  timeout: 60
                  max_errors: 30
                convert:
                  workers: 0
                  input_args:
                    "f": "concat"
                    "safe": "0"
                  output_args:
                    "c:a": "copy"
                    "c:v": "h264"
                    "preset": "veryfast"
                output:
                  path: /data
                `)
				c.SetConfigType("yaml")
				c.ReadConfig(bytes.NewBuffer(d))
				return c
			},
		},
		{
			inputEnv:    map[string]string{"RECORDER_SSH_SERVER": "1.2.3.4:22", "RECORDER_RECORD_WORKERS": "0"},
			expectedErr: errors.New("config record.workers should bigger than 0"),
		},
	}
	for _, test := range tests {
		for k, v := range test.inputEnv {
			os.Setenv(k, v)
		}
		defer func() {
			for k := range test.inputEnv {
				os.Unsetenv(k)
			}
		}()

		c, err := getConfig()

		require.Equal(t, test.expectedErr, err)

		if test.expectedConfigFunc != nil {
			expectedConfig := test.expectedConfigFunc()
			require.Equal(t, expectedConfig.AllSettings(), c.AllSettings())
		}

	}
}
