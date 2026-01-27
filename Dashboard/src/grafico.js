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
                beginAtZero: true
            }
        }
    }
});

function aggiornaGrafo(staticData, dynamicData) {

    const valori_array  = dynamicData.data.map(nodo => nodo.value);
    const labels        = dynamicData.data.map(nodo => nodo.id);

    // Applica al grafico
    chart.data.datasets[0].data = valori_array;
    chart.data.labels = labels;

    // Colori basati sulla soglia da staticData
    const soglia = staticData.soglia;
    chart.data.datasets[0].backgroundColor = valori_array.map(v =>
        v > soglia ? "rgba(255, 80, 80, 0.8)" : "rgba(80, 255, 80, 0.8)"
    );

    chart.update();
}
