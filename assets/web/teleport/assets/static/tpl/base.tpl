{{ define "base" }}
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{ template "title" . }}</title>

    <link href="{{Path "/static/css/bootstrap.min.css"}}" rel="stylesheet">
    <link href="{{Path "/static/font-awesome/css/font-awesome.min.css"}}" rel="stylesheet">
    <link href="{{Path "/static/css/animate.css"}}" rel="stylesheet">
    <link href="{{Path "/static/css/style.min.css"}}" rel="stylesheet">
    <link href="{{Path "/static/css/plugins/chosen/chosen.css"}}" rel="stylesheet">
    <link href="{{Path "/static/css/plugins/toastr/toastr.min.css"}}" rel="stylesheet">
    <link href="{{Path "/static/css/plugins/fileupload/jquery.fileupload.css"}}" rel="stylesheet">
    <link href="{{Path "/static/css/plugins/fileupload/jquery.fileupload-ui.css"}}" rel="stylesheet">
    <link href="{{Path "/static/css/plugins/jsTree/style.min.css"}}" rel="stylesheet">
    <link href="{{Path "/static/css/plugins/datapicker/datepicker3.css"}}" rel="stylesheet">
</head>

<body>
    {{ template "body" . }}

    <!-- Mainly scripts -->
    <script src="{{Path "/static/js/JSXTransformer.js"}}"></script>
    <script src="{{Path "/static/js/react.js"}}"></script>

    <script src="{{Path "/static/js/jquery-2.1.1.js"}}"></script>
    <script src="{{Path "/static/js/bootstrap.min.js"}}"></script>
    <script src="{{Path "/static/js/plugins/metisMenu/jquery.metisMenu.js"}}"></script>
    <script src="{{Path "/static/js/plugins/slimscroll/jquery.slimscroll.min.js"}}"></script>
    
    <!-- Custom and plugin javascript -->
    <script src="{{Path "/static/js/inspinia.js"}}"></script>
    <script src="{{Path "/static/js/plugins/chosen/chosen.jquery.js"}}"></script>

    <script src="{{Path "/static/js/jquery.ui.widget.js"}}"></script>
    <script src="{{Path "/static/js/jquery.iframe-transport.js"}}"></script>
    <script src="{{Path "/static/js/jquery.fileupload.js"}}"></script>
    <script src="{{Path "/static/js/jquery.fileupload-process.js"}}"></script>
    <script src="{{Path "/static/js/plugins/toastr/toastr.min.js"}}"></script>
    <script src="{{Path "/static/js/plugins/jsTree/jstree.min.js"}}"></script>
    <script src="{{Path "/static/js/plugins/download/jquery.fileDownload.js"}}"></script>
    <script src="{{Path "/static/js/plugins/datapicker/bootstrap-datepicker.js"}}"></script>

    <!-- Gravity stuff -->
    <script type="text/javascript">
       grv = {
           prefix: "{{.Cfg.URLPrefix}}",
           path: function() {
               var path = ["{{.Cfg.URLPrefix}}" != ""? "{{.Cfg.URLPrefix}}":""];
               for(var i = 0; i < arguments.length; i++) {
                    path.push(arguments[i]);
               } 
               return path.join("/");
           },
           nav_sections: {{.Cfg.NavSections}}
       };
    </script>    
    <script src="{{Path "/static/js/grv/lib.js"}}"></script>    
    <script type="text/jsx" src="{{Path "/static/js/grv/modal.jsx"}}"></script>
    <script type="text/jsx" src="{{Path "/static/js/grv/frame.jsx"}}"></script>
    {{ template "script" . }}
</body>
</html>
{{ end }}

{{ define "script" }}{{ end }}
