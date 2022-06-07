package simpwebserv

import (
	"crypto/tls"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

func App() *AppStruct { //创建一个app实例
	app := AppStruct{nil, &UrlNode{make(map[string]*UrlNode), false, nil}, false, nil, nil, nil, false, false, false, 4, 60}
	return &app
}

func (app *AppStruct) Register(function func(*Request) *Response, path string, includeBack bool) { //注册一个路径到一个函数上
	pathSplit := strings.Split(path, "/")
	pathSplit = pathSplit[1:]
	if pathSplit[len(pathSplit)-1] == "" {
		pathSplit = pathSplit[0 : len(pathSplit)-1]
	}
	nowNode := app.urlRootNode
	for i := 0; i < len(pathSplit); i++ {
		if v, ok := nowNode.NextLayer[pathSplit[i]]; ok {
			nowNode = v
		} else {
			nowNode.NextLayer[pathSplit[i]] = &UrlNode{make(map[string]*UrlNode), false, nil}
			nowNode = nowNode.NextLayer[pathSplit[i]]
		}
	}
	nowNode.IncludeBack = includeBack
	nowNode.Function = function
}

func (app *AppStruct) SetTls(pemPath string, keyPath string) error { //设置TLS
	cert, err := tls.LoadX509KeyPair(pemPath, keyPath)
	if err != nil {
		return err
	}
	app.useTls = true
	app.HTTPSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
	return nil
}

func (app *AppStruct) SetDebugMode(onoff bool) { //设置debugMode（就是在出现500时默认会不会在网页上显示堆栈跟踪）
	app.debugMode = onoff
}

func (app *AppStruct) SetEnableConsoleLog(onoff bool) { //设置enableConsoleLog（是否在命令行里显示访问信息，可能对qps有影响）
	app.enableConsoleLog = onoff
}

func (app *AppStruct) SetEnableKeepAlive(onoff bool) { //设置enableKeepAlive（是否在命令行里显示访问信息，可能对qps有影响）
	app.enableKeepAlive = onoff
}

func (app *AppStruct) SetKeepAliveTimeout(timeout uint64) { //设置keepAliveTimeout（启用keep-alive的时候的连接超时时间）
	app.keepAliveTimeout = time.Duration(timeout)
}

func (app *AppStruct) loadConfig(config Config) error {
	app.debugMode = config.DebugMode
	app.enableConsoleLog = !config.DisableConsoleLog
	app.enableKeepAlive = !config.DisableKeepAlive
	if config.KeepAliveTimeout != 0 {
		app.keepAliveTimeout = config.KeepAliveTimeout
	}
	if config.UseTls {
		cert, err := tls.LoadX509KeyPair(config.TlsPemPath, config.TlsKeyPath)
		if err != nil {
			return err
		}
		app.useTls = true
		app.HTTPSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
	}
	if config.MultiThreadAcceptNum != 0 {
		app.multiThreadAcceptNum = config.MultiThreadAcceptNum
	}
	return nil
}

func (app *AppStruct) Run(config Config) { //运行服务
	allHost := config.Host + ":" + strconv.Itoa(int(config.Port))
	err := app.loadConfig(config)
	if err != nil {
		panic(err)
	}

	if app.useTls {
		app.listener, err = tls.Listen("tcp", allHost, app.HTTPSConfig)
	} else {
		app.listener, err = net.Listen("tcp", allHost)
	}
	if err != nil {
		log.Fatal("Server listen error: " + err.Error())
		return
	}

	if app.useTls {
		if config.Port == 443 {
			log.Println("Server is starting at: https://" + config.Host)
		} else {
			log.Println("Server is starting at: https://" + allHost)
		}
	} else {
		if config.Port == 80 {
			log.Println("Server is starting at: http://" + config.Host)
		} else {
			log.Println("Server is starting at: http://" + allHost)
		}
	}
	for i := 0; i < int(app.multiThreadAcceptNum)-1; i++ {
		go app.acceptConn()
	}
	app.acceptConn()
}

func (app *AppStruct) acceptConn() {
	for {
		conn, err := app.listener.Accept()
		if err != nil {
			log.Fatal("Server accept error: " + err.Error())
			continue
		}
		go connectionHandler(conn, app)
	}
}
