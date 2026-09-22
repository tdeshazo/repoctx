from pathlib import Path
import subprocess,time,json
out=Path(__file__).resolve().parent
binary='/tmp/repoctx-first-use-1dc9657'
root='/tmp/repoctx-m6-07-20260922/source-export'
steps=[('discover',[binary,'discover','-root',root,'-query','discovery max bytes output','-max-bytes','3000']),('compile',[binary,'compile','-root',root,'-o',str(out/'source.ir.json.gz')]),('validate',[binary,'validate',str(out/'source.ir.json.gz')]),('context',[binary,'context','-root',root,'-query','chooseSymbols','-depth','1','-max-bytes','12000',str(out/'source.ir.json.gz')])]
records=[]
for name,args in steps:
 start=time.monotonic(); r=subprocess.run(args,capture_output=True,timeout=90)
 (out/(name+'.stdout')).write_bytes(r.stdout);(out/(name+'.stderr')).write_bytes(r.stderr)
 record={'name':name,'command':args,'exit_code':r.returncode,'wall_seconds':time.monotonic()-start,'stdout_bytes':len(r.stdout),'stdout':name+'.stdout','stderr':name+'.stderr'}
 records.append(record); print(json.dumps(record),flush=True)
(out/'records.json').write_text(json.dumps(records,indent=2)+'\n')
