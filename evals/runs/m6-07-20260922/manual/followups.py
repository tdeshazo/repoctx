from pathlib import Path
import subprocess,json,time
out=Path(__file__).resolve().parent
binary='/tmp/repoctx-first-use-1dc9657';root='/tmp/repoctx-m6-07-20260922/source-export'
records=json.loads((out/'records.json').read_text())
bundle=json.loads((out/'context.stdout').read_text())
steps=[('read',[binary,'read','-root',root,'-file','README.md:267:290','-max-bytes','5000']),('expand',[binary,'context','-root',root,'-symbol',bundle['symbols'][0]['id'],'-expect-snapshot',bundle['snapshot']['id'],'-direction','in','-depth','1','-max-bytes','12000',str(out/'source.ir.json.gz')])]
for name,args in steps:
 start=time.monotonic();r=subprocess.run(args,capture_output=True,timeout=90)
 (out/(name+'.stdout')).write_bytes(r.stdout);(out/(name+'.stderr')).write_bytes(r.stderr)
 record={'name':name,'command':args,'exit_code':r.returncode,'wall_seconds':time.monotonic()-start,'stdout_bytes':len(r.stdout),'stdout':name+'.stdout','stderr':name+'.stderr'}
 records.append(record);print(json.dumps(record),flush=True)
(out/'records.json').write_text(json.dumps(records,indent=2)+'\n')
