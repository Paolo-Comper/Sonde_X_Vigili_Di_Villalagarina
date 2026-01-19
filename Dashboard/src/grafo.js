mermaid.initialize(
{
    startOnLoad: false
});

let staticData;
let dynamicData;

async function caricaDati()
{
    const s = await fetch("data/static_data.json");
    const d = await fetch("data/dynamic_data.json");

    staticData = await s.json();
    dynamicData = await d.json();

    aggiornaGrafo();
}

async function aggiornaGrafo()
{
    const SOGLIA = staticData.soglia;

    let testo = "flowchart TD\n";
    testo += "classDef ok fill:#7CFF7C,stroke:#2E8B57;\n";
    testo += "classDef alert fill:#FF7C7C,stroke:#8B0000;\n";

    staticData.nodi.forEach(n =>
    {
        const valore = dynamicData.valori[n.id];
        const classe = valore > SOGLIA ? "alert" : "ok";

        testo += `${n.id}(${n.label}: ${valore}):::${classe}\n`;
    });

    staticData.archi.forEach(a =>
    {
        testo += `${a.from} --> ${a.to}\n`;
    });

    const container = document.getElementById("mermaid-container");

    // Render off-screen per evitare flicker
    const { svg } = await mermaid.render(
        "mermaid-live",
        testo
    );

    // Sostituisce SOLO l’SVG
    container.innerHTML = svg;

    // Mantiene il tuo comportamento di scala/viewport
    const nuovoSvg = container.querySelector("svg");
    nuovoSvg.setAttribute(
        "preserveAspectRatio",
        "xMinYMin meet"
    );
}



//! carica TUTTO
caricaDati();

//! aggiorna periodicamente SOLO i dati dinamici
setInterval(async () =>
{
    const r = await fetch("data/dynamic_data.json");
    dynamicData = await r.json();
    aggiornaGrafo();
}, 2000);

