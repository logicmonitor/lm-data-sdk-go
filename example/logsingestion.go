package main

import (
	"fmt"
	"time"

	"github.com/logicmonitor/go-data-sdk/api/logs"
)

func main_logs() {
	logstr := "This is a veryyyyyyyyyyyyyyyyyyyy longgggggggggggggggggggg msgThis is a veryyyyyyyyyyyyyyyyyyyy longgggggggggggggggggggg msgThis is a veryyyyyyyyyyyyyyyyyyyy longgggggggggggggggggggg msgThis is a veryyyyyyyyyyyyyyyyyyyy longgggggggggggggggggggg msgThis is a veryyyyyyyyyyyyyyyyyyyy longgggggggggggggggggggg msgThis is a veryyyyyyyyyyyyyyyyyyyy longgggggggggggggggggggg msgThis is a veryyyyyyyyyyyyyyyyyyyy longgggggggggggggggggggg msgThis is a veryyyyyyyyyyyyyyyyyyyy longgggggggggggggggggggg msgThis is a veryyyyyyyyyyyyyyyyyyyy longgggggggggggggggggggg msgThis is a veryyyyyyyyyyyyyyyyyyyy longgggggggggggggggggggg msgThis is a veryyyyyyyyyyyyyyyyyyyy longgggggggggggggggggggg msgThis is a veryyyyyyyyyyyyyyyyyyyy longgggggggggggggggggggg msgThis is a veryyyyyyyyyyyyyyyyyyyy longgggggggggggggggggggg msg"
	logstr2 := "This is 2nd log"
	logstr3 := "this is 3rd log"
	lmLog := logs.NewLMLogIngest(true, 10)
	lmLog.Start()

	fmt.Println("Sending log1....")
	lmLog.SendLogs(logstr, map[string]string{"system.aws.resourceid": "ksh-al2-trial-1"}, map[string]string{"testkey": "testvalue"})
	//logs.SendLogs(logstr, map[string]string{"system.aws.resourceid": "ksh-al2-trial-1"})

	time.Sleep(2 * time.Second)
	fmt.Println("Sending log2....")
	lmLog.SendLogs(logstr2, map[string]string{"system.aws.resourceid": "ksh-al2-trial-1"}, map[string]string{"testkey": "testvalue"})

	time.Sleep(3 * time.Second)
	fmt.Println("Sending log3....")
	lmLog.SendLogs(logstr3, map[string]string{"system.aws.resourceid": "ksh-al2-trial-1"}, map[string]string{"testkey": "testvalue"})

	time.Sleep(10 * time.Minute)
}
