package simpwebserv

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func PanicTrace() []byte {
	s := []byte("/src/runtime/panic.go")
	e := []byte("\ngoroutine ")
	line := []byte("\n")
	stack := make([]byte, 4096)
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

func getFunction(app *AppStruct, path string) (bool, func(*Request) *Response) { //按照url路径获取对应的函数
	if path == "/" {
		if app.urlRootNode.Function != nil {
			return true, app.urlRootNode.Function
		}
		return false, nil
	}
	pathSplit := strings.Split(path, "/")
	pathSplit = pathSplit[1:]
	if len(pathSplit) == 0 {
		return false, nil
	}
	if pathSplit[len(pathSplit)-1] == "" {
		pathSplit = pathSplit[0 : len(pathSplit)-1]
	}
	nowNode := app.urlRootNode
	for i := 0; i < len(pathSplit); i++ {
		if v, ok := nowNode.NextLayer[pathSplit[i]]; ok {
			nowNode = v
		} else {
			if nowNode.IncludeBack {
				return true, nowNode.Function
			}
			return false, nil
		}
	}
	return true, nowNode.Function
}

func httpReadHeaderLine(conn net.Conn, buffer *[]byte, recv *bytes.Buffer, first *string, second *string, err *error, i *int) { //读HTTP的一行header
	for {
		*i, *err = conn.Read(*buffer)
		if *err != nil {
			return
		}
		if *i == 0 {
			continue
		}
		if (*buffer)[0] == ':' || (*buffer)[0] == '\n' {
			*first = recv.String()
			recv.Reset()
			if (*buffer)[0] == ':' {
				conn.Read(*buffer)
				break
			}
			*first = ""
			*second = ""
			return
		}
		recv.Write(*buffer)
		if recv.Len() > bufferMaxSize {
			*err = ErrBufferTooBig
			return
		}
	}
	for {
		*i, *err = conn.Read(*buffer)
		if *err != nil {
			return
		}
		if *i == 0 {
			continue
		}
		if (*buffer)[0] == '\n' {
			*second = recv.String()
			*second = (*second)[:len(*second)-1]
			recv.Reset()
			break
		}
		recv.Write(*buffer)
		if recv.Len() > bufferMaxSize {
			*err = ErrBufferTooBig
			return
		}
	}
}

func connectionHandler(conn net.Conn, app *AppStruct) { //处理请求
	var request Request
	var buffer []byte
	var recv *bytes.Buffer = new(bytes.Buffer)
	var err error
	var i int
	var n int
	var i64 int64
	var first string
	var second string
	var found bool
	var function func(*Request) *Response
	var response *Response
	var v string
	var ok bool

	defer func() { //错误处理
		if r := recover(); r != nil {
			response = Build500DefaultResponse()
			if app.debugMode {
				response.Body.Reset()
				data := r.(error).Error() + "\n" + string(PanicTrace())
				fmt.Println(data)
				lineSplit := strings.Split(data, "\n")
				response.Body.WriteString("<html><body>")
				for i = 0; i < len(lineSplit); i++ {
					response.Body.WriteString("<p>")
					response.Body.WriteString(lineSplit[i])
					response.Body.WriteString("</p>")
				}
				response.Body.WriteString("</body></html>")
			}
			response.Header["Content-length"] = strconv.Itoa(response.Body.Len())
			request.SendHeader(response)
			conn.Write(response.Body.Bytes())
			if app.enableConsoleLog {
				log.Println(request.Host + " " + request.Method + " " + request.Path + " " + response.Code + " " + response.CodeName)
			}
			conn.Close()
		}
	}()

	for {
		request = Request{conn, app.enableKeepAlive, []byte{}, app.keepAliveTimeout, 0, "", "", "", "", "", make(map[string]string), 0, 0}
		request.Host = conn.RemoteAddr().String()
		buffer = make([]byte, 1)
		i = 0
		first = ""
		second = ""

		if app.enableKeepAlive {
			conn.SetReadDeadline(time.Now().Add(time.Second * app.keepAliveTimeout))
		}
		for { //读请求方法
			i, err = conn.Read(buffer)
			if err != nil {
				conn.Close()
				return
			}
			if i == 0 {
				continue
			}
			if buffer[0] == ' ' {
				request.Method = recv.String()
				//fmt.Println(request.Method)
				recv.Reset()
				break
			}
			recv.Write(buffer)
			if recv.Len() > bufferMaxSize {
				panic(ErrBufferTooBig)
			}
		}

		for { //读纯路径
			i, err = conn.Read(buffer)
			if err != nil {
				conn.Close()
				return
			}
			if i == 0 {
				continue
			}
			if buffer[0] == ' ' || buffer[0] == '?' {
				request.Path = recv.String()
				//fmt.Println(request.Path)
				recv.Reset()
				if buffer[0] == ' ' {
					break
				}
				for { //读URL传参
					i, err = conn.Read(buffer)
					if err != nil {
						conn.Close()
						return
					}
					if i == 0 {
						continue
					}
					if buffer[0] == ' ' {
						request.UrlParameter = recv.String()
						//fmt.Println(request.UrlParameter)
						recv.Reset()
						break
					}
					recv.Write(buffer)
					if recv.Len() > bufferMaxSize {
						panic(ErrBufferTooBig)
					}
				}
				recv.Reset()
				break
			}
			recv.Write(buffer)
			if recv.Len() > bufferMaxSize {
				panic(ErrBufferTooBig)
			}
		}

		for { //读协议
			i, err = conn.Read(buffer)
			if err != nil {
				conn.Close()
				return
			}
			if i == 0 {
				continue
			}
			if buffer[0] == '\r' {
				request.Protocol = recv.String()
				//fmt.Println(recv.Bytes())
				recv.Reset()
				if app.enableKeepAlive {
					conn.SetReadDeadline(time.Now().Add(time.Second * app.keepAliveTimeout))
				}
				conn.Read(buffer)
				break
			}
			recv.Write(buffer)
			if recv.Len() > bufferMaxSize {
				panic(ErrBufferTooBig)
			}
		}

		for { //读header
			if app.enableKeepAlive {
				conn.SetReadDeadline(time.Now().Add(time.Second * app.keepAliveTimeout))
			}
			httpReadHeaderLine(conn, &buffer, recv, &first, &second, &err, &i)
			if err != nil {
				conn.Close()
				return
			}
			//fmt.Println(first, second)
			if first == "" || second == "" {
				break
			}
			request.Header[first] = second
		}

		if v, ok = request.Header["Content-Length"]; ok { //获取content-length
			i64, err = strconv.ParseInt(v, 10, 64)
			request.bodyBytes = uint64(i64)
			if err != nil {
				panic(err)
			}
		}

		found, function = getFunction(app, request.Path)
		if !found {
			response = Build404Response()
		} else {
			response = function(&request)
		}

		if app.enableConsoleLog {
			log.Println(request.Host + " " + request.Method + " " + request.Path + " " + response.Code + " " + response.CodeName)
		}

		if !response.sendedHeader {
			response.Header["Content-length"] = strconv.Itoa(response.Body.Len())
			if app.enableKeepAlive {
				response.Header["Connection"] = "keep-alive"
			} else {
				response.Header["Connection"] = "close"
			}
			request.SendHeader(response)
		}

		if request.Method != "HEAD" {
			n, err = conn.Write(response.Body.Bytes())
			if n != response.Body.Len() || err != nil {
				conn.Close()
				return
			}
		}

		if !app.enableKeepAlive {
			break
		}

		if app.enableKeepAlive {
			conn.SetReadDeadline(time.Now().Add(time.Second * app.keepAliveTimeout))
		}
		for request.bodyBytes > request.readedBytes { //清空body
			i, err = request.ConnRead(buffer)
			if uint64(i) != request.bodyBytes-request.readedBytes || err != nil {
				conn.Close()
				return
			}
		}
	}
}
