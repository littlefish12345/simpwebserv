package main
import (
	"fmt"
	"simpwebserv"
)

func root(r *simpwebserv.SimpwebservRequest) *simpwebserv.SimpwebservResponse {
	fmt.Println("root")
	response := simpwebserv.BuildResponse()
	response.Body.Write([]byte("<!DOCTYPE html><html><body><h1>root</h1></body></html>"))
	return response
}

func index(r *simpwebserv.SimpwebservRequest) *simpwebserv.SimpwebservResponse {
	fmt.Println("index")
	response := simpwebserv.BuildResponse()
	response.Body.Write([]byte("<!DOCTYPE html><html><body><h1>index</h1></body></html>"))
	return response
}


func main() {
	app := simpwebserv.App()
	app.Register(root, "//")
	//app.Register(index, "/index/")
	app.Run("192.168.15.175", 5000)
}