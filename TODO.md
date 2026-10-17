# À faire

## Interface pour lancer les commandes et services aibox

Une petite interface, agréable à utiliser, pour se passer du terminal pour les commandes de [docs/commandes.md](docs/commandes.md) :

- **Lancer** : `aibox-auto-update --dry-run` ou `--force`, `aibox-diag`, `aibox-tv-mode gamescope|labwc`, `llm-unload`, `aibox-keyboard`, `aibox-resolution`…
- **Voir l'état** : services et minuteurs (actif, en échec, prochain passage), dernière décision de la mise à jour (`~/.local/state/aibox/auto-update/derniere-decision.json`), sortie d'`aibox-mem`, partages du NAS (`aibox-nas list`).
- **Lire les journaux** de chaque service.
- **Hors de l'interface**, ou derrière une confirmation explicite : `aibox-setup-disk`, `aibox-update-boot` et les redémarrages ([AGENTS.md](AGENTS.md), règle 5).

À trancher :
- **Forme** : une page web servie par le bureau d'administration, à côté de noVNC et code-server sur le port 6080, accessible aussi depuis le téléphone ; ou une appli GTK dans le bureau ; ou une tuile du lanceur de la session TV, utilisable à la manette.
- **Droits** : les commandes `sudo` passent par polkit ou par une liste blanche, jamais par un shell libre.
- **Emplacement** : dans l'image (`archiso/`), comme les autres scripts.
