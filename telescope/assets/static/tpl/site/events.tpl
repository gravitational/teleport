{{ define "title" }} 
   Site events
{{ end }}

{{ define "body" }}
{{ end }}

{{ define "content" }} 
{{ end }}

{{ define "script" }}
    <script type="text/javascript" src="/static/js/term.js"></script>
    <script type="text/javascript" src="/static/js/grv/lib.js"></script>
    <script type="text/javascript">
       site = {
           name: {{.SiteName}}
       };
    </script>
    <script type="text/jsx" src="/static/js/grv/site-events.js"></script>
{{ end }}