function getCurrentTheme() {
    return document.body.className;
}

window.onload = () => {
    document.body.addEventListener("themeChanged", (evt) => {
        console.log(evt.detail.value);
        document.body.className = evt.detail.value;
        const themeIconSpan = document.querySelector(".theme-icon");
        const syntaxThemeSelect = document.querySelector(".syntax-theme-select");

        if (syntaxThemeSelect.value !== evt.detail.syntaxTheme) {
            syntaxThemeSelect.value = evt.detail.syntaxTheme;
            htmx.ajax("GET", "/syntax-theme/" + evt.detail.syntaxTheme, "#syntax-highlight", {
                headers: {
                    "hx-target": "#syntax-highlight",
                    "hx-swap": "innerHTML",
                },
            });
        }

        htmx.trigger(themeIconSpan, "load");
    });
}
