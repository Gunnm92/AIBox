#!/usr/bin/env bash
# Compile le binaire aibox dans un conteneur Go, puis le range dans l'image :
# archiso/airootfs/usr/local/bin/aibox. Le binaire est versionné : ne le
# reconstruire que lorsque tools/aibox/ change, pour qu'il bouge peu.
#
#   tools/aibox/build.sh
#
# Le moteur de conteneurs est docker s'il existe, sinon podman (CONTENEUR=podman
# pour forcer). L'image Go vient de GO_IMAGE. Stdlib seule : aucun téléchargement
# de modules, donc pas besoin de proxy.golang.org.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
image="${GO_IMAGE:-docker.io/library/golang:1.24-alpine}"
sortie="archiso/airootfs/usr/local/bin/aibox"

moteur="${CONTENEUR:-}"
if [[ -z "$moteur" ]]; then
    if command -v docker >/dev/null 2>&1; then moteur=docker; else moteur=podman; fi
fi

# Le fichier produit doit appartenir à l'utilisateur, pas à root. Avec podman
# rootless, --userns=keep-id garde les UID de l'hôte ; avec docker, --user suffit.
# (sur cette machine, « docker » est un alias vers podman)
droits=(--user "$(id -u):$(id -g)")
if "$moteur" --version 2>/dev/null | grep -qi podman; then
    droits=(--userns=keep-id)
fi

# Tests et vet d'abord : un binaire qui échoue aux tests n'est pas rangé.
"$moteur" run --rm \
    "${droits[@]}" \
    -e HOME=/tmp -e GOCACHE=/tmp/gocache -e CGO_ENABLED=0 -e GOFLAGS=-mod=readonly \
    -v "$repo:/src" -w /src/tools/aibox \
    "$image" \
    sh -c "go vet ./... && go test ./... && go build -trimpath -ldflags='-s -w' -o /src/$sortie ."

chmod 755 "$repo/$sortie"
echo "Binaire : $sortie"
sha256sum "$repo/$sortie"
