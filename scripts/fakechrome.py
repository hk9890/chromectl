import json, socket, struct, subprocess, sys, os, time, threading
rt = sys.argv[1]
env = dict(os.environ, XDG_RUNTIME_DIR=rt)
# Chrome passes the origin, not "host": the manifest cannot add arguments.
p = subprocess.Popen(["./bin/chromectl", "chrome-extension://mpnpeobidlmllklmdpnkhhhblkbhpmcp/", "--parent-window=0"], stdin=subprocess.PIPE, stdout=subprocess.PIPE, env=env)
def read():
    n = struct.unpack("=I", p.stdout.read(4))[0]
    return json.loads(p.stdout.read(n))
def write(m):
    b = json.dumps(m).encode(); p.stdin.write(struct.pack("=I", len(b)) + b); p.stdin.flush()
def fail(why):
    print("FAIL:", why, flush=True); p.kill(); sys.exit(1)
# The path appears at bind, before listen: wait until the host accepts.
sock = os.path.join(rt, "chromectl.sock")
for _ in range(500):
    try:
        with socket.socket(socket.AF_UNIX) as s: s.connect(sock)
        break
    except OSError: time.sleep(0.01)
else: fail("the host never accepted on " + sock)
def extension():
    for _ in range(5):
        req = read()
        print("host->ext:", req, flush=True)
        if req["cmd"] == "list":
            write({"seq": req["seq"], "result": [{"id": 4, "window": 1, "active": True, "title": "Pull requests", "url": "https://github.com/pulls?q=is%3Aopen&sort=updated"}]})
        elif req["cmd"] == "open":
            write({"seq": req["seq"], "result": {"id": 5, "window": 1}})
        else:
            write({"seq": req["seq"], "error": "No tab with id: 99."})
t = threading.Thread(target=extension, daemon=True); t.start()
url = "https://github.com/pulls?q=is%3Aopen&sort=updated"
listed = json.dumps([{"id": 4, "window": 1, "active": True, "title": "Pull requests", "url": url}], separators=(",", ":"))
# Each case: the arguments, then the exit code, stdout and stderr the CLI must give.
cases = (
    (["list"], 0, "4\t1\t*\tPull requests\t" + url + "\n", ""),
    (["list", "--json"], 0, listed + "\n", ""),
    (["open", "https://example.com", "1"], 0, "5\t1\n", ""),
    (["open", "--json", "https://example.com"], 0, '{"id":5,"window":1}\n', ""),
    (["activate", "99"], 1, "", "chromectl: No tab with id: 99.\n"),
)
for args, code, out, err in cases:
    r = subprocess.run(["./bin/chromectl"] + args, env=env, capture_output=True, text=True, timeout=5)
    print("$ chromectl", *args, "->", r.returncode, repr(r.stdout), repr(r.stderr), flush=True)
    if (r.returncode, r.stdout, r.stderr) != (code, out, err):
        fail("want %d %r %r" % (code, out, err))
t.join(timeout=5)
if t.is_alive(): fail("the extension did not get all five requests")
p.stdin.close(); p.wait(timeout=2)
print("host exit:", p.returncode, flush=True)
if p.returncode != 0: fail("the host exited %d" % p.returncode)
r = subprocess.run(["./bin/chromectl", "list"], env=env, capture_output=True, text=True, timeout=5)
print("$ chromectl list, host gone ->", r.returncode, repr(r.stderr))
if r.returncode != 1 or "no chromectl host is listening" not in r.stderr: fail("want ErrNoHost")
print("PASS")
