import json, struct, subprocess, sys, os, time, threading
rt = sys.argv[1]
env = dict(os.environ, XDG_RUNTIME_DIR=rt)
p = subprocess.Popen(["./bin/chromectl", "host"], stdin=subprocess.PIPE, stdout=subprocess.PIPE, env=env)
def read():
    n = struct.unpack("=I", p.stdout.read(4))[0]
    return json.loads(p.stdout.read(n))
def write(m):
    b = json.dumps(m).encode(); p.stdin.write(struct.pack("=I", len(b)) + b); p.stdin.flush()
sock = os.path.join(rt, "chromectl.sock")
while not os.path.exists(sock): time.sleep(0.01)
def extension():
    for _ in range(3):
        req = read()
        print("host->ext:", req, flush=True)
        if req["cmd"] == "list":
            write({"seq": req["seq"], "result": [{"id": 4, "window": 1, "active": True, "title": "Pull requests", "url": "https://github.com/pulls"}]})
        elif req["cmd"] == "open":
            write({"seq": req["seq"], "result": {"id": 5, "window": 1}})
        else:
            write({"seq": req["seq"], "error": "No tab with id: 99."})
t = threading.Thread(target=extension); t.start()
for args in (["list"], ["open", "https://example.com", "1"], ["activate", "99"]):
    r = subprocess.run(["./bin/chromectl"] + args, env=env, capture_output=True, text=True)
    print("$ chromectl", *args, "->", r.returncode, repr(r.stdout), repr(r.stderr), flush=True)
t.join()
p.stdin.close(); p.wait(timeout=2)
print("host exit:", p.returncode, "socket gone:", not os.path.exists(sock))
