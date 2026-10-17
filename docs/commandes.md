# Commandes et automatismes d'aibox

Index de tout ce qu'aibox ajoute au système : les commandes à taper, et ce qui tourne tout seul. Chaque ligne renvoie à la section qui détaille l'usage. Les scripts sont dans `archiso/airootfs/usr/local/` (`bin/`, `sbin/`, `lib/aibox/`).

## Commandes de tous les jours

| Commande | À quoi ça sert | Usage | Détails |
|---|---|---|---|
| `aibox-run-exe` | Lancer un programme Windows (.exe, .msi) avec un Proton déjà installé, dans un préfixe Wine partagé | `aibox-run-exe prog.exe [args]`, ou double-clic dans Thunar | [exploitation.md](exploitation.md#programmes-windows-exe-msi) |
| `wsquashfs-launcher` | Lancer un jeu Windows au format Batocera (`.wsquashfs`) | `wsquashfs-launcher "Jeu.wsquashfs"`, `--info` | [exploitation.md](exploitation.md#jeux-windows-wsquashfs) |
| `aibox-tv-mode` | Mode de la session TV : Steam sous gamescope ou labwc avec le lanceur | `aibox-tv-mode [gamescope\|labwc]` | [exploitation.md](exploitation.md#session-tv) |
| `aibox-auto-update` | Mise à jour de l'image décidée par le LLM local (lancée aussi chaque nuit) | `--dry-run`, `--no-install`, `--force` | [exploitation.md](exploitation.md#mise-à-jour-automatique-de-limage) |
| `aibox-nas` | Partages réseau (SMB) montés à la demande | `aibox-nas list`, `sudo aibox-nas add //hôte/partage /mnt/X`, `sudo aibox-nas remove /mnt/X` | [exploitation.md](exploitation.md#partages-du-nas) |
| `aibox-keyboard` | Disposition du clavier du bureau d'administration (noVNC) | `aibox-keyboard [mac\|pc\|<xkb> [variante]]` | [exploitation.md](exploitation.md#bureau-dadministration) |
| `aibox-resolution` | Taille de l'écran virtuel du bureau d'administration | `aibox-resolution [1920x1080[@60]\|client]` | [exploitation.md](exploitation.md#sunshine-diffusion-vers-moonlight) |
| `aibox-mem` | État de la mémoire : mémoire bloquée, GPU, image en RAM, clé, zram | `aibox-mem` | [exploitation.md](exploitation.md#état-général) |
| `aibox-diag` | Rapport de diagnostic complet, enregistré sur la clé Ventoy ou sur AIBOXBOOT | `sudo aibox-diag` | [exploitation.md](exploitation.md#état-général) |

## Commandes d'administration (destructrices ou sur le démarrage)

À lancer seulement en connaissance de cause (voir [AGENTS.md](../AGENTS.md), règle 5).

| Commande | À quoi ça sert | Usage | Détails |
|---|---|---|---|
| `aibox-update-boot` | Installer une nouvelle image sur AIBOXBOOT, sans toucher aux données ; garde la précédente pour revenir en arrière | `sudo aibox-update-boot --iso /var/lib/aibox/state/build/out/aibox-latest.iso [--keep 2]` | [iso.md](iso.md) |
| `aibox-setup-disk` | **Efface un disque** et le prépare (AIBOXBOOT + AIBOXDATA). Une seule fois | `sudo aibox-setup-disk /dev/nvmeXnY [--swap 16G] [--iso F]` | [iso.md](iso.md) |
| `aibox-build` | Construire l'image depuis GitHub (`origin/main`), avec ses caches et ses secrets dans `/var/lib/aibox/state/build/` ; jamais `-dirty` | `sudo aibox-build [--ref <branche ou commit>]`, `--fetch-only`, `--import ~/Workspace/AIBox` (une fois) ; ou `sudo systemctl start aibox-build` | [iso.md](iso.md#5-construire) |
| `./build.sh` (dépôt) | Build à la main dans le dépôt (`keys/`, `work/`, `out/`), pour un essai en VM ; pas pour une image installée | `./build.sh [--test-key]` | [iso.md](iso.md#5-construire) |

## Outils internes appelables à la main

Ils sont dans `/usr/local/lib/aibox/`, hors du `PATH`.

| Script | À quoi ça sert | Usage |
|---|---|---|
| `llm-unload` | Décharger les modèles GPU de Lemonade (inactifs, ou tous à l'ouverture de la session TV) | `/usr/local/lib/aibox/llm-unload --idle\|--tv [--dry-run]` ; réglages : `/etc/aibox/llm.conf` |
| `lemonade-backends-update` | Mettre à jour les moteurs de Lemonade déclarés périmés | lancé au démarrage par `aibox-lemonade-backends` |
| `code-server-install` | Installer code-server et l'extension Claude Code dans le home | `/usr/local/lib/aibox/code-server-install` |

## Ce qui tourne tout seul

### Minuteurs

| Minuteur | Quand | Ce qu'il fait |
|---|---|---|
| `aibox-auto-update.timer` (utilisateur) | Chaque nuit à 4 h, rattrapé au démarrage | `aibox-auto-update` : nouveaux commits poussés ou avis du LLM sur les paquets, build par `aibox-build`, installation, message Discord. Jamais de redémarrage |
| `aibox-llm-idle.timer` (utilisateur) | Toutes les 5 min | `llm-unload --idle` : décharge les modèles GPU inutilisés depuis 30 min |
| `aibox-diag.timer` | 2 min après le démarrage | `aibox-diag` |
| `aibox-tv-idle.timer` | 2 min après l'extinction de la manette | Arrête la session TV |

### Services déclenchés par un événement

| Service | Déclencheur | Ce qu'il fait | Détails |
|---|---|---|---|
| `aibox-build` | `aibox-auto-update`, ou à la main (`systemctl start`) | Build de l'ISO (`aibox-build`) | [iso.md](iso.md#5-construire) |
| `aibox-tv-watch` | Manette allumée ou éteinte (scrute le dongle toutes les 2 s) | Démarre la session TV, ou programme son arrêt | [architecture.md](architecture.md) |
| `aibox-tv` | `aibox-tv-watch`, `aibox-tv-mode` | Session TV sur l'écran réel (`tv-session`). À son démarrage : `llm-unload --tv` | [exploitation.md](exploitation.md#session-tv) |
| `ds5-haptics-loopback` (utilisateur) | Branchement du dongle (udev) | Envoie le son du PC au dongle pour les auto-haptiques | [exploitation.md](exploitation.md#manette-dualsense-dongle-ds5dongle) |
| `steamos-session-select` | « Passer au bureau » dans Steam | Bascule la session TV sur labwc | [architecture.md](architecture.md) |

### Services de démarrage

| Service | Ce qu'il fait |
|---|---|
| `aibox-data-mount`, `aibox-data` | Monte AIBOXDATA et ses sous-volumes (`data-mount`), sinon mode éphémère ; prépare les dossiers (`data-check`) |
| `aibox-nas` | Monte les partages du NAS (`aibox-nas mount`) |
| `aibox-ssh-hostkeys` | Clés d'hôte SSH gardées sur le disque |
| `aibox-flathub` | Flathub, Bazaar et Gear Lever, au premier démarrage avec le disque |
| `aibox-lemonade-backends` (utilisateur) | `lemonade-backends-update` |
| `aibox-code-server` (utilisateur) | code-server (VS Code dans le navigateur) |
| `aibox-admin.target` (utilisateur) | Bureau d'administration : labwc headless, wayvnc, noVNC, panneau, XFCE (`admin-session`, `admin-wayvnc`, `xfce-defaults`) |
| `aibox-evdev-bridge` (utilisateur) | Clavier et souris de Moonlight rejoués dans le bureau d'administration |

## Fichiers de réglages

Les réglages de l'image sont dans `/etc/aibox/`. Pour en changer un, copie le fichier dans `~/.config/aibox/`, qui est sur le disque de données et prime sur `/etc`.

| Fichier | Pour |
|---|---|
| `auto-update.conf` | `aibox-auto-update` : dossier et branche du build, modèle, destinataire. Journal : `~/.local/state/aibox/auto-update/journal.log` |
| `llm.conf` | `llm-unload` : délai d'inactivité, déchargement à l'ouverture de la TV |
| `~/.config/aibox/proton.conf` | `aibox-run-exe` : Proton imposé (`PROTON=…`) |
| `~/.config/aibox/tv-mode` | `aibox-tv-mode` |
| `~/.config/aibox/tv-apps.json` | Applis du lanceur de la session TV |
| `~/.config/aibox/keyboard.env` | `aibox-keyboard` |
