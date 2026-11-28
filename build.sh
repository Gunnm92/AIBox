#!/usr/bin/env bash
# build.sh : construit l'ISO aibox dans un conteneur CachyOS (Docker).
#
#   ./build.sh               ISO avec keys/authorized_keys, keys/vnc-password (mot de
#                            passe du bureau web, nom historique) et,
#                            s'ils existent, keys/nas-mounts + keys/nas.cred (partages NAS)
#   ./build.sh --test-key    ajoute la clé SSH de test de la VM (work/vm/id_test)
#
# Sortie : out/aibox-<version>-x86_64.iso, et le lien out/aibox-latest.iso.
# Version : AAAA.MM.JJ.HHMM-<commit>[-dirty] (heure locale du build, commit du
# dépôt ; -dirty : archiso/ ou build.sh modifiés sans commit). AIBOX_VERSION
# l'impose. Unique à chaque build : aibox boot ne remplace plus la
# version précédente, qui reste au menu de démarrage pour revenir en arrière.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$root/tools/engine.sh"
# Emplacements des secrets, des caches et de l'ISO : dans le dépôt par défaut
# (build à la main) ; aibox build les met sur le disque de données
# (/var/lib/aibox/state/build), hors du dépôt.
keys_dir="${AIBOX_KEYS:-$root/keys}"
work_dir="${AIBOX_WORK:-$root/work}"
out_dir="${AIBOX_OUT:-$root/out}"
export AIBOX_WORK="$work_dir"   # pour tools/whitesur.sh
# Selkies (bureau d'administration dans le navigateur), installé par pip dans
# /opt/selkies : version épinglée, comme SteamBox.
selkies_version="2.2.1"
test_key=0
[[ "${1:-}" == "--test-key" ]] && test_key=1

mkdir -p "$out_dir" "$work_dir/pkgcache" "$work_dir/vm" "$work_dir/dl" "$work_dir/pip"

# --- Version de l'image ---
if [[ -z "${AIBOX_VERSION:-}" ]]; then
    AIBOX_VERSION="$(date +%Y.%m.%d.%H%M)"
    if commit="$(git -C "$root" rev-parse --short=7 HEAD 2>/dev/null)"; then
        AIBOX_VERSION+="-$commit"
        [[ -n "$(git -C "$root" status --porcelain -- archiso build.sh)" ]] && AIBOX_VERSION+="-dirty"
    fi
fi
# Nom de dossier sur AIBOXBOOT (FAT32) et d'entrée de systemd-boot : pas d'espace
# ni de caractère spécial.
[[ "$AIBOX_VERSION" =~ ^[0-9][A-Za-z0-9._-]*$ ]] || { echo "Version invalide : $AIBOX_VERSION" >&2; exit 1; }
echo "Version : $AIBOX_VERSION" >&2

# --- Clés SSH ---
keys="$work_dir/authorized_keys"
: > "$keys"
if [[ -s "$keys_dir/authorized_keys" ]]; then
    cat "$keys_dir/authorized_keys" >> "$keys"
else
    echo "Attention : keys/authorized_keys absent, pas d'accès SSH avec ta clé (console seulement)." >&2
fi
if (( test_key )); then
    [[ -f "$work_dir/vm/id_test" ]] || ssh-keygen -q -t ed25519 -N '' -C 'aibox-vm-test' -f "$work_dir/vm/id_test"
    cat "$work_dir/vm/id_test.pub" >> "$keys"
    echo "Clé de test ajoutée : ne pas utiliser cette ISO sur la vraie machine." >&2
fi

# --- Mot de passe du bureau web (Selkies, code-server) ---
# Le bureau d'administration a un sudo sans mot de passe : l'accès web est
# toujours protégé. Sans keys/vnc-password (nom du temps de noVNC, gardé), un
# mot de passe est généré.
if [[ ! -s "$keys_dir/vnc-password" ]]; then
    head -c 15 /dev/urandom | base64 | tr -d '/+=' > "$keys_dir/vnc-password"
    chmod 600 "$keys_dir/vnc-password"
    echo "Mot de passe du bureau web généré dans keys/vnc-password : $(cat "$keys_dir/vnc-password")" >&2
fi
# Fichier dérivé du mot de passe : lisible par son propriétaire seulement.
( umask 077; : > "$work_dir/web.htpasswd" )
chmod 600 "$work_dir/web.htpasswd"
printf 'default:%s\n' "$(openssl passwd -6 -stdin < <(head -n 1 "$keys_dir/vnc-password"))" > "$work_dir/web.htpasswd"

# --- HTTPS (Selkies, code-server) : autorité de certification locale ---
# Une CA aibox, créée une fois et gardée dans keys/ : à ajouter une fois au
# trousseau du poste (« Toujours approuver »). Un certificat auto-signé accepté
# d'un clic ne suffit pas : le navigateur refuse alors les service workers,
# dont les vues web de code-server (panneau Claude Code) ont besoin.
# La CA est limitée aux noms aibox et aux adresses privées (nameConstraints).
ca_key="$keys_dir/aibox-ca.key" ca_crt="$keys_dir/aibox-ca.crt"
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
if [[ -s "$keys_dir/tls-ips" ]]; then
    while read -r ip; do [[ -n "$ip" && "$ip" != \#* ]] && san+=",IP:$ip"; done < "$keys_dir/tls-ips"
fi
openssl req -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes -subj "/CN=aibox" \
    -keyout "$work_dir/web-tls.key" -out "$work_dir/web-tls.csr" 2>/dev/null
openssl x509 -req -in "$work_dir/web-tls.csr" -CA "$ca_crt" -CAkey "$ca_key" -CAcreateserial \
    -days 397 -extfile <(printf 'subjectAltName=%s\nbasicConstraints=CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=serverAuth\n' "$san") \
    -out "$work_dir/web-tls.crt" 2>/dev/null
cat "$ca_crt" >> "$work_dir/web-tls.crt"
echo "Certificat HTTPS : $san" >&2

# --- Thème WhiteSur (thème GTK, icônes, curseurs), compilé une fois puis en cache ---
ws_tar="$("$root/tools/whitesur.sh")"

# Le dossier de travail de mkarchiso reste dans le conteneur (supprimé à la fin) ;
# seuls le cache des paquets et l'ISO sont gardés sur le disque.
engine run --rm --privileged \
    -v "$root:$root" -v "$keys_dir:$keys_dir" -v "$work_dir:$work_dir" -v "$out_dir:$out_dir" \
    -v "$work_dir/pkgcache:/var/cache/pacman/pkg" \
    -e KEYS="$keys_dir" -e WORK="$work_dir" -e OUT="$out_dir" \
    -e ROOT="$root" -e HOST_UID="$(id -u)" -e HOST_GID="$(id -g)" \
    -e AIBOX_VERSION="$AIBOX_VERSION" -e WS_TAR="$ws_tar" -e SELKIES_VERSION="$selkies_version" \
    "$image" bash -euo pipefail -c '
        pacman -Syu --noconfirm --needed archiso grub erofs-utils libisoburn mtools dosfstools \
            gcc make wayland libxkbcommon python >/dev/null
        cp -a "$ROOT/archiso" /tmp/profile
        # evdev-bridge (clavier et souris de Moonlight dans le bureau d administration)
        cp -a "$ROOT/evdev-bridge" /tmp/evdev-bridge
        make -s -C /tmp/evdev-bridge
        air=/tmp/profile/airootfs
        install -D -m 0644 "$WORK/authorized_keys" "$air/etc/ssh/authorized_keys/default"
        install -m 0600 "$WORK/web.htpasswd" "$air/etc/aibox/web.htpasswd"
        install -D -m 0755 /tmp/evdev-bridge/evdev-bridge-native "$air/usr/local/bin/evdev-bridge-native"
        # WhiteSur : usr/share/themes et usr/share/icons (tools/whitesur.sh)
        tar --zstd -C "$air" -xf "$WS_TAR"
        # Partages NAS par défaut (aibox nas) : keys/nas-mounts et keys/nas.cred
        if [[ -s "$KEYS/nas-mounts" && -s "$KEYS/nas.cred" ]]; then
            install -m 0644 "$KEYS/nas-mounts" "$air/etc/aibox/nas-mounts"
            install -m 0600 "$KEYS/nas.cred" "$air/etc/aibox/nas.cred"
        fi
        install -m 0644 "$WORK/web-tls.crt" "$air/etc/aibox/web-tls.crt"
        install -m 0600 "$WORK/web-tls.key" "$air/etc/aibox/web-tls.key"
        # Certificat de la CA, téléchargeable (https://aibox:6080/aibox-ca.crt)
        # pour l approuver sur un poste.
        install -m 0644 "$KEYS/aibox-ca.crt" "$air/etc/aibox/aibox-ca.crt"

        # Selkies dans un venv à son chemin définitif (/opt/selkies : les scripts
        # du venv y pointent), avec le Python de ce conteneur, le même que celui
        # de l image (mêmes dépôts, même jour). Cache pip dans work/pip.
        python -m venv /opt/selkies
        /opt/selkies/bin/pip install -q --disable-pip-version-check \
            --cache-dir "$WORK/pip" "selkies==$SELKIES_VERSION"
        # Nom et logo AIBox dans le bureau navigateur (28/11/2026, comme SteamBox
        # 0c9c0c6) : titre de l en-tête et de l onglet par SELKIES_UI_TITLE
        # (aibox-admin-selkies.service) ; ici, ce que Selkies ne règle pas : le
        # titre de la page avant connexion, le nom de l appli installée
        # (manifest) et les icônes (icons/aibox-icon-512.png, logo blanc sur
        # fond sombre). Le logo de l en-tête est intégré au JavaScript : gardé.
        # Échec = build en échec.
        W="$(/opt/selkies/bin/python -c "import os,selkies;print(os.path.dirname(selkies.__file__)+\"/selkies_web\")")"
        ICON="$ROOT/icons/aibox-icon-512.png"
        cp "$ICON" "$W/icon-512.png"
        /opt/selkies/bin/python -c "from PIL import Image;import sys;i=Image.open(sys.argv[1]).convert(\"RGBA\");i.resize((180,180),Image.LANCZOS).save(sys.argv[2]+\"/icon.png\");i.save(sys.argv[2]+\"/favicon.ico\",sizes=[(16,16),(32,32),(48,48),(64,64)])" "$ICON" "$W"
        sed -i "s|<title>Selkies</title>|<title>AIBox</title>|" "$W/index.html"
        /opt/selkies/bin/python -c "import json,sys;p=sys.argv[1];m=json.load(open(p));m[\"name\"]=m[\"short_name\"]=\"AIBox\";json.dump(m,open(p,\"w\"))" "$W/manifest.json"
        { grep -q "<title>AIBox</title>" "$W/index.html" && grep -q "\"name\": \"AIBox\"" "$W/manifest.json"; } \
            || { echo "Selkies : nom et logo AIBox non appliqués" >&2; exit 1; }
        mkdir -p "$air/opt"
        cp -a /opt/selkies "$air/opt/selkies"
        chown -R "$HOST_UID:$HOST_GID" "$WORK/pip"

        mkarchiso -v -r -w /tmp/work -o "$OUT" /tmp/profile
        # L ISO contient des secrets (clé TLS, mot de passe haché, identifiants NAS).
        chmod 600 "$OUT/aibox-$AIBOX_VERSION-x86_64.iso"
        chown -R "$HOST_UID:$HOST_GID" "$OUT" "$WORK/pkgcache"
    '

# Lien vers la dernière ISO ; seules les 3 plus récentes sont gardées (1,7 Go chacune).
iso="aibox-$AIBOX_VERSION-x86_64.iso"
[[ -f "$out_dir/$iso" ]] || { echo "ISO introuvable : out/$iso" >&2; exit 1; }
ln -sfn "$iso" "$out_dir/aibox-latest.iso"
find "$out_dir" -maxdepth 1 -type f -name 'aibox-*-x86_64.iso' -printf '%T@ %p\n' \
    | sort -rn | tail -n +4 | cut -d' ' -f2- | xargs -r rm -f --
ls -lh "$out_dir"/
echo "ISO : $out_dir/$iso (lien : $out_dir/aibox-latest.iso)" >&2
