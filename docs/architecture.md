# Architecture

## Vue d'ensemble

```
AIBOXBOOT (FAT32, NVMe) ── systemd-boot ──► noyau + initramfs archiso
        └─ aibox-<version>/x86_64/airootfs.erofs ──copytoram──► système en RAM (EROFS lz4hc)
                                                                + overlay tmpfs (1 Go max, perdu au redémarrage)
AIBOXDATA (btrfs, NVMe) ──► sous-volumes montés au démarrage (données persistantes)
```

- **Machine** : AZW GTR Pro, AMD Ryzen AI Max+ 395 (Strix Halo), Radeon 8060S, 128 Go de mémoire unifiée.
- **Disque NVMe** (`/dev/nvme0n1`) :
  - partition 1 `AIBOXBOOT` (FAT32, 8 Go) : systemd-boot et une ou deux versions de l'image ;
  - partition 2 `AIBOXDATA` (btrfs) : les données.
- Une **clé Ventoy** (`sda`) peut rester branchée. Elle sert de secours : la machine démarre sur une ISO qu'elle contient.

## Ce qui vit où

| Emplacement | Contenu | Persistant ? |
|---|---|---|
| Image (EROFS, en RAM) | `/usr`, `/etc` d'origine, noyau, paquets, scripts aibox | non : refaite à chaque build |
| Overlay | toute écriture dans `/etc`, `/usr`, `/var`… | non : 1 Go max, perdu au redémarrage |
| `/home` (`@home`) | configuration utilisateur, Steam, jeux (`~/Games`), Proton, Podman, code-server, dépôts (`~/Workspace`) | oui |
| `/var/lib/flatpak` (`@flatpak`) | applis Flatpak | oui |
| `/srv/models` (`@models`) | modèles LLM et backends de Lemonade | oui |
| `/var/lib/aibox/state` (`@state`) | stockage Podman du build, `/var/tmp`, cache pacman, coredumps, partages NAS ajoutés | oui |
| `@bluetooth`, `@nm`, `@ssh`, `@swap` | appairages, connexions réseau, clés d'hôte SSH, swap de secours | oui |

Les montages sont dans `/etc/aibox/fstab.data`, faits par `aibox-data-mount.service`. La liste des sous-volumes est dans `/usr/local/lib/aibox/subvolumes`. Un sous-volume ajouté à cette liste est créé tout seul au démarrage suivant.

Sans disque `AIBOXDATA`, le système démarre en **mode éphémère** : `/home` est alors en RAM (16 Go max), et Lemonade ne démarre pas.

## Démarrage

1. systemd-boot lance l'entrée par défaut (`/loader/entries/aibox-<version>.conf`). Les options importantes sont `archisobasedir=aibox-<version>`, `archisodevice=UUID=<AIBOXBOOT>` et `copytoram=y`.
2. L'initramfs archiso copie `airootfs.erofs` en RAM, puis bascule dessus.
3. systemd monte le disque de données, puis démarre les services.

## Services principaux

| Unité | Rôle |
|---|---|
| `aibox-data-mount.service` | monte `AIBOXDATA` (ou passe en mode éphémère) |
| `aibox-tv.service` (système, `tv.slice`) | session TV sur l'écran réel, `tty1` ; démarrée par la manette (`aibox-tv-watch`), arrêtée 2 min après son extinction (`aibox-tv-idle.timer`) ou quand on quitte Steam |
| `aibox-admin.target` (utilisateur, `admin.slice`) | bureau d'administration : Selkies (`aibox-admin-selkies`, son propre compositeur Wayland), KWin et Plasma imbriqués dedans (`aibox-admin-plasma`, `admin-session`), nginx (`aibox-admin-web`) |
| `nginx` (`/etc/aibox/nginx.conf`) | HTTPS sur le port 6080 : Selkies (`/`, relayé vers `127.0.0.1:8082`) et code-server (`/code/`), avec mot de passe |
| `aibox-code-server.service` (utilisateur) | VS Code dans le navigateur |
| `lemond.service` (utilisateur, `llm.slice`) | Lemonade, serveur de LLM sur le port 13305 |
| `aibox-nas.service` | partages SMB du NAS, montés à la demande |
| `ds5-usbip.service` (utilisateur) | manette allumée seulement (`tv-watch`) : récepteur `ds5_usbip_haptics` du son de la DualSense émulée dans SteamBox (RTP, port 46100), relié au dongle et à lui seul |
| `ds5-haptics-loopback.service` (utilisateur, mode son seulement) | envoie le son du PC au dongle DualSense |
| `aibox-diag.timer` | rapport de diagnostic 2 minutes après le démarrage |

L'utilisateur est `default` (uid 1000). Il a `sudo` sans mot de passe et n'a pas de mot de passe Unix. Ses services utilisateur démarrent sans connexion (« linger »).

## Session TV

`aibox-tv.service` lance `/usr/local/lib/aibox/tv-session`, qui choisit le mode d'après `$XDG_RUNTIME_DIR/aibox-tv-mode-once` s'il existe (mode pour cette session seulement, effacé aussitôt lu, perdu au redémarrage), sinon d'après `~/.config/aibox/tv-mode` (mode durable ; bigscreen s'il n'existe pas). `aibox tv-mode` écrit le second, `aibox tv-mode --once` le premier. Les anciens modes `hyprland` et `bureau` (retirés le 27/11/2026) valent `bigscreen`.

**Cycle de vie** :
- **Démarrage et arrêt** : `aibox-tv-watch.service` (`/usr/local/lib/aibox/tv-watch`) mesure toutes les 2 secondes, sur une demi-seconde, les rapports envoyés par le dongle DualSense sur son hidraw. Au moins deux rapports différents signifient une manette connectée (compteur, gyroscope, environ 640 rapports par seconde). Le service réagit aux changements d'état : manette allumée, il démarre la session et annule l'arrêt programmé ; manette éteinte, il programme l'arrêt. La présence du dongle et ses événements udev ne suffisent pas : il s'annonce aussi manette éteinte, à la mise sous tension du PC, et ne se reconnecte pas toujours à l'allumage de la manette.
- **Arrêt** : `aibox-tv-idle.timer` arrête la session 2 minutes après l'extinction de la manette. Il est annulé si elle revient.
- **Codes de sortie de `tv-session`** :
  - 75 : Steam a été fermé par l'utilisateur, ou Steam tourne déjà ailleurs. Fin de la session, sans relance (`RestartPreventExitStatus=75`) ;
  - 0 : « Passer au bureau », relance dans le nouveau mode ;
  - 1 : plantage, relance.
- **Scope logind** : avec `PAMName=login`, les processus de la session vont dans un scope logind (`session-N.scope`), pas dans le cgroup du service, et Plasma ne s'arrête pas avec `dbus-run-session`. `tv-session` note le numéro de session (`$XDG_RUNTIME_DIR/aibox-tv-session-id`), et un `ExecStopPost` arrête ce scope.
- **Veille** : à l'arrêt, `fb0` est mis en veille, pour que la TV perde le signal.

Les modes :
- **bigscreen** (par défaut) : `dbus-run-session plasma-bigscreen-wayland`. KWin pilote la TV (DRM, siège logind), sans HDR ; plasmashell avec le shell Bigscreen, navigation à la manette (SDL). Bus de session propre : le Plasma du bureau d'administration tient le bus principal. Sans les unités `plasma-*.service` de systemd (`/etc/xdg/startkderc`, `systemdBoot=false`), qui n'existent qu'une fois par utilisateur : `startplasma` lance ses composants lui-même (ksmserver). Tuiles d'aibox (`/usr/local/share/applications/`) : « Steam (TV) » (`aibox tv-mode --once gamescope`) et « Bureau » (`tv-bureau`, qui appelle `plasma-bigscreen-swap-session` : bureau Plasma sur la TV et retour, sans relancer la session ; sans effet hors de la TV). L'autostart `aibox-session-env` publie l'affichage de la session (`WAYLAND_DISPLAY`, `DISPLAY`, `XAUTHORITY`) dans `$XDG_RUNTIME_DIR/aibox-tv.env`, pour `aibox tv-run` ; il est retiré à l'arrêt.
- **gamescope** : `gamescope --steam -e --hdr-enabled -- steam -gamepadui -steamos3 -steampal -steamdeck`. gamescope pilote directement la TV, en 4K à 60 Hz, avec HDR et VRR. Le HDR, le FSR et la limite d'images se règlent dans Steam.

« Passer au bureau » dans Steam appelle `steamos-session-select`, qui bascule en mode bigscreen **pour cette session seulement**. Au démarrage suivant, c'est le mode durable, bigscreen par défaut. Avant le 17/10/2026, la bascule était durable.

Pas de HDMI-CEC : `amdgpu` ne pilote pas la ligne CEC de la sortie HDMI (pas de `/dev/cec*`, vérifié le 27/11/2026). Bigscreen s'en servirait (libcec) avec un adaptateur USB-CEC.

## Bureau d'administration

Selkies (`/opt/selkies`, venv Python installé par `build.sh`, version épinglée) fait tourner son propre compositeur Wayland (pixelflux, rendu et encodage VA-API sur le GPU) et sert le bureau en local (`127.0.0.1:8082`) ; nginx le publie en HTTPS sur 6080, avec le mot de passe. Dans ce compositeur, `admin-session` lance `startplasma-wayland` : KWin s'y imbrique, avec Xwayland, et Plasma tourne sur le bus de session principal, sans les unités systemd de Plasma. KWin donne la forme du pointeur à Selkies, que le navigateur affiche (un seul pointeur), et le bureau prend la taille de la fenêtre du navigateur.

**Nom et logo** : « AIBox » dans l'en-tête et l'onglet (`SELKIES_UI_TITLE`) ; `build.sh` réécrit dans le venv le titre de la page, le nom du manifeste et les icônes (`icons/aibox-icon-512.png`, logo blanc sur fond sombre), comme SteamBox. Le logo de l'en-tête, intégré au JavaScript de Selkies, est gardé.

**Sockets Wayland** : aucun n'est fixe. Selkies et le wrapper de KWin (`kwin_wayland_wrapper`) prennent chacun le premier `wayland-N` libre à partir de 1 (jamais 0), selon l'ordre de démarrage. `admin-session` retrouve celui de Selkies par ses sockets en écoute (`ss -xlp`). L'autostart `aibox-session-env` relie `$XDG_RUNTIME_DIR/wayland-admin` au socket du KWin du bureau : Selkies y envoie le clavier, la souris et le presse-papiers (`SELKIES_APP_WAYLAND_DISPLAY=wayland-admin`, recherché toutes les 2 s), et c'est l'affichage par défaut des services de l'utilisateur (`/etc/systemd/user.conf.d/10-aibox-admin-display.conf`), jusqu'à ce que `startplasma` pousse le sien. `admin-session` efface le lien au démarrage du bureau : sinon il pourrait viser un socket repris par la TV.

**Son** : les applis du bureau sortent dans le puits `selkies` (`PULSE_SINK`, `pipewire-pulse.conf.d/50-aibox-selkies.conf`), dont Selkies capture le moniteur. Ce puits n'est jamais la sortie par défaut : la TV garde la sienne. Le micro et les manettes de Selkies sont coupés (les manettes seraient des périphériques du noyau, vus par Steam sur la TV).

Les applications au démarrage (`~/.config/autostart`, `/etc/xdg/autostart`) sont lancées par Plasma, dans chacune des deux sessions (`~/.config` est partagé). Les notifications passent par le plasmashell du bureau, sur le bus principal.

## Son

- **TV** : carte HDMI `alsa_card.pci-0000_c4_00.1`, sur la sortie HDMI 2. La règle `/etc/wireplumber/wireplumber.conf.d/52-hdmi-surround.conf` désactive UCM pour cette carte, ce qui donne accès au profil 5.1 (`output:hdmi-surround-extra1`). La TV n'accepte que 6 canaux en PCM.
- **Dongle DualSense** (DS5Dongle-Studio) : sortie `ds5_dongle_sink`, en 4 canaux. Flux RTP entrant (ds5-usbip) : SteamBox envoie en L16 48 kHz, 4 canaux, sur le port 46100 d'AIBox ; le récepteur `ds5_usbip_haptics` (`ds5-usbip.service`, chargé par `pw-cli` à l'allumage de la manette, déchargé à son extinction ; jamais relié par WirePlumber) est relié par `ds5-usbip-link` à FL, FR, RL, RR de `ds5_dongle_sink`. Manette éteinte, le port 46100 est fermé. Canaux 1-2 : haut-parleur de la manette ; canaux 3-4 : haptiques (actionneurs). Pour les haptiques natives, le dongle est en mode « Off » (en « Replace », il ignore les canaux 3-4). Le loopback du son du PC (canaux 1-2) n'existe qu'en mode son (`aibox ds5-mode son`, dongle en « Replace ») : il ne peut pas tourner avec les haptiques natives.

## Construction

`build.sh` lance `mkarchiso` dans un conteneur CachyOS privilégié. Sur aibox, c'est Podman en root, avec son stockage sur le disque de données. Le service `aibox build` lance `build.sh` depuis sa copie du dépôt (`origin/main`, dans `/var/lib/aibox/state/build/src`), avec les secrets (`keys/`), les caches (`work/`) et les ISO (`out/`) dans `/var/lib/aibox/state/build/` : variables `AIBOX_KEYS`, `AIBOX_WORK` et `AIBOX_OUT` de `build.sh`. Le profil est dans `archiso/` :

- `packages.x86_64` : les paquets ;
- `profiledef.sh` : le format de l'image et les **droits des fichiers** ;
- `airootfs/` : les fichiers copiés dans l'image ;
- `grub/` : le menu de l'ISO.

Le build ajoute aussi Selkies (venv pip dans `/opt/selkies`, version épinglée dans `build.sh`), evdev-bridge (compilé) et les secrets de `keys/`.

Détails pratiques : [iso.md](iso.md).
