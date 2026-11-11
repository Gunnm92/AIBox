// Météo : température du moment, puis les trois jours suivants avec leur amplitude.
import QtQuick

Card {
    id: card
    property var weather: null
    property string city: ""
    width: 680 * u
    height: 240 * u

    // Icône de ligne néon (tv-icons/weather), repli sur les nuages.
    function iconSource(icon) {
        var known = ["clear", "partly", "cloud", "fog", "rain", "snow", "storm"];
        return "file:///usr/local/share/aibox/tv-icons/weather/" + (known.indexOf(icon) >= 0 ? icon : "cloud") + ".svg";
    }

    // Plage de températures sur les trois jours, pour dimensionner les barres.
    readonly property real lo: weather ? Math.min(...weather.days.map(d => d.min)) : 0
    readonly property real hi: weather ? Math.max(...weather.days.map(d => d.max)) : 1

    Row {
        anchors.fill: parent
        spacing: 26 * card.u

        Column {
            width: 210 * card.u
            spacing: 2 * card.u
            Row {
                spacing: 16 * card.u
                Image {
                    source: card.iconSource(card.weather ? card.weather.icon : "cloud")
                    width: 66 * card.u
                    height: width
                    sourceSize.width: width
                    sourceSize.height: height
                    smooth: true
                    anchors.verticalCenter: parent.verticalCenter
                }
                Text {
                    text: card.weather ? card.weather.temp + "°" : "--°"
                    color: Theme.text
                    font.family: "DejaVu Sans"
                    font.pixelSize: 72 * card.u
                    font.weight: Font.DemiBold
                    anchors.verticalCenter: parent.verticalCenter
                }
            }
            Text {
                text: card.weather ? card.weather.label : "météo indisponible"
                color: Theme.text2
                font.family: "DejaVu Sans"
                font.pixelSize: 24 * card.u
            }
            Text {
                text: card.city
                color: Theme.dim
                font.family: "DejaVu Sans"
                font.pixelSize: 22 * card.u
            }
        }

        Row {
            width: parent.width - 210 * card.u - parent.spacing
            height: parent.height
            spacing: 14 * card.u

            Repeater {
                model: card.weather ? card.weather.days : []
                Column {
                    width: (parent.width - 2 * parent.spacing) / 3
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 6 * card.u

                    Text {
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: modelData.day
                        color: Theme.dim
                        font.family: "DejaVu Sans"
                        font.pixelSize: 22 * card.u
                    }
                    Image {
                        source: card.iconSource(modelData.icon)
                        width: 34 * card.u
                        height: width
                        sourceSize.width: width
                        sourceSize.height: height
                        smooth: true
                        anchors.horizontalCenter: parent.horizontalCenter
                    }
                    // Barre de l'amplitude du jour, sur l'échelle de la semaine.
                    Rectangle {
                        anchors.horizontalCenter: parent.horizontalCenter
                        width: parent.width
                        height: 8 * card.u
                        radius: height / 2
                        color: Theme.track
                        Rectangle {
                            x: parent.width * (modelData.min - card.lo) / Math.max(1, card.hi - card.lo)
                            width: parent.width * (modelData.max - modelData.min) / Math.max(1, card.hi - card.lo)
                            height: parent.height
                            radius: height / 2
                            gradient: Gradient {
                                orientation: Gradient.Horizontal
                                GradientStop { position: 0.0; color: Theme.accent }
                                GradientStop { position: 1.0; color: Theme.accent2 }
                            }
                        }
                    }
                    Text {
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: modelData.max + "° / " + modelData.min + "°"
                        color: Theme.text2
                        font.family: "DejaVu Sans"
                        font.pixelSize: 22 * card.u
                    }
                }
            }
        }
    }
}
