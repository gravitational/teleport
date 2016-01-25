{{ define "title" }} New User {{ end }}

{{ define "body" }}
    <div class="middle-box text-center loginscreen  animated fadeInDown">
        <div>
            <div>
                <h1 class="logo-name">G</h1>
            </div>
            <h3>Welcome to Gravity</h3>
            <p>Create password.</p>
            <div align="left">
                <font color="white">
                    1) Create and enter a new password<br>
                    2) Install Google Authenticator on your smartphone<br>
                    3) Open Google Authenticator and create a new account using provided barcode<br>
                    4) Generate Authenticator token and enter it below<br>
                </font>
            </div>
            <form class="m-t" role="form" action="/web/finishnewuser" method="POST">
                <div class="form-group">
                    <input type="hidden" name="token" class="form-control" value="{{.Token}}">
                </div>
                <div class="form-group">
                    <input type="test" name="username" disabled class="form-control" placeholder="Username" required="" value="{{.Username}}">
                </div>
                <div class="form-group">
                    <input type="password" name="password" id="password" class="form-control" placeholder="Password" required="" onchange="checkPasswords()">
                </div>
                <div class="form-group">
                    <input type="password" name="password_confirm" id="password_confirm" class="form-control" placeholder="Confirm password" required="" onchange="checkPasswords()">
                </div>
                <div class="form-group">
                    <input type="test" name="hotp_token" id="hotp_token" class="form-control" placeholder="hotp token" required="">
                </div>

                <button type="submit" class="btn btn-primary block full-width m-b">Confirm</button>

                <script language='javascript' type='text/javascript'>
                    var password = document.getElementById('password');
                    var password_confirm = document.getElementById('password_confirm');

                    function checkPasswords() {
                        if (password.value != password_confirm.value) {
                            password_confirm.setCustomValidity('Password Must be Matching.');
                        } else {
                            password_confirm.setCustomValidity('');
                        }
                    }
                </script>

            </form>
            <img src="data:image/png;base64,{{.QR}}">
            <p class="m-t"> <small>Gravitational Inc on Bootstrap 3 &copy; 2015</small> </p>
        </div>
    </div>
{{ end }}
