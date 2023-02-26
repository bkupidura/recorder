FROM golang:1.19-alpine

WORKDIR /go/src/app
COPY . .

RUN apk add --no-cache ffmpeg libva-intel-driver

RUN go build -v .

CMD ["./recorder"]
