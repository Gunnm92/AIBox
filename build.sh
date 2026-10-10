#!/usr/bin/env bash
# build.sh : construit l'ISO aibox dans un conteneur CachyOS (Docker).
#
#   ./build.sh               ISO avec keys/authorized_keys, keys/vnc-password et,
#                            s'ils existent, keys/nas-mounts + keys/nas.cred (partages NAS)
#   ./build.sh --test-key    ajoute la clé SSH de test de la VM (work/vm/id_test)
#
# Sortie : out/aibox-AAAA.MM.JJ-x86_64.iso
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$root/tools/engine.sh"
novnc_version="1.7.0"
novnc_sha256="b1003a11b6e6e8d8f7f5e5586daae7f8ca651d8aee0aa155ff9ac841c48f52c6"
test_key=0
[[ "${1:-}" == "--test-key" ]] && test_key=1

mkdir -p "$root/out" "$root/work/pkgcache" "$root/work/vm" "$root/work/dl"

# --- Clés SSH ---
keys="$root/work/authorized_keys"
: > "$keys"
if [[ -s "$root/keys/authorized_keys" ]]; then
    cat "$root/keys/authorized_keys" >> "$keys"
else
    echo "Attention : keys/authorized_keys absent, pas d'accès SSH avec ta clé (console seulement)." >&2
fi
if (( test_key )); then
    [[ -f "$root/work/vm/id_test" ]] || ssh-keygen -q -t ed25519 -N '' -C 'aibox-vm-test' -f "$root/work/vm/id_test"
    cat "$root/work/vm/id_test.pub" >> "$keys"
    echo "Clé de test ajoutée : ne pas utiliser cette ISO sur la vraie machine." >&2
fi

# --- Mot de passe noVNC ---
# Le bureau d'administration a un sudo sans mot de passe : l'accès noVNC est
# toujours protégé. Sans keys/vnc-password, un mot de passe est généré.
if [[ ! -s "$root/keys/vnc-password" ]]; then
    head -c 15 /dev/urandom | base64 | tr -d '/+=' > "$root/keys/vnc-password"
    chmod 600 "$root/keys/vnc-password"
    echo "Mot de passe noVNC généré dans keys/vnc-password : $(cat "$root/keys/vnc-password")" >&2
fi
printf 'default:%s\n' "$(openssl passwd -6 -stdin < <(head -n 1 "$root/keys/vnc-password"))" > "$root/work/novnc.htpasswd"
# wayvnc local pour xrdp : authentification VNC classique (8 caractères max).
printf 'enable_auth=true\nallow_broken_crypto=true\nrelax_encryption=true\npassword=%s\n' "$(head -n 1 "$root/keys/vnc-password" | cut -c1-8)" > "$root/work/wayvnc-rdp.conf"

# --- HTTPS (noVNC, code-server) : autorité de certification locale ---
# Une CA aibox, créée une fois et gardée dans keys/ : à ajouter une fois au
# trousseau du poste (« Toujours approuver »). Un certificat auto-signé accepté
# d'un clic ne suffit pas : le navigateur refuse alors les service workers,
# dont les vues web de code-server (panneau Claude Code) ont besoin.
# La CA est limitée aux noms aibox et aux adresses privées (nameConstraints).
ca_key="$root/keys/aibox-ca.key" ca_crt="$root/keys/aibox-ca.crt"
if [[ ! -s "$ca_key" ]]; then
    openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes -days 3650 \
        -subj "/CN=aibox CA locale" \
        -addext "basicConstraints=critical,CA:TRUE,pathlen:0" \
        -addext "keyUsage=critical,keyCertSign,cRLSign" \
        -addext "nameConstraints=critical,permitted;DNS:aibox,permitted;DNS:aibox.local,permitted;IP:10.0.0.0/255.0.0.0,permitted;IP:172.16.0.0/255.240.0.0,permitted;IP:192.168.0.0/255.255.0.0,permitted;IP:127.0.0.0/255.0.0.0" \
        -keyout "$ca_key" -out "$ca_crt" 2>/dev/null
    chmod 600 "$ca_key"
    echo "CA locale créée : keys/aibox-ca.crt (à approuver une fois sur ton poste)." >&2
fi
# Certificat du serveur, signé par la CA, refait à chaque build : noms aibox et
# aibox.local (avahi), plus les adresses IP de keys/tls-ips (une par ligne).
san="DNS:aibox,DNS:aibox.local,IP:127.0.0.1"
if [[ -s "$root/keys/tls-ips" ]]; then
    while read -r ip; do [[ -n "$ip" && "$ip" != \#* ]] && san+=",IP:$ip"; done < "$root/keys/tls-ips"
fi
openssl req -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes -subj "/CN=aibox" \
    -keyout "$root/work/novnc-tls.key" -out "$root/work/novnc-tls.csr" 2>/dev/null
openssl x509 -req -in "$root/work/novnc-tls.csr" -CA "$ca_crt" -CAkey "$ca_key" -CAcreateserial \
    -days 397 -extfile <(printf 'subjectAltName=%s\nbasicConstraints=CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=serverAuth\n' "$san") \
    -out "$root/work/novnc-tls.crt" 2>/dev/null
cat "$ca_crt" >> "$root/work/novnc-tls.crt"
echo "Certificat HTTPS : $san" >&2

# --- noVNC (pas de paquet Arch/CachyOS) ---
novnc_tar="$root/work/dl/noVNC-$novnc_version.tar.gz"
[[ -f "$novnc_tar" ]] || curl -fsSL -o "$novnc_tar" "https://github.com/novnc/noVNC/archive/refs/tags/v$novnc_version.tar.gz"
echo "$novnc_sha256  $novnc_tar" | sha256sum -c --quiet

# --- wsquashfs-launcher (jeux Windows au format Batocera), version figée ---
wsq_ref="${WSQUASHFS_REF:-a1d64ca329325dc3ea1e4834d1283f49b4e10e13}"
wsq="$root/work/dl/wsquashfs-launcher-$wsq_ref"
[[ -s "$wsq" ]] || curl -fsSL -o "$wsq" "https://raw.githubusercontent.com/Gunnm92/wsquashfs-launcher/$wsq_ref/wsquashfs-launcher"
bash -n "$wsq"

# --- Paquets AUR (xrdp), compilés une fois, servis à mkarchiso par un dépôt local ---
# --nocheck : 4 tests de la suite de xrdp (configuration du codec graphique)
# échouent dans le conteneur de build.
aur_pkgs=(xrdp)
mkdir -p "$root/work/aur/repo"
for pkg in "${aur_pkgs[@]}"; do
    ls "$root/work/aur/repo/$pkg"-[0-9]*.pkg.tar.zst >/dev/null 2>&1 && continue
    echo "Compilation AUR : $pkg" >&2
    engine run --rm -v "$root:$root" -v "$root/work/pkgcache:/var/cache/pacman/pkg" \
        -e ROOT="$root" -e PKG="$pkg" -e HOST_UID="$(id -u)" -e HOST_GID="$(id -g)" \
        "$image" bash -euo pipefail -c '
            pacman -Syu --noconfirm --needed base-devel git >/dev/null
            useradd -m builder
            echo "builder ALL=(ALL) NOPASSWD: /usr/bin/pacman" > /etc/sudoers.d/builder
            su builder -c "git clone -q https://aur.archlinux.org/$PKG.git /home/builder/$PKG \
                && cd /home/builder/$PKG && makepkg -s --noconfirm --needed --nocheck >/dev/null"
            install -m 0644 /home/builder/$PKG/$PKG-[0-9]*.pkg.tar.zst "$ROOT/work/aur/repo/"
            chown -R "$HOST_UID:$HOST_GID" "$ROOT/work/aur"
        '
done
engine run --rm -v "$root:$root" "$image" sh -c \
    "cd '$root/work/aur/repo' && rm -f aibox-aur.db* aibox-aur.files* && repo-add -q aibox-aur.db.tar.zst *.pkg.tar.zst && chown -R $(id -u):$(id -g) ."

# Le dossier de travail de mkarchiso reste dans le conteneur (supprimé à la fin) ;
# seuls le cache des paquets et l'ISO sont gardés sur le disque.
engine run --rm --privileged \
    -v "$root:$root" \
    -v "$root/work/pkgcache:/var/cache/pacman/pkg" \
    -e ROOT="$root" -e NOVNC_TAR="$novnc_tar" -e WSQ="$wsq" -e HOST_UID="$(id -u)" -e HOST_GID="$(id -g)" \
    "$image" bash -euo pipefail -c '
        pacman -Syu --noconfirm --needed archiso grub erofs-utils libisoburn mtools dosfstools >/dev/null
        cp -a "$ROOT/archiso" /tmp/profile
        # Dépôt local des paquets AUR, pour ce build seulement
        printf "\n[aibox-aur]\nSigLevel = Optional TrustAll\nServer = file://%s/work/aur/repo\n" "$ROOT" >> /tmp/profile/pacman.conf
        air=/tmp/profile/airootfs
        install -D -m 0644 "$ROOT/work/authorized_keys" "$air/etc/ssh/authorized_keys/default"
        install -m 0600 "$ROOT/work/novnc.htpasswd" "$air/etc/aibox/novnc.htpasswd"
        install -m 0600 "$ROOT/work/wayvnc-rdp.conf" "$air/etc/aibox/wayvnc-rdp.conf"
        install -D -m 0755 "$WSQ" "$air/usr/local/bin/wsquashfs-launcher"
        # Partages NAS par défaut (aibox-nas) : keys/nas-mounts et keys/nas.cred
        if [[ -s "$ROOT/keys/nas-mounts" && -s "$ROOT/keys/nas.cred" ]]; then
            install -m 0644 "$ROOT/keys/nas-mounts" "$air/etc/aibox/nas-mounts"
            install -m 0600 "$ROOT/keys/nas.cred" "$air/etc/aibox/nas.cred"
        fi
        install -m 0644 "$ROOT/work/novnc-tls.crt" "$air/etc/aibox/novnc-tls.crt"
        install -m 0600 "$ROOT/work/novnc-tls.key" "$air/etc/aibox/novnc-tls.key"

        # noVNC : seulement les fichiers servis au navigateur.
        mkdir -p "$air/usr/share/novnc"
        tar -xzf "$NOVNC_TAR" -C "$air/usr/share/novnc" --strip-components=1 --wildcards \
            "*/app" "*/core" "*/vendor" "*/vnc.html" "*/package.json" "*/defaults.json" "*/mandatory.json" "*/LICENSE.txt"
        # Page d accueil (reprise de SteamBox) : même hôte et port, WebSocket sur /websockify.
        printf "%s\n" "<!doctype html><meta charset=utf-8><title>aibox</title><script>location.replace(\"vnc.html?autoconnect=true&resize=remote&reconnect=true\");</script>" \
            > "$air/usr/share/novnc/index.html"
        # Certificat de la CA, téléchargeable pour l approuver sur un poste.
        install -m 0644 "$ROOT/keys/aibox-ca.crt" "$air/usr/share/novnc/aibox-ca.crt"

        mkarchiso -v -r -w /tmp/work -o "$ROOT/out" /tmp/profile
        chown -R "$HOST_UID:$HOST_GID" "$ROOT/out" "$ROOT/work/pkgcache"
    '

ls -lh "$root/out"/*.iso
