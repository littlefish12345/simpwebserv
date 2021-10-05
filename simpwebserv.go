package simpwebserv

import (
	//"io"
	"log"
	"net"
	"time"
	"bytes"
	//"bufio"
	//"errors"
	"strings"
	"strconv"
	"container/list"
)

const (
	bufferSize = 16384
)

type SimpwebservResponse struct {
	Protocol string
	Code string
	CodeName string
	Header map[string]string
	Body *bytes.Buffer
	ToDoCommand string
}

type SimpwebservRequest struct {
	Method string
	Path string
	Protocol string
	Header map[string]string
	Conn net.Conn
}

type SimpwebservUrlNode struct {
	Name string
	NextLayer *list.List
	IncludeBack bool
	Function func(*SimpwebservRequest) *SimpwebservResponse
}

type SimpwebservApp struct {
	Listener net.Listener
	UrlMap SimpwebservUrlNode
}

func App() SimpwebservApp {
	app := SimpwebservApp{nil, SimpwebservUrlNode{"root", list.New(),false ,nil}}
	app.UrlMap.Name = "root"
	app.UrlMap.NextLayer = list.New()
	app.UrlMap.Function = nil
	return app
}

func (app *SimpwebservApp) Register (function func(*SimpwebservRequest) *SimpwebservResponse, path string) {
	pathList := strings.Split(path, "/")[1:]
	includeBack := false
	if pathList[len(pathList) - 1] == "" {
		includeBack = true
		pathList = pathList[:len(pathList)-1]
	}
	nowNode := &app.UrlMap
	var tempNode *SimpwebservUrlNode
	for i := 0; i < len(pathList); i++ {
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

func BuildResponse() *SimpwebservResponse {
	request := SimpwebservResponse{"HTTP/1.1", "200", "OK", make(map[string]string), new(bytes.Buffer), ""}
	request.Header["Date"] = time.Now().UTC().Format(time.RFC1123)
	request.Header["Content-Type"] = "text/html"
	request.Header["Connection"] = "Close"
	return &request
}

func runFunction (path string, request *SimpwebservRequest, app *SimpwebservApp) *SimpwebservResponse {
	path = strings.Split(path, "?")[0]
	if path == "/" && app.UrlMap.Function != nil {
		return app.UrlMap.Function(request)
	}
	pathList := strings.Split(path, "/")[1:]
	nowNode := &app.UrlMap
	var tempNode *SimpwebservUrlNode
	for i := 0; i < len(pathList); i++ {
		j := nowNode.NextLayer.Front()
		for {
			if j == nil {
				response := BuildResponse()
				response.Code = "404"
				response.CodeName = "Not Found"
				response.Body.Write([]byte("404 Not Found"))
				return response
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
	if nowNode.Function == nil {
		response := BuildResponse()
		response.Code = "404"
		response.CodeName = "Not Found"
		response.Body.Write([]byte("404 Not Found"))
		return response
	}
	blurryPath:
	response := nowNode.Function(request)
	return response
}

func connectionHandler(conn net.Conn, app *SimpwebservApp, j int) {
	defer conn.Close()
	buffer := make([]byte, bufferSize)
	for {
		request := SimpwebservRequest{"", "", "", make(map[string]string), conn}
		tempByte := make([]byte, 1)
		var err error
		var byteCount int
		for i := 0; ; i++ {
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
		headerList := strings.Split(string(buffer), "\r\n")
		headerList = headerList[:len(headerList)-2]

		requestList := strings.Split(headerList[0], " ")
		requestMethod := requestList[0]
		requestPath := requestList[1]
		requestProtocol := requestList[2]

		headerList = headerList[1:]
		request.Method = requestMethod
		request.Path = requestPath
		request.Protocol = requestProtocol

		for i := 0; i < len(headerList); i++ {
			lineList := strings.Split(headerList[i], ": ")
			if len(lineList) == 2 {
				request.Header[lineList[0]] = lineList[1]
			}
		}

		response := runFunction(request.Path, &request, app)
		response.Header["Content-Length"] = strconv.Itoa(response.Body.Len())
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
	conn.Close()
}

func (app *SimpwebservApp) Run (host string, port uint16) {
	allHost := host + ":" + strconv.Itoa(int(port))
	log.Println("Server is starting at: " + allHost)
	listener, err := net.Listen("tcp", allHost)
	if err != nil {
		log.Fatal("Server listen error: " + err.Error())
		return
	}
	app.Listener = listener
	i := 0
	for {
		conn, err := app.Listener.Accept()
		if err != nil {
			log.Fatal("Server accept error: " + err.Error())
			continue
		}
		go connectionHandler(conn, app, i)
		i++
	}
}