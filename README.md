# simpwebserv

## 简介

一个简单的给python的http服务器库

## 使用方法

首先你得import这个库
```
import simpwebserv
```

然后创建一个服务器实例
```
app = simpwebserv.server()
```

之后添加你想要的页面的函数
```
def some_page():
	do something
	return http_need_to_send
```

然后注册这个函数到要的路径上
```
app.register(some_page,'/path/you/want/to')
```

最后运行这个实例
```
app.run()
```

然后你就可以在127.0.0.1:5000上看到你的页面了