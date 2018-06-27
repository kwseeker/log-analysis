/**
模块功能：
	模拟日志系统（logger）不断抓取 Nginx 服务器日志到日志文件 ./test.log

	0.01-0.19s时间随机生成一条日志文件，并写入到 ./test.log

日志格式：
	172.0.0.12 - - [04/Mar/2018:13:49:52 +0000] http "GET /foo?query=t HTTP/1.0" 200 2133 "-" "KeepAliveClient" "-" 1.005 1.854
	172.0.0.12 - - [22/Dec/2017:03:31:35 +0000]	https "GET /status.html HTTP/1.0" 200 3 "-"	"KeepAliveClient" 	"-"	- 0.000
 */

package main

import (
	"time"
	"fmt"
	"math/rand"
	"strconv"
	"os"
	"bufio"
)

func LogTime() string {
	timeNow := time.Now()
	year, mon, day := timeNow.Date()
	hour, min, sec := timeNow.Clock()
	timeNum := [...]int{day, int(mon), year, hour, min, sec}
	midStr := ""
	for i:=0; i<6; i++ {
		if len(strconv.Itoa(timeNum[i])) < 2 && i != 1 {
			midStr += "0"
		}
		if i < 1 {
			midStr += strconv.Itoa(timeNum[i]) + "/"
		} else if i == 1 {
			midStr += time.Month(timeNum[i]).String() + "/"
		} else if i < 5 {
			midStr += strconv.Itoa(timeNum[i]) + ":"
		} else {
			midStr += strconv.Itoa(timeNum[i])
		}
	}
	//fmt.Println("Current time: ", midStr)
	return midStr
}

func main() {
	logFile := "./test.log"
	head := "172.0.0.12 - - ["
	tail := " +0000] http \"GET /foo?query=t HTTP/1.0\" 200 2133 \"-\" \"KeepAliveClient\" \"-\" 1.005 1.854"
	//timeFmt := "01__02-2006 3.04.05 PM"
	//fmt.Println("Current time: " + time.Now().Format(timeFmt)) 	// time.Format只支持某些固定格式

	fp, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println("OpenFile failed: ", err.Error())
		return
	}
	defer fp.Close()

	for {
		interval := rand.Intn(20)
		//fmt.Println("Current interval: ", interval)
		time.Sleep(time.Duration(interval) * 100 * time.Millisecond)

		logLine := head + LogTime() + tail + "\n"

		//写文件./test.log
		writer := bufio.NewWriter(fp)
		writer.WriteString(logLine)
		writer.Flush()
	}
}