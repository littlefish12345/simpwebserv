package simpwebserv

import (
	"bytes"
	"io/ioutil"
	"os"
	"time"
)

func getGMTTime(offset string) string { //获取GMT时间
	now := time.Now().UTC()
	t, err := time.ParseDuration(offset)
	if err == nil {
		now = now.Add(t)
	}
	utcTime := now.Format(time.RFC1123)
	return utcTime[:len(utcTime)-3] + "GMT"
}

func BuildBasicResponse() *Response { //创建200的默认的响应
	response := Response{"HTTP/1.1", "200", "OK", make(map[string]string), new(bytes.Buffer), make([]string, 0), false}
	response.Header["Date"] = getGMTTime("")
	response.Header["Content-Type"] = "text/html; charset=utf-8"
	response.Header["Connection"] = "keep-alive"
	return &response
}

func Build404DefaultResponse() *Response { //创建404的默认响应
	response := Response{"HTTP/1.1", "404", "Not Found", make(map[string]string), new(bytes.Buffer), make([]string, 0), false}
	response.Header["Date"] = getGMTTime("")
	response.Header["Content-Type"] = "text/html; charset=utf-8"
	response.Header["Connection"] = "keep-alive"
	response.Body.WriteString(default404Page)
	return &response
}

func Build500DefaultResponse() *Response { //创建500的默认响应
	response := Response{"HTTP/1.1", "500", "Internal Server Error", make(map[string]string), new(bytes.Buffer), make([]string, 0), false}
	response.Header["Date"] = getGMTTime("")
	response.Header["Content-Type"] = "text/html; charset=utf-8"
	response.Header["Connection"] = "close"
	response.Body.WriteString(default500Page)
	return &response
}

func Build404Response() *Response { //未来自定义404页面使用
	return Build404DefaultResponse()
}

func BuildStaticFileResponse(path string, contentType string) *Response {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Build404Response()
		}
		panic(err)
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		panic(err)
	}
	response := BuildBasicResponse()
	response.Header["Content-Type"] = contentType
	response.Body.Write(data)
	return response
}
