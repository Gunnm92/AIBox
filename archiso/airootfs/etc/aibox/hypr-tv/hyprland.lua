-- Session TV sous Hyprland : cartes de widgets (quickshell-tv) et lanceur aibox en plein écran.
-- Sortie virtuelle parasite « WAYLAND-1 » (1228x720) : désactivée, la TV reste seule.
hl.monitor({
    output   = "WAYLAND-1",
    disabled = true,
})

hl.monitor({
    output   = "",
    mode     = "preferred",
    position = "auto",
    scale    = 1,
})

hl.config({
    general = {
        gaps_in = 0,
        gaps_out = 0,
        border_size = 0,
    },
    decoration = {
        rounding = 0,
    },
    animations = {
        enabled = false,
    },
    misc = {
        disable_hyprland_logo = true,
        disable_splash_rendering = true,
    },
})

-- Cartes de widgets : verre dépoli (flou du fond derrière les couches).
hl.layer_rule({
    name  = "aibox-widgets-blur",
    match = { namespace = "^aibox-tv-widgets$" },
    blur  = true,
    ignore_alpha = 0.2,
})

-- Le lanceur prend tout l'écran.
hl.window_rule({
    name  = "aibox-launcher",
    match = { title = "^aibox$" },
    fullscreen = true,
})

hl.on("hyprland.start", function ()
    hl.exec_cmd("quickshell -p /etc/aibox/quickshell-tv")
    hl.exec_cmd("sh -c 'while true; do aibox-tv-launcher; sleep 1; done'")
end)
