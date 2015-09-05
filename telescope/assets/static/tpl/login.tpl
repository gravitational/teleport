{{ define "title" }} Login {{ end }}

{{ define "body" }}
    <div class="middle-box text-center loginscreen  animated fadeInDown">
        <div>
            <div>
                <h1 class="logo-name">G</h1>
            </div>
            <h3>Welcome to Gravity</h3>
            <p>Login in.</p>
            <form class="m-t" role="form" action="/web/auth" method="POST">
                <div class="form-group">
                    <input type="test" name="username" class="form-control" placeholder="Username" required="">
                </div>
                <div class="form-group">
                    <input type="password" name="password" class="form-control" placeholder="Password" required="">
                </div>
                <button type="submit" class="btn btn-primary block full-width m-b">Login</button>

                <a href="#"><small>Forgot password?</small></a>
                <p class="text-muted text-center"><small>Do not have an account?</small></p>
                <a class="btn btn-sm btn-white btn-block" href="register.html">Create an account</a>
            </form>
            <p class="m-t"> <small>Gravitational Inc on Bootstrap 3 &copy; 2015</small> </p>
        </div>
    </div>
{{ end }}
