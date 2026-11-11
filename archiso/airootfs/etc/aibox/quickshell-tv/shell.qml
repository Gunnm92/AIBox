// Widgets de la session TV : quatre cartes flottantes dans les coins de l'écran,
// au-dessus du lanceur (sans zone exclusive : le lanceur garde tout l'écran).
// Données : /usr/local/lib/aibox/tv-widgets-data (JSON), rafraîchies chaque minute.
// Tailles en « u » : 1 u = 1 pixel sur un écran 1080 p, mis à l'échelle sur l'écran réel.
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import QtQuick

ShellRoot {
    id: root

    property var data: ({})
    property int newsIndex: 0
    readonly property var news: data.news || []
    readonly property real u: (Quickshell.screens.length ? Quickshell.screens[0].height : 1080) / 1080
    readonly property real margin: 50 * u

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

    Timer {
        interval: 9000
        running: root.news.length > 1
        repeat: true
        onTriggered: root.newsIndex = (root.newsIndex + 1) % root.news.length
    }

    PanelWindow {
        anchors { top: true; left: true }
        margins { top: root.margin; left: root.margin }
        implicitWidth: clockCard.width
        implicitHeight: clockCard.height
        exclusiveZone: 0
        color: "transparent"
        WlrLayershell.layer: WlrLayer.Top
        WlrLayershell.namespace: "aibox-tv-widgets"

        ClockWidget { id: clockCard; u: root.u }
    }

    PanelWindow {
        anchors { top: true; right: true }
        margins { top: root.margin; right: root.margin }
        implicitWidth: weatherCard.width
        implicitHeight: weatherCard.height
        exclusiveZone: 0
        color: "transparent"
        WlrLayershell.layer: WlrLayer.Top
        WlrLayershell.namespace: "aibox-tv-widgets"

        WeatherWidget {
            id: weatherCard
            u: root.u
            weather: root.data.weather || null
            city: root.data.city || ""
        }
    }

    PanelWindow {
        anchors { bottom: true; left: true }
        margins { bottom: root.margin; left: root.margin }
        implicitWidth: newsCard.width
        implicitHeight: newsCard.height
        exclusiveZone: 0
        color: "transparent"
        WlrLayershell.layer: WlrLayer.Top
        WlrLayershell.namespace: "aibox-tv-widgets"

        NewsWidget {
            id: newsCard
            u: root.u
            news: root.news
            index: root.newsIndex
        }
    }

    PanelWindow {
        anchors { bottom: true; right: true }
        margins { bottom: root.margin; right: root.margin }
        implicitWidth: systemCard.width
        implicitHeight: systemCard.height
        exclusiveZone: 0
        color: "transparent"
        WlrLayershell.layer: WlrLayer.Top
        WlrLayershell.namespace: "aibox-tv-widgets"

        SystemWidget {
            id: systemCard
            u: root.u
            sys: root.data.system || ({})
            updated: root.data.updated || ""
        }
    }
}
