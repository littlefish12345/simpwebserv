from urllib import parse
import threading
import traceback
import datetime
import socket
import time
import ssl

__version__ = '0.1'
__author__ = ['little_fish12345']

__simpwebserv_coding__ = 'utf-8'
__simpwebserv_buffer_size__ = 524288

class response():
    def __init__(self):
        self.status_code = '200'
        self.status_code_text = 'OK'
        self.body = b''
        self.Content_Type = 'text/html'
        self.Content_Disposition = None
        self.Set_Cookie = None
        self.Location = None
    def set_status_code(self,status_code,text):
        self.status_code = status_code
        self.status_code_text = text
    def set_Content_Type(self,Content_Type):
        self.Content_Type = Content_Type
    def set_Content_Disposition(self,Content_Disposition):
        self.Content_Disposition = Content_Disposition
    def set_Cookie(self,key,value, #键值
                   path=None, #指定路径
                   domain=None, #指定域名
                   maxage=None): #过期时间(s)
        if self.Set_Cookie == None:
            self.Set_Cookie = []
        set_cookie_text = 'Set-Cookie: '+parse.quote(key)+'='+parse.quote(value)
        if path != None:
            set_cookie_text = set_cookie_text+'; Path='+path
        if domain != None:
            set_cookie_text = set_cookie_text+'; Domain='+domain
        if maxage != None:
            set_cookie_text = set_cookie_text+'Max-Age='+str(maxage)
        self.Set_Cookie.append(set_cookie_text)
    def del_Cookie(self,key, #键
                   path=None, #指定路径
                   domain=None): #指定域名
        if self.Set_Cookie == None:
            self.Set_Cookie = []
        set_cookie_text = 'Set-Cookie: '+parse.quote(key)+'='
        if path != None:
            set_cookie_text = set_cookie_text+'; Path='+path
        if domain != None:
            set_cookie_text = set_cookie_text+'; Domain='+domain
        set_cookie_text = set_cookie_text+'Max-Age=0'
        self.Set_Cookie.append(set_cookie_text)
    def jump_to(self,Location):
        self.Location = Location
        self.status_code = '302'
        self.status_code_text = 'JUMP'
    def set_text(self,text):
        self.Content_Type = 'text/plain'
        self.body = text.encode(__simpwebserv_coding__)
    def set_html(self,html):
        self.Content_Type = 'text/html'
        self.body = html.encode(__simpwebserv_coding__)
    def set_css(self,css):
        self.Content_Type = 'text/css'
        self.body = css.encode(__simpwebserv_coding__)
    def set_js(self,js):
        self.Content_Type = 'application/x-javascript'
        self.body = js.encode(__simpwebserv_coding__)
    def transform_file(self,file,filename):
        self.Content_Type = 'application/octet-stream'
        self.body = file
        self.Content_Disposition = 'attachment; filename='+filename

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
            #t1 = time.time()
            data = conn.recv(__simpwebserv_buffer_size__)
            if data == b'':
                conn.close()
                return
            data_split = data.split(b'\r\n\r\n')
            header_split = data_split[0].decode(__simpwebserv_coding__).split('\r\n') #报文头
            body = data_split[1] #报文体
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
                    if len(body) < int(header_map['Content-Length']):
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
                    post_parameter_body_list = body.decode(__simpwebserv_coding__).split('&')
                    post_parameter_map = {} #post参数
                    for i in post_parameter_body_list:
                        i_split = i.split('=')
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
                            args['post_parameter'] = post_parameter_map
                        else:
                            args['post_parameter'] = {}
                    if full_function_path_requier_map[(path,method)][4]:
                        args['header'] = header_split
                    if full_function_path_requier_map[(path,method)][5]:
                        args['body'] = body
                    if full_function_path_requier_map[(path,method)][6]:
                        if method == 'HEAD':
                            args['method'] = 'GET'
                        else:
                            args['method'] = method
                    if method == 'HEAD':
                        result = full_function_path_map[(path,'GET')](args=args)
                    else:
                        result = full_function_path_map[(path,method)](args=args)
                else:
                    if method == 'HEAD':
                        result = full_function_path_map[(path,'GET')]()
                    else:
                        result = full_function_path_map[(path,method)]()
                if isinstance(result,str):
                    conn.send(('HTTP/1.1 200 OK\r\nServer: python/simpwebserv\r\nContent-Type: text/html\r\nExpires: '+datetime.datetime.utcnow().strftime('%a, %d %b %Y %H:%M:%S GMT')+'\r\nConnection: close\r\n\r\n'+result).encode(__simpwebserv_coding__))
                    conn.close()
                    status_code = '200'
                else:
                    status_code = result.status_code
                    http_send = 'HTTP/1.1 '+status_code+' '+result.status_code_text+'\r\nServer: python/simpwebserv\r\nConnection: close\r\nContent-Type: '+result.Content_Type+'\r\n'
                    if result.Content_Disposition != None:
                        http_send = http_send+'Content-Disposition: '+result.Content_Disposition+'\r\n'
                    if result.Set_Cookie != None:
                        for i in result.Set_Cookie:
                            http_send = http_send+i+'\r\n'
                    if result.Location != None:
                        http_send = http_send+'Location: '+result.Location+'\r\n\r\n'
                    http_send = http_send+'\r\n'
                    conn.send(http_send.encode(__simpwebserv_coding__)+result.body)
                    conn.close()
            except KeyError as e:
                conn.send(('HTTP/1.1 404 NOT FOUND\r\nServer: python/simpwebserv\r\nContent-Type: text/html\r\nExpires: '+datetime.datetime.utcnow().strftime('%a, %d %b %Y %H:%M:%S GMT')+'\r\nConnection: close\r\n\r\n404 NOT FOUND').encode(__simpwebserv_coding__))
                conn.close()
                status_code = '404'
            except Exception as e:
                if debug:
                    conn.send(('HTTP/1.1 500 ERROR\r\nServer: python/simpwebserv\r\nContent-Type: text/html\r\nExpires: '+datetime.datetime.utcnow().strftime('%a, %d %b %Y %H:%M:%S GMT')+'\r\nConnection: close\r\n\r\n500 error\r\n\r\nlog:\r\n'+traceback.format_exc()).encode(__simpwebserv_coding__))
                else:
                    conn.send('HTTP/1.1 500 ERROR\r\nServer: python/simpwebserv\r\nContent-Type: text/html\r\nExpires: '+datetime.datetime.utcnow().strftime('%a, %d %b %Y %H:%M:%S GMT')+'\r\nConnection: close\r\n\r\n500 error'.encode(__simpwebserv_coding__))
                conn.close()
                status_code = '500'
            print(method+' '+path+' '+status_code+' '+addr[0])
            #t2 = time.time()
            #print(str((t2-t1)*1000)+'ms')
        sock = socket.socket(socket.AF_INET,socket.SOCK_STREAM)
        sock.bind((host,port))
        sock.listen(5)
        print('Server is running on http://'+host+':'+str(port))
        while True:
            conn,addr = sock.accept()
            t = threading.Thread(target=__simpwebserv_process_func__,args=(conn,addr,self.full_function_path_map,self.full_function_path_requier_map))
            t.start()
