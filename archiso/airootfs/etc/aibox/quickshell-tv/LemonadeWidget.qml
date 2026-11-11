// Lemonade (serveur LLM local) : débit du moment, requêtes et modèle chargé.
import QtQuick

Card {
    id: card
    property var lemonade: null
    width: 420 * u
    height: 246 * u

    readonly property bool online: lemonade !== null
    readonly property int active: online ? lemonade.active : 0
    readonly property real tps: online ? lemonade.tps : 0

    // Le débit glisse vers la nouvelle valeur au lieu de sauter.
    property real shownTps: 0
    Behavior on shownTps { NumberAnimation { duration: 700; easing.type: Easing.OutCubic } }
    onTpsChanged: shownTps = tps

    Column {
        anchors.fill: parent
        spacing: 6 * card.u

        Row {
            spacing: 14 * card.u
            Rectangle {
                id: dot
                width: 16 * card.u
                height: width
                radius: width / 2
                anchors.verticalCenter: parent.verticalCenter
                color: card.online ? Theme.lemon : Theme.dim
                SequentialAnimation on opacity {
                    running: card.active > 0
                    loops: Animation.Infinite
                    NumberAnimation { from: 1; to: 0.25; duration: 600; easing.type: Easing.InOutSine }
                    NumberAnimation { from: 0.25; to: 1; duration: 600; easing.type: Easing.InOutSine }
                }
            }
            Text {
                text: "LEMONADE"
                color: Theme.lemon
                font.family: "DejaVu Sans"
                font.pixelSize: 22 * card.u
                font.weight: Font.DemiBold
                anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: "local · " + (card.online ? (card.active > 0 ? card.active + " requête(s)" : "au repos") : "hors ligne")
                color: Theme.dim
                font.family: "DejaVu Sans"
                font.pixelSize: 20 * card.u
                anchors.verticalCenter: parent.verticalCenter
            }
        }

        Row {
            spacing: 18 * card.u
            Text {
                text: Math.round(card.shownTps)
                color: Theme.text
                font.family: "DejaVu Sans"
                font.pixelSize: 64 * card.u
                font.weight: Font.DemiBold
            }
            Column {
                anchors.bottom: parent.bottom
                anchors.bottomMargin: 6 * card.u
                Text {
                    text: "tokens / s"
                    color: Theme.dim
                    font.family: "DejaVu Sans"
                    font.pixelSize: 22 * card.u
                }
            }
        }

        // Jauge du débit, pleine à 60 tokens/s.
        Rectangle {
            width: parent.width
            height: 10 * card.u
            radius: height / 2
            color: Theme.track
            Rectangle {
                width: parent.width * Math.min(1, card.shownTps / 60)
                height: parent.height
                radius: height / 2
                gradient: Gradient {
                    orientation: Gradient.Horizontal
                    GradientStop { position: 0.0; color: Theme.lemon }
                    GradientStop { position: 1.0; color: Theme.accent2 }
                }
                Behavior on width { NumberAnimation { duration: 700; easing.type: Easing.OutCubic } }
            }
        }

        Text {
            width: parent.width
            text: !card.online ? "serveur Lemonade hors ligne"
                : (card.lemonade.loaded.length ? "Modèle : " + card.lemonade.loaded[0] : "aucun modèle chargé")
            color: Theme.text2
            font.family: "DejaVu Sans"
            font.pixelSize: 22 * card.u
            elide: Text.ElideRight
        }

        Text {
            text: card.online
                ? "requêtes " + card.lemonade.requests + "   ↑ " + card.lemonade.tokens_in + "   ↓ " + card.lemonade.tokens_out
                : ""
            color: Theme.dim
            font.family: "DejaVu Sans"
            font.pixelSize: 20 * card.u
        }
    }
}
