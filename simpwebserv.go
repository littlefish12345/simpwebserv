package simpwebserv

import (
	"os"
	"log"
	"net"
	"time"
	"bytes"
	"strings"
	"strconv"
	"net/url"
	"io/ioutil"
	"container/list"
)

const (
	bufferSize = 16384
)

type SimpwebservResponse struct { //响应的结构体
	Protocol string
	Code string
	CodeName string
	Header map[string]string
	Body *bytes.Buffer
	ToDoCommand string
}

type SimpwebservRequest struct { //请求的结构体
	Method string
	Path string
	Protocol string
	Header map[string]string
	Conn net.Conn
}

type SimpwebservUrlNode struct { //单个path的节点
	Name string
	NextLayer *list.List
	IncludeBack bool
	Function func(*SimpwebservRequest) *SimpwebservResponse
}

type SimpwebservApp struct { //实例的结构体
	Listener net.Listener
	UrlMap SimpwebservUrlNode
}

func App() SimpwebservApp { //生成新实例
	app := SimpwebservApp{nil, SimpwebservUrlNode{"root", list.New(),false ,nil}}
	app.UrlMap.Name = "root"
	app.UrlMap.NextLayer = list.New()
	app.UrlMap.Function = nil
	return app
}

func (app *SimpwebservApp) Register (function func(*SimpwebservRequest) *SimpwebservResponse, path string) { //注册一个路径到一个函数上
	pathList := strings.Split(path, "/")[1:]
	includeBack := false
	if pathList[len(pathList) - 1] == "" { //如果路径最后一个字符是/，那么以后的路径都会匹配到这个函数上
		includeBack = true
		pathList = pathList[:len(pathList)-1]
	}
	nowNode := &app.UrlMap
	var tempNode *SimpwebservUrlNode
	for i := 0; i < len(pathList); i++ { //节点树的遍历，有就进入，没就创造
		j := nowNode.NextLayer.Front()
		for {
			if j == nil {
				newNode := SimpwebservUrlNode{pathList[i], list.New(), false, nil}
				nowNode.NextLayer.PushBack(&newNode)
				nowNode = &newNode
				break
			}
			tempNode, _ = (j.Value).(*SimpwebservUrlNode)
			if tempNode.Name == pathList[i] {
				nowNode = tempNode
				break
			}
			j = j.Next()
		}
	}
	nowNode.IncludeBack = includeBack
	nowNode.Function = function
}

func BuildBasicResponse() *SimpwebservResponse { //创建默认的响应
	response := SimpwebservResponse{"HTTP/1.1", "200", "OK", make(map[string]string), new(bytes.Buffer), ""}
	response.Header["Date"] = time.Now().UTC().Format(time.RFC1123) //懒得把UTC改成GMT了
	response.Header["Content-Type"] = "text/html; charset=utf-8"
	return &response
}

func BuildNotFoundResponse() *SimpwebservResponse { //创建404响应
	response := BuildBasicResponse()
	response.Code = "404"
	response.CodeName = "Not Found"
	response.Body.Write([]byte("404 Not Found"))
	return response
}

func BuildJumpResponse(target string) *SimpwebservResponse { //创建302响应
	response := BuildBasicResponse()
	response.Code = "302"
	response.CodeName = "Found"
	response.Header["Location"] = target
	return response
}

func SendStaticFile(path string, contentType string) *SimpwebservResponse { //传输一个静态文件
	f, err := os.Open(path)
	if err != nil {
		return BuildNotFoundResponse()
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return BuildNotFoundResponse()
	}
	response := BuildBasicResponse()
	response.Header["Content-Type"] = contentType
	response.Body.Write(data)
	return response
}

func DecodeGETRequest(request *SimpwebservRequest) map[string]string { //解码GET请求参数
	pathList := strings.Split(request.Path, "?")
	GETMap := make(map[string]string)
	if len(pathList) == 1 {
		return GETMap
	}
	GETList := strings.Split(pathList[1], "&")
	for i := 0; i < len(GETList); i++ {
		lineList := strings.Split(GETList[i], "=")
		GETMap[lineList[0]] = lineList[1]
	}
	return GETMap
}

func DecodeFormRequest(request *SimpwebservRequest) map[string]string { //解码POST的form表单
	FormMap := make(map[string]string)
	if request.Method == "POST" {
		if contentType, ok := request.Header["Content-Type"]; ok {
			if contentLength, ok := request.Header["Content-Length"]; ok {
				if contentType == "application/x-www-form-urlencoded" {
					dataLength, _ := strconv.Atoi(contentLength)
					data := make([]byte, dataLength)
					byteCount, err := request.Conn.Read(data)
					if err != nil || byteCount != dataLength {
						return FormMap
					}
					dataString, _ := url.QueryUnescape(string(data))
					FormList := strings.Split(dataString, "&")
					for i := 0; i < len(FormList); i++ {
						lineList := strings.Split(FormList[i], "=")
						FormMap[lineList[0]] = lineList[1]
					}
				}
			}
		}
	}
	return FormMap
}

func runFunction(path string, request *SimpwebservRequest, app *SimpwebservApp) *SimpwebservResponse { //通过path搜索函数并运行获取返回值
	path = strings.Split(path, "?")[0] //去掉GET请求部分
	if path == "/" && app.UrlMap.Function != nil { //对于根目录的特殊处理
		return app.UrlMap.Function(request)
	}
	pathList := strings.Split(path, "/")[1:]
	nowNode := &app.UrlMap
	var tempNode *SimpwebservUrlNode
	for i := 0; i < len(pathList); i++ {
		j := nowNode.NextLayer.Front()
		for {
			if j == nil { //路径没注册返回404
				return BuildNotFoundResponse()
			}
			tempNode, _ = (j.Value).(*SimpwebservUrlNode)
			if tempNode.Name == pathList[i] {
				nowNode = tempNode
				if nowNode.IncludeBack {
					goto blurryPath
				}
				break
			}
			j = j.Next()
		}
	}
	if nowNode.Function == nil { //函数不存在返回404
		return BuildNotFoundResponse()
	}
	blurryPath:
	response := nowNode.Function(request)
	return response
}

func connectionHandler(conn net.Conn, app *SimpwebservApp) { //处理连接
	defer conn.Close()
	buffer := make([]byte, bufferSize)
	request := SimpwebservRequest{"", "", "", make(map[string]string), conn}
	tempByte := make([]byte, 1)
	var err error
	var byteCount int
	var headerList []string
	for {
		for i := 0; ; i++ { //获取请求
			byteCount, err = conn.Read(tempByte)
			if err != nil {
				return
			}
			if byteCount != 1 {
				break
			}
			buffer[i] = tempByte[0]
			if i >= 3{
				if bytes.Equal(buffer[i-3:i+1], []byte("\r\n\r\n")) {
					break
				}
			}
		}
		headerList = strings.Split(string(buffer), "\r\n")
		headerList = headerList[:len(headerList)-2] //去掉最后的空项

		requestList := strings.Split(headerList[0], " ") //解析协议，请求方式和路径
		headerList = headerList[1:]
		request.Method = requestList[0]
		request.Path, _ = url.QueryUnescape(requestList[1])
		request.Protocol = requestList[2]

		for i := 0; i < len(headerList); i++ { //解析头部
			lineList := strings.Split(headerList[i], ": ")
			if len(lineList) == 2 {
				request.Header[lineList[0]] = lineList[1]
			}
		}

		response := runFunction(request.Path, &request, app) //生成响应
		response.Header["Content-Length"] = strconv.Itoa(response.Body.Len())
		response.Header["Connection"] = "Close" //虽然理论上支持长连接了，但是以后处理请求的body会很麻烦，所以先全部短连接
		log.Println(conn.RemoteAddr().String() + " " + request.Method + " " + request.Path + " " + response.Code + " " + response.CodeName)

		conn.Write([]byte(response.Protocol + " " + response.Code + " " + response.CodeName + "\r\n"))
		header := ""
		for key, value := range(response.Header) {
			header = header + key + ": " + value + "\r\n"
		}
		header = header + "\r\n"
		conn.Write([]byte(header))
		conn.Write(response.Body.Bytes())
	}
}

func (app *SimpwebservApp)Run (host string, port uint16) { //运行实例
	allHost := host + ":" + strconv.Itoa(int(port))
	log.Println("Server is starting at: " + allHost)
	listener, err := net.Listen("tcp", allHost)
	if err != nil {
		log.Fatal("Server listen error: " + err.Error())
		return
	}
	app.Listener = listener
	for {
		conn, err := app.Listener.Accept()
		if err != nil {
			log.Fatal("Server accept error: " + err.Error())
			continue
		}
		go connectionHandler(conn, app)
	}
}