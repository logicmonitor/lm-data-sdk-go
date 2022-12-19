package main

import (
	"context"
	"fmt"
	"time"

	"github.com/logicmonitor/lm-data-sdk-go/api/logs"
	"github.com/logicmonitor/lm-data-sdk-go/utils/translator"
)

func main() {
	logstr := "This is a test message"
	logstr2 := "This is 2nd log"
	logstr3 := "this is 3rd log"

	options := []logs.Option{
		logs.WithLogBatchingDisabled(),
		logs.WithRateLimit(2),
	}

	lmLog, err := logs.NewLMLogIngest(context.Background(), options...)
	if err != nil {
		fmt.Println("Error in initilaizing log ingest ", err)
		return
	}

	fmt.Println("Sending log1....")
	logInput1 := translator.ConvertToLMLogInput(logstr, time.Now().String(), map[string]string{"system.displayname": "example-cart-service"}, map[string]string{"testkey": "testvalue"})
	err = lmLog.SendLogs(context.Background(), logInput1)
	if err != nil {
		fmt.Println("Error in sending log: ", err)
	}

	time.Sleep(2 * time.Second)

	fmt.Println("Sending log2....")
	logInput2 := translator.ConvertToLMLogInput(logstr2, time.Now().String(), map[string]string{"system.displayname": "example-cart-service"}, map[string]string{"testkey": "testvalue"})
	err = lmLog.SendLogs(context.Background(), logInput2)
	if err != nil {
		fmt.Println("Error in sending log: ", err)
	}

	time.Sleep(3 * time.Second)

	fmt.Println("Sending log3....")
	logInput3 := translator.ConvertToLMLogInput(logstr3, time.Now().String(), map[string]string{"system.displayname": "example-cart-service"}, map[string]string{"testkey": "testvalue"})
	err = lmLog.SendLogs(context.Background(), logInput3)
	if err != nil {
		fmt.Println("Error in sending log: ", err)
	}

	time.Sleep(10 * time.Second)
}
