package simpwebserv

import (
	"bytes"
	"container/list"
	"crypto/tls"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	bufferSize = 16384
)

var FileOverSize = errors.New("File over size")
var IncorrectRequest = errors.New("Incorrect request")
var IncompleteFile = errors.New("Incomplete file")

type SimpwebservResponse struct { //响应的结构体
	Protocol      string
	Code          string
	CodeName      string
	Header        map[string]string
	Body          *bytes.Buffer
	ToDoCommand   string
	SetCookieList []string
}

type SimpwebservRequest struct { //请求的结构体
	Method      string
	Path        string
	PurePath    string
	Protocol    string
	Host        string
	Header      map[string]string
	Conn        net.Conn
	readedBytes int
}

type SimpwebservUrlNode struct { //单个path的节点
	Name        string
	NextLayer   *list.List
	IncludeBack bool
	Function    func(*SimpwebservRequest) *SimpwebservResponse
}

type SimpwebservApp struct { //实例的结构体
	Listener                   net.Listener
	UrlMap                     SimpwebservUrlNode
	UseHTTPS                   bool
	HTTPSConfig                *tls.Config
	NotFoundHandler            func(*SimpwebservRequest) *SimpwebservResponse
	InternalServerErrorHandler func(error) *SimpwebservResponse
	DebugMode                  bool
	EnableConsoleLog           bool
}

func App() SimpwebservApp { //生成新实例
	app := SimpwebservApp{nil, SimpwebservUrlNode{"root", list.New(), false, nil}, false, nil, nil, nil, false, true}
	app.UrlMap.Name = "root"
	app.UrlMap.NextLayer = list.New()
	app.UrlMap.Function = nil
	return app
}

func (app *SimpwebservApp) Register(function func(*SimpwebservRequest) *SimpwebservResponse, path string) { //注册一个路径到一个函数上
	pathList := strings.Split(path, "/")[1:]
	includeBack := false
	if pathList[len(pathList)-1] == "" { //如果路径最后一个字符是/，那么以后的路径都会匹配到这个函数上
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

func (app *SimpwebservApp) RegisterInternalServerErrorFunction(function func(error) *SimpwebservResponse) { //注册500函数
	app.InternalServerErrorHandler = function
}

func (app *SimpwebservApp) RegisterNotFoundFunction(function func(*SimpwebservRequest) *SimpwebservResponse) { //注册404函数
	app.NotFoundHandler = function
}

func getGMTTime(offset string) string { //获取GMT时间
	now := time.Now().UTC()
	t, err := time.ParseDuration(offset)
	if err == nil {
		now = now.Add(t)
	}
	utcTime := now.Format(time.RFC1123)
	return utcTime[:len(utcTime)-3] + "GMT"
}

func BuildBasicResponse() *SimpwebservResponse { //创建默认的响应
	response := SimpwebservResponse{"HTTP/1.1", "200", "OK", make(map[string]string), new(bytes.Buffer), "", make([]string, 0)}
	response.Header["Date"] = getGMTTime("")
	response.Header["Content-Type"] = "text/html; charset=utf-8"
	return &response
}

func BuildInternalServerErrorResponse() *SimpwebservResponse { //创建500响应
	response := BuildBasicResponse()
	response.Code = "500"
	response.CodeName = "Internal Server Error"
	return response
}

func BuildNotFoundResponse() *SimpwebservResponse { //创建404响应
	response := BuildBasicResponse()
	response.Code = "404"
	response.CodeName = "Not Found"
	return response
}

func BuildJumpResponse(target string) *SimpwebservResponse { //创建302响应
	response := BuildBasicResponse()
	response.Code = "302"
	response.CodeName = "Found"
	response.Header["Location"] = target
	return response
}

func (request *SimpwebservRequest) DecodeGETRequest() map[string]string { //解码GET请求参数
	pathList := strings.Split(request.Path, "?")
	GETMap := make(map[string]string)
	if len(pathList) == 1 {
		return GETMap
	}
	GETList := strings.Split(pathList[1], "&")
	var lineFirst string
	for i := 0; i < len(GETList); i++ {
		lineList := strings.Split(GETList[i], "=")
		if len(lineList) == 2 {
			lineFirst, _ = url.QueryUnescape(lineList[0])
			GETMap[lineFirst], _ = url.QueryUnescape(lineList[1])
		}
	}
	return GETMap
}

func (request *SimpwebservRequest) GetPOSTBody() *bytes.Buffer { //解码POST的body部分
	if request.Method == "POST" {
		if contentLength, ok := request.Header["Content-Length"]; ok {
			contentLengthInt, _ := strconv.Atoi(contentLength)
			if contentLengthInt > bufferSize {
				return nil
			}
			buffer := make([]byte, contentLengthInt)
			byteCount, err := request.Conn.Read(buffer)
			request.readedBytes = byteCount
			if byteCount != contentLengthInt || err != nil {
				return nil
			}
			return bytes.NewBuffer(buffer)
		}
	}
	return nil
}

func (request *SimpwebservRequest) DecodePOSTFormRequest() map[string]string { //解码POST的form表单
	formMap := make(map[string]string)
	if request.Method == "POST" {
		if contentType, ok := request.Header["Content-Type"]; ok {
			if contentLength, ok := request.Header["Content-Length"]; ok {
				if contentType == "application/x-www-form-urlencoded" {
					contentLengthInt, _ := strconv.Atoi(contentLength)
					if contentLengthInt > bufferSize {
						return formMap
					}
					buffer := make([]byte, contentLengthInt)
					byteCount, err := request.Conn.Read(buffer)
					request.readedBytes = byteCount
					if byteCount != contentLengthInt || err != nil {
						return formMap
					}
					dataString := string(buffer)
					formList := strings.Split(dataString, "&")
					var lineFirst string
					for i := 0; i < len(formList); i++ {
						lineList := strings.Split(formList[i], "=")
						if len(lineList) == 2 {
							lineFirst, _ = url.QueryUnescape(lineList[0])
							formMap[lineFirst], _ = url.QueryUnescape(lineList[1])
						}
					}
				}
			}
		}
	}
	return formMap
}

func (request *SimpwebservRequest) DecodeCookie() map[string]string { //解码cookie
	cookieMap := make(map[string]string)
	if cookie, ok := request.Header["Cookie"]; ok {
		cookieList := strings.Split(cookie, "; ")
		var cookieFirst string
		for i := 0; i < len(cookieList); i++ {
			singleCookieList := strings.Split(cookieList[i], "=")
			if len(singleCookieList) == 2 {
				cookieFirst, _ = url.QueryUnescape(singleCookieList[0])
				cookieMap[cookieFirst], _ = url.QueryUnescape(singleCookieList[1])
			}
		}
	}
	return cookieMap
}

func (response *SimpwebservResponse) SetCookie(cookieKey string, cookieValue string, expiresTime string, domain string, path string, secure bool, httpOnly bool) { //设置cookie
	cookieString := url.QueryEscape(cookieKey) + "=" + url.QueryEscape(cookieValue)
	if expiresTime != "" {
		cookieString = cookieString + "; Expires=" + getGMTTime(expiresTime)
	}
	if domain != "" {
		cookieString = cookieString + "; Domain=" + domain
	}
	if path != "" {
		cookieString = cookieString + "; Path=" + path
	}
	if secure {
		cookieString = cookieString + "; Secure"
	}
	if httpOnly {
		cookieString = cookieString + "; HttpOnly"
	}
	response.SetCookieList = append(response.SetCookieList, cookieString)
}

func SendStaticFile(path string, contentType string) *SimpwebservResponse { //传输一个静态文件
	f, err := os.Open(path)
	if err != nil {
		response := BuildNotFoundResponse()
		response.Body.Write([]byte("404 Not Found"))
		response.Header["Content-Type"] = "text/plain; charset=utf-8"
		return response
	}
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		response := BuildNotFoundResponse()
		response.Body.Write([]byte("404 Not Found"))
		response.Header["Content-Type"] = "text/plain; charset=utf-8"
		return response
	}
	response := BuildBasicResponse()
	response.Header["Content-Type"] = contentType
	response.Body.Write(data)
	return response
}

func (request *SimpwebservRequest) RecvFile(storePath string, name string, maxSize int) error { //储存提交的文件
	if request.Method == "POST" {
		if contentType, ok := request.Header["Content-Type"]; ok {
			if _, ok := request.Header["Content-Length"]; ok {
				contentTypeList := strings.Split(contentType, "; ")
				if contentTypeList[0] == "multipart/form-data" && len(contentTypeList) == 2 {
					boundaryList := strings.Split(contentTypeList[1], "=")
					if boundaryList[0] == "boundary" {
						boundary := boundaryList[1]
						if boundary[0] == '"' && boundary[len(boundary)-1] == '"' {
							boundary = boundary[1 : len(boundary)-1]
						}
						boundary = "--" + boundary
						var byteCount int
						var err error
						buffer := make([]byte, len(boundary)+2)
						readByteCount := 0
						byteCount, err = request.Conn.Read(buffer)
						readByteCount = readByteCount + byteCount
						if err != nil {
							return err
						}
						if byteCount != len(boundary)+2 || string(buffer) != boundary+"\r\n" {
							return IncorrectRequest
						}
						var data bytes.Buffer
						tempByte := make([]byte, 1)
						for i := 0; ; i++ { //获取头
							byteCount, err = request.Conn.Read(tempByte)
							readByteCount = readByteCount + byteCount
							if err != nil {
								return err
							}
							if byteCount != 1 {
								break
							}
							data.Write(tempByte)
							if i >= 3 {
								if bytes.Equal(data.Bytes()[i-3:i+1], []byte("\r\n\r\n")) {
									break
								}
							}
						}
						headerList := strings.Split(data.String(), "\r\n")
						data.Reset()
						headerList = headerList[:len(headerList)-2] //去掉最后的空项
						headerMap := make(map[string]string)
						for j := 0; j < len(headerList); j++ {
							lineList := strings.Split(headerList[j], ": ")
							if len(lineList) == 2 {
								headerMap[lineList[0]] = lineList[1]
							}
						}
						contentDispositionList := strings.Split(headerMap["Content-Disposition"], "; ")
						if contentDispositionList[0] == "form-data" {
							contentDispositionMap := make(map[string]string)
							for j := 1; j < len(contentDispositionList); j++ {
								contentDispositionParameter := strings.Split(contentDispositionList[j], "=")
								contentDispositionMap[contentDispositionParameter[0]] = contentDispositionParameter[1]
							}
							if filename, ok := contentDispositionMap["filename"]; ok {
								if name == "" {
									if filename[0] == '"' && filename[len(filename)-1] == '"' {
										filename = filename[1 : len(filename)-1]
									}
								} else {
									filename = name
								}
								f, err := os.OpenFile(storePath+"/"+filename, os.O_WRONLY|os.O_CREATE, 0666)
								defer f.Close()
								buffer = make([]byte, bufferSize)
								var byteList [][]byte
								var fileByteCount int
								lastBytes := make([]byte, len(boundary)+2)
								allByteCount := 0
								for {
									byteCount, err = request.Conn.Read(buffer)
									readByteCount = readByteCount + byteCount
									if err != nil {
										os.Remove(storePath + "/" + filename)
										return err
									}
									byteList = bytes.Split(append(lastBytes, buffer...), []byte("\r\n"+boundary))
									if byteCount < bufferSize && len(byteList) == 1 { //特殊处理返回的长度不足bufferSize的数据包
										fileByteCount, err = f.Write(byteList[0][len(boundary)+2 : byteCount+len(boundary)+2])
										if err != nil {
											os.Remove(storePath + "/" + filename)
											return err
										}
										if fileByteCount != byteCount {
											os.Remove(storePath + "/" + filename)
											return IncompleteFile
										}
										allByteCount = allByteCount + fileByteCount
										if allByteCount > maxSize && maxSize != 0 {
											os.Remove(storePath + "/" + filename)
											return FileOverSize
										}
										continue
									}
									fileByteCount, err = f.Write(byteList[0][len(boundary)+2:])
									if err != nil {
										os.Remove(storePath + "/" + filename)
										return err
									}
									if fileByteCount != len(byteList[0])-len(boundary)-2 {
										os.Remove(storePath + "/" + filename)
										return IncompleteFile
									}
									allByteCount = allByteCount + fileByteCount
									if allByteCount > maxSize && maxSize != 0 {
										os.Remove(storePath + "/" + filename)
										return FileOverSize
									}
									if len(byteList) > 1 {
										break
									}
									lastBytes = byteList[0][len(byteList[0])-len(boundary)-2 : len(byteList[0])]
								}
								f.Close()
							}
						}
						request.readedBytes = readByteCount
						return nil
					}
				}
			}
		}
	}
	return IncorrectRequest
}

func SendFile(request *SimpwebservRequest, contentType string, filePath string, fileName string) *SimpwebservResponse { //支持断点续传的文件下载（占用内存小）
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	response := BuildBasicResponse()
	response.Header["Accept-Ranges"] = "bytes"
	response.Header["Content-Disposition"] = "attachment; filename=" + fileName
	response.Header["Content-Type"] = contentType
	f, err := os.Stat(filePath)
	if err != nil {
		response := BuildNotFoundResponse()
		response.Body.Write([]byte("404 Not Found"))
		response.Header["Content-Type"] = "text/plain; charset=utf-8"
		return response
	}
	fileEnd := f.Size() - 1
	startPos := 0
	endPos := int(fileEnd)
	response.Header["Content-Length"] = strconv.Itoa(endPos - startPos + 1)
	if dataRange, ok := request.Header["Range"]; ok {
		dataRangeList := strings.Split(dataRange, "=")
		if len(dataRangeList) == 2 {
			rangeList := strings.Split(dataRangeList[1], "-")
			if len(rangeList) == 2 {
				response.Code = "206"
				response.CodeName = "Partial Content"
				if rangeList[0] != "" {
					startPos, _ = strconv.Atoi(rangeList[0])
				}
				if rangeList[1] != "" {
					endPos, _ = strconv.Atoi(rangeList[1])
				}
				if endPos > int(fileEnd) {
					endPos = int(fileEnd)
				}
				if startPos > endPos {
					startPos = endPos
				}
				response.Header["Content-Length"] = strconv.Itoa(endPos - startPos + 1)
				response.Header["Content-Range"] = "bytes " + strconv.Itoa(startPos) + "-" + strconv.Itoa(endPos) + "/" + strconv.Itoa(int(fileEnd)+1)
			}
		}
	}
	response.ToDoCommand = "SendFile " + strconv.Itoa(startPos) + " " + strconv.Itoa(endPos) + " " + filePath
	return response
}

func panicTrace() []byte { //获取pannic堆栈信息
	s := []byte("/src/runtime/panic.go")
	e := []byte("\ngoroutine ")
	line := []byte("\n")
	stack := make([]byte, bufferSize)
	length := runtime.Stack(stack, true)
	start := bytes.Index(stack, s)
	stack = stack[start:length]
	start = bytes.Index(stack, line) + 1
	stack = stack[start:]
	end := bytes.LastIndex(stack, line)
	if end != -1 {
		stack = stack[:end]
	}
	end = bytes.Index(stack, e)
	if end != -1 {
		stack = stack[:end]
	}
	stack = bytes.TrimRight(stack, "\n")
	return stack
}

func runFunction(request *SimpwebservRequest, app *SimpwebservApp) *SimpwebservResponse { //通过path搜索函数并运行获取返回值
	if request.PurePath == "/" && app.UrlMap.Function != nil { //对于根目录的特殊处理
		return app.UrlMap.Function(request)
	}
	pathList := strings.Split(request.PurePath, "/")[1:]
	nowNode := &app.UrlMap
	var tempNode *SimpwebservUrlNode
	for i := 0; i < len(pathList); i++ {
		j := nowNode.NextLayer.Front()
		for {
			if j == nil { //路径没注册返回404
				if app.NotFoundHandler != nil {
					return app.NotFoundHandler(request)
				} else {
					response := BuildNotFoundResponse()
					response.Body.Write([]byte("404 Not Found"))
					response.Header["Content-Type"] = "text/plain; charset=utf-8"
					return response
				}
			}
			tempNode, _ = (j.Value).(*SimpwebservUrlNode)
			if tempNode.Name == pathList[i] {
				nowNode = tempNode
				if nowNode.IncludeBack {
					response := nowNode.Function(request)
					return response
				}
				break
			}
			j = j.Next()
		}
	}
	if nowNode.Function == nil { //函数不存在返回404
		if app.NotFoundHandler != nil {
			return app.NotFoundHandler(request)
		} else {
			response := BuildNotFoundResponse()
			response.Body.Write([]byte("404 Not Found"))
			response.Header["Content-Type"] = "text/plain; charset=utf-8"
			return response
		}
	}
	response := nowNode.Function(request)
	return response
}

func connectionHandler(conn net.Conn, app *SimpwebservApp, num int) { //处理连接
	defer func() { //处理pannic
		if err := recover(); err != nil {
			var response *SimpwebservResponse
			if app.InternalServerErrorHandler != nil {
				response = app.InternalServerErrorHandler((err).(error))
			} else {
				response = BuildInternalServerErrorResponse()
				response.Header["Content-Type"] = "text/plain; charset=utf-8"
				response.Body.Write([]byte("500 Internal Server Error"))
				if app.DebugMode {
					response.Body.Write([]byte("\r\n"))
					response.Body.Write(panicTrace())
				}
			}
			conn.Write([]byte(response.Protocol + " " + response.Code + " " + response.CodeName + "\r\n"))
			header := ""
			for key, value := range response.Header {
				header = header + key + ": " + value + "\r\n"
			}

			if len(response.SetCookieList) != 0 {
				for i := 0; i < len(response.SetCookieList); i++ {
					header = header + "Set-Cookie: " + response.SetCookieList[i] + "\r\n"
				}
			}

			header = header + "\r\n"
			conn.Write([]byte(header))
			conn.Write(response.Body.Bytes())
			response.Body.Reset()
		}
		conn.Close()
		runtime.GC()
	}()
	var request SimpwebservRequest
	tempByte := make([]byte, 1)
	var err error
	var byteCount int
	var data bytes.Buffer
	var headerKey string
	var headerValue string
	byteCount++
	for {
		request = SimpwebservRequest{"", "", "", "", "", make(map[string]string), conn, 0}
		request.Host = conn.RemoteAddr().String()
		for times := 0; times < 3; times++ { //获取请求头
			for i := 0; ; i++ {
				byteCount, err = conn.Read(tempByte)
				if err != nil {
					conn.Close()
					return
				}
				if tempByte[0] == ' ' {
					break
				}
				data.Write(tempByte)
				if data.Len() >= 2 {
					if data.String()[data.Len()-2:] == "\r\n" {
						break
					}
				}
			}
			if times == 0 {
				if data.String()[0:2] == "\r\n" {
					request.Method = data.String()[2:data.Len()]
				} else {
					request.Method = data.String()
				}
			} else if times == 1 {
				request.Path = data.String()
				request.PurePath, _ = url.QueryUnescape(strings.Split(request.Path, "?")[0])
			} else {
				request.Protocol = data.String()[:data.Len()-2]
			}
			data.Reset()
		}
		for {
			for i := 0; ; i++ {
				byteCount, err = conn.Read(tempByte)
				if err != nil {
					conn.Close()
					return
				}
				data.Write(tempByte)
				if data.Len() >= 2 {
					if data.String()[data.Len()-2:] == ": " {
						headerKey = data.String()[0 : data.Len()-2]
						break
					}
					if data.String()[data.Len()-2:] == "\r\n" && data.Len() == 2 {
						goto recvHeaderEnd
					}
				}
			}
			data.Reset()
			for i := 0; ; i++ {
				byteCount, err = conn.Read(tempByte)
				if err != nil {
					conn.Close()
					return
				}
				data.Write(tempByte)
				if data.Len() >= 2 {
					if data.String()[data.Len()-2:] == "\r\n" {
						headerValue = data.String()[0 : data.Len()-2]
						break
					}
				}
			}
			data.Reset()
			request.Header[headerKey] = headerValue
		}

	recvHeaderEnd:
		response := runFunction(&request, app) //生成响应

		if contentLength, ok := request.Header["Content-Length"]; ok { //清空读取缓冲区
			contentLengthInt, _ := strconv.Atoi(contentLength)
			wasteByte := make([]byte, bufferSize)
			byteCount := 0
			allByteCount := 0
			for {
				if allByteCount+request.readedBytes == contentLengthInt {
					break
				}
				byteCount, err = conn.Read(wasteByte)
				if err != nil {
					conn.Close()
					return
				}
				allByteCount = allByteCount + byteCount
			}
		}

		commandList := strings.Split(response.ToDoCommand, " ") //解析命令
		var startPos int
		var endPos int
		var filePath string
		if len(commandList) != 0 {
			if commandList[0] == "SendFile" { //目前只支持下载文件的命令
				startPos, _ = strconv.Atoi(commandList[1])
				endPos, _ = strconv.Atoi(commandList[2])
				filePath = strings.Join(commandList[3:], " ")
			} else {
				response.Header["Content-Length"] = strconv.Itoa(response.Body.Len())
			}
		} else {
			response.Header["Content-Length"] = strconv.Itoa(response.Body.Len())
		}

		response.Header["Connection"] = "keep-alive" //支持keep-alive了

		if app.EnableConsoleLog {
			log.Println(request.Host + " " + request.Method + " " + request.PurePath + " " + response.Code + " " + response.CodeName)
		}

		conn.Write([]byte(response.Protocol + " " + response.Code + " " + response.CodeName + "\r\n"))
		header := ""
		for key, value := range response.Header {
			header = header + key + ": " + value + "\r\n"
		}

		if len(response.SetCookieList) != 0 {
			for i := 0; i < len(response.SetCookieList); i++ {
				header = header + "Set-Cookie: " + response.SetCookieList[i] + "\r\n"
			}
		}

		header = header + "\r\n"
		conn.Write([]byte(header))
		if request.Method != "HEAD" {
			if len(commandList) != 0 {
				if commandList[0] == "SendFile" { //下载文件的分段读取发送
					f, _ := os.Open(filePath)
					f.Seek(int64(startPos), io.SeekStart)
					readLength := endPos - startPos + 1
					buffer := make([]byte, bufferSize)
					i := 0
					for {
						if readLength <= i+bufferSize {
							break
						}
						byteCount, err := f.Read(buffer)
						if err != nil {
							log.Println(err.Error())
							return
						}
						conn.Write(buffer)
						i = i + byteCount
					}
					buffer = make([]byte, readLength-i)
					_, err := f.Read(buffer)
					if err != nil {
						log.Println(err.Error())
						return
					}
					conn.Write(buffer)
				} else {
					conn.Write(response.Body.Bytes())
				}
			} else {
				conn.Write(response.Body.Bytes())
			}
			response.Body.Reset()
		}
		runtime.GC()
	}
}

func (app *SimpwebservApp) SetHTTPS(pemPath string, keyPath string) error { //设置TLS
	cert, err := tls.LoadX509KeyPair(pemPath, keyPath)
	if err != nil {
		return err
	}
	app.UseHTTPS = true
	app.HTTPSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
	return nil
}

func (app *SimpwebservApp) SetDebugMode(onoff bool) { //设置DebugMode（就是在出现500时默认会不会在网页上显示err）
	app.DebugMode = onoff
}

func (app *SimpwebservApp) SetEnableConsoleLog(onoff bool) { //设置EnableConsoleLog（是否在命令行里显示访问信息，可能对qps有影响）
	app.EnableConsoleLog = onoff
}

func (app *SimpwebservApp) Run(host string, port uint16) { //运行实例
	allHost := host + ":" + strconv.Itoa(int(port))
	if app.UseHTTPS {
		log.Println("Server is starting at: https://" + allHost)
	} else {
		log.Println("Server is starting at: http://" + allHost)
	}

	var err error
	if app.UseHTTPS {
		app.Listener, err = tls.Listen("tcp", allHost, app.HTTPSConfig)
	} else {
		app.Listener, err = net.Listen("tcp", allHost)
	}
	if err != nil {
		log.Fatal("Server listen error: " + err.Error())
		return
	}
	i := 0
	for {
		conn, err := app.Listener.Accept()
		if err != nil {
			log.Fatal("Server accept error: " + err.Error())
			continue
		}
		runtime.GC()
		go connectionHandler(conn, app, i)
		i++
	}
}
