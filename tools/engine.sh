# tools/engine.sh : moteur de conteneurs pour build.sh et test-vm.sh (à sourcer).
#   - un vrai Docker s'il répond (hôte de développement) ;
#   - sinon, sur aibox : Podman en root (mkarchiso et QEMU ont besoin d'un conteneur
#     privilégié, impossible sans root), avec son stockage sur le disque de données
#     (/var/lib/containers est dans l'image, en RAM).
# Image de base en nom complet : Podman ne devine pas le registre d'un nom court.
image="docker.io/cachyos/cachyos:latest"

if docker info >/dev/null 2>&1 && ! docker --version 2>/dev/null | grep -qi podman; then
    engine() { docker "$@"; }
else
    command -v podman >/dev/null || { echo "Ni Docker ni Podman disponible." >&2; exit 1; }
    engine_store=/var/lib/aibox/state/containers
    if ! mountpoint -q /var/lib/aibox/state; then
        echo "Pas de disque de données (mode éphémère) : construire l'ISO demande le disque." >&2
        exit 1
    fi
    engine() { sudo podman --root "$engine_store/storage" --runroot /run/aibox-podman "$@"; }
fi
