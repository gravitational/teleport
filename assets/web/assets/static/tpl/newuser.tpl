{{ define "title" }} New User {{ end }}

{{ define "body" }}
    <div class="middle-box text-center loginscreen  animated fadeInDown">
        <div>
            <div>
                <h1 class="logo-name">G</h1>
            </div>
            <h3>Welcome to Gravity</h3>
            <p>Create password.</p>
            <font color="red">{{.ErrorString}}</font>
            <form class="m-t" role="form" action="/web/auth" method="POST">
                <div class="form-group">
                    <input type="hidden" name="token" class="form-control" value="{{.Token}}">
                </div>
                <div class="form-group">
                    <input type="test" name="username" disabled class="form-control" placeholder="Username" required="" value="{{.Username}}">
                </div>
                <div class="form-group">
                    <input type="password" name="password" class="form-control" placeholder="Password" required="">
                </div>
                <div class="form-group">
                    <input type="password" name="password2" class="form-control" placeholder="Confirm password" required="">
                </div>
                <div class="form-group">
                    <input type="test" name="qr" class="form-control" placeholder="hotp token" required="">
                </div>


                <button type="submit" class="btn btn-primary block full-width m-b">Confirm</button>

            </form>
            <img src="data:image/png;base64,{{.QR}}">
            <p class="m-t"> <small>Gravitational Inc on Bootstrap 3 &copy; 2015</small> </p>
        </div>
    </div>
{{ end }}
