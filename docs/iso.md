# Modifier, construire et installer l'image

Ce guide s'adresse à l'agent (LLM local) comme à l'utilisateur. Suis les étapes dans l'ordre. Chaque commande se lance **sur aibox**, dans `~/Workspace/AIBox`, en tant que `default`. `sudo` ne demande pas de mot de passe.

## 0. Principe

- Le système qui tourne est une **image en lecture seule chargée en RAM**. Toute modification de `/etc` ou de `/usr`, et tout `pacman -S`, est **perdu au redémarrage**.
- Pour un changement durable, on **modifie le dépôt**, on **reconstruit l'image** (`./build.sh`), puis on **l'installe** sur la partition `AIBOXBOOT` (`aibox-update-boot`). Elle sert ensuite au démarrage suivant.
- Les données (`/home`, Flatpak, modèles…) ne sont jamais touchées par ces opérations.

Cycle complet :

```
modifier le dépôt → tester en RAM → commit → ./build.sh → vérifier l'image → aibox-update-boot → arrêt + rallumage → vérifier
```

## 1. Avant de commencer

```sh
cd ~/Workspace/AIBox
git status --short            # rien d'inattendu en cours
git log --oneline -5
mountpoint /var/lib/aibox/state && echo "disque de données OK"   # obligatoire pour construire
ls keys/                      # authorized_keys, vnc-password, aibox-ca.*, tls-ips…
cat /proc/cmdline | grep -o 'archisobasedir=[^ ]*'                # version qui tourne
```

- **Sans le disque de données** (mode éphémère), on ne peut pas construire : arrête-toi.
- **Si `keys/` est vide ou incomplet**, le build en génère de nouveaux secrets : nouveau mot de passe noVNC, nouvelle CA HTTPS à réapprouver sur les postes. Les partages NAS par défaut disparaissent, et l'accès SSH aussi si `authorized_keys` manque. **Demande à l'utilisateur avant de construire.** `keys/README.md` décrit chaque fichier.

## 2. Où faire la modification

| Je veux… | Fichier du dépôt | À ne pas oublier |
|---|---|---|
| Ajouter ou retirer un paquet | `archiso/packages.x86_64` | un paquet par ligne, avec un commentaire qui dit pourquoi ; le paquet doit exister dans les dépôts CachyOS ou Arch |
| Ajouter ou modifier un fichier de configuration | `archiso/airootfs/<chemin réel>` (par exemple `archiso/airootfs/etc/aibox/nginx.conf`) | les droits : voir plus bas |
| Ajouter un **script exécutable** | `archiso/airootfs/usr/local/bin/…` (commandes) ou `…/usr/local/lib/aibox/…` (internes) | **le déclarer dans `file_permissions` de `archiso/profiledef.sh`** (`"0:0:755"`), sauf sous `/usr/local/lib/aibox/`, déjà couvert |
| Ajouter un fichier secret | `keys/` + une ligne `install -m 0600 …` dans `build.sh` | et une entrée `"0:0:600"` dans `profiledef.sh` |
| Ajouter un service système | `archiso/airootfs/etc/systemd/system/x.service` | l'activer avec un lien : `ln -s ../x.service archiso/airootfs/etc/systemd/system/multi-user.target.wants/x.service` |
| Ajouter un service utilisateur | `archiso/airootfs/etc/systemd/user/x.service` | lien dans `…/etc/systemd/user/default.target.wants/` |
| Changer les options du noyau | `kernel_params_x86_64` dans `archiso/profiledef.sh` ; `copytoram` dans `archiso/grub/grub.cfg` | `aibox-update-boot` reprend les options de l'ISO |
| Mettre à jour wsquashfs-launcher | `wsq_ref=` dans `build.sh` | le commit doit être **poussé sur GitHub**, le build le télécharge depuis `raw.githubusercontent.com` |
| Ajouter un sous-volume de données | `archiso/airootfs/usr/local/lib/aibox/subvolumes` et `…/etc/aibox/fstab.data` | il est créé tout seul au démarrage suivant |

**Les droits des fichiers.** mkarchiso remet **tous** les fichiers de `airootfs/` en `0644 root:root`, sauf ceux listés dans `file_permissions` de `profiledef.sh`. Un `chmod +x` dans le dépôt ne suffit pas. Ce piège a déjà frappé deux fois : `wsquashfs-launcher` non exécutable, et `nas.cred` lisible par tous.

## 3. Tester en RAM, avant de construire

On applique la modification sur le système qui tourne. Elle disparaîtra au redémarrage, ce qui est voulu.

```sh
# Fichier de configuration ou script
sudo install -m 0644 archiso/airootfs/etc/aibox/x.conf /etc/aibox/x.conf
sudo install -m 0755 archiso/airootfs/usr/local/bin/x /usr/local/bin/x

# Service système modifié
sudo systemctl daemon-reload && sudo systemctl restart x.service
systemctl status x.service --no-pager
journalctl -b -u x.service --no-pager | tail -30

# Service utilisateur
systemctl --user daemon-reload && systemctl --user restart x.service

# Paquet (essai seulement : l'overlay est limité à 1 Go)
sudo pacman -Sy --needed x
```

Cas particuliers :

- **Session TV** : `sudo systemctl restart aibox-tv` relance l'écran de la TV et ferme Steam et le jeu en cours. Préviens l'utilisateur avant. Tu ne vois pas la TV, mais tu peux vérifier ce que gamescope affiche :
  ```sh
  GAMESCOPE_WAYLAND_DISPLAY=gamescope-0 gamescopectl focus_info
  journalctl -b --no-pager --since "-10s" | grep "focus:"
  ```
  `Global focus window: none` veut dire que l'écran est noir.
- **Son (WirePlumber)** : `systemctl --user restart wireplumber`, puis `pactl list short sinks`. Ça coupe le son un instant : vérifie avant qu'aucun jeu ne tourne (`pgrep -af Win64`).
- **Bureau d'administration** : `systemctl --user restart aibox-admin-panel` (panneau) ou `thunar`.
- **Script Python** : vérifie la syntaxe avec `python3 -c "import ast,sys; ast.parse(open(sys.argv[1]).read())" fichier`. N'utilise pas `py_compile` : il laisse un dossier `__pycache__` dans le dépôt.
- **Attention à `pgrep -f` et `pkill -f`** : le motif se retrouve dans ta propre ligne de commande, et tu risques de tuer ton shell ou de faire une boucle infinie. Préfère `pgrep -x nom`, ou mets le motif entre crochets : `pgrep -f "[g]amescope"`.

Ne construis l'image que si le test est concluant. Si la modification concerne l'écran de la TV, **demande à l'utilisateur de confirmer ce qu'il voit**.

## 4. Commit

```sh
git add -A archiso build.sh README.md spec.md docs   # seulement ce qui fait partie du changement
git status --short                                   # jamais keys/, work/, out/
git commit -F - <<'EOF'
Titre court en français

- détail du changement et de sa raison

Co-Authored-By: …
EOF
```

Ne pousse pas (`git push`) sans l'accord de l'utilisateur.

## 5. Construire

```sh
nice -n 10 ./build.sh > /tmp/build.log 2>&1; echo "exit=$?"
tail -3 /tmp/build.log                     # → ISO : out/aibox-<version>-x86_64.iso (lien : out/aibox-latest.iso)
```

- La durée est de 3 à 6 minutes. Lance-le en arrière-plan si ton outil le permet.
- Il faut environ 10 Go libres sur le disque de données.
- Un échec affiche son erreur à la fin de `/tmp/build.log`. Cherche la première ligne `error` ou `cannot`.
- **Version** : `AAAA.MM.JJ.HHMM-<commit>`, par exemple `2026.10.10.2145-cf122c2`, avec la date et l'heure du build et le commit du dépôt. Le suffixe `-dirty` veut dire que `archiso/` ou `build.sh` avaient des changements non commités : commite d'abord. `AIBOX_VERSION=… ./build.sh` impose une version.
- `out/` garde les 3 ISO les plus récentes ; `out/aibox-latest.iso` pointe sur la dernière.

## 6. Vérifier l'image

Monte l'ISO, puis son système (EROFS), en lecture seule, et contrôle ce que tu as modifié.

```sh
iso=out/aibox-latest.iso
m=$(mktemp -d); mkdir -p $m/iso $m/root
sudo sh -c "mount -o loop,ro $iso $m/iso && mount -t erofs -o loop,ro $m/iso/aibox/x86_64/airootfs.erofs $m/root"
cat $m/iso/aibox/version                                  # version (date, heure, commit)
grep -E '^(gamescope|linux-cachyos|mesa) ' $m/iso/aibox/pkglist.x86_64.txt   # versions des paquets
ls -l $m/root/usr/local/bin/                              # droits des scripts
diff archiso/airootfs/etc/aibox/x.conf $m/root/etc/aibox/x.conf && echo identique
sudo umount $m/root; sleep 1; sudo umount $m/iso || sudo umount -l $m/iso
```

Lance `mount` et `umount` dans le **même** `sudo sh -c "…"` si ton shell tourne dans un autre espace de noms de montage : sinon, les fichiers montés ne sont pas visibles.

## 7. Installer

```sh
sudo blkid -L AIBOXBOOT                                   # la partition existe
sudo aibox-update-boot --iso out/aibox-latest.iso
```

Le script copie l'image dans `AIBOXBOOT/aibox-<version>/`, vérifie sa somme SHA-512, écrit les entrées de systemd-boot et la met par défaut. Il garde les **2** versions les plus récentes (`--keep 2`).

Chaque build a sa propre version : la précédente reste donc au menu de démarrage. La version qui tourne n'est jamais supprimée. Les images d'avant le 10/10/2026 au soir s'appelaient seulement `aibox-AAAA.MM.JJ` : deux builds du même jour s'y écrasaient.

## 8. Redémarrer

Le redémarrage est la décision de l'utilisateur : demande-lui. Conseille-lui un **arrêt complet puis un rallumage** plutôt que `reboot`. Après un redémarrage à chaud, le Bluetooth interne bloque le démarrage environ 65 s.

## 9. Vérifier après le redémarrage

```sh
cat /proc/cmdline | grep -o 'archisobasedir=[^ ]*'   # nouvelle version
systemctl is-system-running; systemctl --failed --no-pager
systemd-analyze                                      # environ 30 s normalement
aibox-mem                                            # mémoire, disque, image
```

Puis vérifie précisément ce qui a changé : le service, le fichier, le comportement.

## 10. Revenir en arrière

1. **Au menu de démarrage** (systemd-boot, 3 secondes) : choisis la version précédente. Pour qu'elle redevienne la version par défaut, réinstalle son ISO avec `aibox-update-boot --iso` : `out/` garde les 3 dernières.
2. **Reconstruire une version précédente** depuis git :
   ```sh
   git log --oneline                       # repérer le bon commit
   git stash -u                            # si des changements sont en cours
   git checkout <commit> && ./build.sh && sudo aibox-update-boot --iso out/aibox-latest.iso
   git checkout main
   ```
   L'ISO porte la date du jour et le commit choisi.
3. **Clé Ventoy** : démarre dessus (menu du BIOS), sur une ISO aibox qu'elle contient, puis réinstalle depuis là : `sudo aibox-update-boot` sans `--iso`.

Pour un problème limité à la session TV, il y a plus simple : `aibox-tv-mode labwc` remet l'ancienne session sans changer d'image, et ce choix tient aux redémarrages.

## Aide-mémoire

| Action | Commande |
|---|---|
| Construire | `./build.sh` |
| Installer | `sudo aibox-update-boot --iso out/aibox-latest.iso` |
| Versions installées | `sudo mount -o ro /dev/disk/by-label/AIBOXBOOT /mnt && ls /mnt; sudo umount /mnt` |
| Version qui tourne | `grep -o 'archisobasedir=[^ ]*' /proc/cmdline` |
| Paquets de l'image | `pkglist.x86_64.txt` dans l'ISO montée |
| Tester l'ISO en VM | `./build.sh --test-key && ./test-vm.sh start` (voir le README) |
| Préparer un disque neuf (efface tout) | `sudo aibox-setup-disk /dev/nvmeXnY` : **uniquement sur demande explicite** |
