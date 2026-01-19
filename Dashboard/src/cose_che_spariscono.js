document.querySelectorAll("[data-target]").forEach(link =>
{
    link.addEventListener("click", function()
    {
        const targetId = this.getAttribute("data-target");

        //! NASCONDE TUTTTO
        document.querySelectorAll(".sezione").forEach(sec =>
        {
            sec.style.display = "none";
        });

        //! MOSTRA QUELLO CHE VOLEVI VEDERE
        const sezione = document.getElementById(targetId);
        sezione.style.display = "block";

        //se stiamo aprendo la mappa, ricalcola le dimensioni
        if (targetId === "sezione-mappa" && window.maplibre_map)
        {
            setTimeout(() =>
            {
                window.maplibre_map.resize();
            }, 100);
        }

    });
});

document.getElementById("sezione-intro").style.display = "block";
