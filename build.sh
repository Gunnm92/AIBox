#!/usr/bin/env bash
# build.sh : construit l'ISO aibox dans un conteneur CachyOS (Docker).
#
#   ./build.sh               ISO avec keys/authorized_keys, keys/vnc-password et,
#                            s'ils existent, keys/nas-mounts + keys/nas.cred (partages NAS)
#   ./build.sh --test-key    ajoute la clé SSH de test de la VM (work/vm/id_test)
#
# Sortie : out/aibox-<version>-x86_64.iso, et le lien out/aibox-latest.iso.
# Version : AAAA.MM.JJ.HHMM-<commit>[-dirty] (heure locale du build, commit du
# dépôt ; -dirty : archiso/ ou build.sh modifiés sans commit). AIBOX_VERSION
# l'impose. Unique à chaque build : aibox-update-boot ne remplace plus la
# version précédente, qui reste au menu de démarrage pour revenir en arrière.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$root/tools/engine.sh"
# Emplacements des secrets, des caches et de l'ISO : dans le dépôt par défaut
# (build à la main) ; aibox-build les met sur le disque de données
# (/var/lib/aibox/state/build), hors du dépôt.
keys_dir="${AIBOX_KEYS:-$root/keys}"
work_dir="${AIBOX_WORK:-$root/work}"
out_dir="${AIBOX_OUT:-$root/out}"
export AIBOX_WORK="$work_dir"   # pour tools/whitesur.sh
novnc_version="1.7.0"
novnc_sha256="b1003a11b6e6e8d8f7f5e5586daae7f8ca651d8aee0aa155ff9ac841c48f52c6"
test_key=0
[[ "${1:-}" == "--test-key" ]] && test_key=1

mkdir -p "$out_dir" "$work_dir/pkgcache" "$work_dir/vm" "$work_dir/dl"

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

# --- Mot de passe noVNC ---
# Le bureau d'administration a un sudo sans mot de passe : l'accès noVNC est
# toujours protégé. Sans keys/vnc-password, un mot de passe est généré.
if [[ ! -s "$keys_dir/vnc-password" ]]; then
    head -c 15 /dev/urandom | base64 | tr -d '/+=' > "$keys_dir/vnc-password"
    chmod 600 "$keys_dir/vnc-password"
    echo "Mot de passe noVNC généré dans keys/vnc-password : $(cat "$keys_dir/vnc-password")" >&2
fi
printf 'default:%s\n' "$(openssl passwd -6 -stdin < <(head -n 1 "$keys_dir/vnc-password"))" > "$work_dir/novnc.htpasswd"
# wayvnc local pour xrdp : authentification VNC classique (8 caractères max).
printf 'enable_auth=true\nallow_broken_crypto=true\nrelax_encryption=true\npassword=%s\n' "$(head -n 1 "$keys_dir/vnc-password" | cut -c1-8)" > "$work_dir/wayvnc-rdp.conf"

# --- HTTPS (noVNC, code-server) : autorité de certification locale ---
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
    -keyout "$work_dir/novnc-tls.key" -out "$work_dir/novnc-tls.csr" 2>/dev/null
openssl x509 -req -in "$work_dir/novnc-tls.csr" -CA "$ca_crt" -CAkey "$ca_key" -CAcreateserial \
    -days 397 -extfile <(printf 'subjectAltName=%s\nbasicConstraints=CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=serverAuth\n' "$san") \
    -out "$work_dir/novnc-tls.crt" 2>/dev/null
cat "$ca_crt" >> "$work_dir/novnc-tls.crt"
echo "Certificat HTTPS : $san" >&2

# --- noVNC (pas de paquet Arch/CachyOS) ---
novnc_tar="$work_dir/dl/noVNC-$novnc_version.tar.gz"
[[ -f "$novnc_tar" ]] || curl -fsSL -o "$novnc_tar" "https://github.com/novnc/noVNC/archive/refs/tags/v$novnc_version.tar.gz"
echo "$novnc_sha256  $novnc_tar" | sha256sum -c --quiet

# --- wsquashfs-launcher (jeux Windows au format Batocera), version figée ---
wsq_ref="${WSQUASHFS_REF:-a1d64ca329325dc3ea1e4834d1283f49b4e10e13}"
wsq="$work_dir/dl/wsquashfs-launcher-$wsq_ref"
[[ -s "$wsq" ]] || curl -fsSL -o "$wsq" "https://raw.githubusercontent.com/Gunnm92/wsquashfs-launcher/$wsq_ref/wsquashfs-launcher"
bash -n "$wsq"

# --- Thème WhiteSur (thème GTK, icônes, curseurs), compilé une fois puis en cache ---
ws_tar="$("$root/tools/whitesur.sh")"

# --- Paquets AUR (xrdp), compilés une fois, servis à mkarchiso par un dépôt local ---
# --nocheck : 4 tests de la suite de xrdp (configuration du codec graphique)
# échouent dans le conteneur de build.
aur_pkgs=(xrdp)
mkdir -p "$work_dir/aur/repo"
for pkg in "${aur_pkgs[@]}"; do
    ls "$work_dir/aur/repo/$pkg"-[0-9]*.pkg.tar.zst >/dev/null 2>&1 && continue
    echo "Compilation AUR : $pkg" >&2
    engine run --rm -v "$root:$root" -v "$work_dir/pkgcache:/var/cache/pacman/pkg" \
        -v "$work_dir:$work_dir" -e WORK="$work_dir" \
        -e ROOT="$root" -e PKG="$pkg" -e HOST_UID="$(id -u)" -e HOST_GID="$(id -g)" \
        "$image" bash -euo pipefail -c '
            pacman -Syu --noconfirm --needed base-devel git >/dev/null
            useradd -m builder
            echo "builder ALL=(ALL) NOPASSWD: /usr/bin/pacman" > /etc/sudoers.d/builder
            su builder -c "git clone -q https://aur.archlinux.org/$PKG.git /home/builder/$PKG \
                && cd /home/builder/$PKG && makepkg -s --noconfirm --needed --nocheck >/dev/null"
            install -m 0644 /home/builder/$PKG/$PKG-[0-9]*.pkg.tar.zst "$WORK/aur/repo/"
            chown -R "$HOST_UID:$HOST_GID" "$WORK/aur"
        '
done
engine run --rm -v "$work_dir:$work_dir" "$image" sh -c \
    "cd '$work_dir/aur/repo' && rm -f aibox-aur.db* aibox-aur.files* && repo-add -q aibox-aur.db.tar.zst *.pkg.tar.zst && chown -R $(id -u):$(id -g) ."

# Le dossier de travail de mkarchiso reste dans le conteneur (supprimé à la fin) ;
# seuls le cache des paquets et l'ISO sont gardés sur le disque.
engine run --rm --privileged \
    -v "$root:$root" -v "$keys_dir:$keys_dir" -v "$work_dir:$work_dir" -v "$out_dir:$out_dir" \
    -v "$work_dir/pkgcache:/var/cache/pacman/pkg" \
    -e KEYS="$keys_dir" -e WORK="$work_dir" -e OUT="$out_dir" \
    -e ROOT="$root" -e NOVNC_TAR="$novnc_tar" -e WSQ="$wsq" -e HOST_UID="$(id -u)" -e HOST_GID="$(id -g)" \
    -e AIBOX_VERSION="$AIBOX_VERSION" -e WS_TAR="$ws_tar" \
    "$image" bash -euo pipefail -c '
        pacman -Syu --noconfirm --needed archiso grub erofs-utils libisoburn mtools dosfstools \
            gcc make wayland libxkbcommon >/dev/null
        cp -a "$ROOT/archiso" /tmp/profile
        # evdev-bridge (clavier et souris de Moonlight dans le bureau d administration)
        cp -a "$ROOT/evdev-bridge" /tmp/evdev-bridge
        make -s -C /tmp/evdev-bridge
        # Dépôt local des paquets AUR, pour ce build seulement
        printf "\n[aibox-aur]\nSigLevel = Optional TrustAll\nServer = file://%s/aur/repo\n" "$WORK" >> /tmp/profile/pacman.conf
        air=/tmp/profile/airootfs
        install -D -m 0644 "$WORK/authorized_keys" "$air/etc/ssh/authorized_keys/default"
        install -m 0600 "$WORK/novnc.htpasswd" "$air/etc/aibox/novnc.htpasswd"
        install -m 0600 "$WORK/wayvnc-rdp.conf" "$air/etc/aibox/wayvnc-rdp.conf"
        install -D -m 0755 "$WSQ" "$air/usr/local/bin/wsquashfs-launcher"
        install -D -m 0755 /tmp/evdev-bridge/evdev-bridge-native "$air/usr/local/bin/evdev-bridge-native"
        # WhiteSur : usr/share/themes et usr/share/icons (tools/whitesur.sh)
        tar --zstd -C "$air" -xf "$WS_TAR"
        # Partages NAS par défaut (aibox-nas) : keys/nas-mounts et keys/nas.cred
        if [[ -s "$KEYS/nas-mounts" && -s "$KEYS/nas.cred" ]]; then
            install -m 0644 "$KEYS/nas-mounts" "$air/etc/aibox/nas-mounts"
            install -m 0600 "$KEYS/nas.cred" "$air/etc/aibox/nas.cred"
        fi
        install -m 0644 "$WORK/novnc-tls.crt" "$air/etc/aibox/novnc-tls.crt"
        install -m 0600 "$WORK/novnc-tls.key" "$air/etc/aibox/novnc-tls.key"

        # noVNC : seulement les fichiers servis au navigateur.
        mkdir -p "$air/usr/share/novnc"
        tar -xzf "$NOVNC_TAR" -C "$air/usr/share/novnc" --strip-components=1 --wildcards \
            "*/app" "*/core" "*/vendor" "*/vnc.html" "*/package.json" "*/defaults.json" "*/mandatory.json" "*/LICENSE.txt"
        # Page d accueil (reprise de SteamBox) : même hôte et port, WebSocket sur /websockify.
        printf "%s\n" "<!doctype html><meta charset=utf-8><title>aibox</title><script>location.replace(\"vnc.html?autoconnect=true&resize=remote&reconnect=true\");</script>" \
            > "$air/usr/share/novnc/index.html"
        # Certificat de la CA, téléchargeable pour l approuver sur un poste.
        install -m 0644 "$KEYS/aibox-ca.crt" "$air/usr/share/novnc/aibox-ca.crt"

        mkarchiso -v -r -w /tmp/work -o "$OUT" /tmp/profile
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
