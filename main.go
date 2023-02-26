package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"recorder/internal/api"
	"recorder/internal/metric"
	"recorder/internal/pool"
	"recorder/internal/task"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Printf("starting recorder")

	config, err := getConfig()
	if err != nil {
		log.Panicf("unable to read config: %v", err)
	}

	workingPools := make(map[string]*pool.Pool)

	ctxRecord := context.Background()
	ctxRecord = context.WithValue(ctxRecord, "outputDir", config.GetString("output.path"))
	ctxRecord = context.WithValue(ctxRecord, "ffmpegInputArgs", config.GetStringMapString("record.input_args"))
	ctxRecord = context.WithValue(ctxRecord, "ffmpegOutputArgs", config.GetStringMapString("record.output_args"))

	ctxUpload := context.Background()
	ctxUpload = context.WithValue(ctxUpload, "sshUser", config.GetString("ssh.user"))
	ctxUpload = context.WithValue(ctxUpload, "sshKey", config.GetString("ssh.key"))
	ctxUpload = context.WithValue(ctxUpload, "sshServer", config.GetString("ssh.server"))
	ctxUpload = context.WithValue(ctxUpload, "timeout", config.GetInt("upload.timeout"))
	ctxUpload = context.WithValue(ctxUpload, "maxError", config.GetInt("upload.max_errors"))

	ctxConvert := context.Background()
	ctxConvert = context.WithValue(ctxConvert, "ffmpegInputArgs", config.GetStringMapString("convert.input_args"))
	ctxConvert = context.WithValue(ctxConvert, "ffmpegOutputArgs", config.GetStringMapString("convert.output_args"))

	for poolName, poolOptions := range map[string]*pool.Options{
		"record": {
			NoWorkers:  config.GetInt("record.workers"),
			PoolSize:   100,
			ResultSize: 100,
			Ctx:        ctxRecord,
		},
		"upload": {
			NoWorkers:  config.GetInt("upload.workers"),
			PoolSize:   150,
			ResultSize: 150,
			Ctx:        ctxUpload,
		},
		"convert": {
			NoWorkers:  config.GetInt("convert.workers"),
			PoolSize:   30,
			ResultSize: 30,
			Ctx:        ctxConvert,
		},
	} {
		workingPools[poolName] = pool.New(poolOptions)
	}

	metric.Initialize(&metric.Options{
		WorkingPools: workingPools,
	})

	httpRouter := api.NewRouter(&api.Options{
		RecordingPath: config.GetString("output.path"),
		WorkingPools:  workingPools,
		AuthUsers:     config.GetStringMapString("api.user"),
	})

	go dispatcher(workingPools)
	http.ListenAndServe(fmt.Sprintf(":%d", api.HTTPPort), httpRouter)
}

// dispatcher handles results from different working pools.
func dispatcher(workingPools map[string]*pool.Pool) {
	for {
		select {
		// Record working pool triggers recording flow (record -> upload -> convert).
		case recordResult := <-workingPools["record"].ResultChan():
			switch result := recordResult.(type) {
			// Single recording was done, lets upload it to remote sftp server.
			case *task.SingleRecordResult:
				tUpload := &task.Upload{
					FileName: result.FileName,
				}
				if workingPools["upload"].Running() {
					workingPools["upload"].Execute(tUpload.Do)
				}
				// All recordings are done, lets start convert action.
			case *task.MultipleRecordResult:
				tConvert := &task.Convert{
					FileNames: result.FileNames,
					Prefix:    result.Prefix,
					Length:    result.TotalLength,
				}
				if workingPools["convert"].Running() {
					workingPools["convert"].Execute(tConvert.Do)
				}
			}
			// Upload task generates result only on failure.
			// This is used to retry uploads.
		case uploadResult := <-workingPools["upload"].ResultChan():
			tUpload := &task.Upload{
				FileName:  uploadResult.(*task.UploadResult).FileName,
				NoError:   uploadResult.(*task.UploadResult).NoError,
				LastError: uploadResult.(*task.UploadResult).LastError,
			}
			workingPools["upload"].Execute(tUpload.Do)
		}
	}
}
