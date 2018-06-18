package main

import (
	"time"
	"fmt"
	"strings"
)


type Reader struct {
	path string		//读取文件路径
}
type ReadFromFile interface {
	Read(rc chan string)
}
// Reader结构体实现ReadFromFile接口
func (r *Reader) Read(rc chan string) {
	line := "message"
	rc <- line
}

type Writer struct {
	influxDBDsn string
}
type WriteToInfluxDB interface {
	Write(wc chan string)
}
// Writer结构体实现Write接口
func (r *Writer) Write(wc chan string) {
	fmt.Println(<-wc)
}

type LogProcessor struct {
	rc chan string
	wc chan string
	read ReadFromFile
	write WriteToInfluxDB
}
func (l *LogProcessor) Process() {
	data := <- l.rc
	l.wc <- strings.ToUpper(data)
}

func main() {

	r := &Reader{
		path: "/tmp/access.log",
	}
	w := &Writer{
		influxDBDsn: "/username&password..",
	}

	lp := &LogProcessor{
		rc: make(chan string),
		wc: make(chan string),
		read: r,
		write: w,
	}

	go lp.read.Read(lp.rc)
	go lp.Process()
	go lp.write.Write(lp.wc)

	time.Sleep(1 * time.Second)
}
