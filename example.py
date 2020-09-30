import simpwebserv

app = simpwebserv.server()

def main(args):
    if args['method'] == 'GET':
        return str(args['get_parameter'])
    else:
        return str(args['post_parameter'])

def test_file():
    resp = simpwebserv.response()
    filename = 'simpwebserv.py'
    f = open(filename,'rb')
    resp.transform_file(f.read(),filename)
    f.close()
    return resp

#app.register(main,'/',accept_methods=['GET','POST'],requier_args=True,requier_get_parameter=True,requier_post_parameter=True,requier_method=True)

app.register(test_file,'/')
app.run(debug=True)
