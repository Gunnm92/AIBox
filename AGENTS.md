# Consignes pour les agents (LLM local, Claude Code…)

Ce dépôt construit **aibox** : un système CachyOS qui tourne entièrement en RAM, avec les données sur un disque séparé. Tu travailles le plus souvent **sur aibox elle-même**, dans `~/Workspace/AIBox`.

Lis d'abord [docs/README.md](docs/README.md). Pour toute modification du système, suis [docs/iso.md](docs/iso.md) pas à pas.

**Contexte** :
- **SteamBox** est un autre projet, plus ancien : un conteneur sur le serveur Unraid, dépôt `Gunnm92/SteamBox`. Ne travaille que sur aibox, et ne pousse que sur `Gunnm92/AIBox`.
- **Ordre de vérité** : le code de `archiso/`, puis `docs/` et ce fichier (l'état actuel), puis `spec.md`, qui est la spécification et le journal daté des décisions, avec des passages devenus historiques.

## Règles

1. **Le système est jetable.** Un changement dans `/etc`, `/usr` ou un `pacman -S` disparaît au redémarrage. Un changement durable se fait **dans le dépôt** (`archiso/`), puis en reconstruisant l'image.
2. **Teste en RAM avant de construire.** Copie le fichier modifié à sa place sur la machine, relance le service, vérifie. Construis l'image seulement quand ça marche.
3. **Vérifie l'image avant de l'installer** : monte-la et regarde le fichier modifié et ses droits ([docs/iso.md](docs/iso.md), « Vérifier une image »).
4. **Commite avant de construire.** La version de l'image (`AAAA.MM.JJ.HHMM-<commit>`) dit de quel commit elle vient ; `-dirty` signale un build avec des changements non commités, à éviter pour une image installée.
5. **Ne lance jamais sans accord explicite de l'utilisateur** : `aibox-setup-disk` (efface un disque), `dd`, `mkfs`, un redémarrage de la machine (`systemctl reboot`), un `git push`, ou la suppression de fichiers dans `~/Games` ou sur le NAS.
6. **Redémarrage** : préfère un arrêt complet puis rallumage. Un redémarrage à chaud bloque le démarrage environ 65 s (Bluetooth MT7925, voir [docs/problemes-connus.md](docs/problemes-connus.md)).
7. **Secrets** : `keys/` n'est pas versionné. Ne l'affiche pas, ne le copie pas ailleurs, ne le commite jamais.
8. **Commits** : en français, à l'impératif ou au nominatif, comme l'historique (`git log`). Écris des commentaires de code en français, dans le style des fichiers voisins.
9. **Documentation** : quand tu changes un comportement, mets à jour `README.md`, `spec.md` et `docs/` dans le même commit.
