const ctx = document.getElementById("barChart").getContext("2d");

//! TEMPORANEO POI SI FA CON IL JSON
const SOGLIA = 5;

const dati =
{
    labels: ["A", "B", "C", "D", "E"],
    values: [3, 7, 4.5, 8, 2]
};

//? PENSO FACCIA UN ARRAY DI COLORI ????
const colori = dati.values.map(v => {
    return v > SOGLIA ? "rgba(255, 80, 80, 0.8)"
                      : "rgba(80, 255, 80, 0.8)";
});

const chart = new Chart(ctx,
    {
        type: "bar",
        data:
        {
            labels: dati.labels,
            datasets:
                [
                    {
                        label: "Valori sensori",
                        data: dati.values,
                        backgroundColor: colori
                    }
                ]
        },
        options:
        {
            animation:
            {
                duration: 100
            },
            scales:
            {
                y:
                {
                    beginAtZero: true
                }
            }
        }
    });

function aggiornaGrafico(nuoviValori) {
    chart.data.datasets[0].data = nuoviValori;

    chart.data.datasets[0].backgroundColor =
        nuoviValori.map(v =>
            v > SOGLIA ? "rgba(255, 80, 80, 0.8)"
                       : "rgba(80, 255, 80, 0.8)"
        );

    chart.update();
}
