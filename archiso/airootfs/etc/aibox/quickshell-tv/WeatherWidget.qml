// Météo : icône et température du moment (80 px), libellé et ville, puis les trois
// jours suivants (22 px) avec leur amplitude sur l'échelle des trois jours.
// Carte 660×200 de maquette.
import QtQuick

Card {
    id: card
    property var weather: null
    property string city: ""
    width: 660 * u
    height: 200 * u

    // Icône de ligne (tv-icons/weather), repli sur les nuages.
    function iconSource(icon) {
        var known = ["clear", "partly", "cloud", "fog", "rain", "snow", "storm"];
        return "file:///usr/local/share/aibox/tv-icons/weather/" + (known.indexOf(icon) >= 0 ? icon : "cloud") + ".svg";
    }

    readonly property var days: weather && weather.days ? weather.days : []
    readonly property real lo: days.length ? Math.min(...days.map(d => d.min)) : 0
    readonly property real hi: days.length ? Math.max(...days.map(d => d.max)) : 1

    Row {
        id: now
        spacing: 18 * card.u
        height: 80 * card.u

        Image {
            source: card.iconSource(card.weather ? card.weather.icon : "cloud")
            width: 56 * card.u
            height: width
            sourceSize.width: width
            sourceSize.height: height
            smooth: true
            anchors.verticalCenter: parent.verticalCenter
        }
        FlashText {
            y: 74 * card.u - baselineOffset
            text: card.weather ? card.weather.temp + "°" : "--°"
            font.pixelSize: 80 * card.u
            font.bold: true
        }
        Column {
            anchors.verticalCenter: parent.verticalCenter
            leftPadding: 10 * card.u
            Text {
                text: card.weather ? card.weather.label : "météo indisponible"
                color: Theme.text
                font.family: "DejaVu Sans"
                font.pixelSize: 26 * card.u
            }
            Text {
                text: card.city
                color: Theme.dim
                font.family: "DejaVu Sans"
                font.pixelSize: 22 * card.u
            }
        }
    }

    Row {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        spacing: 24 * card.u

        Repeater {
            model: card.days
            Column {
                width: (parent.width - 2 * parent.spacing) / 3
                spacing: 6 * card.u
                Row {
                    spacing: 10 * card.u
                    Text {
                        text: modelData.day
                        color: Theme.dim
                        font.family: "DejaVu Sans"
                        font.pixelSize: 22 * card.u
                    }
                    Text {
                        text: modelData.max + "° / " + modelData.min + "°"
                        color: Theme.text
                        font.family: "DejaVu Sans"
                        font.pixelSize: 22 * card.u
                    }
                }
                Track {
                    u: card.u
                    width: parent.width
                    from: (modelData.min - card.lo) / Math.max(1, card.hi - card.lo)
                    to: (modelData.max - card.lo) / Math.max(1, card.hi - card.lo)
                }
            }
        }
    }
}
