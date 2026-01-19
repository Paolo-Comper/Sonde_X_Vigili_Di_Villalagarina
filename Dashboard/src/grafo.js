import { onDatiAggiornati } from "./fetch_data.js";

onDatiAggiornati((staticData, dynamicData) =>
{
    aggiornaGrafo(staticData, dynamicData);
});

mermaid.initialize(
    {
        startOnLoad: false
    });

async function aggiornaGrafo(staticData, dynamicData) {
    const SOGLIA = staticData.soglia;

    let testo = "flowchart TD\n";

    staticData.data.forEach(n => {
        const valore = dynamicData.valori[n.id];
        const classe = valore > SOGLIA ? "alert" : "ok";

        testo += `${n.id}(${n.label}: ${valore}):::${classe}\n`;
    });

    testo += "\n";

    staticData.links.forEach(a => {
        testo += `${a.from} --> ${a.to}\n`;
    });

    testo += "\n";

    testo += "classDef ok fill:#7CFF7C,stroke:#2E8B57;\n";
    testo += "classDef alert fill:#FF7C7C,stroke:#8B0000;\n";

    testo += "\n";

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
