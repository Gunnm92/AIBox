// Actualités : une image plein cadre par article, zoom lent, fondu entre les titres,
// dégradé pour la lecture et barre de progression avant le titre suivant.
import QtQuick

Card {
    id: card
    property var news: []
    property int index: 0
    width: 640 * u
    height: 720 * u
    clip: true

    readonly property var current: news.length ? news[index % news.length] : null

    // Image de l'article, rognée et lentement zoomée.
    Rectangle {
        id: photo
        anchors.fill: parent
        anchors.margins: 12 * card.u
        radius: card.radius - 12 * card.u
        color: "#0B0E16"
        clip: true

        Image {
            id: img
            anchors.fill: parent
            source: card.current && card.current.image ? "file://" + card.current.image : ""
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
            smooth: true
            opacity: 1
            transformOrigin: Item.Center

            NumberAnimation on scale {
                id: zoom
                from: 1.0
                to: 1.10
                duration: 9500
                easing.type: Easing.InOutSine
                running: true
            }

            // Fondu : on baisse l'image, on change la source, on la remonte.
            NumberAnimation on opacity {
                id: fadeOut
                running: false
                to: 0
                duration: 260
                onStopped: {
                    img.source = card.current && card.current.image ? "file://" + card.current.image : "";
                    fadeIn.restart();
                }
            }
            NumberAnimation on opacity {
                id: fadeIn
                running: false
                to: 1
                duration: 480
                easing.type: Easing.OutCubic
            }
        }

        // Dégradé sombre en bas pour lire le titre.
        Rectangle {
            anchors.fill: parent
            gradient: Gradient {
                GradientStop { position: 0.0; color: "#00000000" }
                GradientStop { position: 0.45; color: "#2A0B0E16" }
                GradientStop { position: 1.0; color: "#F20B0E16" }
            }
        }
    }

    onIndexChanged: {
        zoom.restart();
        fadeOut.restart();
        progress.restart();
    }

    Column {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: track.top
        anchors.bottomMargin: 18 * card.u
        anchors.leftMargin: 30 * card.u
        anchors.rightMargin: 30 * card.u
        spacing: 12 * card.u

        Row {
            spacing: 14 * card.u
            Rectangle {
                height: 34 * card.u
                width: chip.width + 26 * card.u
                radius: height / 2
                gradient: Gradient {
                    orientation: Gradient.Horizontal
                    GradientStop { position: 0.0; color: Theme.accent }
                    GradientStop { position: 1.0; color: Theme.accent2 }
                }
                Text {
                    id: chip
                    anchors.centerIn: parent
                    text: card.current ? card.current.source.toUpperCase() : "ACTUALITÉS"
                    color: Theme.accentText
                    font.family: "DejaVu Sans"
                    font.pixelSize: 24 * card.u
                    font.weight: Font.DemiBold
                }
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: card.news.length ? (card.index % card.news.length + 1) + " / " + card.news.length : ""
                color: Theme.text2
                font.family: "DejaVu Sans"
                font.pixelSize: 24 * card.u
            }
        }

        Text {
            width: parent.width
            text: card.current ? card.current.title : "titres indisponibles"
            color: Theme.text
            font.family: "DejaVu Sans"
            font.pixelSize: 40 * card.u
            font.weight: Font.DemiBold
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
        anchors.margins: 30 * card.u
        anchors.bottomMargin: 26 * card.u
        height: 5 * card.u
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
                GradientStop { position: 1.0; color: Theme.accent2 }
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
    Component.onCompleted: progress.start()
}
