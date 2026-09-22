from pathlib import Path
import subprocess,json,time,hashlib
out=Path(__file__).resolve().parent
root=Path('/tmp/repoctx-m6-07-20260922/source-export')
args=['/tmp/repoctx-first-use-1dc9657','read','-root',str(root),'-file','pkg/agentctx/units.go:223:0','-max-bytes','12000']
start=time.monotonic();r=subprocess.run(args,capture_output=True,timeout=15)
(out/'eof-read.stdout').write_bytes(r.stdout);(out/'eof-read.stderr').write_bytes(r.stderr)
d=json.loads(r.stdout);e=d['results'][0]['evidence'];b=(root/'pkg/agentctx/units.go').read_bytes()
assert r.returncode==0 and not d['incomplete'];assert b[e['start_byte']:e['end_byte']].decode()==e['text'];assert e['end_byte']==len(b);assert hashlib.sha256(b).hexdigest()==e['sha256']
record={'classification':'operator validation after replay; not an agent trial','command':args,'exit_code':r.returncode,'wall_seconds':time.monotonic()-start,'stdout_bytes':len(r.stdout),'incomplete':d['incomplete'],'exact_source_through_eof':True,'observed_start_line':e['start_line'],'observed_end_line':e['end_line'],'stdout':'eof-read.stdout','stderr':'eof-read.stderr'}
(out/'eof-read.json').write_text(json.dumps(record,indent=2)+'\n');print(json.dumps(record))
