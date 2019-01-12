package main

import (
	"os"
	"fmt"
	"time"
)

func main()  {
	/*172.0.0.12 - - [04/Mar/2018:13:49:52 +0000] http "GET / foot?query=t HTTP/1.0" 200 2133 "-" "KeepAliveClient" "-" 1.005 1.854*/
	//r :=regexp.MustCompile(`([\d\.]+)\s+([^ \[]+)\s+([^\[]+)\s+\[([^\]]+)\]\s+([a-z]+)\s+\"([^"]+)\"\s+(\d{3})\s+(
	//\d+)\s+\"([^"]+)\"\s+\"(.*?)\"\s+\"([\d\.-]+)\"\s+([\d\.-]+)\s([\d\.-]+)`)
	path := "./access.log"
	WriteFile(path)
}
//写文件
func WriteFile(path string)  {
	//打开或新建文件
	f,err := os.Create(path)
	if err != nil {
		fmt.Println("err=",err)
		return
	}
	//使用完毕后关闭文件
	defer f.Close()

	var buff string
	for {
		timeNow :=time.Now().Format("02/Jan/2006:15:04:05 +0000")
		s1 := `172.0.0.12 - - [`
		s2 := fmt.Sprintf("%s",timeNow)
		s3 := `] http "GET /foot?query=t HTTP/1.0" 200 2133 "-" "KeepAliveClient" "-" - 1.854`
		buff = fmt.Sprintf("%s%s%s\n",s1,s2,s3)
		_,err := f.WriteString(buff)
		if err != nil {
			fmt.Println("err=",err)
			return
		}
		time.Sleep(time.Millisecond*100)
		//fmt.Println(s2)
	}
}
