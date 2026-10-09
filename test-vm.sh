#!/usr/bin/env bash
# test-vm.sh : démarre l'ISO dans QEMU (UEFI, KVM), depuis une clé USB virtuelle
# qu'on peut retirer à chaud, avec un disque NVMe virtuel de 64 Go (work/vm/nvme.qcow2)
# qui simule le futur disque de données (à préparer avec aibox-setup-disk).
#
#   ./test-vm.sh start        démarre la VM en arrière-plan (RAM=8G par défaut)
#                             GPU=1 : GPU de l'hôte partagé (virtio-gpu, Venus/virgl),
#                             écran de la VM en VNC sur le port 5902 (mot de passe noVNC)
#   ./test-vm.sh ssh [cmd]    SSH vers default (ISO construite avec --test-key)
#   ./test-vm.sh unplug       retire la clé USB virtuelle
#   ./test-vm.sh reboot       redémarre la VM (attend le retour de SSH)
#   ./test-vm.sh wipe-disk    supprime le disque NVMe virtuel (VM arrêtée)
#   ./test-vm.sh novnc        capture d'écran de noVNC (Chromium sans écran) → work/vm/novnc.png
#   ./test-vm.sh log          console série
#   ./test-vm.sh screen       capture de l'écran de la VM (session TV) → work/vm/screen.png
#   ./test-vm.sh stop         arrête la VM
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
vm="$root/work/vm"
name="aibox-vm"
ram="${RAM:-8G}"
gpu="${GPU:-0}"

ssh_vm() {
    docker exec -i "$name" ssh -q -p 2222 -i "$vm/id_test" \
        -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5 \
        default@127.0.0.1 "$@"
}

monitor() {
    docker exec "$name" sh -c "printf '%s\n' '$1' | socat - UNIX-CONNECT:$vm/monitor.sock" >/dev/null
}

case "${1:-}" in
start)
    iso="$(ls -t "$root"/out/*.iso | head -n 1)"
    mkdir -p "$vm" "$root/work/pkgcache"
    rm -f "$vm/serial.log" "$vm/monitor.sock"
    echo "ISO : $iso  (RAM ${ram})"
    docker rm -f "$name" >/dev/null 2>&1 || true
    gpu_docker=() gpu_pkgs="" gpu_qemu="-vga std -display none"
    if [[ "$gpu" == 1 ]]; then
        # GPU partagé : la VM utilise le GPU de l'hôte à travers QEMU (Venus pour
        # Vulkan, virgl pour OpenGL), sans le retirer à l'hôte (pas de VFIO).
        gpu_docker=(--device /dev/dri -p 5902:5902)
        gpu_pkgs="qemu-hw-display-virtio-gpu qemu-hw-display-virtio-vga qemu-hw-display-virtio-vga-gl qemu-hw-display-virtio-gpu-gl qemu-ui-egl-headless qemu-ui-opengl virglrenderer vulkan-radeon mesa"
        gpu_qemu="-object memory-backend-memfd,id=mem,size=$ram,share=on -machine memory-backend=mem \
            -device virtio-vga-gl,hostmem=4G,blob=true,venus=true \
            -display egl-headless,rendernode=/dev/dri/renderD128 -vnc 0.0.0.0:2,password=on"
    fi
    docker run -d --privileged --name "$name" --device /dev/kvm -p 6080:6080 "${gpu_docker[@]}" \
        -v "$root:$root" -v "$root/work/pkgcache:/var/cache/pacman/pkg" \
        cachyos/cachyos:latest bash -euo pipefail -c "
            pacman -Sy --noconfirm --needed qemu-system-x86 qemu-img edk2-ovmf openssh socat $gpu_pkgs >/dev/null
            [[ -f '$vm/nvme.qcow2' ]] || qemu-img create -q -f qcow2 '$vm/nvme.qcow2' 64G
            cp /usr/share/edk2/x64/OVMF_VARS.4m.fd /tmp/vars.fd
            exec qemu-system-x86_64 -enable-kvm -cpu host -smp 4 -m '$ram' -machine q35 \
                -drive if=pflash,format=raw,readonly=on,file=/usr/share/edk2/x64/OVMF_CODE.4m.fd \
                -drive if=pflash,format=raw,file=/tmp/vars.fd \
                -device qemu-xhci,id=xhci \
                -drive if=none,id=stick,format=raw,readonly=on,file='$iso' \
                -device usb-storage,bus=xhci.0,drive=stick,id=stickdev,removable=on,bootindex=0 \
                -drive if=none,id=nvm,format=qcow2,file='$vm/nvme.qcow2' \
                -device nvme,serial=AIBOXNVME,drive=nvm \
                -netdev user,id=n0,hostfwd=tcp:127.0.0.1:2222-:22,hostfwd=tcp::6080-:6080 -device virtio-net-pci,netdev=n0 \
                $gpu_qemu \
                -serial file:'$vm/serial.log' \
                -monitor unix:'$vm/monitor.sock',server,nowait
        " >/dev/null
    if [[ "$gpu" == 1 ]]; then
        sleep 5
        monitor "change vnc password $(head -c 8 "$root/keys/vnc-password")"
        echo "Écran de la VM (GPU partagé) : vnc://<ip-de-l-hôte>:5902, mot de passe = 8 premiers caractères de keys/vnc-password"
    fi
    echo "VM démarrée. Console : ./test-vm.sh log"
    ;;
ssh)    shift; ssh_vm "$@" ;;
reboot)
    ssh_vm 'sudo systemctl reboot' || true
    sleep 5
    for _ in $(seq 1 60); do ssh_vm true 2>/dev/null && { echo "VM redémarrée."; exit 0; }; sleep 3; done
    echo "SSH ne répond pas après le redémarrage." >&2; exit 1
    ;;
wipe-disk) rm -f "$vm/nvme.qcow2" "$vm/data.qcow2"; echo "Disque NVMe virtuel supprimé." ;;
unplug) monitor 'device_del stickdev'; echo "Clé USB virtuelle retirée." ;;
log)    cat "$vm/serial.log" ;;
screen)
    # Capture de l'écran de la TV (compositeur wayland-1) avec grim → work/vm/screen.png
    ssh_vm 'command -v grim >/dev/null || sudo pacman -Sy --noconfirm grim >/dev/null 2>&1
            WAYLAND_DISPLAY=wayland-1 grim -' > "$vm/screen.png"
    ls -l "$vm/screen.png"
    ;;
novnc)
    # Chromium dans le même espace réseau que la VM, piloté par son protocole de
    # débogage : ouvre noVNC avec le mot de passe de test, attend la connexion,
    # affiche l'état de noVNC et capture la page.
    pass="$(head -n 1 "$root/keys/vnc-password")"
    url="${URL:-https://default:${pass}@127.0.0.1:6080/vnc.html?autoconnect=true&resize=${RESIZE:-remote}}"
    rm -f "$vm/novnc.png"
    docker run --rm --network "container:$name" -v "$root:$root" -v "$root/work/pkgcache:/var/cache/pacman/pkg" \
        cachyos/cachyos:latest bash -c "
            pacman -Sy --noconfirm --needed chromium ttf-dejavu python-websocket-client nss >/dev/null 2>&1
            # Comme un vrai poste : seule la CA aibox est approuvée, aucune exception de certificat.
            mkdir -p ~/.pki/nssdb && certutil -N -d sql:\$HOME/.pki/nssdb --empty-password
            certutil -A -d sql:\$HOME/.pki/nssdb -n aibox-ca -t C,, -i '$root/keys/aibox-ca.crt'
            chromium --headless --no-sandbox --disable-gpu --hide-scrollbars \
                --window-size=1600,900 --remote-debugging-port=9222 '$url' >/dev/null 2>&1 &
            python3 '$root/tools/cdp-shot.py' '$vm/novnc.png' \${WAIT:-20} '${JS_FILE:-}'
            chown $(id -u):$(id -g) '$vm/novnc.png'
        "
    ;;
stop)   docker rm -f "$name" >/dev/null && echo "VM arrêtée." ;;
*)      sed -n '2,10p' "$0"; exit 1 ;;
esac
