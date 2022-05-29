Recorder is simple golang service to record rtsp camera stream, and upload it to remote server thru ssh.
It supports recording length and number of videos to record (`burst`).

## Overview
Main task of Recorder is to record RTSP camera stream (record) and upload it to remote server over ssh (upload).
But it can also transform (convert) recorded video e.g. convert can be used to change h265 stream to h264. Converted videos are not uploaded to remote server.

Upload will take place right after single recording is finished.

Recorder is using MQTT to get new tasks. It also exposes HTTP endpoints.

## Build
`cd src && go build .`

## Usage

```
docker run -d \
  --name recorder \
  -p 8080:8080 \
  -v /recorder/data:/data \
  -v /recorder/config:/config \
  -v /recorder/secret:/secret \
  recorder:latest
```

## Config
```
mqtt:
  topic: recorder
  server: mqtt.local:1883
  password: password
  user: recorder
ssh:
  server: 10.10.10.10:22
  user: recorder
  key: /secret/id_rsa
output:
  path: /data
upload:
  workers: 4
  timeout: 60
  max_errors: 30
record:
  workers: 4
  burst_overlap: 2
convert:
  workers: 0
  input_args:
    "f": "concat"
    "vaapi_device": "/dev/dri/renderD128"
    "hwaccel": "vaapi"
    "safe": "0"
  output_args:
    "c:a": "copy"
    "c:v": "h264_vaapi"
    "preset": "veryfast"
    "vf": "format=nv12|vaapi,hwupload"
```

Each config property can be passed as env variable, e.g. `mqtt:server` can be passed as `MQTT_SERVER`.

If you want to disable some services (upload, convert), you need to set `workers: 0` for this service.
Record service can't be disabled.

Config will be readed from `/config/config.yaml`.

## MQTT
Tasks to recorder should be send over MQTT. Recorder expect to get JSON messages.

### Example message
```
{
    "stream": "rtsp://user:password@10.0.0.10:554/Streaming/Channels/101?transportmode=unicast&profile=Profile_1&tcp",
    "prefix": "door-open",
    "cam_name": "cam1_main_door",
    "length": 10,
    "burst": 3
}
```

### Burst
If you provide `burst` parameters in request JSON - recorder will create multiple videos. It can be useful to upload recording to remote server as fast as possible (upload multiple small files instead of one long).

```
{
    "length": 10,
    "burst": 3,
    [...]
}
```
Recorder will record 3 videos, each 10s long. Every video will start 2s before previous video ends.
* part001.mp4 - 0s - 10s
* part002.mp4 - 8s - 18s
* part003.mp4 - 16s - 26s

## Convert
When convert is enabled (`workers > 0`), when recording is finished, convert will be executed. It can be used to e.g. concat (join multiple bursts into single video) and change video encoding.

Using convert re-encode, will burn planty of CPU cycles, if possible use hardware acceleration and single convert worker.

**Converted video will not be uploaded to remote server.**

## HTTP
Recorder exposes multiple HTTP endpoints:

* /healthz - recorder healthcheck
* /metrics - prometheus metrics
* /recordings/ - expose recordings directory listening

Recorder is listening on `:8080` port.
