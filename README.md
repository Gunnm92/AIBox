# aibox

Un home server branché à la TV, avec une **IA toujours disponible qui écoute et pilote la maison**. C'est un système live CachyOS qui démarre sur clé USB, tourne entièrement en RAM, et garde toutes les données de l'utilisateur sur un disque séparé. La partie TV et administration reprend les briques de SteamBox (conteneur Unraid).

> **État : prototype**, en service sur la machine. Toutes les commandes aibox : [docs/commandes.md](docs/commandes.md). Documentation détaillée (architecture, modifier et installer l'image, dépannage, problèmes connus) : [docs/](docs/README.md). Consignes pour les agents : [AGENTS.md](AGENTS.md). Spécification : [spec.md](spec.md).

## En bref

- **Sur la TV** : Steam Big Picture (jeux locaux) et Moonlight vers SteamBox.
- **IA** : LLM locaux servis par Lemonade, en même temps que les jeux. L'assistant écoute, répond à la voix et pilote la maison (détails à définir).
- **Démarrage de la session** : la DualSense s'allume, la session TV démarre. Elle s'éteint, la session s'arrête au bout de 2 minutes. Le PC reste allumé.
- **Administration à distance** : bureau Plasma dans le navigateur, par Selkies (`https://<ip>:6080`, mot de passe), et SSH par clé.
- **Système jetable** : chaque redémarrage repart d'une image propre. Pour personnaliser, on reconstruit l'image, sans jamais toucher aux données.
- **Image minimale** : l'image ne contient que ce qu'il faut pour démarrer, la session TV et l'administration. Les applis (Flatpak, AppImage), les modèles LLM et leur serveur tournent depuis le disque.

## Matériel cible

| Composant | Modèle |
|---|---|
| CPU | AMD Ryzen AI Max+ 395 (Strix Halo) |
| GPU | Radeon 8060S intégré (RDNA 3.5) |
| Mémoire | 128 Go unifiés, partagés CPU/GPU |
| Démarrage | Clé USB Ventoy |
| Données | Disque dédié, formaté en btrfs (étiquette `AIBOXDATA`) |

## Architecture

```
Clé USB (Ventoy + ISO) ──boot──► image système (EROFS lz4hc) copiée en RAM + overlay tmpfs (1 Go max) ─► clé retirable
                                   │  système jetable : chaque redémarrage repart de l'image
Disque (btrfs, étiquette AIBOXDATA) ──► /home (config, Steam, ~/Applications AppImage),
                                     /var/lib/flatpak, /srv/models (LLM), swap de secours,
                                     /var/tmp, cache pacman, journaux, coredumps,
                                     /var/lib/bluetooth, réseau, SSH, machine-id
DualSense allumée (BT) ─udev─► aibox-tv.service (tv.slice) ─► Plasma Bigscreen (KWin), ou gamescope (HDR, FSR) + Steam Big Picture
DualSense éteinte > 2 min ─────────┘ session arrêtée, écran éteint, PC allumé
LLM : llm.service (llm.slice) ─► serveur installé sur le disque, modèles dans /srv/models
Admin : Selkies (son compositeur) ⊃ KWin ⊃ Plasma (admin.slice) ─► Selkies (127.0.0.1:8082) ─► nginx HTTPS + mot de passe :6080  + SSH
```

### Ce qui vit où

| En RAM (image, jetable) | Sur le disque `AIBOXDATA` (persistant) |
|---|---|
| Noyau, firmwares allégés, mesa/vulkan | `/home` : configuration, bibliothèque Steam, Proton, AppImage (`~/Applications`) |
| Steam (bootstrap), gamescope, bureau d'admin | `/var/lib/flatpak` : toutes les applis Flatpak |
| flatpak, `fuse2`/`fuse3`, zram-generator | `/srv/models` : modèles LLM ; serveur Lemonade |
| Paquets installés à la volée (overlay, 1 Go max, perdus au redémarrage) | `/var/tmp`, cache pacman, journaux, coredumps, swapfile |
| | `/var/lib/bluetooth`, connexions NetworkManager, clés d'hôte SSH, `machine-id` |

Sous-volumes btrfs : `@home`, `@flatpak`, `@bluetooth`, `@nm`, `@ssh`, `@state`, `@models` (nodatacow), `@swap` (nocow).

Sans disque `AIBOXDATA`, le système démarre quand même, en mode éphémère, avec un message clair.

### Système en RAM

- **EROFS lz4hc** plutôt que squashfs (`airootfs_image_type="erofs"`) : lectures aléatoires rapides, peu de CPU. La compression est voulue : l'image en RAM y est bloquée, et sans compression les fichiers utilisés y seraient deux fois (image + cache de pages).
- **copytoram** : l'image est copiée en RAM au démarrage, la clé peut être retirée.
- **Overlay tmpfs** (limité à 1 Go) : les changements du système disparaissent au redémarrage.
- **Mémoire unifiée** : pas de découpage fixe.
  - **BIOS** : UMA (VRAM dédiée) au minimum, 512 Mo. Aujourd'hui, la machine en réserve 64 Go : c'est à corriger en premier.
  - **Noyau** : `ttm.pages_limit=28835840` (plafond GPU de 110 Gio, pris à la demande) et `ttm.page_pool_size=4194304` (cache de 16 Gio). `amdgpu.gttsize` est déprécié.
  - **IOMMU** laissé actif : le NPU en a besoin.
  - **Attention** : la mémoire GPU n'est pas comptée dans les cgroups. Les limites par slice ne valent que pour la mémoire CPU ; la taille des modèles se limite dans Lemonade.

### Budget mémoire (cibles, à mesurer)

| Poste | Cible | Mécanisme |
|---|---|---|
| Image EROFS lz4hc | ≤ 3,5 Go | firmwares et paquets allégés |
| Overlay | ≤ 1 Go | `cow_spacesize=1G` |
| `/tmp` / `/dev/shm` | ≤ 2 Go / ≤ 8 Go | options de montage |
| Journal en RAM | ≤ 128 Mo | `RuntimeMaxUse=128M` |
| **Total bloqué par le système** | **≤ 6 Go** | |
| Bureau d'admin (`admin.slice`) | MemoryMax 4 Go | systemd |
| Session TV (`tv.slice`) | MemoryHigh ~16 Go (CPU) | systemd |
| LLM (`llm.slice`) | MemoryMax ~16 Go (CPU) | systemd |
| GPU, tous usages | ≤ 110 Gio | `ttm.pages_limit` |
| Modèles chargés | ~80 Go max | configuration de Lemonade |

Garde-fous : zram (~16 Go), swapfile de secours sur le disque, systemd-oomd avec la session TV protégée en priorité.

### Applis sans faire exploser la RAM

- **Flatpak** : `/var/lib/flatpak` sur le disque (bind mount). Les applis tournent depuis le disque ; leur cache est rendu dès que le système en a besoin. `/var/tmp` et `TMPDIR` sur le disque.
- **AppImage** : lancées via FUSE depuis `~/Applications`. **Ne jamais utiliser `--appimage-extract-and-run`** ni `APPIMAGE_EXTRACT_AND_RUN=1` : ils extraient l'appli dans `/tmp`, donc en RAM.
- **LLM** : serveur et modèles sur le disque. Taille maximale de modèle fixée, déchargement automatique à l'inactivité.
- **`pacman -S` à chaud** : dépannage uniquement (overlay limité à 1 Go).

## Organisation du dépôt

```
archiso/                 profil archiso (écrit à partir de releng/baseline, adapté à CachyOS)
  ├─ profiledef.sh       EROFS lz4hc, options du noyau (copytoram, cow_spacesize, ttm)
  ├─ packages.x86_64     paquets de l'image
  ├─ pacman.conf         dépôts CachyOS + Arch
  ├─ grub/               menu de démarrage UEFI (et loopback.cfg pour Ventoy)
  └─ airootfs/           fichiers ajoutés à l'image : utilisateur, SSH, limites mémoire,
                         services, aibox mem
docs/                    documentation détaillée (commandes, architecture, image, exploitation, problèmes connus)
AGENTS.md                consignes pour les agents (LLM local…)
keys/                    clé SSH, mot de passe et certificat du bureau web (non versionné)
build.sh                 construit l'ISO dans un conteneur CachyOS → out/
test-vm.sh               teste l'ISO dans QEMU (clé USB et disque NVMe virtuels, GPU partagé)
tools/                   scripts d'aide aux tests (Chromium piloté)
```

Le code commun avec SteamBox est repris, pas réécrit (sous-module git figé sur un commit, ou fichiers vendorés s'ils divergent) :

- `plasma-session.sh` et le service Selkies : bureau d'administration (Selkies → KWin → Plasma, depuis le 27/11/2026, à la place de labwc, XFCE, wayvnc et noVNC), transposés en unités `systemd --user` au lieu de s6 ;
- l'installation de WhiteSur (thème, icônes, curseur, libadwaita) : `tools/whitesur.sh`, mêmes commits que SteamBox, pour les applis GTK ;
- la vérification des icônes `.desktop` et le `fc-cache -s` final (sinon glycin est tué par seccomp).

## Prototype

Construit et testé en VM (QEMU/KVM, 8 Go de RAM, clé USB et disque NVMe virtuels) :

- **Système en RAM** : l'ISO démarre, copie le système en RAM, la clé peut être retirée.
- **Bureau d'administration** Plasma en français (Dolphin, Konsole, Kate, paramètres, réseau), dans le navigateur par Selkies en HTTPS avec mot de passe : un seul pointeur, le son dans le navigateur, et l'écran s'adapte à la fenêtre.
- **Disque de données** `AIBOXDATA` : `/home`, Flatpak, réseau, Bluetooth, clés SSH, cache pacman, `/var/tmp`, swap et modèles persistent ; sans disque, mode éphémère.
- **Applications** : magasin **Bazaar** (Flathub) et **Gear Lever** (AppImage), installés automatiquement sur le disque ; les applis s'ajoutent au menu de Plasma.

- **Lemonade** : serveur LLM sur le port 13305, modèles et backends sur le disque.

- **Session TV** : par défaut, Plasma Bigscreen (accueil plein écran à la manette) ; sa tuile « Steam (TV) » lance gamescope, qui pilote la TV (HDR, FSR, VRR) avec Steam Big Picture en mode SteamOS, et sa tuile « Bureau » passe au bureau Plasma et en revient. « Passer au bureau » dans Steam ramène, pour cette session, à Bigscreen. `aibox tv-mode gamescope` rend gamescope durable.

Pas encore : assistant vocal.

**Contenu de l'image** : noyau `linux-cachyos`, firmwares de cette machine seulement (amdgpu, amd pour le NPU, mediatek pour le Wi-Fi et le Bluetooth MT7925), mesa/vulkan, NetworkManager, SSH, Bluetooth, flatpak, fuse2/fuse3, zram, cifs-utils, nss-mdns, KWin, Plasma et Plasma Bigscreen (portail KDE), Selkies (venv Python dans `/opt/selkies`), nginx, podman, distrobox, git, lemonade-server, steam, bibliothèques 32 bits pour wine-tkg (jeux arcade), wl-clipboard et xclip (--tray), wsquashfs-launcher (hors image).

**Mesures en VM** :

| Mesure | Résultat | Cible |
|---|---|---|
| Taille de l'ISO | 1,4 Go | |
| Image EROFS lz4hc en RAM | 1,2 Go | ≤ 3,5 Go |
| Mémoire bloquée par le système | 1,2 Go | ≤ 6 Go |
| Démarrage (noyau + système) | ~14 s (~16 s sans disque) | |
| Retrait de la clé à chaud | le système continue de fonctionner | |
| Bureau d'administration | ~130 Mo en fonctionnement (labwc et XFCE, avant le passage à Plasma : à remesurer) | `admin.slice` ≤ 4 Go |
| Bazaar + Gear Lever (Flatpak) | 3,8 Go sur le disque, **0 Mo de RAM bloquée en plus** | |
| Paquet installé à la volée (`pacman -S`) | disparu au redémarrage | |

### Construire l'ISO

Le build tourne dans un conteneur CachyOS privilégié : Docker s'il y en a un, sinon Podman en root (sur aibox lui-même, avec le disque de données : voir [Mises à jour](#mises-à-jour)). Dans `keys/` (non versionné) :

- `authorized_keys` : ta clé SSH publique ;
- `vnc-password` : mot de passe du bureau web, Selkies et code-server (généré au premier build s'il manque ; nom du temps de noVNC) ;
- `aibox-ca.crt` / `.key` : autorité de certification locale (générée au premier build, puis réutilisée) ; `aibox-ca.crt` est à approuver une fois sur ton poste ;
- `tls-ips` : adresses IP couvertes par le certificat HTTPS, en plus de `aibox` et `aibox.local` ;
- `nas-mounts` et `nas.cred` (facultatifs) : partages du NAS montés par défaut et leurs identifiants (voir `aibox nas`).

Puis :

```sh
./build.sh                 # → out/aibox-AAAA.MM.JJ.HHMM-<commit>-x86_64.iso (lien : out/aibox-latest.iso)
```

`./build.sh --test-key` ajoute une clé de test pour la VM : n'utilise pas cette ISO sur la vraie machine.

### Tester dans une VM

Il faut `/dev/kvm` sur l'hôte Docker. La VM a un disque NVMe virtuel de 64 Go (`work/vm/nvme.qcow2`, conservé entre deux lancements) qui simule le futur disque de données.

```sh
./build.sh --test-key
./test-vm.sh start          # démarre l'ISO depuis une clé USB virtuelle
./test-vm.sh ssh aibox mem  # état de la mémoire, de la clé et du disque
./test-vm.sh ssh 'echo /dev/nvme0n1 | sudo aibox-setup-disk /dev/nvme0n1'   # une fois
./test-vm.sh unplug         # retire la clé à chaud
./test-vm.sh reboot         # redémarre la VM
./test-vm.sh wipe-disk      # repart d'un disque NVMe virtuel vierge (VM arrêtée)
./test-vm.sh web            # ouvre le bureau Selkies dans Chromium et capture l'écran
GPU=1 ./test-vm.sh start    # VM avec le GPU de l'hôte partagé (Venus/virgl), écran en vnc://<ip>:5902
./test-vm.sh screen         # capture de l'écran de la TV (grim : plus sous Plasma Bigscreen)
# depuis le réseau : https://<ip-de-l-hôte>:6080
./test-vm.sh stop
```

### Approuver la CA aibox sur ton poste (une fois)

Le bureau Selkies et code-server sont en HTTPS, avec un certificat signé par une CA locale (`keys/aibox-ca.crt`). Il faut l'approuver une fois : accepter l'avertissement d'un clic ne suffit pas, le navigateur refuse alors les service workers, et le panneau Claude Code de code-server ne s'ouvre pas.

Sur Mac : télécharger `https://<ip>:6080/aibox-ca.crt` (ou prendre `keys/aibox-ca.crt`), puis :

```sh
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain aibox-ca.crt
```

(ou : double-clic → Trousseaux d'accès, Système → « aibox CA locale » → Se fier → Toujours approuver). Relancer le navigateur. Firefox a son propre magasin : Réglages → Vie privée et sécurité → Certificats → Importer.

La CA est limitée aux noms `aibox`/`aibox.local` et aux adresses privées. Le certificat du serveur est refait à chaque build ; ajouter dans `keys/tls-ips` toute adresse IP utilisée pour joindre aibox (`https://aibox.local:6080` marche aussi, grâce à avahi).

### Installer sur la clé

- **Ventoy** : copier l'ISO sur la clé. Garder l'ISO précédente à côté : on revient en arrière en la choisissant au démarrage.
- **Ou directement** : `sudo dd if=out/aibox-latest.iso of=/dev/sdX bs=4M status=progress oflag=sync` (efface la clé).

Démarrage UEFI uniquement. Dans le BIOS : Secure Boot désactivé, et UMA (VRAM dédiée) au minimum, 512 Mo. Au menu, `aibox (RAM)` démarre par défaut ; `sans copie en RAM` sert au dépannage (la clé doit alors rester branchée).

### Sur la machine

- **Installation sur le disque** (une fois) : démarrer sur la clé, puis `sudo aibox-setup-disk /dev/nvme0n1` (retaper le chemin pour confirmer). Il efface le disque et crée deux partitions : **AIBOXBOOT** (FAT32, 8 Go, de quoi garder deux versions : systemd-boot et l'image, pour démarrer sans la clé, en quelques secondes au lieu de ~2 minutes depuis une clé USB 2.0) et **AIBOXDATA** (btrfs : les données). Redémarrer sans la clé. Sans ce disque, le système démarre en mode éphémère et le signale (console, SSH, `aibox-mem`) : `/home` est alors en RAM (16 Go max), Steam et code-server y fonctionnent, tout est perdu au redémarrage.
- **Mises à jour** : voir [Mises à jour](#mises-à-jour).
- **Écran (TV)** : la session TV démarre quand la manette s'allume et s'arrête 2 minutes après son extinction ou quand on quitte Steam (voir [Session TV](#session-tv)) : Plasma Bigscreen (par défaut) ou Steam sous gamescope. Les applis de Bigscreen sont celles du menu (fichiers `.desktop`). Steam ne peut tourner qu'à un endroit à la fois (TV ou bureau d'administration).
- **SSH** : clé uniquement (`keys/authorized_keys`, intégré au build, plus `~/.ssh/authorized_keys` sur le disque, lu sans reconstruire) ; actuellement les clés de code-server et du MacBook Air. Sinon : terminal du bureau web (Selkies) ou code-server.
- **Shell** : fish pour `default` (SSH, terminaux du bureau et de code-server), avec le résumé de la machine (fastfetch) à l'ouverture. Réglages communs dans `/etc/fish/conf.d/aibox.fish` ; les siens dans `~/.config/fish/config.fish`. Les scripts d'aibox restent en bash.
- **Bureau web (Selkies)** : `https://<ip>:6080`, utilisateur `default`, mot de passe de `keys/vnc-password`. Le son des applis du bureau sort dans le navigateur (puits `selkies`).
- **Applications** : Bazaar pour Flathub (au premier lancement, il charge le catalogue pendant une à deux minutes), Gear Lever pour les AppImage. Steam est natif dans l'image : ne pas installer le Flatpak de Steam. Pas de mot de passe demandé (`default` est administrateur, comme pour `sudo`).
- **Sunshine** (diffusion du bureau d'administration vers Moonlight, encodage GPU) : pas lancé au démarrage. Pour le démarrer : `systemctl --user start app-dev.lizardbyte.app.Sunshine`, ou « Sunshine » dans le menu. Interface sur `https://<ip>:47990`, qui demande de créer un identifiant au premier lancement. La configuration est dans `~/.config/sunshine`, sur le disque. Il filme le bureau d'administration, pas la TV. Paquet des dépôts CachyOS, avec `cap_sys_admin` posé à l'installation. Les manettes virtuelles marchent sans session devant l'écran (`99-aibox-sunshine.rules`). Sous le KWin du bureau (depuis le 27/11/2026), le clavier et la souris de Moonlight ne passent plus : `aibox-evdev-bridge` ne parle qu'aux compositeurs wlroots.
- **Conteneurs** : `podman` (sans root ni démon ; la commande `docker` marche aussi), `podman-compose`. Conteneurs et images dans `~/.local/share/containers`, sur le disque.
- **Lemonade** (LLM) : `http://<ip>:13305` (API compatible OpenAI et interface web), démarré automatiquement avec le disque. Modèles dans `/srv/models/huggingface`, backends dans `/srv/models/lemonade`. Commandes : `lemonade list`, `lemonade pull <modèle>`, `lemonade config set …`. Ouvert sur le réseau local sans clé, comme aujourd'hui ; une clé d'API est possible (`LEMONADE_API_KEY`).
- **Développement** : `git`, `jq`, `rsync`, `tmux`, `zip`/`unzip`, `wget`, `imagemagick` (`identify`, `magick`, pour les images des agents d'Hermes) sont dans l'image. Pour le reste, un environnement Distrobox sur le disque :
  ```sh
  distrobox create --name dev --image docker.io/library/archlinux:latest
  distrobox enter dev     # puis : sudo pacman -S base-devel vim …
  ```
  Il partage ton home et ton bureau ; `distrobox-export` ajoute une appli du conteneur au menu de Plasma.
- **Éditeur (code-server)** : `https://<ip>:6080/code/`, même mot de passe que le bureau web. VS Code dans le navigateur, avec l'extension Claude Code ; installé dans le home au premier démarrage avec le disque. Mise à jour : `/usr/local/lib/aibox/code-server-install --update`, puis `systemctl --user restart aibox-code-server`. Interface en anglais (pas de pack de langue compatible sur Open VSX). Autre voie : VS Code Remote-SSH depuis ton poste (`ssh default@<ip>`).
- **Services au démarrage** (ex. Hermes) : unités dans `~/.config/systemd/user/`, puis `systemctl --user enable --now <service>`. Elles démarrent à chaque démarrage de la machine, sans connexion (sur le disque : pas en mode éphémère). Depuis la sauvegarde du NAS, adapter les chemins : `/home/jarvis` → `/home/default`, `/opt/Applications/…` → dans le home (`/opt` fait partie de l'image).
- **Applis graphiques au démarrage** : Paramètres système → Démarrage automatique (`~/.config/autostart`), lancées par Plasma. `~/.config` est partagé : une appli s'y lance dans les deux sessions Plasma (bureau d'administration et TV) ; `OnlyShowIn`/`NotShowIn` ne les distinguent pas (toutes deux `KDE`). Exemple : `wsquashfs-launcher --tray on`.
- **Jeux** : Steam natif et Proton ; `ntsync` est chargé au démarrage ; `gamemoderun %command%` dans les options de lancement d'un jeu active gamemode.
- **Programmes Windows (.exe, .msi)** : double-clic dans Dolphin (ou `aibox run-exe prog.exe`) : lancés avec un Proton déjà installé, partagé avec Steam et wsquashfs-launcher (`~/.local/share/Steam/compatibilitytools.d`), dans un préfixe Wine partagé (`~/.local/share/aibox/wine/default`) : rien n'est réinstallé d'un programme à l'autre. Proton : le `proton-cachyos` le plus récent s'il y en a un, sinon UMU-Proton (téléchargé une fois) ; pour en imposer un : `PROTON=GE-Proton10-…` dans `~/.config/aibox/proton.conf`. Journal : `~/.cache/aibox/run-exe/`.
- **Partages du NAS** : montés automatiquement, à la demande (au premier accès), par `aibox nas` : `/mnt/Media`, `/mnt/Games`, `/mnt/Workspace` (`//nas.local/…`, définis au build par `keys/nas-mounts` et `keys/nas.cred` ; ils marchent dès le premier démarrage, même en mode éphémère). `aibox nas list` (état), `sudo aibox nas add //hôte/partage /mnt/X` (identifiants demandés une fois, gardés sur le disque), `sudo aibox nas remove /mnt/X`. Les noms `.local` sont résolus par mDNS (`nss-mdns`). Pour parcourir d'autres partages : Dolphin, `smb://<nas>/<partage>`.
- **Dongle DualSense (DS5Dongle)** : deux usages. Le son de la manette émulée dans SteamBox (ds5-usbip) arrive en RTP sur le port 46100 et joue sur la manette, par le dongle en mode « Off » (haut-parleur et haptiques natives ; récepteur `ds5-usbip.service`, actif seulement manette allumée). Ou la vibration tirée du son du PC, par le service utilisateur `ds5-haptics-loopback` (configuration de [DS5Dongle-Studio](https://github.com/artzox/DS5Dongle-Studio), licence MIT : règle udev, WirePlumber ; dongle en « Replace »). `aibox ds5-mode` affiche le mode ; `aibox ds5-mode son` active la vibration tirée du son, `aibox ds5-mode natif` (défaut) la coupe. Le volume du dongle est à 100 %. Le flux RTP n'est relié qu'au dongle : sans dongle, il ne joue nulle part ; manette éteinte, le port 46100 est fermé.
- **Decky Loader** (plugins de Steam en mode Big Picture) : installeur officiel, une fois, depuis le bureau d'administration : `curl -L https://github.com/SteamDeckHomebrew/decky-installer/releases/latest/download/install_release.sh | sh`. Decky s'installe dans `~/homebrew` (sur le disque) ; le service `plugin_loader` est dans l'image (l'unité que l'installeur écrit dans `/etc` disparaît au redémarrage, celle de l'image prend le relais) et reste inerte tant que Decky n'est pas installé. Decky se met à jour lui-même.
- **Jeux Windows (.wsquashfs)** : `wsquashfs-launcher` (binaire installé à part, hors de l'image, dans `~/.local/bin`) ; ses runners (proton-cachyos, UMU-Proton, GE-Proton) s'installent sur le disque, dans `~/.local/share/Steam/compatibilitytools.d`.
- **Machines virtuelles** : GNOME Boxes ou virt-manager, à installer depuis Bazaar (Flatpak, avec leur propre QEMU).
- **`aibox diag`** : rapport de diagnostic, écrit aussi tout seul 2 minutes après le démarrage, dans `aibox-diag/` sur la clé Ventoy si elle est branchée, sinon sur la partition AIBOXBOOT.
- **`aibox mem`** : mémoire bloquée, GPU, disque de données, image en RAM, état de la clé, tmpfs, zram.
- **`aibox ds5-mode`** : mode de la manette DualSense, `natif` (haptiques natives de ds5-usbip, défaut) ou `son` (vibration tirée du son du PC, loopback actif). Régler le dongle en conséquence (« Off » ou « Replace », portail DS5Dongle-Studio).

### Mises à jour

L'image est figée : ce qui est dedans se met à jour en refaisant l'image ; tout le reste vit sur le disque et se met à jour sur place.

| Quoi | Comment |
|---|---|
| Système : noyau, pilotes, mesa, Steam (amorce), Lemonade, bureaux KDE, Selkies, scripts aibox | commit poussé sur GitHub, nouvelle ISO (`sudo aibox build`), puis `sudo aibox boot --iso /var/lib/aibox/state/build/out/aibox-latest.iso` et redémarrer. La version précédente reste dans le menu de démarrage pour revenir en arrière (chaque build a sa version : date, heure et commit) ; les données ne sont pas touchées. Procédure complète : [docs/iso.md](docs/iso.md) |
| Applis Flatpak (Bazaar…) | Bazaar (onglet des mises à jour) ou `flatpak update` |
| AppImage (Moonlight…) | Gear Lever, ou à la main (fichier remplacé dans `~/AppImages`) |
| Steam, jeux, Proton | Steam se met à jour seul (dans le home) ; Proton par Steam, ProtonUp ou wsquashfs-launcher |
| AppImage | Gear Lever |
| Lemonade : backends et modèles | backends : par Lemonade ; modèles : `lemonade pull <modèle>` |
| code-server | `/usr/local/lib/aibox/code-server-install --update`, puis `systemctl --user restart aibox-code-server` |
| Decky Loader | son propre outil de mise à jour (menu de Decky) |
| Conteneurs, Distrobox | chaque nuit par `aibox update` (`podman auto-update`, Quadlets avec `AutoUpdate=registry`) ; à la main : `podman auto-update` ; dans un distrobox, son gestionnaire de paquets |

Un `pacman -Syu` sur la machine ne sert à rien : il ne modifie que la RAM et disparaît au redémarrage.

**Mise à jour automatique** : chaque nuit vers 4 h, le minuteur lance d'abord `aibox update --podman` (conteneurs Podman : `podman auto-update`, retour à l'image précédente en cas d'échec), puis `aibox update` pour le système : il construit les nouveaux commits poussés sur GitHub ; sinon, il compare les paquets de l'image installée aux dépôts et demande au LLM local (passerelle LiteLLM, modèle « auto ») si la mise à jour vaut la peine (sécurité, noyau, pilotes graphiques, Steam, Lemonade…). Si oui, il reconstruit l'image (`aibox build`), la vérifie et l'installe sur AIBOXBOOT. Message sur Discord (Hermes) ; jamais de redémarrage automatique. Détails : [docs/exploitation.md](docs/exploitation.md#mise-à-jour-automatique-de-limage).

**Construire l'ISO sur aibox lui-même** : c'est le service `aibox build`. Il construit ce qui est poussé sur GitHub (`origin/main`), dans sa propre copie du dépôt, avec ses secrets et ses caches sur le disque de données (`/var/lib/aibox/state/build/`, sous-volume `@state` ; le stockage de Podman est dans `/var/lib/aibox/state/containers`). Il faut environ 12 Go libres. Le dossier de travail `~/Workspace/AIBox` ne sert qu'à développer :

```sh
sudo aibox build --import ~/Workspace/AIBox            # une fois : keys/ et caches de work/
sudo aibox build                                        # build de origin/main (Podman en root)
sudo aibox boot --iso /var/lib/aibox/state/build/out/aibox-latest.iso   # puis arrêt complet et rallumage
```

Les secrets sont sauvegardés sur le NAS (`Workspace/AIBox/keys/`) : pour une machine neuve, `rsync -a /mnt/Workspace/AIBox/ ~/Workspace/AIBox/`, puis `--import`.

Si la nouvelle version pose problème : choisir la précédente au menu de démarrage, sinon la clé Ventoy ([docs/iso.md](docs/iso.md), « Revenir en arrière »).

## Utilisation complète (prévue)

### 3. Préparer le disque de données (une seule fois)

```sh
sudo aibox-setup-disk /dev/nvmeXnY
```

> ⚠️ Formate le disque indiqué : toutes ses données sont effacées.

### 4. Configuration initiale (une seule fois)

- **Appairer la DualSense** depuis le bureau d'administration ou avec `bluetoothctl`.
- **Moonlight** : AppImage à `~/AppImages/moonlight.appimage`, sur le disque de données. Il apparaît dans Bigscreen par son fichier `.desktop` (`~/.local/share/applications/moonlight.desktop`) ; sous gamescope, l'ajouter dans Steam comme appli non-Steam pointant vers le même fichier. SteamBox est découvert automatiquement sur le réseau local.

Ces réglages sont gardés sur le disque.

### Accès

| Service | Adresse | Notes |
|---|---|---|
| Bureau web (Selkies) | `https://<ip>:6080` | Utilisateur `default`, mot de passe de `keys/vnc-password` ; CA aibox à approuver une fois |
| code-server | `https://<ip>:6080/code/` | Mêmes identifiants |
| SSH | `ssh default@<ip>` | Clé uniquement, déposée au build |

Utilisateur : `default` (uid 1000), sudo, pas de mot de passe root.

## Session TV

`aibox-tv.service` (service système, `User=default`, `PAMName=login`, `TTYPath=/dev/tty1`) ouvre une vraie session logind, puis `tv-session` lance le mode choisi par `aibox tv-mode` (gardé dans `~/.config/aibox/tv-mode`) :

- **bigscreen** (par défaut, sans choix enregistré) : Plasma Bigscreen (`plasma-bigscreen-wayland`) sur la TV, sous KWin, sans HDR, sur un bus de session à lui et sans les unités systemd de Plasma (`/etc/xdg/startkderc`) : le bureau d'administration a aussi son Plasma. Accueil à la manette ; ses applis sont celles du menu. Tuiles d'aibox : « Steam (TV) » (`aibox tv-mode --once gamescope`) et « Bureau » (`tv-bureau` : bureau Plasma sur la TV et retour, par `plasma-bigscreen-swap-session`, sans relancer la session).
- **gamescope** : gamescope pilote la TV (4K, HDR, VRR) et lance Steam en mode SteamOS (`-gamepadui -steamos3 -steampal -steamdeck`, réglages de gamescope-session). HDR, limite d'images et FSR se règlent dans Steam : Paramètres → Affichage (HDR), menu rapide « … » → Performances (filtre de mise à l'échelle FSR, avec une résolution de jeu plus basse). Moonlight (s'il est installé, voir plus haut) s'y ajoute comme jeu non-Steam. « Passer au bureau » ramène à Bigscreen (`steamos-session-select`).

`aibox tv-mode gamescope` ou `aibox tv-mode bigscreen` change de mode (gardé aux redémarrages) et relance la session TV ; avec `--once`, pour cette session seulement. Les anciens modes `hyprland` et `bureau` (retirés le 27/11/2026) valent `bigscreen`. Pas de HDMI-CEC : le GPU ne pilote pas la ligne CEC (voir plus bas).

- **Jeux non-Steam** (`.wsquashfs`…) : pas de `gamescope` dans les options de lancement, la session est déjà dans gamescope ; si un jeu ne propose pas le HDR, `DXVK_HDR=1 %command% …`. Un chemin contenant des espaces se met entre guillemets.

- **Démarrage** : quand la manette s'allume, y compris si elle l'est déjà au démarrage de la machine. Le service `aibox-tv-watch` surveille toutes les 2 secondes le flux de rapports du dongle DualSense (`054c:0ce6` ou `054c:0df2`). Ni la présence du dongle ni ses événements USB ne suffisent : il s'annonce aussi manette éteinte, et ne se reconnecte pas toujours quand on l'allume. Pas de session TV sinon : la machine reste un serveur, joignable par le bureau web.
- **Arrêt** : 2 minutes après l'extinction de la manette (`aibox-tv-idle.timer`, réglable avec `systemctl edit aibox-tv-idle.timer`), sauf si elle revient entre-temps. La session s'arrête aussi quand on quitte Steam (Quitter, Éteindre) : éteindre puis rallumer la manette la relance. « Passer au bureau » la relance sous Bigscreen. Si gamescope ou Steam plante, elle est relancée. Le Plasma de la TV tourne dans le scope logind de la session : `aibox-tv.service` arrête ce scope à l'arrêt.
- **Steam déjà ouvert ailleurs** (bureau d'administration) : la session TV ne démarre pas et lui laisse la place, puisqu'un seul Steam peut tourner à la fois.
- **TV** : à l'arrêt de la session, l'écran est mis en veille (fbcon) : la TV perd le signal.

## Paquets principaux

Tous disponibles dans les dépôts CachyOS (vérifié le 26/09/2026) :

- `linux-cachyos`, `amd-ucode`, firmwares allégés (amdgpu, Wi-Fi/BT présents, audio)
- `mesa`, `vulkan-radeon` et `lib32-*`
- `gamescope`, `steam`
- `networkmanager`, `bluez`, `openssh`, `flatpak`, `fuse2`, `fuse3`, `zram-generator`
- `archiso` pour le build

Écartés de l'image : `cachyos-handheld` (tire le matériel du Steam Deck) et `moonlight-qt` (tire Qt6 ; remplacé par l'AppImage).

## Feuille de route

1. **Système live minimal en RAM, avec persistance** : EROFS, copytoram, disque `AIBOXDATA`, mode éphémère.
2. **Session TV déclenchée par la DualSense** : `aibox-tv.service`, règle udev, minuteur d'arrêt, Moonlight.
3. **Administration à distance** : Selkies, KWin et Plasma (avant le 27/11/2026 : labwc headless, XFCE, wayvnc, noVNC), SSH.
4. **Applications et mises à jour** : Flatpak et AppImage depuis le disque, mise à jour par nouvelle ISO.
5. **LLM locaux** : `@models`, serveur sur le disque, `llm.slice`.
6. *(Optionnel)* **Bureau TV** accessible depuis Steam par « Passer au bureau ».

## Points encore ouverts

- **Budget mémoire** : les cibles ci-dessus sont à mesurer sur la machine.
- **Cohabitation jeu/LLM** : décharger le modèle au démarrage de la session TV, ou non ? Taille maximale de modèle ?
- **Réseau et Bluetooth** : Ethernet ou Wi-Fi, quel adaptateur, stabilité de la reconnexion de la DualSense.
- **Allumage de la TV (HDMI-CEC)** : `amdgpu` ne pilote pas la ligne CEC du HDMI (pas de `/dev/cec*`, vérifié le 27/11/2026). Plasma Bigscreen sait s'en servir (libcec) : il faudrait un adaptateur Pulse-Eight USB-CEC et une règle udev pour `/dev/ttyACM*`.

## Tests

**Dans QEMU** (UEFI, ISO et disque virtuel, avec `testiso.sh` du profil CachyOS) :

- la clé se retire après le démarrage ;
- empreinte mémoire et temps de démarrage, EROFS lz4 contre non compressé ;
- `aibox-setup-disk` fonctionne sur le disque virtuel ;
- un fichier dans `/home`, une appli Flatpak et une connexion réseau survivent au redémarrage ;
- un paquet installé à la volée disparaît au redémarrage ;
- avec 16 Go de RAM virtuelle : mémoire bloquée par le système ≤ 6 Go ; une appli Flatpak ou AppImage ne fait monter que le cache récupérable ; 5 Go écrits dans `/tmp` échouent proprement ; `flatpak install` ne fait pas grossir l'overlay ; un stress mémoire dans `llm.slice` ne touche pas la session TV ;
- une DualSense simulée via uhid démarre la session ; la débrancher l'arrête après le délai, la rebrancher pendant le délai annule l'arrêt ;
- le bureau web demande le mot de passe puis affiche le bureau d'administration, SSH par clé fonctionne.

**Sur la machine Strix Halo** :

- démarrage en RAM, disque `AIBOXDATA` monté ;
- DualSense allumée : Steam Big Picture à l'écran ;
- Moonlight vers SteamBox : un jeu en streaming, puis un jeu en local ;
- DualSense éteinte : session fermée après 2 minutes ; rallumée : la session revient ;
- un modèle d'environ 30 Go chargé, puis un jeu lancé : pas d'OOM (GTT vérifié avec `amdgpu_top`).
