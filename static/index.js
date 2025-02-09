function getCurrentTheme() {
    return document.body.className;
}

window.onload = () => {
    window.MathJax = {
        tex: {
            inlineMath: [['$', '$'], ['\\(', '\\)']],
            displayMath: [['$$', '$$'], ['\\[', '\\]']]
        },
        svg: {
            scale: 1.2,
            minScale: 1.2,
            matchFontHeight: true,
            mtextInheritFont: true
        },

        startup: {
            pageReady: () => {
                return MathJax.startup.defaultPageReady().then(() => {
                    const content = document.getElementById('post-content');
                    if (content) {
                        content.classList.add('mathjax-ready');
                    }
                });
            }
        }
    };

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
