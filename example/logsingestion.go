package main

import (
	"fmt"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/api/logs"
)

func main_logs() {
	logstr := "This is a test message"
	logstr2 := "This is 2nd log"
	logstr3 := "this is 3rd log"
	lmLog, err := logs.NewLMLogIngest(true, 10)
	if err != nil {
		fmt.Println("Error in initilaizing log ingest ", err)
	}

	fmt.Println("Sending log1....")
	lmLog.SendLogs(logstr, map[string]string{"system.displayname": "demo_OTEL_71086"}, map[string]string{"testkey": "testvalue"})

	time.Sleep(2 * time.Second)
	fmt.Println("Sending log2....")
	lmLog.SendLogs(logstr2, map[string]string{"system.displayname": "demo_OTEL_71086"}, map[string]string{"testkey": "testvalue"})

	time.Sleep(3 * time.Second)
	fmt.Println("Sending log3....")
	lmLog.SendLogs(logstr3, map[string]string{"system.displayname": "demo_OTEL_71086"}, map[string]string{"testkey": "testvalue"})

	time.Sleep(2 * time.Minute)
}
