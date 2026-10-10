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
| `aibox-tv.service` (système, `tv.slice`) | session TV sur l'écran réel, `tty1` ; démarrée par la manette (`71-aibox-tv.rules`), arrêtée 2 min après son extinction (`aibox-tv-idle.timer`) ou quand on quitte Steam |
| `aibox-admin.target` (utilisateur, `admin.slice`) | bureau d'administration : labwc headless (`wayland-0`, `:0`), XFCE, wayvnc, noVNC |
| `xrdp.service` | RDP vers le même bureau d'administration |
| `nginx` (`/etc/aibox/nginx.conf`) | HTTPS sur le port 6080 : noVNC et code-server (`/code/`), avec mot de passe |
| `aibox-code-server.service` (utilisateur) | VS Code dans le navigateur |
| `lemond.service` (utilisateur, `llm.slice`) | Lemonade, serveur de LLM sur le port 13305 |
| `aibox-nas.service` | partages SMB du NAS, montés à la demande |
| `ds5-haptics-loopback.service` (utilisateur) | envoie le son du PC au dongle DualSense |
| `aibox-diag.timer` | rapport de diagnostic 2 minutes après le démarrage |

L'utilisateur est `default` (uid 1000). Il a `sudo` sans mot de passe et n'a pas de mot de passe Unix. Ses services utilisateur démarrent sans connexion (« linger »).

## Session TV

`aibox-tv.service` lance `/usr/local/lib/aibox/tv-session`, qui choisit le mode d'après `~/.config/aibox/tv-mode`. Ce fichier est modifié par `aibox-tv-mode`.

**Cycle de vie** :
- **Démarrage** : la manette s'allume, le dongle DualSense apparaît ou se reconnecte sur l'USB, et `71-aibox-tv.rules` démarre le service.
- **Démarrage de la machine** : `aibox-tv-boot.service` (`/usr/local/lib/aibox/tv-boot-check`) démarre la session si la manette est déjà allumée. L'événement udev n'est pas émis pour un périphérique déjà présent. Et le dongle s'annonce même manette éteinte, jusqu'à la première connexion de la manette : le script lit une seconde de rapports sur son hidraw, et seuls des rapports qui changent (compteur, gyroscope) signifient une manette connectée.
- **Arrêt** : la manette s'éteint, le dongle disparaît, et `aibox-tv-idle.timer` arrête la session 2 minutes plus tard. Il est annulé si la manette revient.
- **Codes de sortie de `tv-session`** :
  - 75 : Steam a été fermé par l'utilisateur, ou Steam tourne déjà ailleurs. Fin de la session, sans relance (`RestartPreventExitStatus=75`) ;
  - 0 : « Passer au bureau », relance dans le nouveau mode ;
  - 1 : plantage, relance.
- **Veille** : à l'arrêt, `fb0` est mis en veille, pour que la TV perde le signal.

- **gamescope** (par défaut) : `gamescope --steam -e --hdr-enabled -- steam -gamepadui -steamos3 -steampal -steamdeck`. gamescope pilote directement la TV, en 4K à 60 Hz, avec HDR et VRR. Le HDR, le FSR et la limite d'images se règlent dans Steam.
- **labwc** : labwc avec la configuration `/etc/aibox/labwc-tv`, et le lanceur à tuiles `aibox-tv-launcher`. Ses applis sont dans `/etc/aibox/tv-apps.json`, ou dans `~/.config/aibox/tv-apps.json` s'il existe. Le fond d'écran vient de Bing et change toutes les 30 minutes. Ce mode n'a pas de HDR.

« Passer au bureau » dans Steam appelle `steamos-session-select`, qui bascule en mode labwc.

Le bureau d'administration doit avoir pris `wayland-0` et `:0` avant la session TV. Les services utilisateur ont cet affichage par défaut (`/etc/systemd/user.conf.d/10-aibox-admin-display.conf`).

## Son

- **TV** : carte HDMI `alsa_card.pci-0000_c4_00.1`, sur la sortie HDMI 2. La règle `/etc/wireplumber/wireplumber.conf.d/52-hdmi-surround.conf` désactive UCM pour cette carte, ce qui donne accès au profil 5.1 (`output:hdmi-surround-extra1`). La TV n'accepte que 6 canaux en PCM.
- **Dongle DualSense** (DS5Dongle-Studio) : sortie `ds5_dongle_sink`, en 4 canaux. Les canaux 0/1 vont au haut-parleur de la manette et servent aux auto-haptiques du firmware. Les canaux 2/3 vont aux actionneurs haptiques.

## Construction

`build.sh` lance `mkarchiso` dans un conteneur CachyOS privilégié. Sur aibox, c'est Podman en root, avec son stockage sur le disque de données. Le profil est dans `archiso/` :

- `packages.x86_64` : les paquets ;
- `profiledef.sh` : le format de l'image et les **droits des fichiers** ;
- `airootfs/` : les fichiers copiés dans l'image ;
- `grub/` : le menu de l'ISO.

Le build ajoute aussi `wsquashfs-launcher`, téléchargé depuis GitHub à un commit figé, ainsi que noVNC, xrdp (compilé depuis l'AUR) et les secrets de `keys/`.

Détails pratiques : [iso.md](iso.md).
