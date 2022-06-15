package main

import (
	"context"
	"fmt"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/api/logs"
)

func main_logs() {
	logstr := "This is a test message"
	logstr2 := "This is 2nd log"
	logstr3 := "this is 3rd log"

	var options []logs.Option
	options = []logs.Option{
		logs.WithLogBatchingEnabled(3 * time.Second),
	}

	lmLog, err := logs.NewLMLogIngest(context.Background(), options...)
	if err != nil {
		fmt.Println("Error in initilaizing log ingest ", err)
		return
	}

	fmt.Println("Sending log1....")
	lmLog.SendLogs(context.Background(), logstr, map[string]string{"system.displayname": "TestLogexporterSDK_OTEL_30033"}, map[string]string{"testkey": "testvalue"})

	time.Sleep(2 * time.Second)
	fmt.Println("Sending log2....")
	lmLog.SendLogs(context.Background(), logstr2, map[string]string{"system.displayname": "TestLogexporterSDK_OTEL_30033"}, map[string]string{"testkey": "testvalue"})

	time.Sleep(3 * time.Second)
	fmt.Println("Sending log3....")
	lmLog.SendLogs(context.Background(), logstr3, map[string]string{"system.displayname": "TestLogexporterSDK_OTEL_30033"}, map[string]string{"testkey": "testvalue"})

	time.Sleep(10 * time.Second)
}
