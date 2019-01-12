package main

import (
	"strings"
	"fmt"
	"time"
	"os"
	"bufio"
	"io"
	"regexp"
	"log"
	"strconv"
	"net/url"
	"github.com/influxdata/influxdb/client/v2"
	"flag"
	"net/http"
	"encoding/json"
)

type Reader interface {
	Read(rc chan []byte)
}

type Writer interface{
	Write(wc chan *Message)
}

type LogProcess struct{
	rc chan []byte
	wc chan *Message
	read Reader
	write Writer
}

type Message struct{
	TimeLocal   time.Time
	BytesSent int
	Path,Method,Scheme,Status string
	UpstreamTime,RequestTime float64

}

type ReadFromFile struct{
	path string  //读取文件路径
}
type WriteToinfluxDB struct{
	influxDBDsn string  //influx data source
}

type SystemInfo struct {
	HandleLine int `json:"handleLine"`
	Tps float64 `json:"tps"`
	ReadChanLen int `json:"readChanLen"`
	WriteChanLen int `json:"writeChanLen"`
	RunTime string `json:"runTime"`
	ErrNum int `json:"errNum"`
}

const (
	TypeHandeLine  = 0
	TypeErrNum =1
)
var TypeMonitorChan = make(chan int,200)

type Monitor struct {
	startTime time.Time
	data SystemInfo
	tpSlic []int
}

func (m *Monitor)start(lp *LogProcess){

	go func() {
		for n := range TypeMonitorChan {
			switch n {
			case TypeErrNum:
				m.data.ErrNum += 1
			case TypeHandeLine:
				m.data.HandleLine +=1
			}
		}
	}()

	ticker :=time.NewTicker(time.Second*5)
	go func() {
		<- ticker.C
		m.tpSlic = append(m.tpSlic,m.data.HandleLine)
		if len(m.tpSlic)>2 {
			m.tpSlic = m.tpSlic[1:]
		}
	}()
	http.HandleFunc("/monitor",func(writer http.ResponseWriter,request *http.Request){
		m.data.RunTime = time.Now().Sub(m.startTime).String()
		m.data.ReadChanLen=len(lp.rc)
		m.data.WriteChanLen=len(lp.wc)
		if len(m.tpSlic ) >= 2{
			m.data.Tps = float64(m.tpSlic[1]-m.tpSlic[0])/5
		}

		ret,_ :=json.MarshalIndent(m.data,"","\t")
		io.WriteString(writer,string(ret))
	})
	http.ListenAndServe(":9193",nil)
}


func (l *ReadFromFile)Read(rc chan []byte){
	//读取模块

	//打开文件
	f,err :=os.Open(l.path)
	if err !=nil{
		panic(fmt.Sprintf("Open file error %s",err.Error()))
	}
	//从文件末尾逐行读取文件
	//光标移至末尾
	f.Seek(0,2)
	rd :=bufio.NewReader(f)
	for{
	line,err := rd.ReadBytes('\n')
	if err == io.EOF{
		time.Sleep(500*time.Millisecond)
		continue
	}else if err != nil{
		panic(fmt.Sprintf("read file error %s",err.Error()))
	}
	TypeMonitorChan <- TypeHandeLine
	rc <- line[:len(line)-1]
	}
}

func (w *WriteToinfluxDB)Write(wc chan *Message){
	//写入模块

	infSli :=strings.Split(w.influxDBDsn,"@")

	// Create a new HTTPClient
	c, err := client.NewHTTPClient(client.HTTPConfig{
		Addr:     infSli[0],
		Username: infSli[1],
		Password: infSli[2],
	})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()


	// Create a new point batch
	bp, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database:  infSli[3],
		Precision: "s",
	})
	if err != nil {
		log.Fatal(err)
	}

	for v:= range wc {

		// Create a point and add to batch
		//Tag: Path,Method,Scheme,Status
		tags := map[string]string{"Path": v.Path, "Method":v.Method,"Scheme":v.Scheme,"Status":v.Status}
		fields := map[string]interface{}{
			"UpstreamTime":   v.UpstreamTime,
			"RequestTime": v.RequestTime,
			"ByteSent":   v.BytesSent,
		}

		pt, err := client.NewPoint("nginix_log", tags, fields, v.TimeLocal)
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
		log.Println("write sucess!")
	}

}


func (l *LogProcess) Process()  {
	//解析模块

	//日志格式例
	/*172.0.0.12 - - [04/Mar/2018:13:49:52 +0000] http "GET / foot?query=t HTTP/1.0" 200 2133 "-" "KeepAliveClient" "-" 1.005 1.854*/
	//172.0.0.12 - - [04/Mar/2018:13:49:52 +0000] http "GET /foot?query=t HTTP/1.0" 200 2133 "-" "KeepAliveClient" "-" 1.005 1.854

	//正则表达式
	//匹配日志内容
	/* ([\d\.]+)\s+([^ \[]+)\s+([^\[]+)\s+\[([^\]]+)\]\s+([a-z]+)\s+\"([^"]+)\"\s+(\d{3})\s+(
	\d+)\s+\"([^"]+)\"\s+\"(.*?)\"\s+\"([\d\.-]+)\"\s+([\d\.-]+)\s([\d\.-]+)*/
	//r :=regexp.MustCompile(`([\d+\.?]+)\s+(-)\s+(-)\s+\[(.*?)\]\s+([a-z]+)\s+\"(.*?)\"\s+(\d{3})\s+(\d+)\s+\"(.*?)\"\s+\"(.*?)\"\s+\"(.*?)\"\s+(.*?)\s([\d\.]+)`)
	r :=regexp.MustCompile(`([\d\.]+)\s+([^ \[]+)\s+([^ \[]+)\s+\[([^\]]+)\]\s+([a-z]+)\s+\"([^"]+)\"\s+(\d{3})\s+(\d+)\s+\"([^"]+)\"\s+\"(.*?)\"\s+\"([\d\.-]+)\"\s+([\d\.-]+)\s([\d\.-]+)`)
	loc,_ :=time.LoadLocation("Asia/Shanghai")
	for v :=range l.rc{
		ret := r.FindStringSubmatch(string(v))
		if len(ret) != 14 {
			fmt.Println(len(ret))
			TypeMonitorChan <- TypeErrNum
			log.Println("FindStringSubmatch fail:",string(v))
			continue
		}
		message :=&Message{}
		t,err :=time.ParseInLocation("02/Jan/2006:15:04:05 +0000",ret[4],loc)
		if err !=nil {
			TypeMonitorChan <- TypeErrNum
			log.Println("ParseInLocation fail:",err.Error())
			continue
		}
		message.TimeLocal =t

		byteSent,_ :=strconv.Atoi(ret[8])
		message.BytesSent = byteSent

		//GET /foot?query=t HTTP/1.0
		reqSli := strings.Split(ret[6]," ")
		if len(ret) !=14{
			TypeMonitorChan <- TypeErrNum
			log.Println("strings.Split fail",ret[6])
			continue
		}
		message.Method = reqSli[0]

		u,err :=url.Parse(reqSli[1])
		if err !=nil{
			TypeMonitorChan <- TypeErrNum
			log.Println("url parse fail:",err)
			continue
		}
		message.Path = u.Path

		message.Scheme = ret[5]
		message.Status =ret[7]

		upstreamTime,_ :=strconv.ParseFloat(ret[12],64)
		requestTime,_ :=strconv.ParseFloat(ret[13],64)

		message.UpstreamTime = upstreamTime
		message.RequestTime = requestTime
	l.wc <- message
	}
}

func main()  {
	var path,influxDsn string
	flag.StringVar(&path,"path","./access.log","read file path")
	flag.StringVar(&influxDsn,"influxDsn","http://127.0.0.1:8086@admin@admin@data_log","influx data source")
	flag.Parse()

	r := &ReadFromFile{
		path:path,
	}

	w := &WriteToinfluxDB{
		influxDBDsn:influxDsn,
	}
	lp := &LogProcess{
		rc :make(chan []byte),
		wc : make(chan *Message),
		read:r,
		write:w,

	}
	go lp.read.Read(lp.rc)
	for i :=0;i<2 ;i++  {
		go lp.Process()
	}
	for i :=0;i<4 ;i++  {
		go lp.write.Write(lp.wc)
	}

	m := Monitor{
		startTime:time.Now(),
		data:SystemInfo{},
	}
	m.start(lp)
}
