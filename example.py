import simpwebserv

app = simpwebserv.server()

a = 0

def main(args):
    global a
    a = a+1
    return str(a)
    '''
    if args['method'] == 'GET':
        return str(args['get_parameter'])
    else:
        return str(args['post_parameter'])
'''
app.register(main,'/',accept_methods=['GET','POST'],requier_args=True,requier_get_parameter=True,requier_post_parameter=True,requier_method=True)

app.run(host='192.168.15.175',port=3000,debug=True)
