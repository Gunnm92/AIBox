# À faire

## Interface pour lancer les commandes et services aibox

Une petite interface, agréable à utiliser, pour se passer du terminal pour les commandes de [docs/commandes.md](docs/commandes.md) :

- **Lancer** : `aibox update --dry-run` ou `--force`, `aibox diag`, `aibox tv-mode gamescope|bigscreen`, `llm-unload`…
- **Voir l'état** : services et minuteurs (actif, en échec, prochain passage), dernière décision de la mise à jour (`~/.local/state/aibox/auto-update/derniere-decision.json`), sortie d'`aibox mem`, partages du NAS (`aibox nas list`).
- **Lire les journaux** de chaque service.
- **Hors de l'interface**, ou derrière une confirmation explicite : `aibox-setup-disk`, `aibox boot` et les redémarrages ([AGENTS.md](AGENTS.md), règle 5).

À trancher :
- **Forme** : une page web servie par le bureau d'administration, à côté de Selkies et code-server sur le port 6080, accessible aussi depuis le téléphone ; ou une appli dans le bureau Plasma ; ou une tuile de Plasma Bigscreen, utilisable à la manette.
- **Droits** : les commandes `sudo` passent par polkit ou par une liste blanche, jamais par un shell libre.
- **Emplacement** : dans l'image (`archiso/`), comme les autres scripts.

## Suites du passage à KDE (27/11/2026)

- **Sunshine** : clavier et souris de Moonlight sous le KWin du bureau d'administration. evdev-bridge ne parle qu'aux protocoles de wlroots ; à adapter (fake-input ou libei de KWin), ou retirer Sunshine.
- **Widgets de la TV** : actualités, météo, machine (anciennes cartes quickshell, retirées) en plasmoïdes pour l'accueil de Bigscreen, si l'accueil seul ne suffit pas.
- **Favoris de Bigscreen** : poser Steam (TV), Moonlight et Bureau en favoris par défaut au premier démarrage, s'ils manquent.
- **HDMI-CEC** : adaptateur Pulse-Eight USB-CEC (libcec, déjà tiré par Bigscreen) et règle udev pour `/dev/ttyACM*`, si on veut piloter la TV à la télécommande.
- **`./test-vm.sh screen`** : grim ne capture plus l'écran de la TV sous KWin ; passer par `spectacle` ou le D-Bus de capture de KWin.
- **Mémoire** : remesurer l'image en RAM et le bureau d'administration (Plasma et Selkies) sur la machine.
