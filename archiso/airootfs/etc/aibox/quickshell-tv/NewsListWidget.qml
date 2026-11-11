// Les quatre titres qui suivent le titre principal : source (18 px, majuscules,
// 150 px de large) et titre (26 px, une ligne). Carte 1032×260 de maquette.
import QtQuick

Card {
    id: card
    property var news: []
    property int index: 0
    width: 1032 * u
    height: 260 * u
    padY: 14

    Column {
        anchors.fill: parent
        Repeater {
            model: card.news.length > 1 ? Math.min(4, card.news.length - 1) : 0
            Item {
                readonly property var item: card.news[(card.index + 1 + modelData) % card.news.length]
                width: parent.width
                height: 58 * card.u
                Rectangle {
                    visible: modelData > 0
                    width: parent.width
                    height: Math.max(1, card.u)
                    color: "#12FFFFFF"
                }
                Row {
                    anchors.verticalCenter: parent.verticalCenter
                    width: parent.width
                    spacing: 20 * card.u
                    Text {
                        width: 150 * card.u
                        anchors.verticalCenter: parent.verticalCenter
                        text: item.source.toUpperCase()
                        color: Theme.dim
                        font.family: "DejaVu Sans"
                        font.pixelSize: 18 * card.u
                        font.letterSpacing: 2 * card.u
                        elide: Text.ElideRight
                    }
                    Text {
                        width: parent.width - 170 * card.u
                        anchors.verticalCenter: parent.verticalCenter
                        text: item.title
                        color: Theme.text
                        font.family: "DejaVu Sans"
                        font.pixelSize: 26 * card.u
                        elide: Text.ElideRight
                    }
                }
            }
        }
    }
}
