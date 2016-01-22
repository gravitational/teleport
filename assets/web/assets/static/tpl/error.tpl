{{ define "title" }} Error {{ end }}

{{ define "body" }}
    <div class="middle-box text-center loginscreen  animated fadeInDown">
        <div>
            <div>
                <h1 class="logo-name">G</h1>
            </div>
            <h3><font color = "white">{{.ErrorString}}</font></h3>
            <p class="m-t"> <small>Gravitational Inc on Bootstrap 3 &copy; 2015</small> </p>
        </div>
    </div>
{{ end }}
