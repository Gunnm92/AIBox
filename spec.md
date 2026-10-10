aibox : un home server physique branché à la TV, avec une IA toujours disponible qui écoute et pilote la maison.
L'utilisateur apprécie le système SteamBox (conteneur Unraid : CachyOS, labwc, XFCE WhiteSur, Sunshine/Moonlight, noVNC). aibox en reprend les briques pour la partie TV et administration.

Matériel : AMD Ryzen AI Max+ 395 (Strix Halo), iGPU Radeon 8060S (RDNA 3.5), 128 Go de mémoire unifiée partagée entre le CPU et le GPU. Le disque est dédié aux données et formatable.
Sur la TV : Steam (jeux locaux) et Moonlight client vers SteamBox. La session se lance quand la DualSense s'allume et se ferme quand elle s'éteint. Le PC reste allumé.
IA : LLM locaux servis par Lemonade, en même temps que les jeux. L'assistant écoute (micro, mot d'activation, reconnaissance vocale), répond (synthèse vocale) et pilote la maison.
Administration à distance par noVNC, comme SteamBox.
Démarrage sur clé USB, système entier en RAM, conçu pour la vitesse (pas forcément en squashfs). But : redémarrer sur un système propre, et le personnaliser ou le reconstruire sans jamais toucher aux données.
Tout ce qui est à l'utilisateur est sur le disque : applis Flatpak et AppImage, configuration, bibliothèque Steam, modèles LLM, appairages Bluetooth, réseau.
Construction de l'ISO : sur aibox lui-même (décidé le 10/10/2026 ; Ubuntu et son Docker disparaissent à l'installation).
Ce qu'offrent déjà les dépôts CachyOS (vérifié le 26/09) :

archiso 90 ;
le profil ISO officiel CachyOS/CachyOS-Live-ISO, maintenu (dossier archiso/) ;
gamescope 3.16 ;
steam (multilib), moonlight-qt 6.1, flatpak, bluez.
Le méta-paquet cachyos-handheld est écarté : il tire Plasma et le matériel du Steam Deck.
Steam reste natif dans l'image : le Flatpak de Steam (com.valvesoftware.Steam, X11) ne fonctionne pas pour cet usage, confirmé par l'utilisateur le 09/10/2026. Ne pas l'installer depuis Bazaar.
moonlight-qt n'est pas mis dans l'image : il tire Qt6 (plusieurs centaines de Mo bloqués en RAM). Moonlight est installé en Flatpak (com.moonlight_stream.Moonlight), sur le disque.

Architecture
Clé USB (Ventoy + ISO) ──boot──► image système (EROFS lz4hc) copiée en RAM + overlay tmpfs (1 Go max) ─► clé retirable
                                   │  système jetable : chaque redémarrage repart de l'image
Disque (btrfs, étiquette AIBOXDATA) ──► /home (config, Steam, ~/Applications AppImage),
                                     /var/lib/flatpak, /srv/models (LLM), swap de secours,
                                     /var/tmp, cache pacman, journaux, coredumps,
                                     /var/lib/bluetooth, réseau, SSH, machine-id
DualSense allumée (BT) ─udev─► aibox-tv.service (tv.slice) ─► gamescope (HDR, FSR) + Steam Big Picture (ou labwc + lanceur)
DualSense éteinte > 2 min ─────────┘ session arrêtée, écran éteint, PC allumé
LLM : llm.service (llm.slice) ─► serveur installé sur le disque, modèles dans /srv/models
Admin : labwc headless + XFCE (admin.slice) ─► wayvnc (127.0.0.1:5900) ─► websocat (127.0.0.1:5901) ─► nginx HTTPS + mot de passe :6080 (noVNC)  + SSH

Système en RAM, conçu pour la vitesse
Format EROFS plutôt que squashfs : archiso le propose (airootfs_image_type="erofs"). EROFS est fait pour des lectures rapides en accès aléatoire, avec peu de CPU.
Compression lz4hc par défaut, et non « aucune compression ». L'image copiée en RAM y est bloquée : c'est un tmpfs, sans swap. Les fichiers lus depuis l'EROFS sont en plus recopiés dans le cache de pages. Sans compression, les fichiers utilisés occupent donc la RAM deux fois. Avec lz4hc, la partie bloquée est environ moitié plus petite, le cache reste récupérable par le noyau, et la décompression lz4 est négligeable sur Zen 5. Le non-compressé reste mesuré en QEMU pour comparer.
copytoram : l'image entière est copiée en RAM au démarrage. Plus aucune lecture sur la clé, qui peut être retirée. Au-dessus, archiso monte un overlay en tmpfs : ce qui change pendant la session (paquets installés à la volée, fichiers système) disparaît au redémarrage. C'est ce qui rend le système « immuable ».
Mémoire unifiée : ce qui est en RAM est pris sur les 128 Go partagés avec le GPU. On ne découpe pas la mémoire à l'avance. Configuration retenue (documentation AMD ROCm « Strix Halo system optimization » et mesures de la communauté, vérifiées le 09/10/2026) :
BIOS : UMA (VRAM dédiée) au minimum, 512 Mo. Tout ce qui est réservé ici est retiré de la RAM, définitivement. Sur la machine actuelle, le BIOS réserve 64 Go : le système ne voit que 62 Go de RAM et le GPU n'a que 31 Go de GTT en plus. C'est à corriger en premier.
Noyau : ttm.pages_limit=28835840, soit 110 Gio en pages de 4 Kio. C'est un plafond, pas une réservation : le GPU prend la mémoire à la demande, jusqu'à ce plafond. Les guides pour machines 100 % LLM montent à 120–128 Gio ; on garde 110 Gio pour laisser la place au système en RAM, à Steam et aux processus côté CPU.
Noyau : ttm.page_pool_size=4194304 (16 Gio). Ce cache garde les pages libérées par le GPU pour les réallouer vite ; ce qu'il retient n'est plus disponible pour le reste du système. Les guides LLM le mettent égal au plafond ; ici on le garde modéré. À mesurer (temps de chargement d'un gros modèle).
amdgpu.gttsize : déprécié (le noyau l'indique), non utilisé.
IOMMU laissé actif (pas d'amd_iommu=off) : le NPU (amdxdna) en a besoin, et le gain mesuré sans IOMMU est faible (environ 2 % en génération de tokens avec llama.cpp).
Noyau minimum : 6.18.4 (correctifs TTM requis par AMD) ; linux-cachyos est en 7.2.
Vérification : aibox-mem (section GPU), /sys/module/ttm/parameters/pages_limit, amdgpu_top.
Limite importante : la mémoire GTT n'est pas comptée dans les cgroups (memcg). AMD a confirmé le 18/09/2026 qu'ils allaient le faire, sans date. Les MemoryMax des slices ne limitent donc que la mémoire côté CPU ; la mémoire GPU n'a qu'un plafond global, ttm.pages_limit. La taille des modèles chargés se limite dans Lemonade, et les jeux ne sont pas limités.
Pour personnaliser le système, on reconstruit l'image, puis on redémarre dessus. Les données ne sont jamais touchées.

Contraintes mémoire
Principe : l'image ne contient que ce qu'il faut pour démarrer, la session TV et l'administration. Tout le reste s'exécute depuis le disque.
Ce qui est dans l'image coûte de la RAM en permanence, même sans être utilisé. Ce qui tourne depuis le disque ne coûte que pendant l'exécution, et le cache de pages est rendu dès que le système en a besoin.

Dans l'image : noyau, firmwares allégés, mesa/vulkan (et lib32), Steam (le bootstrap ; son runtime vit dans /home), gamescope, labwc/XFCE/WhiteSur, wayvnc, websocat, nginx et noVNC, NetworkManager, bluez, openssh, flatpak, fuse2 et fuse3, zram-generator.
Sur le disque : Moonlight et toutes les autres applis (Flatpak), AppImage, runtime Steam, Proton, compatdata et cache de shaders, serveur LLM et modèles.

Budget (128 Go), à mesurer sur la machine :
Poste                                  Cible                Mécanisme
UMA du BIOS                            512 Mo               BIOS
Image EROFS lz4hc (bloquée)            ≤ 3,5 Go             firmwares et paquets allégés
Overlay (écritures système)            ≤ 1 Go               cow_spacesize=1G
/tmp                                   ≤ 2 Go               tmp.mount, size=2G
/dev/shm                               ≤ 8 Go               option de montage size=8G
Journal en RAM                         ≤ 128 Mo             RuntimeMaxUse=128M, ou journal persistant sur @state
Total bloqué par le système            ≤ 6 Go               un test échoue au-delà
Bureau d'admin (admin.slice)           MemoryMax=4G         systemd --user
Session TV (tv.slice)                  MemoryHigh≈16G       mémoire CPU du jeu et de Steam
LLM (llm.slice)                        MemoryMax≈16G        mémoire CPU de Lemonade
GPU, tous usages (GTT)                 ≤ 110 Gio            ttm.pages_limit, plafond global
Modèles chargés par Lemonade           ≈ 80 Go max          configuration de Lemonade (taille, contexte)
Les slices ne voient pas la mémoire GPU (voir « Mémoire unifiée »).

Pourquoi ces limites : par défaut, /tmp et /dev/shm peuvent monter à 50 % de la RAM (64 Go), et tout ce qui est écrit dans le système (/var/tmp, /var/cache/pacman, /var/log, coredumps) atterrit dans l'overlay, donc en RAM.
Écritures système redirigées vers le disque (@state) : /var/tmp, /var/cache/pacman, /var/lib/systemd/coredump, /var/log si le journal est persistant.
Garde-fous contre le manque de mémoire :
zram (zstd, environ 16 Go) pour absorber les pics ;
swapfile de secours sur le disque (sous-volume @swap, nocow, 16 à 32 Go) ;
systemd-oomd sur les slices, la session TV protégée en priorité (ManagedOOMPreference=avoid).
Sans disque AIBOXDATA (mode éphémère), pas de swap disque : seul zram reste, et le serveur LLM ne démarre pas.

Organisation : dépôt dédié
Nouveau dépôt aibox, comme l'a demandé l'utilisateur.

archiso/ : profil écrit à partir des profils releng et baseline d'archiso, adapté à CachyOS (plus simple que CachyOS-Live-ISO, sans installateur Calamares). Il contient packages.x86_64, profiledef.sh (EROFS lz4hc, copytoram), pacman.conf, les fichiers de démarrage GRUB/EFI et airootfs/ (unités systemd, slices, règles udev, scripts).
build.sh : prépare les éléments partagés, puis lance mkarchiso dans un conteneur CachyOS privilégié. tools/engine.sh choisit le moteur : Docker s'il y en a un vrai (hôte de développement), sinon Podman en root sur aibox (mkarchiso et QEMU demandent un conteneur privilégié, impossible sans root), avec son stockage sur le disque de données (/var/lib/aibox/state/containers) ; refusé en mode éphémère. Testé en VM le 10/10/2026 : l'ISO construite dans aibox s'installe par aibox-update-boot et démarre.
Code partagé avec SteamBox, repris plutôt que réécrit. Dépôt SteamBox en sous-module git figé sur un commit, ou fichiers vendorés s'ils divergent :
root-cachyos/scripts/labwc-session.sh et xfce-defaults.sh : bureau d'administration, clavier, thème labwc assorti au thème GTK ;
la logique desktop_exec de desktop-env.sh, transposée en unités systemd --user au lieu de s6 ;
l'installation de WhiteSur (thème, icônes, curseur, libadwaita) du Dockerfile SteamBox, à sortir dans un script de build commun (install-whitesur) qui reprend les 4 pièges documentés de l'installateur ;
la vérification des icônes des .desktop et le fc-cache -s final : glycin tué par seccomp si le cache de polices est incomplet ;
les réglages wayvnc et noVNC de SteamBox, dont la page d'accueil (resize=scale, reconnect). websockify n'est pas repris (pas de paquet, installé par pip dans SteamBox ; voir étape 3).

Étapes
1. Système live minimal en RAM, avec persistance
Paquets :
linux-cachyos (noyau récent, nécessaire pour Strix Halo), amd-ucode ;
firmwares allégés : linux-firmware complet pèse plus d'1 Go. On ne garde que amdgpu, le Wi-Fi/Bluetooth réellement présent et l'audio (paquets linux-firmware-* séparés, ou tri au build) ;
mesa, vulkan-radeon et leurs versions lib32 ;
networkmanager, bluez, openssh, flatpak, fuse2, fuse3, zram-generator. Plus le démarrage EROFS lz4hc et copytoram.
Ligne de commande du noyau : copytoram=y, cow_spacesize=1G, ttm.pages_limit=28835840, ttm.page_pool_size=4194304.
BIOS : UMA au minimum (512 Mo), Secure Boot désactivé.
Montages en RAM limités : /tmp (2 Go), /dev/shm (8 Go), journal (RuntimeMaxUse=128M).
Utilisateur default (uid 1000), sudo, sans mot de passe root. Clé SSH déposée au build.
Disque de données : un outil aibox-setup-disk /dev/nvmeXnY, lancé une seule fois à la main, formate le disque en btrfs avec l'étiquette AIBOXDATA et les sous-volumes @home, @flatpak, @bluetooth, @nm, @ssh, @state, @models (nodatacow, sans compression) et @swap (nocow, avec le swapfile).
Au démarrage, des unités de montage systemd (par étiquette) montent /home et /srv/models, et relient par bind mount (pas de lien symbolique) /var/lib/flatpak, /var/lib/bluetooth, /etc/NetworkManager/system-connections, /var/tmp, /var/cache/pacman, /var/lib/systemd/coredump, les clés d'hôte SSH et le machine-id. Le swapfile est activé.
Sans disque AIBOXDATA, le système démarre en mode éphémère avec un message clair, au lieu d'échouer. En mode éphémère, /home est un tmpfs de 16 Go maximum (la RAM n'est prise que pour ce qui y est écrit) : l'overlay du système (1 Go) ne suffisait pas au premier lancement de Steam (2,4 Go). code-server s'y installe aussi (~700 Mo), pour pouvoir corriger à chaud ; Bazaar/Flathub et Lemonade restent réservés au disque.
Démarrage depuis le disque (audit sur la vraie machine du 10/10/2026, docs/audit-2026-10-10.md) : depuis la clé Ventoy (SanDisk Cruzer Blade, USB 2.0), la copie en RAM prenait ~2 minutes (démarrage total 3 min 23 s). aibox-setup-disk crée maintenant une table GPT : AIBOXBOOT (FAT32, 8 Go — deux images même plus grosses, ex. KDE — systemd-boot + l'image dans aibox-<version>/, entrées désignant la partition par archisodevice=UUID) et AIBOXDATA (btrfs, le reste). aibox-update-boot installe une nouvelle ISO sur AIBOXBOOT, la met par défaut et garde la précédente (jamais celle qui tourne). La clé Ventoy ne sert plus qu'au secours et à apporter les nouvelles ISO. Testé en VM : installation, démarrage depuis le NVMe sans la clé, mise à jour et nettoyage des anciennes versions.
Autres points de l'audit appliqués : plus de console=ttyS0 (pas de port série : serial-getty redémarrait toutes les 10 s) ; copytoram par entrée de menu ; dosfstools, thunar-volman, rtkit, wireless-regdb ajoutés ; LLMNR désactivé (avahi suffit) ; aibox-diag écrit sur AIBOXBOOT sans la clé, son minuteur compte depuis le démarrage de systemd. À surveiller : DualSense vue comme manette Xbox dans Steam jusqu'à un rebranchement (Steam lancé avant la connexion au compte ?), 9 messages « Invalid ELF header magic », périphérique USB qui ne s'énumère pas sur le port 3-5, BIOS GTRP110 (« RDSEED32 is broken »).
Correctif du 10/10/2026 (constaté sur la vraie machine avec aibox-diag) : les montages du disque ne sont plus dans /etc/fstab mais dans /etc/aibox/fstab.data, montés par aibox-data-mount.service seulement si le disque est présent. Avec /etc/fstab, un disque absent laissait des montages en échec, et systemd annulait les services qui en dépendent implicitement : la session TV (WorkingDirectory=/home/default) et Bluetooth (/var/lib/bluetooth).
Fait et testé en VM (disque NVMe virtuel) le 09/10/2026 : montages par /etc/fstab (LABEL=AIBOXDATA, nofail, délai de 5 s, ordre explicite avec x-systemd.before), aibox-data.service (mode éphémère, dossiers de l'utilisateur), clés d'hôte SSH dans @ssh (aibox-ssh-hostkeys.service), swapfile de 16 Go dans @swap. aibox-setup-disk demande de retaper le chemin du disque et refuse la clé de démarrage ou un disque monté.
Pas encore persistant : le machine-id (il doit être en place avant le démarrage de systemd ; à traiter dans l'initramfs).
2. Session TV (première version faite et testée en VM avec GPU partagé le 10/10/2026)
Choix de l'utilisateur : une session « compositeur + lanceur » avec les applis voulues (pour l'instant Steam et Moonlight), plutôt que Steam Big Picture seul.
aibox-tv.service (système, User=default, PAMName=login, TTYPath=/dev/tty1, Slice=tv.slice) ouvre une vraie session logind sur le siège, puis tv-session lance, selon ~/.config/aibox/tv-mode (aibox-tv-mode), gamescope sur l'écran réel avec Steam en mode SteamOS (défaut, depuis le 10/10/2026) ou, en secours, labwc avec sa propre configuration (/etc/aibox/labwc-tv : pas de décorations, chaque fenêtre agrandie) et le lanceur. Elle prend tty1 (plus de console en connexion automatique). Déclenchement par la manette (fait le 11/10/2026) : aibox-tv-watch.service (tv-watch) mesure toutes les 2 s, sur 0,5 s, les rapports du hidraw du dongle DS5Dongle ; au moins deux rapports différents = manette connectée (environ 640 rapports par seconde, avec un compteur). Passage à « allumée » : démarrage de aibox-tv.service, arrêt annulé ; passage à « éteinte » : aibox-tv-idle.timer (2 min). Écarté : la règle udev sur le dongle (apparition et disparition), car le dongle s'annonce aussi manette éteinte, à la mise sous tension du PC, et ne se reconnecte pas toujours à l'allumage de la manette (vu le 11/10/2026). Plus de démarrage avec la machine, sauf manette déjà allumée. Quitter Steam termine la session (Steam note sa sortie, tv-session sort en 75 : RestartPreventExitStatus) ; « Passer au bureau » la relance en mode labwc ; un plantage la relance. Steam déjà ouvert sur le bureau d'administration : la session TV laisse la place (75), sans quoi le Steam de la TV lui passait la main et gamescope tombait, en boucle. À l'arrêt, fb0 mis en veille pour que la TV perde le signal. La fonction « Wake PC on PS Button » du dongle sert à sortir le PC de veille : inutile ici, aibox ne se met jamais en veille.
Lanceur aibox-tv-launcher (Python/pygame, SDL) : grandes tuiles, manette (croix directionnelle ou stick, Croix/A) ou clavier (flèches, Entrée) ; l'appli lancée passe au premier plan, on revient au lanceur quand elle se termine. Applis : /etc/aibox/tv-apps.json, remplaçable par ~/.config/aibox/tv-apps.json. Fond : image du jour de Bing (téléchargée une fois par jour dans ~/.cache/aibox, assombrie, avec sa légende ; fond uni sans réseau).
Steam natif (steam -gamepadui, lib32-mesa, lib32-vulkan-radeon) ; Moonlight n'est plus installé d'office (10/10/2026 : plus utile sur aibox ; à installer en Flatpak au besoin) ; son par PipeWire. Coût : image en RAM 1,5 Go (+300 Mo).
Pièges rencontrés (et réglés) :
deux compositeurs pour le même utilisateur : le bureau d'administration doit prendre wayland-0/:0 d'abord (la session TV l'attend) ; la TV a wayland-1/:1 ;
labwc pousse son affichage dans l'environnement d'activation D-Bus/systemd au démarrage et l'efface à l'arrêt : la session TV le remet sur l'administration (activation-env-admin), sinon Thunar, les portails, etc. s'ouvrent sur la TV ;
la session arrive avec CAP_WAKE_ALARM en capacité ambiante : le bac à sable de Steam (bwrap) refuse alors de démarrer (« Unexpected capabilities but not setuid ») et Steam affiche à tort « requires user namespaces » ; tv-session retire ces capacités (setpriv) ;
le lanceur ne doit pas être en plein écran (labwc le garderait au-dessus des applis) : fenêtre sans bordure agrandie ;
la boucle de relance du lanceur ne tourne que tant que son labwc vit (sinon elle survit à la session).
Steam ne tourne qu'en un exemplaire par utilisateur : ouvert sur le bureau d'administration, il ne peut pas l'être sur la TV.
Test en VM : GPU de l'hôte partagé (virtio-gpu, Venus pour Vulkan, virgl pour OpenGL : « Virtio-GPU Venus (AMD Radeon 8060S) »), pas de vrai passthrough (l'hôte utilise le GPU). Limite de la VM : Moonlight plante avec Vulkan/VA-API via Venus (réglé dans la VM seulement par un flatpak override) ; sur la vraie machine il utilise radeonsi directement.
Reste à faire : déclenchement par la DualSense (règle udev 054c:0ce6 / 0df2, arrêt après 2 minutes sans manette), quitter une appli à la manette, test sur la vraie machine.
Appairage de la DualSense : une fois, via le bureau d'administration ou bluetoothctl. Il reste enregistré sur le disque.
Dongle DualSense (DS5Dongle, repris de l'hôte le 10/10/2026) : configuration Linux de DS5Dongle-Studio (github.com/artzox/DS5Dongle-Studio, licence MIT) dans l'image, quelques Ko : règle udev (accès hidraw, service utilisateur démarré au branchement), règle WirePlumber (sortie renommée ds5_dongle_sink), service utilisateur ds5-haptics-loopback (pw-loopback du son du jeu vers les canaux AUX du dongle : haptique). Ajout aibox : ExecCondition sur la présence USB du dongle, sinon le service bouclait (Restart=always relance un ExecStartPre en échec).
Decky Loader : seule l'unité plugin_loader.service est dans l'image (chemins /home/default/homebrew, ConditionPathExists sur le binaire : inerte tant que Decky n'est pas installé). Decky lui-même s'installe avec l'installeur officiel dans ~/homebrew, sur le disque, et se met à jour seul ; l'unité que l'installeur écrit dans /etc disparaît au redémarrage.
Partages du NAS : aibox-nas (list, add, remove, mount) les monte à la demande (systemd-mount --automount, cifs, options de l'hôte : vers=3.0, uid/gid 1000, cache=loose, mfsymlinks, noserverino…) au démarrage par aibox-nas.service. Partages par défaut au build (keys/nas-mounts, keys/nas.cred → /etc/aibox, root 600 : Media, Games, Workspace de //nas.local sur /mnt/…, disponibles même en mode éphémère) ; ajouts de l'utilisateur sur le disque (/var/lib/aibox/state/nas), prioritaires. nss-mdns résout les noms .local. Testé en VM le 10/10/2026 (par IP : le NAT de la VM ne laisse pas passer mDNS) : montage au premier accès, persistance après redémarrage, retrait.
3. Administration à distance (noVNC)
Bureau d'administration headless : labwc (WLR_BACKENDS=headless, rendu logiciel pixman pour ne pas toucher au GPU) et XFCE (xfsettingsd, xfdesktop, panneau, nm-applet, Thunar, paramètres), en français (locale fr_FR.UTF-8 générée au build). WhiteSur à venir (Adwaita sombre en attendant). Réglages par défaut repris de xfce-defaults.sh de SteamBox, ainsi que xfdesktop-settings forcé sur Xwayland. L'écran virtuel prend la taille de la fenêtre du navigateur (noVNC resize=remote : wayvnc redimensionne la sortie headless). Ce sont des unités systemd --user (utilisateur default, session permanente par linger) dans admin.slice (MemoryMax=4G), relancées automatiquement, sur le modèle des services s6 de SteamBox. Fait et testé en VM le 09/10/2026.
Chaîne d'accès : wayvnc (127.0.0.1:5900, sans mot de passe, local seulement) ← websocat (pont WebSocket, 127.0.0.1:5901) ← nginx (:6080, HTTPS + mot de passe, sert aussi les pages noVNC 1.7.0). Un seul port ouvert sur le réseau : 6080.
Pourquoi cette chaîne :
noVNC 1.7 exige un contexte sécurisé (HTTPS) ;
l'authentification RSA-AES de wayvnc (types 5 et 129) n'est pas celle de noVNC (type 6) : le mot de passe est donc demandé par nginx (authentification HTTP, dans le TLS), ce qui protège aussi le WebSocket ;
le WebSocket intégré à wayvnc plante à la connexion (neatvnc 1.0.2, bogue #177 corrigé en amont) : websocat le remplace ;
websockify (SteamBox) n'existe pas en paquet Arch/CachyOS (installé par pip) ; websocat est un paquet de 3,7 Mo.
Mot de passe : keys/vnc-password (généré au build s'il manque), utilisateur default. HTTPS : CA locale (keys/aibox-ca.*, créée une fois, limitée par nameConstraints aux noms aibox/aibox.local et aux adresses privées), à approuver une fois sur le poste ; certificat du serveur signé par elle à chaque build (397 jours), pour aibox, aibox.local (avahi activé) et les IP de keys/tls-ips. Un certificat auto-signé accepté d'un clic ne suffit pas : Chrome refuse alors les service workers, et les vues web de code-server (panneau Claude Code) ne s'ouvrent pas (constaté le 10/10/2026).
Clavier : c'est wayvnc qui traduit les touches reçues de noVNC (setxkbmap n'agit que sur Xwayland). noVNC transmet le code physique des touches (événements clavier étendus QEMU) ; or, sur un Mac ISO, macOS et les navigateurs inversent les codes des touches « @ # » et « < > » : fr(mac) seul donnait « < » pour « @ ». Disposition par défaut aibox(fr_mac) (/etc/xkb/symbols/aibox, lu par libxkbcommon) : fr(mac) avec ces deux touches inversées et les deux Option pour les caractères spéciaux, comme sur Mac. Testé le 09/10/2026 en envoyant à wayvnc les codes d'un Mac (@, <, #, >, Option+Maj+L = |). aibox-keyboard mac | pc | <disposition> [variante] change la disposition, gardée dans ~/.config/aibox/keyboard.env ; la même est donnée à labwc.
Il reste indépendant de la session TV, donc toujours disponible.
RDP (xrdp, fait et testé en VM le 10/10/2026) : xrdp (AUR, compilé au build avec --nocheck et servi à mkarchiso par un dépôt local temporaire ; 3,7 Mo) sert de passerelle RDP vers le MÊME bureau d'administration : il parle VNC à une seconde instance de wayvnc (127.0.0.1:5910 et [::1]:5910, authentification VNC classique avec le mot de passe noVNC, relax_encryption ; socket de contrôle séparé), en TLS avec le certificat aibox, port 3389. Pas de session X séparée ni de serveur Xorg (default n'a pas de mot de passe Unix). Pièges : xrdp se connecte en [::1] même avec ip=127.0.0.1 ; l'authentification VNC (DES) demande allow_broken_crypto et relax_encryption ; une seconde instance de wayvnc demande son propre socket de contrôle.
Jeux Windows (Batocera .wsquashfs) : wsquashfs-launcher (github.com/Gunnm92/wsquashfs-launcher, figé sur un commit au build) dans l'image, avec squashfuse, fuse-overlayfs, squashfs-tools et umu-launcher (2 à 3 Mo) ; pas de wine système (590 Mo) : les runners (proton-cachyos, UMU-Proton, GE-Proton) vont sur le disque (~/.local/share/Steam/compatibilitytools.d ou ~/.local/share/wsquashfs/wine).
HDR et FSR (10/10/2026) : gamescope pilote la TV (gamescope --steam -e --xwayland-count 2 --hdr-enabled, sans -W/-H : mode préféré 3840x2160@60) avec les variables de gamescope-session (STEAM_GAMESCOPE_HDR_SUPPORTED, VRR, TEARING, DYNAMIC_FPSLIMITER, FANCY_SCALING, STEAM_MULTIPLE_XWAYLANDS…) ; « HDR output enabled » sur la TV Sony (EDID : PQ, HLG, VRR 48–120 Hz), sans les propriétés de couleur SteamOS du noyau (AMD_PLANE_*). Écarté : gamescope imbriqué dans labwc — avec le rendu Vulkan nécessaire au HDR, labwc refuse les images 4K de gamescope (« DMA-BUF is too large to import (3840x2160 > 2560x2560) ») et ne lui propose pas le HDR (bExposeHDRSupport: false). gamescope manquait à l'image jusqu'au 10/10/2026 (la session TV était décrite comme gamescope-session, jamais mise en place). gamescope-session (AUR) remplacé par tv-session : gamescope lance Steam en mode SteamOS ; steamos-session-select (« Passer au bureau ») bascule en mode labwc (lanceur). Écarté : le lanceur sous gamescope — avec --steam, la stratégie de focus est « SteamControlled » : sans Steam, aucune fenêtre affichée (écran noir, 10/10/2026), et sans --steam, Steam perd son intégration (menu rapide, HDR, FSR).
SSH avec clé uniquement. noVNC toujours protégé par mot de passe, réservé au réseau local (pas d'exposition sur Internet).
4. Applications et mises à jour
Interface graphique (fait et testé en VM le 09/10/2026) : Bazaar (magasin Flathub) et Gear Lever (AppImage), eux-mêmes en Flatpak, installés sur le disque au premier démarrage avec le disque (aibox-flathub.service, qui ajoute aussi Flathub). Règle polkit : le groupe wheel installe sans mot de passe, comme pour sudo. Portail xdg-desktop-portal-gtk pour les sélecteurs de fichiers des applis Flatpak.
Menu XFCE : XDG_DATA_DIRS du bureau inclut les exports Flatpak (système et utilisateur) ; les dossiers de raccourcis sont créés au démarrage pour que le menu les surveille dès le début. Testé : Moonlight, Bazaar et Gear Lever apparaissent dans le menu.
Flatpak (Flathub) dans /var/lib/flatpak, sur le disque :
monté par bind mount depuis @flatpak. Le dépôt OSTree et les déploiements restent sur le même sous-volume, car ils utilisent des liens physiques ;
les applis sont exécutées par mmap depuis le disque : leur cache de pages est récupérable ;
TMPDIR des services flatpak et /var/tmp sur le disque, pour que les téléchargements et les déploiements ne passent pas par la RAM ;
org.freedesktop.Platform.GL.default fournit son propre mesa : flatpak update régulier, par un timer.
AppImage dans ~/Applications, sur le disque :
mode FUSE uniquement : l'image squashfs est lue depuis le disque (fuse2 pour le runtime type 2 classique, fuse3 pour le nouveau runtime statique) ;
interdit : --appimage-extract-and-run et APPIMAGE_EXTRACT_AND_RUN=1, qui extraient l'appli dans /tmp, donc en RAM. En filet de sécurité, TMPDIR de la session utilisateur pointe vers ~/.cache/tmp, sur le disque ;
intégration au menu : .desktop dans ~/.local/share/applications (sur le disque). Optionnel : Gear Lever en Flatpak pour gérer les AppImage.
Démarrage automatique (10/10/2026, testé en VM) : services de l'utilisateur (ex. Hermes) en unités systemd --user dans ~/.config/systemd/user, activées une fois (enable) : elles persistent sur le disque et démarrent au boot (linger, disque monté avant user@1000) ; applis graphiques par ~/.config/autostart (xdg-desktop-autostart.target, déclenché par le labwc d'administration ; fenêtre « Session et démarrage » de xfce4-session, la session XFCE elle-même n'est pas utilisée ; nm-applet et xfsettingsd masqués côté autostart car déjà lancés en services) ; session TV : option "autostart" par appli dans tv-apps.json, désactivée par défaut.
Programmes Windows (10/10/2026, testé en VM avec PuTTY) : aibox-run-exe + association par défaut des .exe/.msi (/etc/xdg/mimeapps.list → aibox-run-exe.desktop) ; umu-run avec un Proton partagé (proton-cachyos le plus récent, sinon UMU-Proton téléchargé une seule fois ; PROTON dans ~/.config/aibox/proton.conf) et un préfixe Wine partagé (~/.local/share/aibox/wine/default, choix de l'utilisateur) : premier lancement ~30 s (téléchargement), suivants ~5 s.
Jeux : ntsync chargé au démarrage (Proton 10+), gamemode et lib32-gamemode, lib32-libpulse ; vm.max_map_count=1048576 et LimitNOFILE=524288 déjà en place.
Conteneurs (développement) : Podman sans root, sans démon (0 Mo quand rien ne tourne), dans l'image (+72 Mo d'image en RAM) ; conteneurs et images dans ~/.local/share/containers, sur le disque ; podman-compose, et podman-docker pour la commande docker. Docker natif essayé puis écarté le 09/10/2026 (+125 Mo d'image, démon permanent d'environ 140 Mo) : l'usage prévu est surtout des applis, et des conteneurs pour le développement.
Outils de développement : dans l'image, seulement les petits outils de base (git, jq, rsync, tmux, unzip, zip, wget, distrobox : +65 Mo d'image en RAM ; git seul tire Perl, +104 Mo avant compression). Le reste (base-devel : 478 Mo, compilateurs, SDK, vim…) va dans des environnements Distrobox (conteneurs Podman sur le disque, qui partagent le home et le bureau). Testé le 10/10/2026 : distrobox Arch avec base-devel et vim, conservé après redémarrage.
Éditeur partagé : code-server (VS Code dans le navigateur), sur https://<ip>:6080/code/, derrière le même nginx (HTTPS + mot de passe) que noVNC. Rien dans l'image : aibox-code-server.service (service utilisateur, dev.slice, MemoryMax=8G) le télécharge dans ~/.local/lib au premier démarrage avec le disque (jamais en mode éphémère), avec l'extension Claude Code (Open VSX), puis l'écoute sur 127.0.0.1:8081. Testé le 10/10/2026 (code-server 4.141.0). Pièges : nginx doit transmettre l'hôte avec le port (Host $http_host), sinon code-server refuse le WebSocket ; le « mode restreint » désactive les extensions (désactivé dans les réglages par défaut) ; pas d'interface en français (Open VSX n'a le pack de langue qu'en 1.131, VS Code exige la même version que lui). Autre voie, sans rien installer : VS Code Remote-SSH depuis un poste.
Machines virtuelles : GNOME Boxes ou virt-manager (avec son extension QEMU) en Flatpak, depuis Bazaar : 0 Mo dans l'image, VM sur le disque ; /dev/kvm est accessible à l'utilisateur. Pas de QEMU ni de libvirt natifs. Leurs dossiers d'images (~/.var/app/org.gnome.Boxes/data/gnome-boxes/images, ~/.var/app/org.virt_manager.virt-manager/data/libvirt/images) sont créés au démarrage sans copie à l'écriture (chattr +C), avant la première VM : sinon btrfs fragmente les disques virtuels.
La configuration et la bibliothèque Steam sont dans /home, sur le disque.
Mises à jour (principe : n'installer dans l'image que le nécessaire ; ce qui est dans l'image se met à jour en refaisant l'image, le reste sur place, sur le disque) :
système (noyau, pilotes, mesa, amorce de Steam, Lemonade, bureau, xrdp, scripts aibox) : nouvelle ISO construite sur aibox (./build.sh), puis aibox-update-boot --iso ; la version précédente reste au menu de démarrage (retour arrière). Un pacman -Syu sur la machine ne survit pas au redémarrage ;
Flatpak : Bazaar ou flatpak update ; AppImage : Gear Lever ;
Steam, jeux, Proton : Steam lui-même (home) ; Proton par Steam, ProtonUp ou wsquashfs-launcher ;
Lemonade : backends par Lemonade, modèles par lemonade pull ;
code-server : code-server-install --update ; Decky Loader : son propre outil ;
conteneurs : podman pull / podman auto-update ; distrobox : son gestionnaire de paquets.
Le dépôt AIBox et ses secrets (keys/) sont sauvegardés sur le NAS (Workspace/AIBox), d'où on le restaure sur aibox pour construire.
Les paquets installés à la volée (pacman -S) restent du dépannage : ils vont dans l'overlay, limité à 1 Go.
Optionnel : un bureau TV (labwc + XFCE WhiteSur) accessible depuis Steam par « Passer au bureau », sur le modèle de os-session-select.sh de SteamBox.
5. LLM locaux (fait et testé en VM le 10/10/2026)
Lemonade : paquet Arch officiel lemonade-server (extra, 14 Mo) dans l'image ; seul le serveur (lemond) y est. Les backends que Lemonade télécharge lui-même (llama.cpp Vulkan/ROCm, sd-cpp, whisper.cpp, kokoro, FastFlowLM pour le NPU… environ 40 Go sur l'installation actuelle) vont dans LEMONADE_CACHE_DIR=/srv/models/lemonade, et les modèles dans HF_HOME=/srv/models/huggingface (sous-volume @models, sans copie à l'écriture). La configuration reste dans ~/.config/lemonade (sur le disque).
Service : lemond.service du paquet, en service utilisateur de default (comme sur l'installation actuelle), avec un complément aibox (/etc/systemd/user/lemond.service.d/aibox.conf) : llm.slice (MemoryHigh=16G, MemoryMax=32G de mémoire CPU ; les poids en GTT y échappent), démarrage seulement si /srv/models est monté, mémoire verrouillable illimitée (FastFlowLM, NPU ; relevée aussi sur user@1000.service). default est dans les groupes render et video (GPU /dev/dri, ROCm /dev/kfd, NPU /dev/accel).
Réseau : port 13305, ouvert sur le réseau local (lemonade config set host=0.0.0.0), comme l'installation actuelle. Clé d'API possible (LEMONADE_API_KEY), non activée.
Rien à migrer depuis l'installation actuelle (426 Go de modèles) : les modèles seront retéléchargés sur aibox ; le disque unique de la machine (NVMe 1,8 To) devient AIBOXDATA.
Testé en VM (sans GPU) : Qwen3-0.6B-GGUF téléchargé sur /srv/models, réponse correcte, toujours là après redémarrage.
Lemonade fournit aussi la reconnaissance vocale (whisper.cpp, moonshine) et la synthèse vocale (kokoro) : base de l'assistant vocal.
Backend llama.cpp : en ROCm, désactiver mmap (--no-mmap) pour les modèles de plus de la moitié de la mémoire, sinon le chargement devient très lent. En Vulkan, sans objet.
Cohabitation avec les jeux : par défaut, le modèle reste chargé et les limites font foi. En option (réglable), aibox-tv.service demande à Lemonade de décharger le modèle à son démarrage (appel unload de son API).

Points encore ouverts
Budget mémoire : les chiffres ci-dessus sont des cibles, à mesurer sur la machine (empreinte réelle de l'image, GTT pris par les jeux et par les modèles).
Assistant vocal : micro (lequel, où ?), mot d'activation, reconnaissance et synthèse vocales (Lemonade les fournit : whisper.cpp, moonshine, kokoro), et pilotage de la maison (Home Assistant ou autre ?) : à définir.
Cohabitation jeu/LLM : décharger le modèle au démarrage de la session TV, ou non ? Taille maximale de modèle acceptée ?
Allumer la TV automatiquement (HDMI-CEC) : pas de CEC sur les GPU AMD. Il faudrait un adaptateur Pulse-Eight USB. Hors périmètre sauf demande.
Réseau : Ethernet ou Wi-Fi ? Et quel adaptateur Bluetooth ? À vérifier sur la machine, notamment la stabilité de la reconnexion de la DualSense. Cela détermine aussi les firmwares à garder dans l'image.

Vérification
QEMU/KVM (UEFI, ISO sur clé USB virtuelle retirable, disque NVMe virtuel de 64 Go), avec test-vm.sh :
démarrage en RAM : la clé se retire après le boot ;
mesure de l'empreinte mémoire et du temps de démarrage, EROFS lz4hc contre non compressé (Shmem, temps de démarrage, temps de lancement de Steam) ;
aibox-setup-disk fonctionne sur le disque virtuel ;
un fichier dans /home, une appli Flatpak et une connexion réseau survivent à un redémarrage ;
un paquet installé à la volée dans le système disparaît au redémarrage.
Mémoire, avec une RAM réduite (par exemple 16 Go) pour faire apparaître les problèmes :
après le démarrage, la mémoire bloquée par le système (Shmem et tmpfs, mesurée avec free/smem) reste ≤ 6 Go ;
lancer une grosse appli Flatpak et une AppImage : Shmem ne bouge pas, seul Cached monte, et il redescend sous pression (echo 1 > /proc/sys/vm/drop_caches) ;
écrire 5 Go dans /tmp (dd) : échec propre avec ENOSPC, sans OOM ;
flatpak install d'un runtime : l'overlay ne grossit pas (df /run/archiso/cowspace) ;
stress mémoire dans llm.slice : seul ce slice est tué ou ralenti, la session TV survit ;
allocation GPU au-delà de ttm.pages_limit : échec propre de l'appli, pas d'OOM du système.
Déclenchement par manette, sans matériel, en simulant une DualSense via uhid comme pour SteamBox :
le branchement démarre aibox-tv.service ;
le débranchement l'arrête après le délai, et une reconnexion pendant le délai l'annule.
noVNC : https://<ip>:6080 demande le mot de passe (401 sans), puis affiche le bureau d'administration ; SSH par clé fonctionne.
Sur la machine Strix Halo (clé Ventoy) :
démarrage en RAM, disque AIBOXDATA monté ;
DualSense allumée : Steam Big Picture à l'écran ;
Moonlight vers SteamBox : un jeu en streaming, et un jeu en local ;
DualSense éteinte : session fermée après 2 minutes ;
rallumage : la session revient ;
un modèle d'environ 30 Go chargé, puis un jeu lancé depuis la DualSense : pas d'OOM ; GTT réel vérifié avec amdgpu_top.
