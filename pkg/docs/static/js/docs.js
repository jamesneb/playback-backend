// High-performance documentation interactive features
(function() {
    'use strict';

    // Performance optimization: Use object pooling for DOM operations
    const domPool = {
        tempDiv: document.createElement('div'),
        tempSpan: document.createElement('span')
    };

    // Copy code blocks to clipboard with visual feedback
    function initCodeCopyButtons() {
        const codeBlocks = document.querySelectorAll('pre code');
        const fragment = document.createDocumentFragment();

        codeBlocks.forEach(function(codeBlock) {
            const pre = codeBlock.parentNode;
            if (pre.querySelector('.copy-btn')) return; // Already initialized

            const copyBtn = document.createElement('button');
            copyBtn.className = 'copy-btn';
            copyBtn.textContent = 'Copy';
            copyBtn.setAttribute('aria-label', 'Copy code to clipboard');

            // Use event delegation for performance
            copyBtn.addEventListener('click', function(e) {
                e.preventDefault();
                copyCodeToClipboard(codeBlock, copyBtn);
            });

            pre.style.position = 'relative';
            pre.appendChild(copyBtn);
        });
    }

    // Optimized clipboard copy with fallback
    function copyCodeToClipboard(codeBlock, button) {
        const text = codeBlock.textContent;

        if (navigator.clipboard && window.isSecureContext) {
            navigator.clipboard.writeText(text).then(function() {
                showCopyFeedback(button, true);
            }).catch(function() {
                fallbackCopyToClipboard(text, button);
            });
        } else {
            fallbackCopyToClipboard(text, button);
        }
    }

    // Fallback copy method for older browsers
    function fallbackCopyToClipboard(text, button) {
        const textArea = document.createElement('textarea');
        textArea.value = text;
        textArea.style.position = 'fixed';
        textArea.style.left = '-999999px';
        textArea.style.top = '-999999px';
        document.body.appendChild(textArea);
        textArea.focus();
        textArea.select();

        try {
            const successful = document.execCommand('copy');
            showCopyFeedback(button, successful);
        } catch (err) {
            showCopyFeedback(button, false);
        }

        document.body.removeChild(textArea);
    }

    // Visual feedback for copy operations
    function showCopyFeedback(button, success) {
        const originalText = button.textContent;
        button.textContent = success ? 'Copied!' : 'Failed';
        button.className = success ? 'copy-btn copy-success' : 'copy-btn copy-error';

        setTimeout(function() {
            button.textContent = originalText;
            button.className = 'copy-btn';
        }, 2000);
    }

    // Smooth scroll navigation with performance optimization
    function initSmoothNavigation() {
        const navLinks = document.querySelectorAll('a[href^="#"]');

        navLinks.forEach(function(link) {
            link.addEventListener('click', function(e) {
                const targetId = this.getAttribute('href').substring(1);
                if (!targetId) return;

                const targetElement = document.getElementById(targetId);
                if (targetElement) {
                    e.preventDefault();

                    // Use requestAnimationFrame for smooth performance
                    requestAnimationFrame(function() {
                        targetElement.scrollIntoView({
                            behavior: 'smooth',
                            block: 'start'
                        });
                    });

                    // Update URL without jumping
                    if (history.pushState) {
                        history.pushState(null, null, '#' + targetId);
                    }
                }
            });
        });
    }

    // Highlight current section in navigation
    function initScrollSpy() {
        const sections = document.querySelectorAll('section[id]');
        const navLinks = document.querySelectorAll('.nav-links a[href^="#"]');

        if (sections.length === 0 || navLinks.length === 0) return;

        let ticking = false;

        function updateActiveNav() {
            const scrollPosition = window.pageYOffset + 100; // Offset for fixed header

            let currentSection = '';
            sections.forEach(function(section) {
                const sectionTop = section.offsetTop;
                const sectionHeight = section.offsetHeight;

                if (scrollPosition >= sectionTop && scrollPosition < sectionTop + sectionHeight) {
                    currentSection = section.getAttribute('id');
                }
            });

            // Update nav link states
            navLinks.forEach(function(link) {
                const href = link.getAttribute('href');
                if (href === '#' + currentSection) {
                    link.classList.add('active');
                } else {
                    link.classList.remove('active');
                }
            });

            ticking = false;
        }

        // Throttled scroll handler for performance
        function onScroll() {
            if (!ticking) {
                requestAnimationFrame(updateActiveNav);
                ticking = true;
            }
        }

        window.addEventListener('scroll', onScroll, { passive: true });
    }

    // Search functionality with debouncing
    function initSearch() {
        const searchInput = document.querySelector('.search-input');
        if (!searchInput) return;

        let searchTimeout;
        const searchResults = document.querySelector('.search-results');

        searchInput.addEventListener('input', function(e) {
            clearTimeout(searchTimeout);

            searchTimeout = setTimeout(function() {
                const query = e.target.value.trim();
                if (query.length > 2) {
                    performSearch(query, searchResults);
                } else {
                    hideSearchResults(searchResults);
                }
            }, 300);
        });

        // Click outside to close search results
        document.addEventListener('click', function(e) {
            if (!searchInput.contains(e.target) && !searchResults.contains(e.target)) {
                hideSearchResults(searchResults);
            }
        });
    }

    // Simple client-side search implementation
    function performSearch(query, resultsContainer) {
        // This would typically call an API endpoint
        // For now, implement simple DOM search
        const searchableElements = document.querySelectorAll('h1, h2, h3, h4, p, code');
        const results = [];
        const queryLower = query.toLowerCase();

        searchableElements.forEach(function(element) {
            const text = element.textContent.toLowerCase();
            if (text.includes(queryLower)) {
                results.push({
                    element: element,
                    text: element.textContent,
                    type: element.tagName.toLowerCase()
                });
            }
        });

        displaySearchResults(results.slice(0, 10), resultsContainer); // Limit to 10 results
    }

    function displaySearchResults(results, container) {
        if (!container) return;

        container.innerHTML = '';

        if (results.length === 0) {
            container.innerHTML = '<div class="search-result">No results found</div>';
        } else {
            results.forEach(function(result) {
                const resultDiv = document.createElement('div');
                resultDiv.className = 'search-result';
                resultDiv.innerHTML = `
                    <div class="search-result-type">${result.type}</div>
                    <div class="search-result-text">${truncateText(result.text, 100)}</div>
                `;

                resultDiv.addEventListener('click', function() {
                    result.element.scrollIntoView({ behavior: 'smooth' });
                    hideSearchResults(container);
                });

                container.appendChild(resultDiv);
            });
        }

        container.style.display = 'block';
    }

    function hideSearchResults(container) {
        if (container) {
            container.style.display = 'none';
        }
    }

    function truncateText(text, maxLength) {
        if (text.length <= maxLength) return text;
        return text.substring(0, maxLength) + '...';
    }

    // Theme switcher (if implemented)
    function initThemeSwitcher() {
        const themeToggle = document.querySelector('.theme-toggle');
        if (!themeToggle) return;

        const currentTheme = localStorage.getItem('theme') || 'light';
        document.documentElement.setAttribute('data-theme', currentTheme);

        themeToggle.addEventListener('click', function() {
            const newTheme = document.documentElement.getAttribute('data-theme') === 'light' ? 'dark' : 'light';
            document.documentElement.setAttribute('data-theme', newTheme);
            localStorage.setItem('theme', newTheme);
        });
    }

    // Performance: Use IntersectionObserver for lazy loading
    function initLazyLoading() {
        const images = document.querySelectorAll('img[data-src]');
        if (images.length === 0) return;

        const imageObserver = new IntersectionObserver(function(entries, observer) {
            entries.forEach(function(entry) {
                if (entry.isIntersecting) {
                    const img = entry.target;
                    img.src = img.dataset.src;
                    img.removeAttribute('data-src');
                    observer.unobserve(img);
                }
            });
        });

        images.forEach(function(img) {
            imageObserver.observe(img);
        });
    }

    // Initialize all features when DOM is ready
    function init() {
        initCodeCopyButtons();
        initSmoothNavigation();
        initScrollSpy();
        initSearch();
        initThemeSwitcher();
        initLazyLoading();
    }

    // Optimized initialization
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();