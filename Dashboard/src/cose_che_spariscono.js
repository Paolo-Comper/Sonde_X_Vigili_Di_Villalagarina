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
        document.getElementById(targetId).style.display = "block";
    });
});

document.getElementById("sezione-intro").style.display = "block";
