function getCurrentTheme() {
    return document.body.className;
}

async function configClerk() {
    await Clerk.load();

    const navbarRight = document.querySelector('.navbar-right');
    const signInButton = document.getElementById('sign-in');
    signInButton.className = 'title-wrapper';
    signInButton.setAttribute('data-tooltip', Clerk.user ? 'Account settings' : 'Sign in');

    const button = document.createElement('button');
    button.innerHTML = Clerk.user ? '<i class="fas fa-user"></i>' : '<i class="fas fa-user-circle"></i>';
    button.onclick = () => {
        if (Clerk.user) {
            Clerk.openUserProfile();
        } else {
            Clerk.openSignIn();
        }
    };

    signInButton.innerHTML = '';
    signInButton.appendChild(button);
    navbarRight.insertBefore(signInButton, navbarRight.firstChild);
}

window.onload = async () => {
    // await configClerk();

    document.body.addEventListener("themeChanged", async (evt) => {
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

    document.body.addEventListener("htmx:afterSwap", function (evt) {
        console.log("0", evt)

        if (evt.detail.target.id === "post-content") {

            console.log("1")

            if (typeof MathJax !== "undefined" && MathJax.typesetPromise) {
                console.log("2")
                const content = evt.detail.target;
                MathJax.typesetPromise([content])
                    .then(() => {
                        content.classList.remove("mathjax-ready");
                        requestAnimationFrame(() => {
                            content.classList.add("mathjax-ready");
                        });
                    })
                    .catch((err) => {
                        console.error("MathJax error:", err);
                    });
            }
        }
    });

    document.body.addEventListener("htmx:beforeSwap", function (evt) {
        if (evt.detail.target.id === "post-content") {
            evt.detail.shouldSwap = true; // Ensure title updates
        }
    });
}
