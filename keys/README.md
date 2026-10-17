Secrets et réglages propres à ta machine, intégrés à l'image par `build.sh`.
Le contenu de ce dossier (sauf ce README) n'est pas versionné : garde-le (sauvegarde NAS).

Le service `aibox-build` utilise sa propre copie, `/var/lib/aibox/state/build/keys` (root, 600, sur le disque de données), reprise une fois par `sudo aibox-build --import ~/Workspace/AIBox`. Ce dossier-ci ne sert qu'au build à la main (`./build.sh`). Un secret changé doit l'être aux deux endroits.

- `authorized_keys` : clés SSH publiques autorisées (`/etc/ssh/authorized_keys/default`).
- `vnc-password` : mot de passe de noVNC, RDP et code-server (généré au premier build s'il manque).
- `aibox-ca.crt` / `aibox-ca.key` : autorité de certification locale (HTTPS), créée au premier build ;
  `aibox-ca.crt` est à approuver une fois sur chaque poste.
- `tls-ips` : adresses IP couvertes par le certificat HTTPS, en plus de `aibox` et `aibox.local`.
- `nas-mounts` : partages NAS montés automatiquement (`SOURCE CIBLE` par ligne).
- `nas.cred` : identifiants SMB de ces partages (`username=…`, `password=…`).
