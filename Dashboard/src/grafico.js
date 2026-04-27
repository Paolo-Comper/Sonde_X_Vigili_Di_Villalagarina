import { onDatiAggiornati } from "./fetch_data.js";

onDatiAggiornati((staticData, dynamicData) => {
    aggiornaGrafo(staticData, dynamicData);
});

const ctx = document.getElementById("barChart").getContext("2d");

window.chart = new Chart(ctx, {
    type: "bar",
    data: {
        labels: [], //! NOMI
        datasets: [
            {
                label: "Valori sensori",
                data: [], //! VALORI
                backgroundColor: [] //! COLORI
            }
        ]
    },
    options: {
        plugins: {
            legend: {
                display: false
            }
        },
        animation: {
            duration: 100
        },
        scales: {
            y: {
                beginAtZero: true,  // Parte da 0
                max: 10,            // Massimo fisso a 10
                ticks: {
                    stepSize: 1       // Opzionale: passo di 1
                }
            }
        }
    }
});

function formattaLabel(label) {
    return label.replace(/[_-]+/g, ' ');
}

function aggiornaGrafo(staticData, dynamicData) {
    const valori_array = dynamicData.data.map(nodo => nodo.value);

    const labels = dynamicData.data.map(nodo =>
        formattaLabel(nodo.label)
    );

    chart.data.datasets[0].data = valori_array;
    chart.data.labels = labels;

    chart.data.datasets[0].backgroundColor = dynamicData.data.map(nodo => {
        const soglia = staticData.soglia[nodo.topic] ?? 5;

        return nodo.value > soglia
            ? "rgba(255, 80, 80, 0.8)"
            : "rgba(80, 255, 80, 0.8)";
    });

    chart.update();
}

