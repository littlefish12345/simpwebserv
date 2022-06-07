package simpwebserv

import (
	"bytes"
	"crypto/tls"
	"net"
	"time"
)

type Response struct { //响应的结构体
	Protocol      string
	Code          string
	CodeName      string
	Header        map[string]string
	Body          *bytes.Buffer
	SetCookieList []string
	sendedHeader  bool
}

type Request struct { //请求的结构体
	conn             net.Conn
	enableKeepAlive  bool
	readRestData     []byte
	keepAliveTimeout time.Duration
	bodyReaded       uint64
	Method           string
	Path             string
	UrlParameter     string
	Protocol         string
	Host             string
	Header           map[string]string
	bodyBytes        uint64
	readedBytes      uint64
}

type UrlNode struct { //单个path的节点
	NextLayer   map[string]*UrlNode
	IncludeBack bool
	Function    func(*Request) *Response
}

type AppStruct struct { //实例的结构体
	listener                   net.Listener
	urlRootNode                *UrlNode
	useTls                     bool
	HTTPSConfig                *tls.Config
	notFoundHandler            func(*Request) *Response
	internalServerErrorHandler func(error) *Response
	debugMode                  bool
	enableConsoleLog           bool
	enableKeepAlive            bool
	multiThreadAcceptNum       uint16
	keepAliveTimeout           time.Duration
}

type Config struct {
	Host                 string
	Port                 uint16
	UseTls               bool
	DebugMode            bool
	DisableConsoleLog    bool
	DisableKeepAlive     bool
	KeepAliveTimeout     time.Duration
	TlsPemPath           string
	TlsKeyPath           string
	MultiThreadAcceptNum uint16
}
