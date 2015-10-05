{{ define "base" }}
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{ template "title" . }}</title>

    <link href="/static/css/bootstrap.min.css" rel="stylesheet">
    <link href="/static/font-awesome/css/font-awesome.min.css" rel="stylesheet">
    <link href="/static/css/animate.css" rel="stylesheet">
    <link href="/static/css/style.css" rel="stylesheet">
</head>

<body>
    {{ template "body" . }}

    <!-- Mainly scripts -->
    <script src="/static/js/JSXTransformer.js"></script>
    <script src="/static/js/react.js"></script>

    <script src="/static/js/jquery-2.1.1.js"></script>
    <script src="/static/js/bootstrap.min.js"></script>
    <script src="/static/js/plugins/metisMenu/jquery.metisMenu.js"></script>
    <script src="/static/js/plugins/slimscroll/jquery.slimscroll.min.js"></script>
    
    <!-- Custom and plugin javascript -->
    <script src="/static/js/inspinia.js"></script>
    <script src="/static/js/plugins/pace/pace.min.js"></script>

    <script type="text/jsx" src="/static/js/grv/modal.js"></script>
    <script type="text/jsx" src="/static/js/grv/frame.js"></script>
    {{ template "script" . }}
</body>
</html>
{{ end }}

{{ define "script" }}{{ end }}
