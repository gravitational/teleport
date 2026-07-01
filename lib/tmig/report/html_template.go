package report

// htmlTemplateSource is the full self-contained HTML template for the readiness report.
// All CSS and JS are inlined — zero external resource references — safe for air-gapped use.
const htmlTemplateSource = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>tmig readiness report — {{.Source.Name}} → {{.Target.Name}}</title>
<style>
  :root{
    --bg:#0d1117; --panel:#161b22; --panel2:#1c2330; --line:#2b3441; --text:#e6edf3;
    --muted:#9aa7b4; --auto:#2ea043; --prereq:#d29922; --pipeline:#388bfd;
    --manual:#f85149; --orphan:#8b949e; --chip:#21262d;
  }
  *{box-sizing:border-box}
  body{margin:0;background:var(--bg);color:var(--text);
    font:14px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif}
  .wrap{max-width:1100px;margin:0 auto;padding:28px 20px 80px}
  h1{font-size:20px;margin:0 0 4px}
  h2{font-size:15px;margin:30px 0 12px;color:var(--text);border-bottom:1px solid var(--line);padding-bottom:6px}
  .sub{color:var(--muted);font-size:13px;margin-bottom:18px}
  .idline{display:flex;flex-wrap:wrap;gap:8px 22px;font-size:12.5px;color:var(--muted);
    background:var(--panel);border:1px solid var(--line);border-radius:8px;padding:12px 16px;margin-bottom:6px}
  .idline b{color:var(--text);font-weight:600}
  .ok{color:var(--auto)} .no{color:var(--manual)}
  .cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:12px;margin:14px 0 6px}
  .card{background:var(--panel);border:1px solid var(--line);border-radius:10px;padding:14px 16px}
  .card .n{font-size:26px;font-weight:700;line-height:1}
  .card .l{font-size:12px;color:var(--muted);margin-top:6px}
  .card.auto{border-left:4px solid var(--auto)}
  .card.prereq{border-left:4px solid var(--prereq)}
  .card.pipeline{border-left:4px solid var(--pipeline)}
  .card.manual{border-left:4px solid var(--manual)}
  .attn{background:var(--panel2);border:1px solid var(--line);border-radius:10px;padding:16px 18px;margin-top:10px}
  .attn .big{font-size:15px;line-height:1.7}
  .attn b{font-weight:700}
  .pill{display:inline-block;font-size:11px;font-weight:700;padding:2px 8px;border-radius:20px;color:#0d1117}
  .pill.auto{background:var(--auto)} .pill.prereq{background:var(--prereq)}
  .pill.pipeline{background:var(--pipeline)} .pill.manual{background:var(--manual)}
  .pill.orphan{background:var(--orphan)}
  .trust{display:inline-block;font-size:10.5px;font-weight:600;padding:1px 6px;border-radius:4px;border:1px solid var(--line);color:var(--muted)}
  .trust.srv{color:var(--auto);border-color:var(--auto)}
  details.rem{background:var(--panel);border:1px solid var(--line);border-radius:8px;margin:10px 0;overflow:hidden}
  details.rem>summary{cursor:pointer;padding:12px 16px;list-style:none;display:flex;gap:10px;align-items:center}
  details.rem>summary::-webkit-details-marker{display:none}
  details.rem>summary .t{font-weight:600;flex:1}
  details.rem>summary .c{font-size:12px;color:var(--muted)}
  .rembody{padding:0 16px 16px;border-top:1px solid var(--line)}
  .note{color:var(--muted);font-size:12.5px;margin:10px 0}
  pre{background:#0b0f14;border:1px solid var(--line);border-radius:6px;padding:12px 14px;
    overflow:auto;font:12px/1.5 ui-monospace,SFMono-Regular,Menlo,monospace;color:#d6dee8}
  table{width:100%;border-collapse:collapse;font-size:12.5px;margin-top:6px}
  th,td{text-align:left;padding:8px 10px;border-bottom:1px solid var(--line);vertical-align:top}
  th{color:var(--muted);font-weight:600;position:sticky;top:0;background:var(--bg)}
  tr.host{cursor:pointer}
  tr.host:hover{background:var(--panel)}
  tr.detail td{background:var(--panel2);font-size:12px}
  .mono{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}
  .controls{display:flex;gap:8px;flex-wrap:wrap;margin:6px 0 4px}
  .controls button{background:var(--chip);color:var(--text);border:1px solid var(--line);
    border-radius:20px;padding:5px 12px;font-size:12px;cursor:pointer}
  .controls button.on{background:var(--text);color:#0d1117;font-weight:600}
  .reason{color:var(--muted)}
  .foot{color:var(--muted);font-size:11.5px;margin-top:40px;border-top:1px solid var(--line);padding-top:14px}
  .tag{display:inline-block;background:var(--chip);border:1px solid var(--line);border-radius:5px;
    padding:1px 6px;font-size:11px;margin:0 4px 4px 0}
</style>
</head>
<body>
<div class="wrap">

  <h1>tmig readiness report</h1>
  <div class="sub">Run <span class="mono">{{.RunID}}</span> · generated {{.GeneratedAt.Format "2006-01-02 15:04 UTC"}} · <b>preflight / dry-run — nothing was changed</b></div>

  <div class="idline">
    <span><b>SOURCE</b> {{.Source.Name}} (v{{.Source.Version}}) · {{.Source.Proxy}} · user {{.Source.User}}</span>
    <span><b>TARGET</b> {{.Target.Name}} (v{{.Target.Version}}){{if .Target.ScopePinned}} · scope-pinned <span class="mono">{{.Target.User}}</span>{{end}}</span>
  </div>
  <div class="idline">
    <span>scopes enabled {{if .ScopesEnabled}}<span class="ok">&#x2713;</span>{{else}}<span class="no">&#x2717;</span>{{end}}</span>
    {{if .Scopable.Roles}}<span>roles scopable: Node {{if roleOK .Scopable.Roles "Node"}}<span class="ok">&#x2713;</span>{{else}}<span class="no">&#x2717;</span>{{end}} · Kube {{if roleOK .Scopable.Roles "Kube"}}<span class="ok">&#x2713;</span>{{else}}<span class="no">&#x2717;</span>{{end}} · App {{if roleOK .Scopable.Roles "App"}}<span class="ok">&#x2713;</span>{{else}}<span class="no">&#x2717;</span>{{end}} · Db {{if roleOK .Scopable.Roles "Db"}}<span class="ok">&#x2713;</span>{{else}}<span class="no">&#x2717;</span>{{end}}</span>{{end}}
  </div>

  <h2>Summary</h2>
  <div class="cards">
    <div class="card auto"><div class="n">{{verdictCount .Summary.ByVerdict "AUTO"}}</div><div class="l">AUTO — fully automatic</div></div>
    <div class="card prereq"><div class="n">{{verdictCount .Summary.ByVerdict "PREREQ"}}</div><div class="l">PREREQ{{if gt .Summary.Blocked 0}} · <b>{{.Summary.Blocked}} blocked</b>{{end}}</div></div>
    <div class="card pipeline"><div class="n">{{verdictCount .Summary.ByVerdict "PIPELINE"}}</div><div class="l">PIPELINE</div></div>
    <div class="card manual"><div class="n">{{verdictCount .Summary.ByVerdict "MANUAL"}}</div><div class="l">MANUAL</div></div>
    {{if gt .Summary.Orphans 0}}<div class="card"><div class="n">{{.Summary.Orphans}}</div><div class="l">orphans (no mapping)</div></div>{{end}}
    <div class="card"><div class="n">{{.Summary.ReadyToEnroll}}</div><div class="l">ready to enroll now</div></div>
  </div>

  <div class="attn">
    <div class="big">
      <b>{{.Summary.Attention.AutomaticHosts}}</b> hosts migrate automatically.
      {{if gt .Summary.Attention.IaCActions 0}}&nbsp;·&nbsp;<b>{{.Summary.Attention.IaCHostsCovered}}</b> hosts are unblocked by <b>{{.Summary.Attention.IaCActions}}</b> one-time IaC applies.{{end}}
      {{if gt .Summary.Attention.PipelineActions 0}}&nbsp;·&nbsp;<b>{{.Summary.Attention.PipelineHostsCovered}}</b> hosts covered by <b>{{.Summary.Attention.PipelineActions}}</b> pipeline migrations.{{end}}
      {{if gt .Summary.Attention.ManualHosts 0}}&nbsp;·&nbsp;<b>{{.Summary.Attention.ManualHosts}}</b> hosts need per-host manual steps.{{end}}
    </div>
  </div>

  {{if .Remediations}}
  <h2>What needs to happen (grouped remediations)</h2>
  {{range .Remediations}}
  <details class="rem">
    <summary><span class="pill prereq">{{.Kind}}</span><span class="t">{{.Title}}</span><span class="c">covers {{len .HostsCovered}} hosts</span></summary>
    <div class="rembody">
      {{if .Note}}<div class="note">{{.Note}}</div>{{end}}
      {{if .YAML}}<pre>{{.YAML}}</pre>{{end}}
      {{if .Terraform}}<pre>{{.Terraform}}</pre>{{end}}
      {{if .TCTL}}<pre>{{.TCTL}}</pre>{{end}}
      {{range .Commands}}<pre>{{.}}</pre>{{end}}
      <div>{{range .HostsCovered}}<span class="tag">{{.}}</span>{{end}}</div>
    </div>
  </details>
  {{end}}
  {{end}}

  <h2>Per-host detail</h2>
  <div class="controls" id="filters">
    <button data-f="all" class="on">All ({{.Summary.Total}})</button>
    <button data-f="AUTO">AUTO</button>
    <button data-f="PREREQ">PREREQ</button>
    <button data-f="PIPELINE">PIPELINE</button>
    <button data-f="MANUAL">MANUAL</button>
  </div>
  <table id="hosts">
    <thead><tr><th>Host</th><th>Verdict</th><th>Status</th><th>Attention</th><th>Reason</th></tr></thead>
    <tbody>
      {{range .Hosts}}
      <tr class="host" data-v="{{.Verdict}}">
        <td class="mono">{{.Hostname}}</td>
        <td><span class="pill {{verdictClass .Verdict}}">{{.Verdict}}</span></td>
        <td>{{statusText .Status}}</td>
        <td>{{attentionText .Attention}}</td>
        <td class="reason">{{.Reason}}</td>
      </tr>
      {{if .ConfigDiff}}
      <tr class="detail" data-v="{{.Verdict}}"><td colspan="5"><pre>{{.ConfigDiff}}</pre></td></tr>
      {{end}}
      {{end}}
    </tbody>
  </table>

  {{if .Warnings}}
  <h2>Warnings</h2>
  {{range .Warnings}}<div class="note">{{.}}</div>{{end}}
  {{end}}

  <div class="foot">
    Generated by <span class="mono">tmig preflight</span>. Run ID: <span class="mono">{{.RunID}}</span>. This report changes nothing — it is a dry run.
  </div>
</div>

<script>
  document.querySelectorAll('tr.host').forEach(function(row){
    row.addEventListener('click', function(){
      var next = row.nextElementSibling;
      if(next && next.classList.contains('detail')){
        next.style.display = (next.style.display === 'table-row') ? 'none' : 'table-row';
      }
    });
  });
  document.querySelectorAll('tr.detail').forEach(function(r){ r.style.display='none'; });
  document.querySelectorAll('#filters button').forEach(function(b){
    b.addEventListener('click', function(){
      document.querySelectorAll('#filters button').forEach(function(x){x.classList.remove('on')});
      b.classList.add('on');
      var f = b.getAttribute('data-f');
      document.querySelectorAll('#hosts tbody tr').forEach(function(tr){
        if(tr.classList.contains('detail')){ tr.style.display='none'; return; }
        tr.style.display = (f==='all' || tr.getAttribute('data-v')===f) ? '' : 'none';
      });
    });
  });
</script>
</body>
</html>`
