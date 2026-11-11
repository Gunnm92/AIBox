// Widgets de la session TV (maquette « Direction 2 v3 ») : actualités à gauche
// (titre principal et liste), horloge, météo et machine à droite, au-dessus du
// lanceur (sans zone exclusive), qui garde la barre d'icônes en bas. Masquées
// quand la fenêtre du lanceur est fermée (une appli lancée tourne).
// Données : /usr/local/lib/aibox/tv-widgets-data (JSON), rafraîchies chaque minute.
// Cotes en pixels de maquette (1920×1080), « u » les met à l'échelle de l'écran.
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import QtQuick
import "palette.js" as Palette

ShellRoot {
    id: root

    property var data: ({})
    property int newsIndex: 0

    // Applique la palette dérivée de la teinte du fond d'écran à tout le thème.
    function applyPalette(hue) {
        var p = Palette.derive(hue);
        Theme.accent = p.accent;
        Theme.accent2 = p.accent2;
        Theme.accentText = p.accentText;
        Theme.cardFill = p.cardFill;
        Theme.cardBorder = p.cardBorder;
        Theme.track = p.track;
        Theme.text = p.text;
        Theme.text2 = p.text2;
        Theme.dim = p.dim;
    }
    readonly property var news: data.news || []
    // Cartes affichées seulement avec le lanceur : il ferme sa fenêtre pendant
    // qu'une appli tourne (Moonlight…), et les cartes resteraient par-dessus.
    readonly property bool launcherShown: ToplevelManager.toplevels.values.some(t => t.appId === "aibox-tv-launcher")
    readonly property real u: (Quickshell.screens.length ? Quickshell.screens[0].height : 1080) / 1080

    Process {
        id: fetch
        command: ["/usr/local/lib/aibox/tv-widgets-data"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    root.data = JSON.parse(this.text);
                } catch (e) {
                    console.log("widgets : réponse invalide du script de données");
                }
            }
        }
    }

    Timer {
        interval: 60 * 1000
        running: true
        repeat: true
        onTriggered: fetch.running = true
    }

    // Titre suivant toutes les 8 s (barre de NewsHeroWidget).
    Timer {
        interval: 8000
        running: root.news.length > 1
        repeat: true
        onTriggered: root.newsIndex = (root.newsIndex + 1) % root.news.length
    }

    // Cartes placées à leurs cotes de maquette (x, y), sur l'écran 16:9 centré.
    readonly property real ox: ((Quickshell.screens.length ? Quickshell.screens[0].width : 1920) - 1920 * u) / 2

    component Place: PanelWindow {
        property real px: 0
        property real py: 0
        anchors { top: true; left: true }
        margins { top: Math.round(py * root.u); left: Math.round(root.ox + px * root.u) }
        exclusiveZone: 0
        color: "transparent"
        visible: root.launcherShown
        WlrLayershell.layer: WlrLayer.Top
        WlrLayershell.namespace: "aibox-tv-widgets"
    }

    Place {
        px: 96; py: 54
        implicitWidth: hero.width
        implicitHeight: hero.height
        NewsHeroWidget { id: hero; u: root.u; news: root.news; index: root.newsIndex }
    }

    Place {
        px: 96; py: 594
        implicitWidth: list.width
        implicitHeight: list.height
        NewsListWidget { id: list; u: root.u; news: root.news; index: root.newsIndex }
    }

    Place {
        px: 1164; py: 54
        implicitWidth: clockCard.width
        implicitHeight: clockCard.height
        ClockWidget { id: clockCard; u: root.u }

        PaletteProbe {
            source: root.data.wallpaper ? "file://" + root.data.wallpaper : ""
            onReady: (hue) => root.applyPalette(hue)
        }
    }

    Place {
        px: 1164; py: 274
        implicitWidth: weatherCard.width
        implicitHeight: weatherCard.height
        WeatherWidget {
            id: weatherCard
            u: root.u
            weather: root.data.weather || null
            city: root.data.city || ""
        }
    }

    Place {
        px: 1164; py: 494
        implicitWidth: machineCard.width
        implicitHeight: machineCard.height
        MachineWidget {
            id: machineCard
            u: root.u
            lemonade: root.data.lemonade !== undefined ? root.data.lemonade : null
            sys: root.data.system || ({})
        }
    }
}
