import { onDatiAggiornati } from "./fetch_data.js";

onDatiAggiornati((staticData, dynamicData) => {
    aggiornaGrafo(staticData, dynamicData);
});

mermaid.initialize({
    startOnLoad: false
});

// Funzione per creare ID validi per Mermaid (senza spazi)
function creaIdMermaid(idOriginale) {
    return idOriginale.replace(/\s+/g, '_');
}

async function aggiornaGrafo(staticData, dynamicData) {
    const SOGLIA = staticData.soglia;

    let testo = "flowchart TD\n";

    // Prima passata: creiamo tutti i nodi con ID validi per Mermaid
    dynamicData.data.forEach(nodo => {
        const idMermaid = creaIdMermaid(nodo.id);
        const valore = nodo.value;
        const classe = valore > SOGLIA ? "alert" : "ok";

        // Mostriamo l'ID originale come label, ma usiamo l'ID senza spazi per Mermaid
        testo += `${idMermaid}(${nodo.id}: ${valore}):::${classe}\n`;
    });

    testo += "\n";

    // Seconda passata: creiamo i collegamenti usando ID Mermaid
    staticData.links.forEach(a => {
        const fromMermaid = creaIdMermaid(a.from);
        const toMermaid = creaIdMermaid(a.to);
        testo += `${fromMermaid} --> ${toMermaid}\n`;
    });

    testo += "\n";

    testo += "classDef ok fill:#7CFF7C,stroke:#2E8B57;\n";
    testo += "classDef alert fill:#FF7C7C,stroke:#8B0000;\n";

    const container = document.getElementById("mermaid-container");

    try {
        // Render off-screen per evitare flicker
        const { svg } = await mermaid.render(
            "mermaid-live",
            testo
        );

        // Sostituisce SOLO l'SVG
        container.innerHTML = svg;

        // Mantiene il tuo comportamento di scala/viewport
        const nuovoSvg = container.querySelector("svg");
        if (nuovoSvg) {
            nuovoSvg.setAttribute(
                "preserveAspectRatio",
                "xMinYMin meet"
            );
        }
    } catch (error) {
        console.error("Errore nel rendering Mermaid:", error);
        container.innerHTML = `<div class="error">Errore nel diagramma: ${error.message}</div>`;
    }
}
