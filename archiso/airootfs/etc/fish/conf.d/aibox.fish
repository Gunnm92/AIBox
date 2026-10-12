# Réglages fish d'aibox, communs à tous les comptes.

# Outils installés par l'utilisateur (hermes, claude…), comme ~/.bashrc pour bash.
# --global : pas de variable universelle écrite dans ~/.config/fish.
fish_add_path --global ~/.local/bin

# Résumé de la machine à l'ouverture d'un terminal interactif.
# Une fonction fish_greeting dans ~/.config/fish/config.fish la remplace.
function fish_greeting
    fastfetch
end
