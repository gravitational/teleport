{{ define "title" }} 
   Connected SSH Servers
{{ end }}

{{ define "body" }}
{{ end }}

{{ define "content" }} 
{{ end }}

{{ define "script" }}
    <script type="text/javascript" src="/static/js/term.js"></script>
    <script type="text/javascript">
       session = {
           id: "{{.SessionID}}",
           first_server: "{{.ServerAddr}}"
       };
    </script>
    <script type="text/javascript" src="/static/js/grv/sessionlib.js"></script>
    <script type="text/javascript" src="/static/js/grv/player.js"></script>
    <script type="text/jsx" src="/static/js/grv/events.jsx"></script>
    <script type="text/jsx" src="/static/js/grv/session.jsx"></script>
{{ end }}
