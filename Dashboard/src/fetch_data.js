let staticData = null;
let dynamicData = null;

//? CALLBACK
const listeners = [];

//? X REGISTRARSI
export function onDatiAggiornati(callback)
{
    listeners.push(callback);
}

//? CHIAMA I CALLBACK
function notify()
{
    listeners.forEach(cb =>
    {
        cb(staticData, dynamicData);
    });
}

//? CARICAMENTO DATI
async function fetchJson(url)
{
    const r = await fetch(url);

    if (!r.ok)
    {
        throw new Error(`Errore fetch ${url}: ${r.status}`);
    }

    return await r.json();
}

//? CARICA DATI ALL'INIZIO
export async function caricaDati()
{
    const [s, d] = await Promise.all([
        fetchJson("https://raw.githubusercontent.com/Paolo-Comper/Sonde_X_Vigili_Di_Villalagarina/refs/heads/main/Dashboard/data/static_data.json"),
        fetchJson("http://localhost:6969/state.json")
    ]);

    staticData = s;
    dynamicData = d;

    notify();
}

//? CARICA DATI RIPETUTAMENTE
async function aggiornaSoloDinamici()
{
    dynamicData = await fetchJson("http://localhost:6969/state.json");
    notify();
}

//? START
caricaDati();

//? LOOP
setInterval(aggiornaSoloDinamici, 2000);

//! API
// import { onDatiAggiornati } from "./fetch_data.js";

// onDatiAggiornati((staticData, dynamicData) =>
// {
//     aggiornaGrafo(staticData, dynamicData);
// });
