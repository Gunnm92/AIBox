// Thème des widgets de la TV : la teinte vient du fond d'écran du jour (PaletteProbe).
// Lemonade garde sa couleur jaune. Les valeurs ci-dessous sont celles par défaut,
// avant l'analyse de l'image.
pragma Singleton
import QtQuick

QtObject {
    property color accent: "#A8C7FA"
    property color accent2: "#D0BCFF"
    property color accentText: "#0E1420"
    property color cardFill: "#80161A22"
    property color cardBorder: "#14FFFFFF"
    property color sheen: "#14FFFFFF"
    property color track: "#4D7A8AA8"
    property color text: "#F1F0F5"
    property color text2: "#CAC6D0"
    property color dim: "#938F99"
    readonly property color lemon: "#FFE14D"
    readonly property color chipText: "#0B0E13"
    // Fond des pistes (barres, anneau) : blanc translucide, quelle que soit la teinte.
    readonly property color trackWhite: "#1FFFFFFF"
}
