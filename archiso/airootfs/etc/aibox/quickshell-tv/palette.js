// Palette dérivée d'une teinte (Material 3 simplifié) : surfaces tonales, accent, texte.
// Les couleurs sont au format #AARRGGBB, comme QML.

function hex2(v) {
    var h = Math.round(Math.max(0, Math.min(1, v)) * 255).toString(16);
    return h.length < 2 ? "0" + h : h;
}

// h en degrés, s et l entre 0 et 1.
function hsl(h, s, l, a) {
    h = ((h % 360) + 360) % 360;
    var c = (1 - Math.abs(2 * l - 1)) * s;
    var x = c * (1 - Math.abs((h / 60) % 2 - 1));
    var m = l - c / 2;
    var r, g, b;
    if (h < 60)       { r = c; g = x; b = 0; }
    else if (h < 120) { r = x; g = c; b = 0; }
    else if (h < 180) { r = 0; g = c; b = x; }
    else if (h < 240) { r = 0; g = x; b = c; }
    else if (h < 300) { r = x; g = 0; b = c; }
    else              { r = c; g = 0; b = x; }
    return "#" + hex2(a === undefined ? 1 : a) + hex2(r + m) + hex2(g + m) + hex2(b + m);
}

// Toute la palette à partir de la teinte d'accent.
function derive(hue) {
    return {
        accent:     hsl(hue, 0.62, 0.78),
        accent2:    hsl(hue + 40, 0.55, 0.72),
        accentText: hsl(hue, 0.45, 0.14),
        cardFill:   hsl(hue, 0.18, 0.11, 0.66),
        cardBorder: hsl(hue, 0.30, 0.70, 0.22),
        sheen:      hsl(hue, 0.20, 0.90, 0.07),
        track:      hsl(hue, 0.15, 0.45, 0.30),
        text:       hsl(hue, 0.10, 0.96),
        text2:      hsl(hue, 0.10, 0.82),
        dim:        hsl(hue, 0.08, 0.63)
    };
}

// Teinte dominante d'une image : histogramme des teintes, pondéré par la saturation
// et par les tons moyens (on ignore le noir, le blanc et les zones délavées).
function dominantHue(data) {
    var bins = new Array(12).fill(0), sums = new Array(12).fill(0);
    for (var i = 0; i < data.length; i += 4) {
        var r = data[i] / 255, g = data[i + 1] / 255, b = data[i + 2] / 255;
        var mx = Math.max(r, g, b), mn = Math.min(r, g, b);
        var l = (mx + mn) / 2, d = mx - mn;
        if (d < 0.05) continue;
        var s = d / (1 - Math.abs(2 * l - 1));
        var h;
        if (mx === r)      h = 60 * (((g - b) / d) % 6);
        else if (mx === g) h = 60 * ((b - r) / d + 2);
        else               h = 60 * ((r - g) / d + 4);
        if (h < 0) h += 360;
        var w = s * (1 - Math.abs(2 * l - 1));
        var k = Math.floor(h / 30) % 12;
        bins[k] += w;
        sums[k] += w * h;
    }
    var best = 0;
    for (var j = 1; j < 12; j++) if (bins[j] > bins[best]) best = j;
    return bins[best] > 0 ? sums[best] / bins[best] : 210;
}
