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
        fetchJson("data/static_data.json"),
        fetchJson("data/dynamic_data.json")
    ]);

    staticData = s;
    dynamicData = d;

    notify();
}

//? CARICA DATI RIPETUTAMENTE
async function aggiornaSoloDinamici()
{
    dynamicData = await fetchJson("data/dynamic_data.json");
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
