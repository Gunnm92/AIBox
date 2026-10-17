#!/usr/bin/env bash
# tools/whitesur.sh : thème WhiteSur (vinceliuice, MIT, style macOS) pour le
# bureau d'administration, compilé dans un conteneur CachyOS et gardé en cache :
#   work/dl/whitesur-<commits>.tar.zst  (usr/share/themes, usr/share/icons)
# Appelé par build.sh, qui l'extrait dans l'image ; affiche le chemin du cache.
#   - WhiteSur-Dark : GTK 2/3/4, libadwaita (CSS dans WhiteSur-Dark/libadwaita,
#     relié dans ~/.config/gtk-4.0 par admin-session), xfwm4 et labwc ;
#   - icônes WhiteSur, WhiteSur-dark et WhiteSur-light ; curseurs WhiteSur-cursors.
# D'autres thèmes s'installent dans le home (~/.local/share/themes, sur le disque).
# Repris de SteamBox (Dockerfile.cachyos, 26/09/2026), mêmes commits. Pièges de
# l'installateur (vécus sur SteamBox et ici) :
#   - -l (libadwaita) refusé en root : généré par un utilisateur temporaire ;
#   - compte déduit de SUDO_USER, logname puis USER, vides dans un conteneur
#     (arrêt silencieux) : USER explicite ; setterm/tput sans TERM : TERM=xterm ;
#   - -d refusé s'il commence par le chemin des sources : /tmp/src-whitesur et
#     /tmp/adw-dest (pas /tmp/ws et /tmp/ws-…) ;
#   - icônes : gtk-update-icon-cache nécessaire.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
. "$root/tools/engine.sh"
gtk_ref="${WHITESUR_REF:-d5782652d412137e26fb8ff55b55a5572e4c6995}"
icons_ref="${WHITESUR_ICONS_REF:-73d8040da51a9ed74e47c7366e7e9ff437601a5c}"
cursors_ref="${WHITESUR_CURSORS_REF:-e190baf618ed95ee217d2fd45589bd309b37672b}"
dl="${AIBOX_WORK:-$root/work}/dl"
out="$dl/whitesur-${gtk_ref:0:7}-${icons_ref:0:7}-${cursors_ref:0:7}.tar.zst"

if [[ ! -s "$out" ]]; then
    echo "Compilation de WhiteSur (une fois, gardé dans work/dl/)…" >&2
    mkdir -p "$dl"
    # pacman -Sy sans -u dans le conteneur : son image de base a parfois des
    # paquets plus récents que les miroirs (rétrogradation de zstd, hook en échec).
    engine run --rm -v "$dl:/out" \
        -e GTK_REF="$gtk_ref" -e ICONS_REF="$icons_ref" -e CURSORS_REF="$cursors_ref" \
        -e OUT="/out/${out##*/}" -e HOST_UID="$(id -u)" -e HOST_GID="$(id -g)" \
        "$image" bash -euo pipefail -c '
            pacman -Sy --noconfirm --needed sassc gtk-update-icon-cache >/dev/null
            dest=/tmp/aibox-root
            themes=$dest/usr/share/themes icons=$dest/usr/share/icons
            mkdir -p "$themes" "$icons"
            git clone -q https://github.com/vinceliuice/WhiteSur-gtk-theme.git /tmp/src-whitesur
            git -C /tmp/src-whitesur checkout -q "$GTK_REF"
            useradd -m wsbuild && chown -R wsbuild /tmp/src-whitesur
            (cd /tmp/src-whitesur && runuser -u wsbuild -- env USER=wsbuild HOME=/home/wsbuild TERM=xterm \
                ./install.sh -d /tmp/adw-dest -c dark -l >/dev/null)
            adw=/home/wsbuild/.config/gtk-4.0
            [[ -f $adw/gtk-Dark.css ]] || { echo "CSS libadwaita de WhiteSur non généré" >&2; exit 1; }
            (cd /tmp/src-whitesur && USER=root TERM=xterm ./install.sh -d "$themes" -c dark >/dev/null)
            [[ -f $themes/WhiteSur-Dark/gtk-4.0/gtk.css ]] || { echo "WhiteSur-Dark non installé" >&2; exit 1; }
            mkdir -p "$themes/WhiteSur-Dark/libadwaita"
            cp -a "$adw/gtk-Dark.css" "$adw/assets" "$themes/WhiteSur-Dark/libadwaita/"
            [[ ! -d $adw/windows-assets ]] || cp -a "$adw/windows-assets" "$themes/WhiteSur-Dark/libadwaita/"
            git clone -q https://github.com/vinceliuice/WhiteSur-icon-theme.git /tmp/src-icons
            git -C /tmp/src-icons checkout -q "$ICONS_REF"
            (cd /tmp/src-icons && ./install.sh -d "$icons" >/dev/null)
            [[ -f $icons/WhiteSur-dark/index.theme ]] || { echo "Icônes WhiteSur-dark non installées" >&2; exit 1; }
            git clone -q https://github.com/vinceliuice/WhiteSur-cursors.git /tmp/src-cursors
            git -C /tmp/src-cursors checkout -q "$CURSORS_REF"
            cp -a /tmp/src-cursors/dist "$icons/WhiteSur-cursors"
            [[ -f $icons/WhiteSur-cursors/index.theme ]] || { echo "Curseurs WhiteSur non installés" >&2; exit 1; }
            chown -R root:root "$dest"
            du -sh "$themes" "$icons"/* >&2
            tar -C "$dest" -c usr | zstd -q -19 -T0 > "$OUT.part"
            mv "$OUT.part" "$OUT" && chown "$HOST_UID:$HOST_GID" "$OUT"
        '
fi
echo "$out"
