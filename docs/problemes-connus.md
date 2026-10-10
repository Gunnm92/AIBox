# Problèmes connus

Chaque entrée donne le symptôme, la cause et ce qu'il faut faire. Ajoute une entrée quand tu résous un problème qui pourrait revenir.

## Démarrage

### Démarrage bloqué environ 65 s après un redémarrage à chaud (10/10/2026)
- **Symptôme** : l'écran reste sur le début du démarrage, avant la copie en RAM. L'utilisateur croit que c'est la manette : allumer ou éteindre la DualSense n'y change rien, c'est une coïncidence.
- **Cause** : après un redémarrage à chaud, le Bluetooth de la carte MediaTek MT7925 (port USB interne 3-5) ne répond plus. Le noyau insiste environ 65 s (`device descriptor read/64, error -110`, puis `device not accepting address`), et le `udevadm settle` de l'initramfs attend.
- **Solution** : faire un arrêt complet puis rallumer. Après un démarrage à froid, tout repart en quelques secondes. Pour confirmer : `journalctl -b -k -o json`, chercher `usb 3-5` avec `_SOURCE_MONOTONIC_TIMESTAMP`.

### Heure fausse au démarrage (10/10/2026)
- **Symptôme** : le journal commence 2 heures en avance, puis l'heure se corrige quand la synchronisation réseau répond (environ 40 s).
- **Cause** : l'horloge du BIOS (RTC) était réglée à l'heure locale. Linux l'attend en UTC (`RTC in local TZ: no`).
- **Solution** : l'horloge du BIOS a été remise à l'heure. Vérifier avec `timedatectl` que `RTC time` est bien en UTC.

## Image

### Un script de l'image n'est pas exécutable, ou un secret est lisible par tous (10/10/2026)
- **Cause** : mkarchiso remet tous les fichiers en `0644`, sauf ceux listés dans `file_permissions` de `archiso/profiledef.sh`.
- **Solution** : déclarer chaque script et chaque secret dans `file_permissions`. Vérifier dans l'image montée ([iso.md](iso.md), § 6).

### Plus de version de secours au menu de démarrage (corrigé le 10/10/2026)
- **Cause** : la version de l'image était la date du jour. Une deuxième image du même jour remplaçait la première, y compris celle qui tournait.
- **Solution** : la version est maintenant `AAAA.MM.JJ.HHMM-<commit>`, unique à chaque build.

### Le build échoue sur `etc/ssh/authorized_keys/default: No such file or directory` (10/10/2026)
- **Cause** : git ne versionne pas les dossiers vides, et le dossier manquait dans un clone neuf.
- **Solution** : `install -D` dans `build.sh` (corrigé).

### Les secrets de `keys/` sont absents
- **Conséquence** : le build génère une nouvelle CA HTTPS, à réapprouver sur les postes, et un nouveau mot de passe noVNC. Les partages NAS par défaut disparaissent, et l'accès SSH aussi si `authorized_keys` manque.
- **Solution** : récupérer `keys/` depuis le NAS (`/mnt/Workspace/AIBox/keys/`) avant de construire, et y recopier `keys/` après un changement.

## Session TV

### Écran noir avec gamescope (10/10/2026)
- **Cause** : avec `--steam`, gamescope n'affiche que la fenêtre choisie par Steam (« SteamControlled »). Un autre programme lancé seul, comme le lanceur à tuiles, reste invisible.
- **Solution** : gamescope lance Steam directement, et le lanceur ne tourne qu'en mode labwc. Pour diagnostiquer : `gamescopectl focus_info`.

### gamescope imbriqué dans labwc : pas d'image, pas de HDR (10/10/2026)
- **Cause** : avec le rendu Vulkan, nécessaire au HDR, labwc refuse les images 4K (« DMA-BUF is too large to import (3840x2160 > 2560x2560) »). Et il ne propose pas le HDR à gamescope (`bExposeHDRSupport: false`).
- **Solution** : abandonné. gamescope pilote directement la TV.

### Pas de 4K à 120 Hz
- **Cause** : le pilote amdgpu n'a pas le FRL du HDMI 2.1, refusé par le HDMI Forum aux pilotes libres. La 4K à 120 Hz ne passe qu'en 4:2:0 8 bits.
- **Solution** : rester en 4K à 60 Hz, ou acheter un adaptateur USB-C ou DP vers HDMI 2.1 « PCON ». Le noyau annonce « DP-HDMI FRL PCON supported ».

## Bureau d'administration

### Une appli Qt ou Flatpak lancée depuis Thunar plante (« could not connect to display ») (10/10/2026)
- **Cause** : Thunar et les portails, réveillés par D-Bus avant que labwc ait publié son affichage, démarraient sans `WAYLAND_DISPLAY` ni `DISPLAY`.
- **Solution** : affichage par défaut dans `/etc/systemd/user.conf.d/10-aibox-admin-display.conf` (corrigé).

### Thunar ne prend pas le nouveau thème
- **Cause** : Thunar tourne en démon et ne recharge pas le thème à chaud.
- **Solution** : `systemctl --user restart thunar`.

## Son et manette

### Son du PC dans le haut-parleur de la manette
- **Cause** : c'est voulu. La boucle `ds5-haptics-loopback` envoie le son aux canaux 0/1 du dongle, que le firmware utilise pour les auto-haptiques et le haut-parleur.
- **Solution** : régler « Auto-mute Speaker » ou le mode Replace dans le firmware du dongle (portail web DS5Dongle-Studio).

### Pas de haptiques natives dans Ride 6 (et sans doute d'autres jeux récents)
- **Cause** : le jeu envoie ses haptiques comme un **son 4 canaux** sur la carte son USB de la manette, en n'envoyant que des rapports `0x31` avec les moteurs à 0 (diagnostic SteamBox du 30/09/2026). Sous Windows, il retrouve cette carte son grâce au ContainerId qu'elle partage avec la manette. Wine ne fait pas ce lien.
- **Pistes** : les auto-haptiques du dongle (mode Replace), qui donnent une approximation tirée de la bande-son ; ou faire présenter `ds5_dongle_sink` par Wine comme la carte son de la manette (chantier ouvert).
- **Attention** : si le dongle est réénuméré pendant le jeu (manette rallumée), un jeu en `HIDRAW=1` perd la manette. Allume la manette **avant** de lancer le jeu.

### Pas de son 5.1 sur la TV
- **Cause** : le profil UCM « HiFi » de la carte HDMI d'AMD ne propose que la stéréo.
- **Solution** : `52-hdmi-surround.conf` désactive UCM pour cette carte (corrigé), puis on choisit `output:hdmi-surround-extra1`.

## Outils

### `pgrep -f` ou `pkill -f` tue ton propre shell, ou boucle sans fin
- **Cause** : le motif cherché figure aussi dans ta propre ligne de commande.
- **Solution** : `pgrep -x nom`, ou un motif entre crochets : `pgrep -f "[g]amescope"`.

### `dmesg : Opération non permise`
- **Solution** : `journalctl -k` (ou `-o json` pour les horodatages d'origine).

### Le build tire une version de paquet plus ancienne que celle de la machine
- **Exemple** : gamescope 3.16.30 dans l'image, alors que 3.16.31 était disponible sur la machine.
- **Cause** : le build passe par les dépôts de `archiso/pacman.conf`, qui ne sont pas forcément ceux de la machine.
- **Solution** : vérifier les versions dans `pkglist.x86_64.txt` de l'ISO montée.

## Session TV (suite)

### La session TV se relance en boucle, gamescope plante (11/10/2026)
- **Cause** : Steam était déjà ouvert sur le bureau d'administration, depuis noVNC. Le Steam de la TV lui passe la main et s'arrête, gamescope tombe avec lui, puis `Restart=always` relance le tout. Le journal montre « Primary child shut down! ».
- **Solution** : `tv-session` ne démarre pas si Steam est déjà ouvert (code 75, sans relance).

### Steam fermé, il revient aussitôt (11/10/2026)
- **Cause** : `Restart=always` relançait la session après « Quitter » dans Steam.
- **Solution** : Steam note sa propre sortie, et `tv-session` termine alors la session (code 75). Rallumer la manette la relance.

## Applis Flatpak

### Un chemin en `/run/user/1000/doc/…` au lieu du vrai fichier (11/10/2026)
- **Exemple** : Steam ROM Manager (Flatpak), exécutable `/usr/local/bin/wsquashfs-launcher` choisi par la fenêtre de sélection.
- **Cause** : une appli Flatpak ne voit jamais le `/usr` de la machine, même avec l'accès `host`. Le portail lui passe alors le fichier par le portail de documents, avec un chemin temporaire.
- **Solution** : taper le chemin à la main (`/usr/local/bin/…`). Le raccourci est lancé par Steam, qui n'est pas en Flatpak. Sinon, prendre la version AppImage de l'appli.
