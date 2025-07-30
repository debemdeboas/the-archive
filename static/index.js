// ===== CONFIGURATION =====
const ENABLE_SCROLL_RESTORATION = false;

// ===== UTILITY FUNCTIONS =====
function getCurrentTheme() {
    return document.body.className;
}

function toggleNavbar() {
    const navbar = document.querySelector('.navbar');
    navbar.classList.toggle('open');
    const icon = document.querySelector('.hamburger i');

    if (navbar.classList.contains('open')) {
        icon.classList.remove('fa-bars');
        icon.classList.add('fa-times');
    } else {
        icon.classList.remove('fa-times');
        icon.classList.add('fa-bars');
    }
}

// ===== SCROLL POSITION RESTORATION =====
function getTopmostVisiblePost() {
    const mainContent = document.getElementById('content');
    const posts = document.querySelectorAll('.post-item');
    const containerTop = mainContent.scrollTop;

    for (let post of posts) {
        const postTop = post.offsetTop - mainContent.offsetTop;
        if (postTop >= containerTop - /* small offset */ 50) {
            return post.id;
        }
    }
    return null;
}

function waitForContentAndRestore(topPostId, source) {
    let attempts = 0;
    const maxAttempts = 50; // Max ~1 second of waiting

    const tryRestore = () => {
        const postElement = document.getElementById(topPostId);
        if (postElement) {
            const mainContent = document.getElementById('content');
            mainContent.style.scrollBehavior = 'auto';
            postElement.scrollIntoView({ block: 'start' });
            console.log(`${source} - scrolled to post:`, topPostId);
            return true;
        }

        attempts++;
        if (attempts < maxAttempts) {
            requestAnimationFrame(tryRestore);
        } else {
            console.log(`${source} - failed to find post after ${maxAttempts} attempts:`, topPostId);
        }
    };

    tryRestore();
}

function setupScrollRestoration() {
    if (!ENABLE_SCROLL_RESTORATION) return;

    // Store scroll position before navigating away from home
    document.body.addEventListener("htmx:beforeRequest", function (evt) {
        if (window.location.pathname === "/") {
            if (evt.detail.elt.classList.contains("post-item")) {
                // Going to a post - store current position
                const topPostId = getTopmostVisiblePost();
                if (topPostId) {
                    sessionStorage.setItem('homeTopPost', topPostId);
                    console.log("Stored top post ID:", topPostId);
                }
            } else if (evt.detail.elt.id === "nav-go-home") {
                // Clicking home while on home - clear stored position to go to top
                sessionStorage.removeItem('homeTopPost');
                console.log("Cleared stored position - will go to top");
            }
        }
    });

    // Restore scroll position when returning to home via HTMX
    document.body.addEventListener("htmx:afterSettle", function (evt) {
        const isReturningHome = evt.detail.elt.id === "nav-go-home" ||
            (evt.detail.xhr?.responseURL && evt.detail.xhr.responseURL.endsWith("/") && !evt.detail.xhr.responseURL.includes("/posts/"));
        if (isReturningHome) {
            const topPostId = sessionStorage.getItem('homeTopPost');
            if (topPostId) {
                console.log("HTMX - restoring to post:", topPostId);
                waitForContentAndRestore(topPostId, "HTMX");
            } else {
                console.log("HTMX - no stored position, staying at top");
            }
        }
    });

    // Handle browser back/forward button
    window.addEventListener('popstate', function (evt) {
        console.log("Popstate event - URL:", window.location.pathname);
        if (window.location.pathname === "/") {
            const topPostId = sessionStorage.getItem('homeTopPost');
            if (topPostId) {
                console.log("Browser back - restoring to post:", topPostId);
                waitForContentAndRestore(topPostId, "Browser");
            }
        }
    });
}

// ===== CLERK AUTHENTICATION =====
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

// ===== THEME HANDLING =====
function setupThemeHandling() {
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
}

// ===== MATHJAX HANDLING =====
function setupMathJax() {
    document.body.addEventListener("htmx:afterSwap", function (evt) {
        if (typeof MathJax !== "undefined" && MathJax.typesetPromise) {
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
    });
}

// ===== HTMX EVENT HANDLERS =====
function setupHTMXHandlers() {
    document.body.addEventListener("htmx:beforeSwap", function (evt) {
        if (evt.detail.target.id === "post-content") {
            evt.detail.shouldSwap = true; // Ensure title updates
        }
    });
}

// ===== INITIALIZATION =====
window.onload = async () => {
    // await configClerk();

    setupScrollRestoration();
    setupThemeHandling();
    setupMathJax();
    setupHTMXHandlers();
}
