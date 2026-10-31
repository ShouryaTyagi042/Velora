"""Per-route latency for Velora: N sequential requests on one keep-alive connection.

usage: python3 -I bench.py <host:port> <token-file> [N] [route ...]

The token file holds a session token. Get one by signing in:
  curl -s -X POST localhost:8080/api/auth/login -d '{"username":"…","password":"…"}' \
    | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])' > /tmp/velora-token
Sign out afterwards (POST /api/auth/logout with that token) to end the session.
Prints p50 / p95 / max (ms), status, body bytes, and the caching headers of the last response.
A second pass per route replays the validators (ETag / Last-Modified) to show what a revisit costs.
"""
import http.client
import statistics
import sys
import time

host = sys.argv[1]
token = open(sys.argv[2]).read().strip()
n = int(sys.argv[3]) if len(sys.argv) > 3 else 100
routes = sys.argv[4:] or [
    "/api/media",
    "/api/media?kind=video",
    "/api/media/3",
    "/api/continue",
    "/api/actors",
    "/api/media/3/thumbnail",
    "/api/media/1/pages/0",
]


def pct(xs, p):
    xs = sorted(xs)
    return xs[min(len(xs) - 1, int(round(p / 100 * (len(xs) - 1))))]


def run(conn, path, extra):
    times, size, status, headers = [], 0, 0, {}
    for _ in range(n):
        t = time.perf_counter()
        conn.request("GET", path, headers={"Authorization": f"Bearer {token}", **extra})
        r = conn.getresponse()
        body = r.read()
        times.append((time.perf_counter() - t) * 1000)
        size, status, headers = len(body), r.status, {k.lower(): v for k, v in r.getheaders()}
    return times, size, status, headers


conn = http.client.HTTPConnection(host, timeout=10)
print(f"{'route':42} {'pass':10} {'status':>6} {'bytes':>8} {'p50 ms':>8} {'p95 ms':>8} {'max ms':>8}  cache")
for path in routes:
    times, size, status, h = run(conn, path, {})
    cache = h.get("cache-control", "-")
    print(f"{path:42} {'cold':10} {status:>6} {size:>8} {pct(times,50):8.2f} {pct(times,95):8.2f} {max(times):8.2f}  {cache}")
    validators = {}
    if "etag" in h:
        validators["If-None-Match"] = h["etag"]
    if "last-modified" in h:
        validators["If-Modified-Since"] = h["last-modified"]
    if validators:
        times, size, status, _ = run(conn, path, validators)
        print(f"{'':42} {'revalidate':10} {status:>6} {size:>8} {pct(times,50):8.2f} {pct(times,95):8.2f} {max(times):8.2f}")
    else:
        print(f"{'':42} {'revalidate':10}   (no ETag or Last-Modified: the client can't revalidate)")
