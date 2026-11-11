// Titres de presse : un à la fois, avec une barre qui se vide avant le suivant.
import QtQuick

Card {
    id: card
    property var news: []
    property int index: 0
    width: 540 * u
    height: 228 * u

    readonly property var current: news.length ? news[index % news.length] : null

    Column {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        spacing: 10 * card.u

        Row {
            spacing: 14 * card.u
            Rectangle {
                height: 34 * card.u
                width: chip.width + 26 * card.u
                radius: height / 2
                color: Theme.accent
                Text {
                    id: chip
                    anchors.centerIn: parent
                    text: card.current ? card.current.source.toUpperCase() : "ACTUALITÉS"
                    color: Theme.accentText
                    font.family: "DejaVu Sans"
                    font.pixelSize: 20 * card.u
                    font.weight: Font.DemiBold
                }
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: card.news.length ? (card.index % card.news.length + 1) + " / " + card.news.length : ""
                color: Theme.dim
                font.family: "DejaVu Sans"
                font.pixelSize: 20 * card.u
            }
        }

        Text {
            width: parent.width
            text: card.current ? card.current.title : "titres indisponibles"
            color: Theme.text
            font.family: "DejaVu Sans"
            font.pixelSize: 29 * card.u
            wrapMode: Text.WordWrap
            maximumLineCount: 3
            elide: Text.ElideRight
            lineHeight: 1.05
        }
    }

    Rectangle {
        id: track
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: 6 * card.u
        radius: height / 2
        color: Theme.track

        Rectangle {
            id: fill
            height: parent.height
            radius: height / 2
            width: 0
            gradient: Gradient {
                orientation: Gradient.Horizontal
                GradientStop { position: 0.0; color: Theme.accent }
                GradientStop { position: 1.0; color: Theme.accent }
            }
        }
    }

    // Une durée de 9 s, comme le changement de titre dans shell.qml.
    NumberAnimation {
        id: progress
        target: fill
        property: "width"
        from: 0
        to: track.width
        duration: 9000
    }
    onIndexChanged: progress.restart()
    Component.onCompleted: progress.start()
}
