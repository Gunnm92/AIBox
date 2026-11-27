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
- **Conséquence** : le build génère une nouvelle CA HTTPS, à réapprouver sur les postes, et un nouveau mot de passe du bureau web. Les partages NAS par défaut disparaissent, et l'accès SSH aussi si `authorized_keys` manque.
- **Solution** : récupérer `keys/` depuis le NAS (`/mnt/Workspace/AIBox/keys/`) avant de construire, et y recopier `keys/` après un changement.

## Session TV

### Écran noir avec gamescope (10/10/2026)
- **Cause** : avec `--steam`, gamescope n'affiche que la fenêtre choisie par Steam (« SteamControlled »). Un autre programme lancé seul, comme le lanceur à tuiles, reste invisible.
- **Solution** : gamescope lance Steam directement ; le lanceur de l'époque (retiré le 27/11/2026) ne tournait que hors de gamescope. Pour diagnostiquer : `gamescopectl focus_info`.

### gamescope imbriqué dans labwc : pas d'image, pas de HDR (10/10/2026)
- **Cause** : avec le rendu Vulkan, nécessaire au HDR, labwc refuse les images 4K (« DMA-BUF is too large to import (3840x2160 > 2560x2560) »). Et il ne propose pas le HDR à gamescope (`bExposeHDRSupport: false`).
- **Solution** : abandonné. gamescope pilote directement la TV.

### Plasma survit à l'arrêt de la session TV (27/11/2026)
- **Symptôme** : après l'extinction de la manette, `kwin_wayland` et `ksmserver` tournent encore, une génération de plus à chaque bascule (Bigscreen → gamescope → Bigscreen), et `loginctl list-sessions` garde des sessions sur `tty1`, avec plusieurs Go de mémoire.
- **Cause** : avec `PAMName=login`, les processus de la session vont dans le scope logind (`session-N.scope`), pas dans le cgroup du service. Seul `dbus-run-session` reçoit l'arrêt ; Plasma ne s'arrête pas avec lui. `loginctl terminate-session` ne suffit pas non plus (scope « abandoned »).
- **Solution** : `tv-session` note `XDG_SESSION_ID` dans `$XDG_RUNTIME_DIR/aibox-tv-session-id`, et `aibox-tv.service` arrête `session-N.scope` en `ExecStopPost`. À la main : `sudo systemctl stop session-<N>.scope`.

### Le jeu ne voit plus la manette après le bouton PS dans Moonlight (27/11/2026)
- **Symptôme** : dans Moonlight, l'interface de Steam suit la manette ; après un appui sur PS (menu de Steam en jeu), le jeu ne la voit plus.
- **Cause** : `plasma-bigscreen-inputhandler` lit lui aussi la DualSense (SDL3, hidraw). Il se met en retrait quand une appli a le focus, mais garde par défaut le bouton PS (`startButtonEnabledWhenSuppressed`) : il revient à l'accueil, Moonlight perd le focus et cesse d'envoyer la manette (`backgroundgamepad=false`). Maintenu, le bouton éteint en plus la manette.
- **Solution** : bouton PS laissé aux applis, réglé par manette (numéro de série) dans `~/.config/plasma-bigscreen-inputhandlerrc`, sur le disque : `StartButtonDisabledWhenSuppressedControllers=serial:…`. Pour une autre manette, depuis la session TV : `busctl --user call org.kde.plasma.bigscreen.inputhandler /InputHandler org.kde.plasma.bigscreen.inputhandler setStartButtonEnabledWhenSuppressed sb "serial:<numéro>" false` (numéros : méthode `connectedControllers`). Pour quitter Moonlight : Select+Start+L1+R1.

### Pas de HDMI-CEC (27/11/2026)
- **Cause** : `amdgpu` ne pilote pas la ligne CEC de la sortie HDMI : pas de `/dev/cec*`, `cec-client -l` ne trouve rien. Le CEC par DisplayPort (`drm_dp_cec`) ne vaut que pour les ports DP.
- **Solution** : un adaptateur USB-CEC (Pulse-Eight), que libcec, et donc Bigscreen, reconnaît. Pas fait.

### Pas de 4K à 120 Hz
- **Cause** : le pilote amdgpu n'a pas le FRL du HDMI 2.1, refusé par le HDMI Forum aux pilotes libres. La 4K à 120 Hz ne passe qu'en 4:2:0 8 bits.
- **Solution** : rester en 4K à 60 Hz, ou acheter un adaptateur USB-C ou DP vers HDMI 2.1 « PCON ». Le noyau annonce « DP-HDMI FRL PCON supported ».

## Bureau d'administration

### Le KWin du bureau ne prend pas le socket attendu (27/11/2026)
- **Symptôme** : clavier et souris sans effet dans Selkies ; le journal d'`aibox-admin-selkies` répète « no longer answers ». Ou la session TV refusait de démarrer en attendant `wayland-0`.
- **Cause** : `kwin_wayland_wrapper`, lancé par `startplasma-wayland`, prend le premier `wayland-N` libre **à partir de 1**, jamais `wayland-0`, et ignore `--socket`. Selkies fait de même. Les noms dépendent donc de l'ordre de démarrage.
- **Solution** : rien n'est écrit en dur. L'autostart `aibox-session-env` relie `wayland-admin` au socket du KWin du bureau, et Selkies vise ce nom (`SELKIES_APP_WAYLAND_DISPLAY`). `admin-session` efface le lien au démarrage. La TV publie le sien dans `aibox-tv.env`.

### Selkies injecte le clavier dans son propre compositeur (27/11/2026)
- **Cause** : sans `SELKIES_APP_WAYLAND_DISPLAY`, Selkies cherche « le » compositeur imbriqué parmi les sockets `wayland-<chiffres>`. Avec la TV en plus, il en voit plusieurs et se rabat sur le sien (« Multiple candidate app-compositor sockets »). Les touches non ASCII se perdent alors (constaté sur SteamBox).
- **Solution** : `SELKIES_APP_WAYLAND_DISPLAY=wayland-admin` (voir plus haut).

### Le son de la TV part dans le navigateur (27/11/2026)
- **Cause** : la sortie par défaut choisie à la main (`default.configured.audio.sink=selkies`, dans `~/.local/state/wireplumber/default-nodes`, sur le disque) vaut pour tout PipeWire, la TV comprise.
- **Solution** : ne pas choisir « Navigateur-Selkies » comme sortie par défaut. Les applis du bureau y sortent d'elles-mêmes (`PULSE_SINK=selkies`). Pour revenir : `pactl set-default-sink alsa_output.pci-0000_c4_00.1.hdmi-surround-extra1`.

### Chrome s'ouvre sur l'autre bureau (27/11/2026)
- **Cause** : Chrome n'a qu'une instance : lancé une seconde fois, il ouvre sa fenêtre là où tourne la première (« Ouverture dans une session de navigateur existante »).
- **Solution** : fermer Chrome sur l'autre bureau (TV ou administration) avant de le lancer.

## Son et manette

### Le récepteur ds5-usbip ne joue que du silence (22/11/2026)
- **Cause** : le tampon visé (`sess.latency.msec`) doit dépasser le délai de la sortie du dongle (USB puis Bluetooth). À 10 ms, `module-rtp-source` le ramène à zéro (« Delay to device is higher than the target buffer size ») et ne joue que du silence.
- **Solution** : `sess.latency.msec = 40` (valeur validée le 22/11), puis 20 ms depuis le 23/11 : la règle WirePlumber `52-ds5dongle-latency.conf` raccourcit le délai de la sortie du dongle (périodes ALSA de 128 trames, `api.alsa.headroom = 0`). Ne pas descendre en dessous sans mesurer, ni retirer la règle sans remonter le tampon à 40 ms.

### Haptiques 24 dB plus faibles : la sortie du dongle s'ouvre à 40 % (22/11/2026)
- **Cause** : WirePlumber ouvre une nouvelle sortie à 40 % de volume, et les haptiques perdent alors 24 dB.
- **Solution** : règle `device.routes.default-sink-volume = 1.0` sur la carte du dongle, dans `51-ds5dongle.conf`. Elle ne joue qu'à la première apparition de la carte : ensuite, WirePlumber restaure le volume mémorisé dans `~/.local/state/wireplumber/default-routes`, sur le disque de données (pas à chaque démarrage, contrairement à ce que dit le doc de ds5-usbip).

### Le son de la manette joué sur la TV sans le dongle (22/11/2026)
- **Cause** : sans verrou, le flux `ds5_usbip_haptics` peut se rabattre sur la sortie par défaut (TV) si le dongle est absent : haut-parleur et haptiques deviennent des grésillements audibles.
- **Écarté** : `node.dont-fallback` + `target.object`. Le flux se relie bien si le dongle est déjà là, mais il disparaît quand la sortie arrive après lui (démarrage de PipeWire) et n'est jamais relié (essais du 22/11/2026).
- **Solution** : le récepteur est en `node.autoconnect = false`, WirePlumber ne le relie jamais. Le script `/usr/local/lib/aibox/ds5-usbip-link` (ExecStartPost de `ds5-usbip.service`) relie ses 4 ports à `ds5_dongle_sink` par `pw-link`. Sans dongle, aucune liaison.

### Son du PC dans le haut-parleur de la manette
- **Cause** : c'est voulu. La boucle `ds5-haptics-loopback` envoie le son aux canaux 0/1 du dongle, que le firmware utilise pour les auto-haptiques et le haut-parleur.
- **Solution** : régler « Auto-mute Speaker » ou le mode Replace dans le firmware du dongle (portail web DS5Dongle-Studio).

### Pas de haptiques natives dans Ride 6 (et sans doute d'autres jeux récents)
- **Cause** : le jeu envoie ses haptiques comme un **son 4 canaux** sur la carte son USB de la manette, en n'envoyant que des rapports `0x31` avec les moteurs à 0 (diagnostic SteamBox du 30/09/2026). Sous Windows, il retrouve cette carte son grâce au ContainerId qu'elle partage avec la manette. Wine ne fait pas ce lien.
- **Pistes** : d'abord ds5-usbip (22/11/2026) : SteamBox envoie le son de la manette émulée en RTP, 4 canaux, haptiques en 3-4 ; AIBox le reçoit et le joue sur le dongle en mode « Off » (`aibox ds5-mode natif`). Le son arrive, mais il reste à vérifier que Ride 6 produit des haptiques sur ce chemin. Autres pistes : les auto-haptiques du dongle (mode Replace, `aibox ds5-mode son`), qui donnent une approximation tirée de la bande-son ; ou faire présenter `ds5_dongle_sink` par Wine comme la carte son de la manette (chantier ouvert).
- **Attention** : si le dongle est réénuméré pendant le jeu (manette rallumée), un jeu en `HIDRAW=1` perd la manette. Allume la manette **avant** de lancer le jeu.

### Pas de son 5.1 sur la TV
- **Cause** : le profil UCM « HiFi » de la carte HDMI d'AMD ne propose que la stéréo.
- **Solution** : `52-hdmi-surround.conf` désactive UCM pour cette carte (corrigé), puis on choisit `output:hdmi-surround-extra1`.

## Mise à jour automatique

### `aibox update` : « le LLM ne répond pas » (27/11/2026)
- **Cause** : LiteLLM ne démarrait plus (`PermissionError: [Errno 13] Permission denied: '/app/config.yaml'`, relance en boucle). Son Quadlet (`~/.config/containers/systemd/litellm.container`, dans le home) avait `User=1000` et `config.yaml` est en 0600. Avec Podman sans root, l'uid 1000 du conteneur est 100999 sur l'hôte : seul l'uid 0 du conteneur est `default`. Le conteneur lancé avant ce changement tournait encore ; la panne n'est apparue qu'au redémarrage suivant.
- **Solution** : `User=0` dans le Quadlet (root du conteneur = `default`, sans privilège), clés gardées en 0600. Lancer aussi `aibox update` sans sudo : sous sudo, il ne trouve pas la clé (il le refuse désormais).

## Réseau

### Dongle Ethernet AX88179B : `ax88179_178a` sans lien (29/10/2026)
- **Constat** : le dongle `0b95:1790` a trois configurations USB : 1 (constructeur, `ax88179_178a`), 2 (NCM, `cdc_ncm`, celle que choisit le noyau) et 3 (ECM, `cdc_ether`). En config 1 (`echo 1 > /sys/bus/usb/devices/6-1.3/bConfigurationValue`), `ax88179_178a` se lie mais le lien ne monte jamais (`Link status is: 0`, `Failed to read reg index 0x0040: -32`) : le pilote du noyau ne gère pas la révision B. Bloquer `cdc_ncm` (essai du 23/10) ne changeait pas de configuration.
- **Mesures** : NCM et ECM font jeu égal en téléchargement (environ 700 Mbit/s sur un flux, 820 sur quatre, 5 paquets perdus sur 500 000) : rien à changer dans l'image. Les pertes vues dans Moonlight restent à mesurer en UDP.

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
- **Cause** : Steam était déjà ouvert sur le bureau d'administration, à distance. Le Steam de la TV lui passe la main et s'arrête, gamescope tombe avec lui, puis `Restart=always` relance le tout. Le journal montre « Primary child shut down! ».
- **Solution** : `tv-session` ne démarre pas si Steam est déjà ouvert (code 75, sans relance).

### Steam fermé, il revient aussitôt (11/10/2026)
- **Cause** : `Restart=always` relançait la session après « Quitter » dans Steam.
- **Solution** : Steam note sa propre sortie, et `tv-session` termine alors la session (code 75). Rallumer la manette la relance.

## Applis Flatpak

### Un chemin en `/run/user/1000/doc/…` au lieu du vrai fichier (11/10/2026)
- **Exemple** : Steam ROM Manager (Flatpak), exécutable `~/.local/bin/wsquashfs-launcher` choisi par la fenêtre de sélection (l'ancien chemin `/usr/local/bin/…` n'existe plus depuis que l'image ne le fournit plus).
- **Cause** : une appli Flatpak ne voit jamais le `/usr` de la machine, même avec l'accès `host`. Le portail lui passe alors le fichier par le portail de documents, avec un chemin temporaire.
- **Solution** : taper le chemin à la main (`/usr/local/bin/…`). Le raccourci est lancé par Steam, qui n'est pas en Flatpak. Sinon, prendre la version AppImage de l'appli.
