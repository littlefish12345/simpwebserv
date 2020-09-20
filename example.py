import simpwebserv

app = simpwebserv.server()

def main(args):
    return str(args['get_parameter'])

app.register(main,'/',requier_args=True,requier_get_parameter=True)

app.run(debug=True)
