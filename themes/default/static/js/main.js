(function() {
    'use strict';

    document.addEventListener('DOMContentLoaded', function() {
        initCodeBlocks();
        initAnchorLinks();
        initSmoothScroll();
        initExternalLinks();
    });

    function initCodeBlocks() {
        var codeBlocks = document.querySelectorAll('pre code');
        codeBlocks.forEach(function(block) {
            var pre = block.parentElement;
            if (!pre.classList.contains('code-block')) {
                pre.classList.add('code-block');

                var lang = '';
                var classList = Array.from(block.classList);
                for (var i = 0; i < classList.length; i++) {
                    if (classList[i].indexOf('language-') === 0) {
                        lang = classList[i].replace('language-', '');
                        break;
                    }
                }

                if (lang) {
                    var langLabel = document.createElement('div');
                    langLabel.className = 'code-lang-label';
                    langLabel.textContent = lang.toUpperCase();
                    pre.insertBefore(langLabel, block);
                }

                var copyBtn = document.createElement('button');
                copyBtn.className = 'code-copy-btn';
                copyBtn.innerHTML = '复制';
                copyBtn.setAttribute('aria-label', '复制代码');
                copyBtn.addEventListener('click', function() {
                    var code = block.textContent;
                    navigator.clipboard.writeText(code).then(function() {
                        copyBtn.textContent = '已复制!';
                        setTimeout(function() {
                            copyBtn.textContent = '复制';
                        }, 2000);
                    }).catch(function() {
                        copyBtn.textContent = '复制失败';
                        setTimeout(function() {
                            copyBtn.textContent = '复制';
                        }, 2000);
                    });
                });
                pre.insertBefore(copyBtn, block);
            }
        });
    }

    function initAnchorLinks() {
        var headings = document.querySelectorAll('.post-content h1[id], .post-content h2[id], .post-content h3[id], .post-content h4[id]');
        headings.forEach(function(heading) {
            if (!heading.querySelector('.anchor-link')) {
                var anchor = document.createElement('a');
                anchor.className = 'anchor-link';
                anchor.href = '#' + heading.id;
                anchor.innerHTML = '¶';
                anchor.setAttribute('aria-label', '链接到本节');
                heading.appendChild(anchor);
            }
        });
    }

    function initSmoothScroll() {
        var links = document.querySelectorAll('a[href^="#"]');
        links.forEach(function(link) {
            link.addEventListener('click', function(e) {
                var href = link.getAttribute('href');
                if (href && href.length > 1) {
                    var target = document.querySelector(href);
                    if (target) {
                        e.preventDefault();
                        var headerOffset = 80;
                        var elementPosition = target.getBoundingClientRect().top;
                        var offsetPosition = elementPosition + window.pageYOffset - headerOffset;

                        window.scrollTo({
                            top: offsetPosition,
                            behavior: 'smooth'
                        });
                    }
                }
            });
        });
    }

    function initExternalLinks() {
        var links = document.querySelectorAll('.post-content a, .page-content a');
        links.forEach(function(link) {
            var href = link.getAttribute('href');
            if (href && (href.indexOf('http://') === 0 || href.indexOf('https://') === 0)) {
                var isInternal = false;
                try {
                    var url = new URL(href);
                    isInternal = url.hostname === window.location.hostname;
                } catch (e) {}

                if (!isInternal) {
                    link.setAttribute('target', '_blank');
                    link.setAttribute('rel', 'noopener noreferrer');
                    if (!link.querySelector('.external-icon')) {
                        var icon = document.createElement('span');
                        icon.className = 'external-icon';
                        icon.innerHTML = ' ↗';
                        icon.setAttribute('aria-hidden', 'true');
                        link.appendChild(icon);
                    }
                }
            }
        });
    }
})();
