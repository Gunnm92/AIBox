# Envoie à wayvnc des touches par code physique (événements clavier étendus
# QEMU, comme noVNC) : arguments = codes XT en hexadécimal, "+" pour maintenir.
import socket, struct, sys, time
s = socket.create_connection(("127.0.0.1", 5900), timeout=10)
s.recv(12); s.sendall(b"RFB 003.008\n")
n = s.recv(1)[0]; s.recv(n); s.sendall(b"\x01"); s.recv(4); s.sendall(b"\x01")
hdr = s.recv(24); s.recv(struct.unpack(">I", hdr[20:24])[0])
s.sendall(struct.pack(">BBHii", 2, 0, 2, 0, -258))  # SetEncodings : Raw + QEMU Extended Key Event
time.sleep(0.5)
def key(code, down):
    s.sendall(struct.pack(">BBHII", 255, 0, down, 0, code)); time.sleep(0.05)
for arg in sys.argv[1:]:
    held = [int(c, 16) for c in arg.split("+")]
    for c in held: key(c, 1)
    for c in reversed(held): key(c, 0)
time.sleep(0.5)
