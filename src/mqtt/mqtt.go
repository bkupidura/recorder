package mqtt

import (
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"recorder/record"

	"github.com/asaskevich/EventBus"
	"github.com/spf13/viper"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

var (
	bus   EventBus.Bus
	topic string
)

type mqttClient struct {
	c MQTT.Client
}

func (m *mqttClient) IsConnected() bool {
	return m.c.IsConnected()
}

func New(c *viper.Viper, evbus EventBus.Bus) (*mqttClient, error) {
	bus = evbus
	topic = c.GetString("mqtt.topic")

	connOpts := MQTT.NewClientOptions().
		AddBroker(c.GetString("mqtt.server")).
		SetCleanSession(true).
		SetKeepAlive(30 * time.Second).
		SetPingTimeout(5 * time.Second).
		SetMaxReconnectInterval(3 * time.Second)

	if c.GetString("mqtt.user") != "" {
		connOpts.SetUsername(c.GetString("mqtt.user"))
		if c.GetString("mqtt.password") != "" {
			connOpts.SetPassword(c.GetString("mqtt.password"))
		}
	}

	connOpts.SetWill(fmt.Sprintf("%s/available", topic), "offline", 1, true)

	tlsConfig := &tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert}
	connOpts.SetTLSConfig(tlsConfig)

	connOpts.OnConnect = onConnect

	client := MQTT.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return &mqttClient{
		c: client,
	}, nil
}

func onConnect(client MQTT.Client) {
	options := client.OptionsReader()
	log.Printf("connected to mqtt %s", options.Servers())
	if token := client.Subscribe(topic, byte(2), onMessage); token.Wait() && token.Error() != nil {
		log.Panicf("unable to subscribe to topic: %v", token.Error())
	}
	if token := client.Publish(fmt.Sprintf("%s/available", topic), 1, true, "online"); token.Wait() && token.Error() != nil {
		log.Panicf("unable to publish availability message: %v", token.Error())
	}
}

func onMessage(client MQTT.Client, message MQTT.Message) {
	r, err := record.NewMsg(message)
	if err != nil {
		log.Printf("unable to create new record message from '%s': %v", message.Payload(), err)
		return
	}
	bus.Publish("recorder:record", r)
}
