Recorder is simple golang service to record rtsp camera stream, and upload it to remote server thru ssh.
It supports recording length and number of videos to record (`burst`).

## Overview
Main task of Recorder is to record RTSP camera stream (record) and upload it to remote server over ssh (upload).
But it can also transform (convert) recorded video e.g. convert can be used to change h265 stream to h264. Converted videos are not uploaded to remote server.

Upload will take place right after single recording is finished.

Recorder is using MQTT to get new tasks. It also exposes HTTP endpoints.

## Build
`go build .`

## Usage

```
docker run -d \
  --name recorder \
  -p 8080:8080 \
  -v /recorder/data:/data \
  -v /recorder/config:/config \
  -v /recorder/secret:/secret \
  ghcr.io/bkupidura/recorder:latest
```

## Config
```
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
  input_args:
    "rtsp_transport": "tcp"
  output_args:
    "c:a": "aac"
    "c:v": "copy"
convert:
  workers: 1
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

Each config property can be passed as env variable, e.g. `ssh:server` can be passed as `RECORDER_SSH_SERVER`.

If you want to disable some services (upload, convert), you need to set `workers: 0` for this service.
Record service can't be disabled.

Config will be readed from `/config/config.yaml`.

## How to trigger recording
Tasks to recorder should be send over HTTP. Recorder expect to get JSON messages.

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

```
curl -v localhost:8080/api/record \
    -H 'Content-Type: application/json' \
    -d '{"stream": "rtsp://user:password@cam1/Streaming/Channels/101?transportmode=unicast&profile=Profile_1&tcp", "burst": 3, "cam_name": "cam1", "prefix": "example", "length": 10}'
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

## K8s definition
```
---
apiVersion: v1
kind: Namespace
metadata:
  name: recorder

---
apiVersion: v1
kind: Service
metadata:
  name: recorder
  namespace: recorder
  labels:
    app.kubernetes.io/name: recorder
spec:
  type: ClusterIP
  publishNotReadyAddresses: false
  ports:
    - name: recorder
      port: 8080
      protocol: TCP
      targetPort: 8080
  selector:
    app.kubernetes.io/name: recorder

---
apiVersion: v1
data:
  id_rsa: <replace_me>
kind: Secret
metadata:
  name: recorder
  namespace: recorder
type: Opaque

---
apiVersion: v1
data:
  config: |
    ssh:
      user: recorder
      key: /secret/id_rsa
      server: <replace_me>
    upload:
      workers: 4
    record:
      workers: 4
    convert:
      workers: 1
kind: ConfigMap
metadata:
  name: recorder
  namespace: recorder

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: recorder
  namespace: recorder
  labels:
    app.kubernetes.io/name: recorder
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app.kubernetes.io/name: recorder
  template:
    metadata:
      labels:
        app.kubernetes.io/name: recorder
      annotations:
        prometheus.io/port: "8080"
        prometheus.io/scrape: "true"
    spec:
      containers:
        - name: recorder
          image: ghcr.io/bkupidura/recorder:latest
          imagePullPolicy: IfNotPresent
          resources:
            requests:
              memory: 512Mi
            limits:
              memory: 1Gi
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
          volumeMounts:
            - mountPath: /secret
              name: id-rsa
            - mountPath: /config
              name: config
          livenessProbe:
            failureThreshold: 3
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 30
            periodSeconds: 10
            successThreshold: 1
            timeoutSeconds: 1
          readinessProbe:
            failureThreshold: 3
            httpGet:
              path: /ready
              port: 8080
              scheme: HTTP
            initialDelaySeconds: 5
            periodSeconds: 10
            successThreshold: 1
            timeoutSeconds: 1
          env:
            - name: TZ
              value: Europe/Warsaw
      volumes:
        - name: id-rsa
          secret:
            secretName: recorder
            defaultMode: 0400
            items:
              - key: id_rsa
                path: id_rsa
        - name: config
          configMap:
            name: recorder
            items:
              - key: config
                path: config.yaml
      terminationGracePeriodSeconds: 5
```

## HTTP
Recorder exposes multiple HTTP endpoints:

* /healthz - recorder healthcheck endpoint
* /ready - recorder readiness endpoint
* /metrics - prometheus metrics
* /recordings/ - expose recordings directory listening
* /api/record - accept recording request

Recorder is listening on `:8080` port.
