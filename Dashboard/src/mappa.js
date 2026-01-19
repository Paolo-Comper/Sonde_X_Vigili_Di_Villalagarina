window.maplibre_map = null;

let last_static_map = null;
let last_dynamic_map = null;

import { onDatiAggiornati } from "./fetch_data.js";

onDatiAggiornati((staticData, dynamicData) => {
    last_static_map = staticData;
    last_dynamic_map = dynamicData;

    if (!window.maplibre_map)
        return;

    aggiornaMappaConNuoviDati(staticData, dynamicData);
});


async function caricaMappa() {
    const map = new maplibregl.Map(
        {
            container: "map",
            style: "https://basemaps.cartocdn.com/gl/positron-gl-style/style.json",
            center: [11.0, 45.9],
            zoom: 9
        });

    window.maplibre_map = map;

    map.on("load", () => {
        if (last_static_map && last_dynamic_map) {
            inizializzaMappa(map, last_static_map, last_dynamic_map);
        }
    });
}

function inizializzaMappa(map, static_data, dynamic_data) {
    //? SOURCE NODI
    map.addSource("nodi",
        {
            type: "geojson",
            data: creaFeatureCollectionNodi(static_data, dynamic_data)
        });

    //? LINK
    aggiungiCollegamenti(map, static_data);

    //? LAYER PUNTI OK
    map.addLayer(
        {
            id: "nodi-ok",
            type: "circle",
            source: "nodi",
            filter: ["==", ["get", "stato"], "ok"],
            paint:
            {
                "circle-radius": 8,
                "circle-color": "#2ecc71",
                "circle-opacity": 0.9
            }
        });

    //? LAYER PUNTI ALERT
    map.addLayer(
        {
            id: "nodi-alert",
            type: "circle",
            source: "nodi",
            filter: ["==", ["get", "stato"], "alert"],
            paint:
            {
                "circle-radius": 9,
                "circle-color": "#e74c3c",
                "circle-opacity": 1.0
            }
        });

    map.on("click", "nodi-ok", e => mostraPopup(e, map));
    map.on("click", "nodi-alert", e => mostraPopup(e, map));
}

function creaFeatureCollectionNodi(static_data, dynamic_data) {
    const SOGLIA = static_data.soglia;

    const features = static_data.data.map(nodo => {
        const valore = dynamic_data.valori[nodo.id];
        const stato = valore > SOGLIA ? "alert" : "ok";

        return {
            type: "Feature",
            properties:
            {
                id: nodo.id,
                nome: nodo.label,
                valore: valore,
                stato: stato
            },
            geometry:
            {
                type: "Point",
                coordinates: [nodo.lon, nodo.lat]
            }
        };
    });

    return {
        type: "FeatureCollection",
        features
    };
}

function aggiornaMappaConNuoviDati(static_data, dynamic_data) {
    const map = window.maplibre_map;
    const source = map.getSource("nodi");

    if (!source)
        return;

    source.setData(
        creaFeatureCollectionNodi(static_data, dynamic_data)
    );
}

function mostraPopup(e, map) {
    const p = e.features[0].properties;

    new maplibregl.Popup()
        .setLngLat(e.lngLat)
        .setHTML(`
            <strong>${p.nome}</strong><br>
            ID: ${p.id}<br>
            Valore: ${p.valore}
        `)
        .addTo(map);
}

function aggiungiCollegamenti(map, static_data) {
    const linee = static_data.links.map(a => {
        const from = static_data.data.find(n => n.id === a.from);
        const to = static_data.data.find(n => n.id === a.to);

        return {
            type: "Feature",
            geometry:
            {
                type: "LineString",
                coordinates:
                    [
                        [from.lon, from.lat],
                        [to.lon, to.lat]
                    ]
            }
        };
    });

    map.addSource("archi",
        {
            type: "geojson",
            data:
            {
                type: "FeatureCollection",
                features: linee
            }
        });

    map.addLayer(
        {
            id: "archi-layer",
            type: "line",
            source: "archi",
            paint:
            {
                "line-width": 1.5,
                "line-color": "#666666",
                "line-opacity": 0.45
            }

        });
}

// AVVIO
caricaMappa();
