# Pilote Chromium par son protocole de débogage (utilisé par ./test-vm.sh novnc) :
# ouvre une page, attend, évalue un script facultatif, puis capture l écran.
# Ouvre noVNC dans Chromium (déjà lancé avec --remote-debugging-port=9222),
# attend la connexion, lit l'état affiché par noVNC et capture l'écran.
import base64, json, sys, time, urllib.request
import websocket  # python-websocket-client
out, wait = sys.argv[1], int(sys.argv[2])
for _ in range(50):
    try:
        tabs = json.load(urllib.request.urlopen("http://127.0.0.1:9222/json"))
        page = next(t for t in tabs if t["type"] == "page"); break
    except Exception:
        time.sleep(0.2)
ws = websocket.create_connection(page["webSocketDebuggerUrl"], suppress_origin=True)
n = 0
def call(method, **params):
    global n; n += 1
    ws.send(json.dumps({"id": n, "method": method, "params": params}))
    while True:
        r = json.loads(ws.recv())
        if r.get("id") == n: return r.get("result", {})
time.sleep(wait)
if len(sys.argv) > 3 and sys.argv[3]:
    expr = open(sys.argv[3]).read()
    print(call("Runtime.evaluate", expression=expr, awaitPromise=True).get("result", {}).get("value"))
js = "JSON.stringify({status: (document.getElementById('noVNC_status')||{}).textContent, connected: document.documentElement.classList.contains('noVNC_connected'), dialogs: [...document.querySelectorAll('.noVNC_open')].map(e => e.id)})"
print(call("Runtime.evaluate", expression=js)["result"]["value"])
open(out, "wb").write(base64.b64decode(call("Page.captureScreenshot")["data"]))
