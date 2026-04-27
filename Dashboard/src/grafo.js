import { onDatiAggiornati } from "./fetch_data.js";

onDatiAggiornati((staticData, dynamicData) => {
    aggiornaGrafo(staticData, dynamicData);
});

mermaid.initialize({
    startOnLoad: false
});

// Funzione per creare ID validi per Mermaid
function creaIdMermaid(idOriginale) {
    return idOriginale.replace(/\s+/g, '_');
}

function formattaLabel(label) {
    return label.replace(/[_-]+/g, ' ');
}

async function aggiornaGrafo(staticData, dynamicData) {
    let testo = "flowchart TD\n";

    // NODI
    dynamicData.data.forEach(nodo => {
        const idMermaid = creaIdMermaid(nodo.topic);
        const valore = nodo.value;
        const soglia = staticData.soglia[nodo.topic] ?? Infinity;
        const classe = valore > soglia ? "alert" : "ok";

        const labelPulita = formattaLabel(nodo.label);

        testo += `${idMermaid}(${labelPulita}: ${valore}):::${classe}\n`;
    });

    testo += "\n";

    // LINK
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
        const { svg } = await mermaid.render("mermaid-live", testo);

        container.innerHTML = svg;

        const nuovoSvg = container.querySelector("svg");
        if (nuovoSvg) {
            nuovoSvg.setAttribute("preserveAspectRatio", "xMinYMin meet");
        }
    } catch (error) {
        console.error("Errore nel rendering Mermaid:", error);
        container.innerHTML = `<div class="error">Errore nel diagramma: ${error.message}</div>`;
    }
}

