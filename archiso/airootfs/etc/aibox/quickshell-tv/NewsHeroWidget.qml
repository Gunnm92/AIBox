// Titre principal : photo de l'article en fond, dégradé sombre en bas, source,
// position, titre (44 px gras, 3 lignes au plus) et barre de 8 s avant le
// suivant. Fondu croisé de 400 ms entre deux titres (deux diapositives qui
// alternent). Carte 1032×520 de maquette.
import QtQuick
import Quickshell.Widgets

Card {
    id: card
    property var news: []
    property int index: 0
    width: 1032 * u
    height: 520 * u
    padX: 0
    padY: 0

    property bool aFront: true

    // Titre lu directement : dans onNewsChanged, « current » n'est pas encore recalculé.
    function show() {
        var front = aFront ? slideA : slideB;
        front.item = news.length ? news[index % news.length] : null;
        front.pos = news.length ? (index % news.length) + 1 : 0;
    }
    onNewsChanged: show()
    onIndexChanged: {
        aFront = !aFront;
        show();
        progress.restart();
    }

    component Slide: Item {
        id: slide
        property var item: null
        property int pos: 0
        anchors.fill: parent
        Behavior on opacity { NumberAnimation { duration: 400; easing.type: Easing.InOutQuad } }

        Image {
            anchors.fill: parent
            source: slide.item && slide.item.image ? "file://" + slide.item.image : ""
            fillMode: Image.PreserveAspectCrop
            // Décodée à la taille de la carte : une photo d'origine peut être très grande.
            sourceSize.width: width
            sourceSize.height: height
            asynchronous: true
            smooth: true
        }
        // Dégradé : 0,96 en bas, 0,55 à 45 %, transparent à 75 % de la hauteur ;
        // en haut, une ombre légère pour lire « Actualités » sur une photo claire.
        Rectangle {
            anchors.fill: parent
            gradient: Gradient {
                GradientStop { position: 0.0; color: "#7307090C" }
                GradientStop { position: 0.22; color: "#0007090C" }
                GradientStop { position: 0.55; color: "#8C07090C" }
                GradientStop { position: 1.0; color: "#F507090C" }
            }
        }
        Column {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.leftMargin: 40 * card.u
            anchors.rightMargin: 40 * card.u
            anchors.bottomMargin: 64 * card.u   // au-dessus de la barre (36 + 6 + 22)
            spacing: 14 * card.u

            Row {
                spacing: 14 * card.u
                Rectangle {
                    height: chip.implicitHeight + 8 * card.u
                    width: chip.implicitWidth + 28 * card.u
                    radius: height / 2
                    visible: slide.item !== null
                    gradient: Gradient {
                        orientation: Gradient.Horizontal
                        GradientStop { position: 0.0; color: Theme.accent }
                        GradientStop { position: 1.0; color: Theme.accent2 }
                    }
                    Text {
                        id: chip
                        anchors.centerIn: parent
                        text: slide.item ? slide.item.source.toUpperCase() : ""
                        color: Theme.chipText
                        font.family: "DejaVu Sans"
                        font.pixelSize: 20 * card.u
                        font.bold: true
                    }
                }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: slide.pos ? slide.pos + " / " + card.news.length : ""
                    color: Theme.dim
                    font.family: "DejaVu Sans"
                    font.pixelSize: 20 * card.u
                }
            }
            Text {
                width: parent.width
                text: slide.item ? slide.item.title : "titres indisponibles"
                color: Theme.text
                font.family: "DejaVu Sans"
                font.pixelSize: 44 * card.u
                font.bold: true
                wrapMode: Text.WordWrap
                maximumLineCount: 3
                elide: Text.ElideRight
                lineHeightMode: Text.ProportionalHeight
                lineHeight: 1.03   // 1,2 en CSS : DejaVu a déjà 1,16 d'interligne
            }
        }
    }

    ClippingRectangle {
        anchors.fill: parent
        radius: card.radius
        color: "#0C1017"

        Slide { id: slideA; opacity: card.aFront ? 1 : 0 }
        Slide { id: slideB; opacity: card.aFront ? 0 : 1 }
    }

    // Bordure par-dessus la photo.
    Rectangle {
        anchors.fill: parent
        radius: card.radius
        color: "transparent"
        border.color: Theme.cardBorder
        border.width: Math.max(1, card.u)
    }

    Text {
        x: 40 * card.u
        y: 30 * card.u
        text: "ACTUALITÉS · " + card.news.length + " TITRES"
        visible: card.news.length > 0
        color: Theme.text
        font.family: "DejaVu Sans"
        font.pixelSize: 22 * card.u
        font.letterSpacing: 3 * card.u
    }

    Track {
        id: track
        u: card.u
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.leftMargin: 40 * card.u
        anchors.rightMargin: 40 * card.u
        anchors.bottomMargin: 36 * card.u
        NumberAnimation on to {
            id: progress
            from: 0
            to: 1
            duration: 8000
            running: true
        }
    }
}
