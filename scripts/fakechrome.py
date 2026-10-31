import glob, json, socket, struct, subprocess, sys, os, time, threading
rt = sys.argv[1]
env = dict(os.environ, XDG_RUNTIME_DIR=rt)
profile = "0123abcd"
# Chrome passes the origin, not "host": the manifest cannot add arguments.
p = subprocess.Popen(["./bin/chromectl", "chrome-extension://mpnpeobidlmllklmdpnkhhhblkbhpmcp/", "--parent-window=0"], stdin=subprocess.PIPE, stdout=subprocess.PIPE, env=env)
def read():
    n = struct.unpack("=I", p.stdout.read(4))[0]
    return json.loads(p.stdout.read(n))
def write(m):
    b = json.dumps(m).encode(); p.stdin.write(struct.pack("=I", len(b)) + b); p.stdin.flush()
def fail(why):
    print("FAIL:", why, flush=True); p.kill(); sys.exit(1)
served = []
# The host asks identify first and opens its socket only after the answer.
def extension():
    for _ in range(6):
        req = read()
        print("host->ext:", req, flush=True)
        if req["cmd"] == "identify":
            write({"seq": req["seq"], "result": {"profile": profile}})
        elif req["cmd"] == "list":
            write({"seq": req["seq"], "result": [{"id": 4, "window": 1, "active": True, "title": "Pull requests", "url": "https://github.com/pulls?q=is%3Aopen&sort=updated"}]})
        elif req["cmd"] == "open":
            write({"seq": req["seq"], "result": {"id": 5, "window": 1}})
        else:
            write({"seq": req["seq"], "error": "No tab with id: 99."})
        served.append(req)
t = threading.Thread(target=extension, daemon=True); t.start()
# A host renames its socket into place once it listens, so a visible name accepts.
for _ in range(500):
    socks = glob.glob(os.path.join(rt, "chromectl", profile + ".*.sock"))
    if socks: break
    time.sleep(0.01)
else: fail("the host never put its socket in " + os.path.join(rt, "chromectl"))
sock = socks[0]
print("socket:", os.path.basename(sock), flush=True)
# This script is the host's parent, so it is the "browser" the name records.
browser = os.path.basename(sock).split(".")[1]
url = "https://github.com/pulls?q=is%3Aopen&sort=updated"
tab, window = profile + ":4", profile + ":1"
listed = json.dumps([{"id": tab, "window": window, "active": True, "browser": browser, "profile": profile, "title": "Pull requests", "url": url}], separators=(",", ":"))
# Each case: the arguments, then the exit code, stdout and stderr the CLI must give.
cases = (
    (["list"], 0, "\t".join([tab, window, "*", browser, "Pull requests", url]) + "\n", ""),
    (["list", "--json"], 0, listed + "\n", ""),
    (["open", "https://example.com", window], 0, profile + ":5\t" + window + "\n", ""),
    (["open", "--json", "https://example.com"], 0, '{"id":"%s:5","window":"%s"}\n' % (profile, window), ""),
    (["activate", profile + ":99"], 1, "", "chromectl: No tab with id: 99.\n"),
)
def cli(args):
    try: return subprocess.run(["./bin/chromectl"] + args, env=env, capture_output=True, text=True, timeout=5)
    except subprocess.TimeoutExpired: fail("chromectl %s did not answer in 5s" % " ".join(args))
for args, code, out, err in cases:
    r = cli(args)
    print("$ chromectl", *args, "->", r.returncode, repr(r.stdout), repr(r.stderr), flush=True)
    if (r.returncode, r.stdout, r.stderr) != (code, out, err):
        fail("want %d %r %r" % (code, out, err))
t.join(timeout=5)
if len(served) != 6: fail("the extension answered %d of six requests" % len(served))
p.stdin.close()
try: p.wait(timeout=2)
except subprocess.TimeoutExpired: fail("the host did not exit 2s after its stdin closed")
print("host exit:", p.returncode, flush=True)
if p.returncode != 0: fail("the host exited %d" % p.returncode)
if os.path.exists(sock): fail("the host left its socket " + sock)
r = cli(["list"])
print("$ chromectl list, host gone ->", r.returncode, repr(r.stderr))
if r.returncode != 1 or "no chromectl host is listening" not in r.stderr: fail("want ErrNoHost")
print("PASS")
