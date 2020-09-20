from urllib import parse
import threading
import traceback
import socket
import ssl

__version__ = '0.1'
__author__ = ['little_fish12345']

__simpwebserv_coding__ = 'utf-8'
__simpwebserv_buffer_size__ = 524288

class server():
    def __init__(self):
        self.full_function_path_map = {}
        self.full_function_path_requier_map = {}
    def register(self,func,path, #注册一个路径没有参数的页面
                 accept_methods=['GET'], #这个函数接受的请求类型
                 requier_args=False, #是否要求一个名为args的字典传入参数
                 requier_cookie=False, #是否在传参字典里加入一个键名为cookie的字典
                 requier_get_parameter=False, #是否在传参字典里加入一个键名为get_parameter的字典
                 requier_post_parameter=False, #是否在传参字典里加入一个键名为post_parameter的字典
                 requier_header=False, #是否在传参字典里加入一个键名为header的字典
                 requier_body=False, #是否在传参字典里加入一个键名为body的byte
                 requier_method=False): #是否在传参字典里加入一个键名为method的字符串
        for i in accept_methods:
            self.full_function_path_map[(path,i)] = func
            self.full_function_path_requier_map[(path,i)] = (requier_args,requier_cookie,requier_get_parameter,requier_post_parameter,requier_header,requier_body,requier_method)
    def run(self,port=5000,host='127.0.0.1',debug=False,KeepAlive=False): #KeepAlive没做好
        def __simpwebserv_process_func__(conn,addr,full_function_path_map,full_function_path_requier_map):
            data = conn.recv(__simpwebserv_buffer_size__)
            data_split = data.split(b'\r\n\r\n')
            header_split = data_split[0].decode(__simpwebserv_coding__).split('\r\n') #报文头
            body = data_split[0] #报文体
            header_first_line = header_split[0].split(' ')
            del header_split[0]
            method = header_first_line[0] #请求方法
            path = header_first_line[1] #路径
            http_version = header_first_line[2] #http协议以及版本
            header_map = {}
            for i in header_split: #解析header
                i_split = i.split(': ')
                header_map[i_split[0]] = i_split[1]
            try:
                while True: #如果没有获取完就不断获取
                    if len(data) < int(header_map['Content-Length']):
                        body = body + conn.recv(__simpwebserv_buffer_size__)
                    else:
                        break
            except KeyError as e:
                pass #没做好
            if 'Cookie' in header_map:
                cookie_split = header_map['Cookie'].split('; ') #解析cookie
                cookie = {} #cookie
                for i in cookie_split:
                    i_split = i.split('=')
                    cookie[parse.unquote(i_split[0])] = parse.unquote(i_split[1])
            if (method == 'GET' or method == 'HEAD') and len(path.split('?'))>=2: #GET请求传参处理
                get_parameter_url_list = path.split('?')[-1].split('&')
                path = '?'.join(path.split('?')[0:-1])
                get_parameter_map = {} #get参数
                for i in get_parameter_url_list:
                    i_split = i.split('=')
                    get_parameter_map[parse.unquote(i_split[0])] = parse.unquote(i_split[1])
            if method == 'POST': #POST请求传参处理
                Content_Type_list = header_map['Content-Type'].split('; ')
                if Content_Type_list[0] == 'application/x-www-form-urlencoded': #原生form
                    post_parameter_body_list = body.deocde(__simpwebserv_coding__).split('&')
                    post_parameter_map = {} #post参数
                    for i in post_parameter_body_list:
                        i_split = i.split('&')
                        post_parameter_map[parse.unquote(i_split[0])] = parse.unquote(i_split[1])
                if Content_Type_list[0] == 'multipart/form-data': #文件上传form
                    pass #没做好
            try:
                if full_function_path_requier_map[(path,method)][0]:
                    args = {}
                    if full_function_path_requier_map[(path,method)][1]:
                        if 'cookie' in vars():
                            args['cookie'] = cookie
                        else:
                            args['cookie'] = {}
                    if full_function_path_requier_map[(path,method)][2]:
                        if 'get_parameter_map' in vars():
                            args['get_parameter'] = get_parameter_map
                        else:
                            args['get_parameter'] = {}
                    if full_function_path_requier_map[(path,method)][3]:
                        if 'post_parameter_map' in vars():
                            args['post_parameter'] = get_parameter_map
                        else:
                            args['post_parameter'] = {}
                    if full_function_path_requier_map[(path,method)][4]:
                        args['header'] = header_split
                    if full_function_path_requier_map[(path,method)][5]:
                        args['body'] = body
                    if full_function_path_requier_map[(path,method)][6]:
                        args['method'] = method
                    result = full_function_path_map[(path,method)](args=args)
                else:
                    result = full_function_path_map[(path,method)]()
                if isinstance(result,str):
                    conn.send(('HTTP/1.1 200 OK\r\nServer: python/simpwebserv\r\nConnection: close\r\n\r\n'+result).encode(__simpwebserv_coding__))
                    conn.close()
                    status_code = '200'
                else:
                    pass #没做好
            except Exception as e:
                if debug:
                    conn.send(('HTTP/1.1 500 ERROR\r\nServer: python/simpwebserv\r\nConnection: close\r\n\r\n500 error\r\n\r\nlog:\r\n'+traceback.format_exc()).encode(__simpwebserv_coding__))
                else:
                    conn.send('HTTP/1.1 500 ERROR\r\nServer: python/simpwebserv\r\nConnection: close\r\n\r\n500 error'.encode(__simpwebserv_coding__))
                conn.close()
                status_code = '500'
            print(method+' '+path+' '+status_code+' '+addr[0])
        sock = socket.socket()
        sock.bind((host,port))
        sock.listen(0)
        while True:
            conn,addr = sock.accept()
            t = threading.Thread(target=__simpwebserv_process_func__,args=(conn,addr,self.full_function_path_map,self.full_function_path_requier_map))
            t.start()
