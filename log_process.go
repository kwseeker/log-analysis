/**
 * 日志监控系统
 * 监控某个协议下的某个请求在某个请求方法的QPS&响应时间&流量
 */
package main

import (
	"time"
	"strings"
	"os"
	"bufio"
	"io"
	"regexp"
	"log"
	"net/url"
	"strconv"
	"github.com/influxdata/influxdb/client/v2"
	"flag"
	"net/http"
	"encoding/json"
	"fmt"
)

//读取模块
//1.打开文件
//2.从文件末尾开始逐行读取
//3.写入Read Channel
type OperationRead interface {
	Read(rc chan []byte)	// 传输 []byte 类型数据
}
type FileForReader struct {
	path string		//读取文件路径
}
// FileForReader结构体实现ReadFromFile接口
func (r *FileForReader) Read(rc chan []byte) {
	f, err := os.Open(r.path)
	if err != nil {
		//panic(fmt.Sprintf("open file error: %s", err.Error()))
		panic(err)
	}
	defer f.Close()

	f.Seek(0, 2)	//偏移0，从末尾开始
	rd := bufio.NewReader(f)
	for {
		line, err := rd.ReadBytes('\n')	//终止符为 \n, 表示读取一行。
		if err == io.EOF {
			time.Sleep(500*time.Millisecond)
			continue
		} else if err != nil {
			panic(fmt.Sprintf("read line error: %s", err.Error()))
		} else {
			rc <- line[:len(line)-1]
		}
	}
}

//写入模块
//1.初始化influxdb client
//2.从Write Channel中读取监控数据
//3.构造数据并写入influxdb (influxdb是时序型数据库，被广泛用于存储系统的监控数据 IoT行业的实时数据等场景)
//influxdb的特性：
// 部署简单，无外部依赖
// 内置http支持，使用http读写
// 类sql的灵活查询（max, min, sum等）
type OperationWrite interface {
	Write(wc chan *Log)
}
type InfluxdbForWrite struct {
	dsn string
}
func (w *InfluxdbForWrite) Write(wc chan *Log) {
	dsnSli := strings.Split(w.dsn, "@")

	// Create a new HTTPClient
	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     dsnSli[0],
		Username: dsnSli[1],
		Password: dsnSli[2],
	})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// Create a new point batch
	bp, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  dsnSli[3],
		Precision: dsnSli[4],
	})
	if err != nil {
		log.Fatal(err)
	}

	for v := range wc {

		fmt.Println("v: ", v.Path, " ", v.Method, " ", v.Scheme, " ", v.Status, " ", v.Status,
			" ", v.BytesSent, " ", v.UpstreamTime, " ", v.RequestTime )
		// Create a point and add to batch
		tags := map[string]string{
			"Path": v.Path,
			"Method": v.Method,
			"Scheme": v.Scheme,
			"Status": v.Status,
		}
		fields := map[string]interface{}{
			"bytesSent":   v.BytesSent,
			"upstreamTime": v.UpstreamTime,
			"RequestTime":   v.RequestTime,
		}

		pt, err := client.NewPoint("log", tags, fields, v.TimeLocal)
		if err != nil {
			log.Fatal(err)
		}
		bp.AddPoint(pt)

		// Write the batch
		if err := c.Write(bp); err != nil {
			log.Fatal(err)
		}

		// Close client resources
		if err := c.Close(); err != nil {
			log.Fatal(err)
		}
	}
}

//解析模块
//1.从Read Channel中读取每行日志数据
//2.正则提取所需的监控数据(path/status/method etc)
//3.写入Write Channel
type Log struct {
	TimeLocal                    time.Time
	BytesSent                    int
	Path, Method, Scheme, Status string
	UpstreamTime, RequestTime    float64
}

type LogProcess struct {
	rc chan []byte
	wc chan *Log
	r  OperationRead
	w  OperationWrite
}
func (l *LogProcess) Process() {
	re := regexp.MustCompile(`([\d\.]+)\s+([^ \[]+)\s+([^ \[]+)\s+\[([^\]]+)\]\s+([a-z]+)\s+\"([^"]+)\"\s+(\d{3})\s+(
\d+)\s+\"([^"]+)\"\s+\"(.*?)\"\s+\"([\d\.-]+)\"\s+([\d\.-]+)\s+([d\.-]+)`)

	loc, _ := time.LoadLocation("PRC")
	for v := range l.rc {
		str := string(v)
		ret := re.FindStringSubmatch(str)
		if len(ret) != 14 {
			log.Println(str)
			continue
		}

		msg := &Log{}
		t, err := time.ParseInLocation("02/Jan/2006:15:04:05 +0000", ret[4], loc)
		if err != nil {
			log.Println(ret[4])
		}
		msg.TimeLocal = t

		byteSent, _ := strconv.Atoi(ret[8])
		msg.BytesSent = byteSent

		// Get /for?query=t HTTP/1.0
		reqSli := strings.Split(ret[6], " ")
		if len(reqSli) != 3 {
			log.Println(ret[6])
			continue
		}
		msg.Method = reqSli[0]
		msg.Scheme = reqSli[2]
		u, err := url.Parse(reqSli[1])
		if err != nil {
			log.Println(reqSli[1])
			continue
		}
		msg.Path = u.Path
		msg.Status = ret[7]
		upTime, _ := strconv.ParseFloat(ret[12], 64)
		reqTime, _ := strconv.ParseFloat(ret[13], 64)
		msg.UpstreamTime = upTime
		msg.RequestTime = reqTime

		l.wc <- msg
	}
}

type SystemInfo struct {
	LogLine int `json:"logline"` // 总日志处理数
	Tps float64 `json:"tps"`
	ReadChanLen int `json:"readchanlen"` // read chan 长度
	WriteChanLen int `json:"writechanlen"` // write chan 长度
	RunTime string `json:"runtime"` // 运行总时间
	ErrNum int `json:"errnum"` // 错误数
}
type Monitor struct {
	startTime time.Time
	data SystemInfo
}
func (m *Monitor) start(lp *LogProcess) {
	http.HandleFunc("/monitor", func(writer http.ResponseWriter, request *http.Request) {
		m.data.RunTime = time.Now().Sub(m.startTime).String()
		m.data.ReadChanLen = len(lp.rc)
		m.data.WriteChanLen = len(lp.wc)

		ret, _ := json.MarshalIndent(m.data, "", "\t")

		io.WriteString(writer, string(ret))
	})

	http.ListenAndServe(":9091", nil)
}

func main() {
	var path, dsn string

	flag.StringVar(&path, "path", "./test.log", "file path")
	flag.StringVar(&dsn, "dsn", "http://localhost:8086@kwseeker@123456@log-analysis-db@s", "influxdb dsn")
	flag.Parse()

	r := &FileForReader{
		path: path,
	}
	w := &InfluxdbForWrite{
		dsn: dsn,
	}

	l := &LogProcess{
		rc: make(chan []byte, 200), // 添加读取限制
		wc: make(chan *Log),
		r:  r,			// 结构体赋值给接口类型变量，然后通过接口类型变量调用接口
		w:  w,
	}

	// 根据任务执行时间, 设置 goroutine 数量
	go l.r.Read(l.rc)
	for i := 0; i < 2; i++ {
		go l.Process()
	}
	for i := 0; i < 2; i++ {
		go l.w.Write(l.wc)
	}

	m := &Monitor{
		startTime: time.Now(),
		data: SystemInfo{},
	}
	m.start(l)

	//time.Sleep(30 * time.Second)
}
