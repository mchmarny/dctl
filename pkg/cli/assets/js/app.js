function isDarkMode() {
    const theme = document.documentElement.getAttribute('data-theme');
    if (theme === 'dark') return true;
    if (theme === 'light') return false;
    return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

function applyChartDefaults() {
    const dark = isDarkMode();
    const textColor = dark ? '#e6edf3' : '#1f2328';
    const gridColor = dark ? 'rgba(230,237,243,0.1)' : 'rgba(31,35,40,0.1)';
    Chart.defaults.color = textColor;
    Chart.defaults.borderColor = gridColor;
    Chart.defaults.transitions = Chart.defaults.transitions || {};
    Chart.defaults.transitions.resize = { animation: { duration: 0 } };
}

function initTheme() {
    const saved = localStorage.getItem('theme');
    if (saved) {
        document.documentElement.setAttribute('data-theme', saved);
    }
    applyChartDefaults();
}

function toggleTheme() {
    const current = document.documentElement.getAttribute('data-theme');
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    let next;
    if (!current) {
        next = prefersDark ? 'light' : 'dark';
    } else if (current === 'dark') {
        next = 'light';
    } else {
        next = 'dark';
    }
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('theme', next);
    applyChartDefaults();
}

initTheme();

const colors = [
    '#0969da',
    '#2da44e',
    '#bf8700',
    '#cf222e',
    '#8250df',
    '#656d76',
    '#0550ae',
    '#116329',
    '#953800',
    '#82071e',
    '#6639ba',
    '#424a53',
    '#368cf9',
    '#4ac26b',
    '#d4a72c',
    '#ff8182'
];

const searchCriteriaView = ["from", "to", "type", "org", "repo", "entity", "user"];
const searchCriteria = {
    "from": null,
    "to": null,
    "type": null,
    "org": null,
    "repo": null,
    "user": null,
    "entity": null,
    "page": 1,
    "page_size": 10,
    init: function () {
        let origValues = {};
        for (let prop in this) {
            if (this.hasOwnProperty(prop) && prop != "origValues") {
                origValues[prop] = this[prop];
            }
        }
        this.origValues = origValues;
    },
    reset: function () {
        for (let prop in this.origValues) {
            this[prop] = this.origValues[prop];
        }
    },
    String: function () {
        let q = [];
        for (let prop in this) {
            if (!searchCriteriaView.includes(prop)) { continue; }
            if (this.hasOwnProperty(prop) && this[prop] != null) {
                q.push(`<b>${prop}</b>: ${this[prop]}`);
            }
        }
        return q.join(", ");
    }
}

let autocomplete_cache = {};
let timeEventsChart;
let leftChart;
let leftChartExcludes = [];
let rightChart;
let rightChartExcludes = [];
let retentionChart;
let prRatioChart;
let timeToMergeChart;
let timeToCloseChart;
let releaseCadenceChart;
let releaseDownloadsChart;
let releaseDownloadsByTagChart;
let reputationChart;
let forksAndActivityChart;
let starsTrendChart;
let forksTrendChart;
let changeFailureRateChart;
let reviewLatencyChart;
let prSizeChart;
let contributorFunnelChart;
let contributorMomentumChart;
let contributorProfileChart;
let containerActivityChart;
let healthActivitySparkline;
let repoMetaSparkline;
let searchItem;

// Tab state
var activeTab = 'health';
var tabLoaded = {};

const searchPrefixes = ['org', 'repo'];

function parseSearchInput(raw) {
    const match = raw.match(/^(org|repo):(.*)$/i);
    if (match) {
        return { scope: match[1].toLowerCase(), query: match[2].trimStart() };
    }
    return { scope: 'all', query: raw };
}

$(function () {
    // Prepend base path to all relative AJAX URLs for reverse proxy support.
    var basePath = $("#base_path").val() || "";
    if (basePath) {
        $.ajaxPrefilter(function (options) {
            if (options.url && options.url.charAt(0) === '/') {
                options.url = basePath + options.url;
            }
        });
    }

    $(window).resize(function () {
        const scrollWidth = $('.tbl-content').width() - $('.tbl-content table').width();
        $('.tbl-header').css({ 'padding-right': scrollWidth });
    });

    // Theme toggle
    $("#theme-toggle").click(function () {
        toggleTheme();
    });

    // Modal close handlers
    $("#modal-close-btn, #modal-ok-btn").click(function () {
        $("#error-modal").removeClass("open");
    });
    $("#error-modal").click(function (e) {
        if (e.target === this) {
            $(this).removeClass("open");
        }
    });

    $("#entity-popover-close").click(function () {
        $("#entity-popover").removeClass("open");
    });


    // Pagination — bind once
    $("#prev-page").click(function (e) {
        e.preventDefault();
        if (searchCriteria.page > 1) {
            searchCriteria.page--;
            submitSearch();
        }
    });
    $("#next-page").click(function (e) {
        e.preventDefault();
        searchCriteria.page++;
        submitSearch();
    });

    $("#logo-home").click(function (e) {
        e.preventDefault();
        $("#search-bar").val("");
        resetSearch();
        resetCharts();
        autocomplete_cache = {};
        leftChartExcludes = [];
        rightChartExcludes = [];
        $(".header-term").html("All imported events");
        var cleanURL = window.location.pathname + "#" + activeTab;
        history.pushState(null, "", cleanURL);
        updatePeriodOptions("", "", function () {
            loadAllCharts($("#period_months").val(), "", "", "");
        });
    });

    if ($("#search-bar").length) {
        searchCriteria.init();
        initUnifiedSearch();
        initSearchFilters();
        initPeriodSelector();
        initTabs();
        var params = new URLSearchParams(window.location.search);
        var paramOrg = params.get("o") || "";
        var paramRepo = params.get("r") || "";
        if (paramRepo && paramOrg) {
            $("#search-bar").val("repo:" + paramRepo);
            searchCriteria.org = paramOrg;
            history.replaceState({ scope: "repo", item: { value: paramRepo, type: "repo" } }, "");
            applySelection("repo", { value: paramRepo, type: "repo" }, true);
            return;
        } else if (paramOrg) {
            $("#search-bar").val("org:" + paramOrg);
            history.replaceState({ scope: "org", item: { value: paramOrg, type: "org" } }, "");
            applySelection("org", { value: paramOrg, type: "org" }, true);
            return;
        } else if (paramRepo) {
            $("#search-bar").val("repo:" + paramRepo);
            history.replaceState({ scope: "repo", item: { value: paramRepo, type: "repo" } }, "");
            applySelection("repo", { value: paramRepo, type: "repo" }, true);
            return;
        }

        history.replaceState(null, "");
        updatePeriodOptions("", "", function () {
            var months = $("#period_months").val();
            loadSummaryBanner(months, "", "", "");
            activateTab(activeTab);
        });
    }
});

function initTabs() {
    var hash = window.location.hash.replace('#', '');
    var validTabs = ['health', 'activity', 'velocity', 'quality', 'community', 'events'];
    if (validTabs.indexOf(hash) !== -1) {
        activeTab = hash;
    }

    $(".tab-btn").on("click", function () {
        var tab = $(this).data("tab");
        activateTab(tab);
        window.location.hash = tab;
    });

    $(window).on("hashchange", function () {
        var hash = window.location.hash.replace('#', '');
        if (hash && hash !== activeTab) {
            activateTab(hash);
        }
    });

    $(window).on("popstate", function (e) {
        var state = e.originalEvent.state;
        if (state && state.scope && state.item) {
            var scope = state.scope;
            var item = state.item;
            $("#search-bar").val(scope + ":" + item.value);
            applySelection(scope, item, true);
        } else {
            $("#search-bar").val("");
            resetSearch();
            resetCharts();
            autocomplete_cache = {};
            leftChartExcludes = [];
            rightChartExcludes = [];
            $(".header-term").html("All imported events");
            updatePeriodOptions("", "", function () {
                loadAllCharts($("#period_months").val(), "", "", "");
            });
        }
    });
}

function activateTab(tab) {
    activeTab = tab;

    $(".tab-btn").removeClass("active");
    $('.tab-btn[data-tab="' + tab + '"]').addClass("active");

    $(".tab-content").removeClass("active");
    $('.tab-content[data-tab="' + tab + '"]').addClass("active");

    if (!tabLoaded[tab]) {
        var months = $("#period_months").val();
        var org = searchCriteria.org || "";
        var repo = searchCriteria.repo || "";
        var entity = searchCriteria.entity || "";
        loadTabCharts(tab, months, org, repo, entity);
        tabLoaded[tab] = true;
    }
}

function loadSummaryBanner(months, org, repo, entity) {
    $.get('/data/insights/summary?m=' + months + '&o=' + org + '&r=' + repo + '&e=' + entity, function (data) {
        $("#banner-orgs").text(data.orgs.toLocaleString());
        $("#banner-repos").text(data.repos.toLocaleString());
        $("#banner-events").text(data.events.toLocaleString());
        $("#banner-contributors").text(data.contributors.toLocaleString());
        $("#banner-last-import").text(data.last_import || '—');
    });
}

function loadTabCharts(tab, months, org, repo, entity) {
    var q = 'm=' + months + '&o=' + org + '&r=' + repo + '&e=' + entity;
    switch (tab) {
        case 'health':
            loadInsightsSummary('/data/insights/summary?' + q);
            loadHealthActivitySparkline('/data/insights/daily-activity?' + q);
            loadRepoMeta('/data/insights/repo-meta?o=' + org + '&r=' + repo);
            if (repo) {
                $("#stars-trend-panel").show();
                $("#forks-trend-panel").show();
                $("#repo-overview-panel").hide();
                loadStarsTrendChart('/data/insights/repo-metric-history?' + q);
                loadForksTrendChart('/data/insights/repo-metric-history?' + q);
            } else {
                $("#stars-trend-panel").hide();
                $("#forks-trend-panel").hide();
                $("#repo-overview-panel").show();
                loadRepoOverview('/data/insights/repo-overview?' + q);
            }
            break;
        case 'activity':
            loadTimeSeriesChart('/data/type?' + q, onTimeSeriesChartSelect);
            loadPRSizeChart('/data/insights/pr-size?' + q);
            loadForksAndActivityChart('/data/insights/forks-and-activity?' + q);
            break;
        case 'velocity':
            loadVelocityChart('/data/insights/time-to-merge?' + q, 'time-to-merge-chart', 'timeToMerge');
            loadChangeFailureRateChart('/data/insights/change-failure-rate?' + q);
            loadReleaseCadenceChart('/data/insights/release-cadence?' + q);
            loadReleaseDownloadsChart('/data/insights/release-downloads?m=' + months + '&o=' + org + '&r=' + repo);
            loadReleaseDownloadsByTagChart('/data/insights/release-downloads-by-tag?m=' + months + '&o=' + org + '&r=' + repo);
            loadContainerActivityChart('/data/insights/container-activity?' + q);
            break;
        case 'quality':
            loadPRRatioChart('/data/insights/pr-ratio?' + q);
            loadReviewLatencyChart('/data/insights/review-latency?' + q);
            loadTimeToCloseChart('/data/insights/time-to-close?' + q, '/data/insights/time-to-restore?' + q);
            loadReputationChart('/data/insights/reputation?' + q);
            break;
        case 'community':
            loadRetentionChart('/data/insights/retention?' + q);
            loadContributorMomentumChart('/data/insights/contributor-momentum?' + q);
            loadContributorFunnelChart('/data/insights/contributor-funnel?' + q);
            (function() {
                var onLeftExclude = function () {
                    leftChart.destroy();
                    var x = leftChartExcludes.join("|");
                    loadLeftChart('/data/entity?' + q + '&x=' + x, onLeftChartSelect, onLeftExclude);
                };
                loadLeftChart('/data/entity?' + q, onLeftChartSelect, onLeftExclude);
            })();
            (function() {
                var onRightExclude = function () {
                    rightChart.destroy();
                    var x = rightChartExcludes.join("|");
                    loadRightChart('/data/developer?' + q + '&x=' + x, onRightChartSelect, onRightExclude);
                };
                loadRightChart('/data/developer?' + q, onRightChartSelect, onRightExclude);
            })();
            initContributorSearch(q);
            break;
        case 'events':
            break;
    }
}

function loadAllCharts(months, org, repo, entity) {
    tabLoaded = {};
    loadSummaryBanner(months, org, repo, entity);
    activateTab(activeTab);
}

function applySelection(scope, item, skipPushState) {
    resetSearch();
    autocomplete_cache = {};
    leftChartExcludes = [];
    rightChartExcludes = [];

    searchItem = item;
    $(".header-term").html(item.value);

    resetCharts();
    tabLoaded = {};

    const months = $("#period_months").val();
    let org = "", repo = "", entity = "";
    switch (scope) {
        case "org":
            org = item.value;
            searchCriteria.org = item.value;
            break;
        case "repo":
            repo = item.value;
            searchCriteria.repo = item.value;
            break;
        case "entity":
            entity = item.value;
            searchCriteria.entity = item.value;
            break;
    }

    if (!skipPushState) {
        var qs = new URLSearchParams(window.location.search);
        if (org) qs.set("o", org); else qs.delete("o");
        if (repo) {
            var parts = repo.split("/");
            if (parts.length === 2) { qs.set("o", parts[0]); qs.set("r", repo); }
            else { qs.set("r", repo); }
        } else { qs.delete("r"); }
        var newURL = window.location.pathname + (qs.toString() ? "?" + qs.toString() : "") + window.location.hash;
        history.pushState({ scope: scope, item: item }, "", newURL);
    }

    submitSearch();
    updatePeriodOptions(org, repo, function () {
        var m = $("#period_months").val();
        loadSummaryBanner(m, org, repo, entity);
        activateTab(activeTab);
    });
}

function initUnifiedSearch() {
    const sel = $("#search-bar");
    const dropdown = $("#ac-dropdown");
    let activeIndex = -1;
    let currentItems = [];
    let currentScope = 'org';

    function showDropdown(items) {
        currentItems = items || [];
        activeIndex = -1;
        dropdown.empty();
        if (currentItems.length === 0) {
            dropdown.removeClass("open");
            return;
        }
        $.each(currentItems, function (i, item) {
            const label = item.type ? `<span class="ac-type">${item.type}</span> ` : '';
            $(`<div class="ac-item">${label}${item.text}</div>`)
                .data("item", item)
                .on("mousedown", function (e) {
                    e.preventDefault();
                    selectItem(item);
                })
                .appendTo(dropdown);
        });
        dropdown.addClass("open");
    }

    function hideDropdown() {
        dropdown.removeClass("open").empty();
        currentItems = [];
        activeIndex = -1;
    }

    function selectItem(item) {
        const scope = item.type || currentScope;
        sel.val(`${scope}:${item.value}`);
        hideDropdown();
        applySelection(scope, item);
    }

    function setActive(index) {
        const items = dropdown.find(".ac-item");
        items.removeClass("active");
        if (index >= 0 && index < items.length) {
            activeIndex = index;
            $(items[index]).addClass("active");
            items[index].scrollIntoView({ block: "nearest" });
        }
    }

    sel.on("input", function () {
        const raw = $(this).val();
        if (raw.length < 1) {
            hideDropdown();
            resetSearch();
            resetCharts();
            autocomplete_cache = {};
            leftChartExcludes = [];
            rightChartExcludes = [];
            $(".header-term").html("All imported events");
            var cleanURL = window.location.pathname + window.location.hash;
            history.pushState(null, "", cleanURL);
            updatePeriodOptions("", "", function () {
                loadAllCharts($("#period_months").val(), "", "", "");
            });
            return false;
        }

        const parsed = parseSearchInput(raw);
        currentScope = parsed.scope;
        const q = parsed.query;

        if (q.length < 1) {
            hideDropdown();
            return false;
        }

        const cacheKey = currentScope + ":" + q;
        if (cacheKey in autocomplete_cache) {
            showDropdown(autocomplete_cache[cacheKey]);
            return false;
        }

        $.getJSON(`/data/query?v=${currentScope}&q=${encodeURIComponent(q)}`, function (data) {
            autocomplete_cache[cacheKey] = data;
            showDropdown(data);
        });
        return false;
    });

    sel.on("keydown", function (e) {
        if (!dropdown.hasClass("open")) return;

        switch (e.key) {
            case "ArrowDown":
                e.preventDefault();
                setActive(Math.min(activeIndex + 1, currentItems.length - 1));
                break;
            case "ArrowUp":
                e.preventDefault();
                setActive(Math.max(activeIndex - 1, 0));
                break;
            case "Enter":
                e.preventDefault();
                if (activeIndex >= 0 && activeIndex < currentItems.length) {
                    selectItem(currentItems[activeIndex]);
                }
                break;
            case "Escape":
                hideDropdown();
                break;
        }
    });

    sel.on("blur", function () {
        setTimeout(hideDropdown, 150);
    });
}

function resetSearch() {
    searchItem = null;
    $("#result-table-content").empty();
    $("#search-results-wrap").hide();
    searchCriteria.reset();
    clearFilterInputs();
    $("#bus-factor-val").text("—");
    $("#pony-factor-val").text("—");
    $("#repo-meta-container").empty().html('<span class="insight-label">No metadata imported yet</span>');
    $("#entity-popover").removeClass("open");
    $("#reputation-popover").removeClass("open");
}

function resetCharts() {
    if (timeEventsChart) {
        timeEventsChart.destroy();
    }
    if (leftChart) {
        leftChart.destroy();
    }
    if (rightChart) {
        rightChart.destroy();
    }
    if (retentionChart) {
        retentionChart.destroy();
    }
    if (prRatioChart) {
        prRatioChart.destroy();
    }
    if (timeToMergeChart) {
        timeToMergeChart.destroy();
    }
    if (timeToCloseChart) {
        timeToCloseChart.destroy();
    }
    if (releaseCadenceChart) {
        releaseCadenceChart.destroy();
    }
    if (releaseDownloadsChart) {
        releaseDownloadsChart.destroy();
    }
    if (releaseDownloadsByTagChart) {
        releaseDownloadsByTagChart.destroy();
    }
    if (containerActivityChart) {
        containerActivityChart.destroy();
    }
    if (reputationChart) {
        reputationChart.destroy();
    }
    if (forksAndActivityChart) {
        forksAndActivityChart.destroy();
    }
    if (changeFailureRateChart) {
        changeFailureRateChart.destroy();
    }
    if (reviewLatencyChart) {
        reviewLatencyChart.destroy();
    }
    if (prSizeChart) {
        prSizeChart.destroy();
    }
    if (contributorFunnelChart) {
        contributorFunnelChart.destroy();
    }
    if (contributorMomentumChart) {
        contributorMomentumChart.destroy();
    }
    if (contributorProfileChart) {
        contributorProfileChart.destroy();
        contributorProfileChart = null;
    }
    $("#contributor-score").text('');
    if (healthActivitySparkline) {
        healthActivitySparkline.destroy();
    }
    if (repoMetaSparkline) {
        repoMetaSparkline.destroy();
    }
    if (starsTrendChart) {
        starsTrendChart.destroy();
    }
    if (forksTrendChart) {
        forksTrendChart.destroy();
    }
}

function onTimeSeriesChartSelect(label, val) {
    searchCriteria.from = label + "-01";
    searchCriteria.to = label + "-31";
    if (val != "Total" && val != "Trend") {
        searchCriteria.type = val;
    }
    syncFiltersToInputs();
    submitSearch();
}

function onLeftChartSelect(label) {
    searchCriteria.entity = label;
    syncFiltersToInputs();
    submitSearch();
    showEntityDevelopers(label);
}

function onRightChartSelect(label) {
    window.open('https://github.com/' + encodeURIComponent(label), '_blank');
}

function submitSearch() {
    $("#tbl-criteria").html(searchCriteria.String());
    const table = $("#result-table-content").empty();
    const criteria = JSON.stringify(searchCriteria);

    $.post("/data/search", criteria).done(function (data) {
        displaySearchResults(table, data);
    }).fail(function (response) {
        handleResponseError(response);
    });
    return false;
}

function parseOptional(val, prefix) {
    if (val) {
        if (prefix) {
            return prefix + val;
        }
        return val;
    }
    return "";
}

function displaySearchResults(table, data) {
    $("#page-number").html(searchCriteria.page);
    table.empty();
    if (data.length == 0) {
        table.append("<tr><td colspan='5'>No results found.</td></tr>");
        $("#search-results-wrap").show();
        return;
    }
    $.each(data, function (key, item) {
        $("<tr>")
            .append(`<td>${item.event.date}</td>`)
            .append(`<td><a href="https://github.com/${item.event.org}/${item.event.repo}" class="link"
                target="_blank">${item.event.org}/${item.event.repo}</a></td>`)
            .append(`<td><a href="${item.event.url}" class="link"
                target="_blank">${item.event.type}</a></td>`)
            .append(`<td><a href="https://github.com/${item.developer.username}" class="link"
                target="_blank">${item.developer.username}</a> ${parseOptional(item.developer.full_name, " - ")}</td>`)
            .append(`<td>${parseOptional(item.developer.entity)}</td>`)
            .appendTo(table);
    });
    $("#search-results-wrap").show();
    return false;
}

function handleResponseError(response) {
    console.log(response);
    if (response.status == 400) {
        if (response.responseJSON && response.responseJSON.message) {
            showErrorModal(response.responseJSON.message);
            return false;
        }
        showErrorModal("Bad request, please check your input.");
        return false;
    }
    showErrorModal("Server error, please try again later.");
    return false;
}

function showErrorModal(message) {
    $("#error-modal-body p").html(message);
    $("#error-modal").addClass("open");
}

function loadTimeSeriesChart(url, fn) {
    $.get(url, function (data) {
        if (timeEventsChart) timeEventsChart.destroy();
        timeEventsChart = new Chart($("#time-series-chart")[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: data.dates,
                datasets: [{
                    label: 'PR',
                    data: data.pr,
                    backgroundColor: colors[0],
                    borderWidth: 1,
                    order: 2
                }, {
                    label: 'PR-Review',
                    data: data.pr_review,
                    backgroundColor: colors[1],
                    borderWidth: 1,
                    order: 3
                }, {
                    label: 'Issue',
                    data: data.issue,
                    backgroundColor: colors[2],
                    borderWidth: 1,
                    order: 4
                }, {
                    label: 'Issue-Comment',
                    data: data.issue_comment,
                    backgroundColor: colors[3],
                    borderWidth: 1,
                    order: 5
                }, {
                    label: 'Fork',
                    data: data.fork,
                    backgroundColor: colors[4],
                    borderWidth: 1,
                    order: 6
                },{
                    label: 'Total',
                    type: 'line',
                    fill: false,
                    data: data.total,
                    borderColor: colors[5],
                    order: 1,
                    borderWidth: 2,
                    pointRadius: 3,
                    tension: 0.2
                },{
                    label: 'Trend',
                    type: 'line',
                    fill: false,
                    data: data.trend,
                    borderColor: colors[3],
                    borderDash: [6, 3],
                    order: 0,
                    borderWidth: 3,
                    pointRadius: 0,
                    tension: 0.4
                }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        display: true
                    }
                },
                scales: {
                    y:
                    {
                        beginAtZero: true,
                        ticks: {
                            precision: 0,
                            font: {
                                size: 14
                            }
                        }
                    }
                    ,
                    x:
                    {
                        ticks: {
                            font: {
                                size: 14
                            }
                        }
                    }
                },
                animations: {
                    tension: {
                        duration: 1000,
                        easing: 'linear',
                        from: 1,
                        to: 0,
                        loop: false
                    }
                },
                onClick: (evt, item) => {
                    if (item.length) {
                        const label = timeEventsChart.data.labels[item[0].index];
                        const val = timeEventsChart.data.datasets[item[0].datasetIndex].label;
                        if (fn) {
                            fn(label, val);
                        }
                        return false;
                    }
                    return false;
                }
            }
        });
    });
}

function loadLeftChart(url, fn, cb) {
    const onLickHandler = function(e, legendItem) {
        leftChartExcludes.push(legendItem.text);
        cb();
    }

    $.get(url, function (data) {
        if (leftChart) leftChart.destroy();
        leftChart = new Chart($("#left-chart")[0].getContext("2d"), {
            type: 'polarArea',
            data: {
                labels: data.labels,
                datasets: [{
                    label: 'Entities',
                    data: data.data,
                    backgroundColor: colors,
                    hoverOffset: 4
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        position: 'right',
                        onClick: onLickHandler
                    }
                },
                animations: {
                    tension: {
                        duration: 1000,
                        easing: 'linear',
                        from: 1,
                        to: 0,
                        loop: false
                    }
                },
                onClick: (evt, item) => {
                    if (item.length) {
                        const label = leftChart.data.labels[item[0].index];
                        if (fn) {
                            fn(label);
                        }
                        return false;
                    }
                    return false;
                }
            }
        });
    });
}

function loadRightChart(url, fn, cb) {
    const onRightHandler = function(e, legendItem) {
        rightChartExcludes.push(legendItem.text);
        cb();
    }

    $.get(url, function (data) {
        if (rightChart) rightChart.destroy();
        rightChart = new Chart($("#right-chart")[0].getContext("2d"), {
            type: 'pie',
            data: {
                labels: data.labels,
                datasets: [{
                    label: 'Repositories',
                    data: data.data,
                    backgroundColor: colors,
                    hoverOffset: 4
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        position: 'right',
                        onClick: onRightHandler
                    }
                },
                animations: {
                    tension: {
                        duration: 1000,
                        easing: 'linear',
                        from: 1,
                        to: 0,
                        loop: false
                    }
                },
                onHover: (evt, item) => {
                    evt.native.target.style.cursor = item.length ? 'pointer' : 'default';
                },
                onClick: (evt, item) => {
                    if (item.length) {
                        const label = rightChart.data.labels[item[0].index];
                        if (fn) {
                            fn(label);
                        }
                    }
                }
            }
        });
    });
}

function loadInsightsSummary(url) {
    $.get(url, function (data) {
        $("#bus-factor-val").text(data.bus_factor);
        $("#pony-factor-val").text(data.pony_factor);
    });
}

function loadHealthActivitySparkline(url) {
    $.get(url, function (data) {
        if (!data || !data.dates || data.dates.length === 0) return;
        if (healthActivitySparkline) healthActivitySparkline.destroy();
        healthActivitySparkline = new Chart($("#health-activity-sparkline")[0].getContext("2d"), {
            type: 'line',
            data: {
                labels: data.dates,
                datasets: [{
                    label: 'Events',
                    data: data.counts,
                    borderColor: colors[0],
                    backgroundColor: colors[0] + '33',
                    fill: true,
                    tension: 0.3,
                    pointRadius: 0,
                    borderWidth: 1.5
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: false }
                },
                scales: {
                    x: { display: false },
                    y: { display: false }
                }
            }
        });
    });
}

function loadRetentionChart(url) {
    $.get(url, function (data) {
        if (retentionChart) retentionChart.destroy();
        retentionChart = new Chart($("#retention-chart")[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: data.months,
                datasets: [{
                    label: 'New',
                    data: data.new,
                    backgroundColor: colors[1],
                    borderWidth: 1
                }, {
                    label: 'Returning',
                    data: data.returning,
                    backgroundColor: colors[0],
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: true }
                },
                scales: {
                    x: { stacked: true, ticks: { font: { size: 14 } } },
                    y: { stacked: true, beginAtZero: true, ticks: { precision: 0, font: { size: 14 } } }
                }
            }
        });
    });
}

function loadPRRatioChart(url) {
    $.get(url, function (data) {
        if (prRatioChart) prRatioChart.destroy();
        prRatioChart = new Chart($("#pr-ratio-chart")[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: data.months,
                datasets: [{
                    label: 'PRs',
                    data: data.prs,
                    backgroundColor: colors[0],
                    borderWidth: 1,
                    yAxisID: 'y',
                    order: 2
                }, {
                    label: 'Reviews',
                    data: data.reviews,
                    backgroundColor: colors[1],
                    borderWidth: 1,
                    yAxisID: 'y',
                    order: 3
                }, {
                    label: 'Ratio',
                    type: 'line',
                    data: data.ratio,
                    borderColor: colors[3],
                    borderWidth: 3,
                    fill: false,
                    yAxisID: 'y1',
                    order: 1,
                    tension: 0.3
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: true }
                },
                scales: {
                    x: { ticks: { font: { size: 14 } } },
                    y: { beginAtZero: true, position: 'left', ticks: { precision: 0, font: { size: 14 } } },
                    y1: { beginAtZero: true, position: 'right', grid: { drawOnChartArea: false }, ticks: { font: { size: 14 } } }
                }
            }
        });
    });
}

function loadTimeToCloseChart(closeURL, restoreURL) {
    $.when($.get(closeURL), $.get(restoreURL)).done(function (closeRes, restoreRes) {
        if (timeToCloseChart) timeToCloseChart.destroy();
        const close = closeRes[0];
        const restore = restoreRes[0];
        timeToCloseChart = new Chart($("#time-to-close-chart")[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: close.months,
                datasets: [{
                    label: 'All Issues',
                    data: close.avg_days,
                    backgroundColor: colors[2],
                    borderWidth: 1,
                    order: 2
                }, {
                    label: 'Bug (near release)',
                    data: restore.avg_days,
                    backgroundColor: colors[3],
                    borderWidth: 1,
                    order: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: true } },
                scales: {
                    x: { ticks: { font: { size: 14 } } },
                    y: { beginAtZero: true, ticks: { font: { size: 14 } },
                        title: { display: true, text: 'Avg Days' } }
                }
            }
        });
    });
}

function loadVelocityChart(url, canvasId, key) {
    $.get(url, function (data) {
        if (timeToMergeChart) timeToMergeChart.destroy();
        const chart = new Chart($(`#${canvasId}`)[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: data.months,
                datasets: [{
                    label: 'Avg Days',
                    data: data.avg_days,
                    backgroundColor: colors[2],
                    borderWidth: 1,
                    yAxisID: 'y',
                    order: 2
                }, {
                    label: 'Count',
                    type: 'line',
                    data: data.count,
                    borderColor: colors[5],
                    borderWidth: 3,
                    fill: false,
                    yAxisID: 'y1',
                    order: 1,
                    tension: 0.3
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: true }
                },
                scales: {
                    x: { ticks: { font: { size: 14 } } },
                    y: { beginAtZero: true, position: 'left', ticks: { font: { size: 14 } },
                        title: { display: true, text: 'Avg Days' } },
                    y1: { beginAtZero: true, position: 'right', grid: { drawOnChartArea: false },
                        ticks: { precision: 0, font: { size: 14 } },
                        title: { display: true, text: 'Count' } }
                }
            }
        });
        if (key === 'timeToMerge') { timeToMergeChart = chart; }
        if (key === 'timeToClose') { timeToCloseChart = chart; }
    });
}

function loadForksAndActivityChart(url) {
    $.get(url, function (data) {
        if (forksAndActivityChart) forksAndActivityChart.destroy();
        forksAndActivityChart = new Chart($("#forks-activity-chart")[0].getContext("2d"), {
            type: 'line',
            data: {
                labels: data.months,
                datasets: [{
                    label: 'Forks',
                    data: data.forks,
                    borderColor: colors[0],
                    backgroundColor: colors[0] + '20',
                    borderWidth: 3,
                    fill: true,
                    yAxisID: 'y',
                    tension: 0.3
                }, {
                    label: 'Events',
                    data: data.events,
                    borderColor: colors[3],
                    borderWidth: 3,
                    fill: false,
                    yAxisID: 'y1',
                    tension: 0.3
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: true }
                },
                scales: {
                    x: { ticks: { font: { size: 14 } } },
                    y: { beginAtZero: true, position: 'left', ticks: { precision: 0, font: { size: 14 } },
                        title: { display: true, text: 'Forks' } },
                    y1: { beginAtZero: true, position: 'right', grid: { drawOnChartArea: false },
                        ticks: { precision: 0, font: { size: 14 } },
                        title: { display: true, text: 'Events' } }
                }
            }
        });
    });
}

function loadRepoMeta(url) {
    $.get(url, function (data) {
        const container = $("#repo-meta-container");
        container.empty();
        if (!data || data.length === 0) {
            container.html('<span class="insight-label">No metadata imported yet</span>');
            return;
        }
        let stars = 0, forks = 0, issues = 0, archived = 0;
        let langs = {}, licenses = {};
        $.each(data, function (i, m) {
            stars += m.stars;
            forks += m.forks;
            issues += m.open_issues;
            if (m.archived) { archived++; }
            if (m.language) { langs[m.language] = (langs[m.language] || 0) + 1; }
            if (m.license) { licenses[m.license] = (licenses[m.license] || 0) + 1; }
        });
        const topLang = Object.keys(langs).sort((a, b) => langs[b] - langs[a])[0] || '—';
        const topLicense = Object.keys(licenses).sort((a, b) => licenses[b] - licenses[a])[0] || '—';

        // Row 1: numeric stats, Row 2: text/categorical stats
        const items = [
            { label: 'Stars', val: stars.toLocaleString() },
            { label: 'Forks', val: forks.toLocaleString() },
            { label: 'Open Issues', val: issues.toLocaleString() },
            { label: 'Language', val: topLang },
            { label: 'License', val: topLicense },
            { label: 'Repos', val: data.length + (archived > 0 ? ` (${archived} archived)` : '') }
        ];
        $.each(items, function (i, item) {
            $('<div class="insight-card">')
                .append(`<span class="insight-label">${item.label}</span>`)
                .append(`<span class="insight-val">${item.val}</span>`)
                .appendTo(container);
        });

        // Sparkline for stars/forks trend.
        var mhParams = 'm=' + ($("#period_months").val() || '6') + '&o=' + (new URLSearchParams(url.split('?')[1]).get('o') || '') + '&r=' + (new URLSearchParams(url.split('?')[1]).get('r') || '');
        $.get('/data/insights/repo-metric-history?' + mhParams, function (hist) {
            if (!hist || hist.length === 0) return;
            var sLabels = [], sStars = [], sForks = [];
            $.each(hist, function (i, d) {
                sLabels.push(d.date);
                sStars.push(d.stars);
                sForks.push(d.forks);
            });
            if (repoMetaSparkline) repoMetaSparkline.destroy();
            repoMetaSparkline = new Chart($("#repo-meta-sparkline")[0].getContext("2d"), {
                type: 'line',
                data: {
                    labels: sLabels,
                    datasets: [
                        {
                            label: 'Stars',
                            data: sStars,
                            borderColor: colors[0],
                            borderWidth: 1.5,
                            pointRadius: 0,
                            tension: 0.3,
                            fill: false
                        },
                        {
                            label: 'Forks',
                            data: sForks,
                            borderColor: colors[1],
                            borderWidth: 1.5,
                            pointRadius: 0,
                            tension: 0.3,
                            fill: false
                        }
                    ]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: { display: true, position: 'bottom', labels: { boxWidth: 10, font: { size: 10 } } }
                    },
                    scales: {
                        x: { display: false },
                        y: { display: false }
                    }
                }
            });
        });
    });
}

function loadStarsTrendChart(url) {
    $.get(url, function (data) {
        if (!data || data.length === 0) return;
        var labels = [];
        var stars = [];
        $.each(data, function (i, d) {
            labels.push(d.date);
            stars.push(d.stars);
        });
        $("#stars-trend-desc").text("Daily star count from " + labels[0] + " to " + labels[labels.length - 1] + ".");
        if (starsTrendChart) starsTrendChart.destroy();
        starsTrendChart = new Chart($("#stars-trend-chart")[0].getContext("2d"), {
            type: 'line',
            data: {
                labels: labels,
                datasets: [{
                    label: 'Stars',
                    data: stars,
                    borderColor: colors[0],
                    backgroundColor: colors[0] + '33',
                    fill: true,
                    tension: 0.3,
                    pointRadius: 2
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: false } },
                scales: {
                    y: { beginAtZero: false, ticks: { precision: 0 } }
                }
            }
        });
    });
}

function loadForksTrendChart(url) {
    $.get(url, function (data) {
        if (!data || data.length === 0) return;
        var labels = [];
        var forks = [];
        $.each(data, function (i, d) {
            labels.push(d.date);
            forks.push(d.forks);
        });
        $("#forks-trend-desc").text("Daily fork count from " + labels[0] + " to " + labels[labels.length - 1] + ".");
        if (forksTrendChart) forksTrendChart.destroy();
        forksTrendChart = new Chart($("#forks-trend-chart")[0].getContext("2d"), {
            type: 'line',
            data: {
                labels: labels,
                datasets: [{
                    label: 'Forks',
                    data: forks,
                    borderColor: colors[1],
                    backgroundColor: colors[1] + '33',
                    fill: true,
                    tension: 0.3,
                    pointRadius: 2
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: false } },
                scales: {
                    y: { beginAtZero: false, ticks: { precision: 0 } }
                }
            }
        });
    });
}

function loadRepoOverview(url) {
    $.get(url, function (data) {
        var $tbody = $("#repo-overview-table tbody");
        $tbody.empty();
        if (!data || data.length === 0) {
            $tbody.append('<tr><td colspan="10" style="text-align:center">No repository data available</td></tr>');
            return;
        }
        $.each(data, function (i, r) {
            var name = r.org + '/' + r.repo;
            var $link = $('<a href="#"></a>').text(name).on('click', function (e) {
                e.preventDefault();
                applySelection('repo', { value: name, label: name });
            });
            var $row = $('<tr></tr>');
            $row.append($('<td></td>').append($link));
            $row.append($('<td class="num"></td>').text(r.stars.toLocaleString()));
            $row.append($('<td class="num"></td>').text(r.forks.toLocaleString()));
            $row.append($('<td class="num"></td>').text(r.open_issues.toLocaleString()));
            $row.append($('<td class="num"></td>').text(r.events.toLocaleString()));
            $row.append($('<td class="num"></td>').text(r.contributors.toLocaleString()));
            $row.append($('<td class="num"></td>').text(r.scored + '/' + r.contributors));
            $row.append($('<td></td>').text(r.language || '—'));
            $row.append($('<td></td>').text(r.license || '—'));
            $row.append($('<td></td>').text(r.last_import || '—'));
            $tbody.append($row);
        });
    });
}

function loadReleaseCadenceChart(url) {
    $.get(url, function (data) {
        if (releaseCadenceChart) releaseCadenceChart.destroy();
        releaseCadenceChart = new Chart($("#release-cadence-chart")[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: data.months,
                datasets: [{
                    label: 'Total',
                    data: data.total,
                    backgroundColor: colors[4],
                    borderWidth: 1
                }, {
                    label: 'Stable',
                    data: data.stable,
                    backgroundColor: colors[1],
                    borderWidth: 1
                }, {
                    label: 'Deployments',
                    type: 'line',
                    data: data.deployments,
                    borderColor: colors[3],
                    borderWidth: 3,
                    borderDash: [5, 5],
                    fill: false,
                    tension: 0.3,
                    order: 0
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: true }
                },
                scales: {
                    x: { ticks: { font: { size: 14 } } },
                    y: { beginAtZero: true, ticks: { precision: 0, font: { size: 14 } } }
                }
            }
        });
    });
}

function loadReleaseDownloadsChart(url) {
    $.get(url, function (data) {
        if (releaseDownloadsChart) releaseDownloadsChart.destroy();
        releaseDownloadsChart = new Chart($("#release-downloads-chart")[0].getContext("2d"), {
            type: 'line',
            data: {
                labels: data.months,
                datasets: [{
                    label: 'Downloads',
                    data: data.downloads,
                    borderColor: colors[0],
                    backgroundColor: colors[0] + '20',
                    borderWidth: 3,
                    fill: true,
                    tension: 0.3
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: true }
                },
                scales: {
                    x: { ticks: { font: { size: 14 } } },
                    y: { beginAtZero: true, ticks: { precision: 0, font: { size: 14 } },
                        title: { display: true, text: 'Downloads' } }
                }
            }
        });
    });
}

function loadReleaseDownloadsByTagChart(url) {
    $.get(url, function (data) {
        if (releaseDownloadsByTagChart) releaseDownloadsByTagChart.destroy();
        releaseDownloadsByTagChart = new Chart($("#release-downloads-by-tag-chart")[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: data.tags,
                datasets: [{
                    label: 'Downloads',
                    data: data.downloads,
                    backgroundColor: colors[0] + '80',
                    borderColor: colors[0],
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                indexAxis: 'y',
                plugins: {
                    legend: { display: false }
                },
                scales: {
                    x: {
                        beginAtZero: true,
                        ticks: { precision: 0, font: { size: 14 } },
                        title: { display: true, text: 'Downloads' }
                    },
                    y: { ticks: { font: { size: 14 } } }
                }
            }
        });
    });
}

function loadContainerActivityChart(url) {
    $.get(url, function (data) {
        if (!data.months || data.months.length === 0) {
            $("#container-activity-chart").closest(".tbl").find(".insight-desc")
                .text("No container images published via GitHub Packages (ghcr.io) for this scope.");
            return;
        }
        if (containerActivityChart) containerActivityChart.destroy();
        containerActivityChart = new Chart($("#container-activity-chart")[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: data.months,
                datasets: [{
                    label: 'Versions Published',
                    data: data.versions,
                    backgroundColor: colors[4] + '80',
                    borderColor: colors[4],
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: false }
                },
                scales: {
                    y: {
                        beginAtZero: true,
                        ticks: { precision: 0 },
                        title: { display: true, text: 'Versions' }
                    }
                }
            }
        });
    });
}

function reputationBarColors(values) {
    return values.map(function (v) {
        if (v >= 0.7) return '#2da44e';
        if (v >= 0.4) return '#bf8700';
        return '#cf222e';
    });
}

function loadReputationChart(url) {
    $.get(url, function (data) {
        if (data.total > 0) {
            $("#reputation-counts").text('Scored: ' + data.scored + ' / Total: ' + data.total);
        } else {
            $("#reputation-counts").text('');
        }
        if (reputationChart) reputationChart.destroy();
        if (!data.labels || data.labels.length === 0) {
            return;
        }
        reputationChart = new Chart($("#reputation-chart")[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: data.labels,
                datasets: [{
                    label: 'Score',
                    data: data.data,
                    backgroundColor: reputationBarColors(data.data),
                    borderWidth: 0,
                    barPercentage: 0.6,
                    categoryPercentage: 0.8
                }]
            },
            options: {
                indexAxis: 'y',
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: false }
                },
                scales: {
                    x: {
                        beginAtZero: true,
                        max: 1.0,
                        ticks: { font: { size: 14 } }
                    },
                    y: {
                        ticks: { font: { size: 14 } },
                        afterFit: (axis) => { axis.paddingTop = 4; axis.paddingBottom = 4; }
                    }
                },
                onHover: (evt, item) => {
                    evt.native.target.style.cursor = item.length ? 'pointer' : 'default';
                },
                onClick: (evt, item) => {
                    if (item.length) {
                        const username = reputationChart.data.labels[item[0].index];
                        window.open('https://github.com/' + encodeURIComponent(username), '_blank');
                    }
                }
            }
        });
    });
}


function syncFiltersToInputs() {
    $("#filter-type").val(searchCriteria.type || "");
    $("#filter-from").val(searchCriteria.from || "");
    $("#filter-to").val(searchCriteria.to || "");
    $("#filter-user").val(searchCriteria.user || "");
    $("#filter-entity").val(searchCriteria.entity || "");
}

function clearFilterInputs() {
    $("#filter-type").val("");
    $("#filter-from").val("");
    $("#filter-to").val("");
    $("#filter-user").val("");
    $("#filter-entity").val("");
}

function initSearchFilters() {
    $("#search-filters").on("submit", function (e) {
        e.preventDefault();
        const t = $("#filter-type").val();
        const from = $("#filter-from").val();
        const to = $("#filter-to").val();
        const user = $("#filter-user").val().trim();
        const entity = $("#filter-entity").val().trim();

        searchCriteria.type = t || null;
        searchCriteria.from = from || null;
        searchCriteria.to = to || null;
        searchCriteria.user = user || null;
        searchCriteria.entity = entity || null;
        searchCriteria.page = 1;

        submitSearch();
    });

    $("#filter-clear").click(function () {
        searchCriteria.type = null;
        searchCriteria.from = null;
        searchCriteria.to = null;
        searchCriteria.user = null;
        searchCriteria.entity = null;
        searchCriteria.page = 1;
        clearFilterInputs();
        $("#result-table-content").empty();
        $("#search-results-wrap").hide();
    });
}

function initPeriodSelector() {
    $("#period-select").on("change", function () {
        const months = $(this).val();
        $("#period_months").val(months);
        resetCharts();

        let org = "", repo = "", entity = "";
        if (searchItem) {
            const scope = ($("#search-bar").val().match(/^(org|repo|entity):/i) || [])[1] || "org";
            switch (scope.toLowerCase()) {
                case "org": org = searchItem.value; break;
                case "repo": repo = searchItem.value; break;
                case "entity": entity = searchItem.value; break;
            }
        }
        loadAllCharts(months, org, repo, entity);
    });
}

function updatePeriodOptions(org, repo, cb) {
    let url = "/data/min-date";
    const params = [];
    if (org) params.push("o=" + encodeURIComponent(org));
    if (repo) params.push("r=" + encodeURIComponent(repo));
    if (params.length) url += "?" + params.join("&");

    const defaultMonths = parseInt($("#default_months").val(), 10) || 6;

    $.get(url, function (data) {
        const sel = $("#period-select");
        const currentVal = parseInt($("#period_months").val(), 10) || defaultMonths;
        sel.empty();

        let maxMonths = defaultMonths;
        if (data.min_date) {
            const minDate = new Date(data.min_date);
            const now = new Date();
            maxMonths = Math.max(1,
                (now.getFullYear() - minDate.getFullYear()) * 12 +
                (now.getMonth() - minDate.getMonth()) + 1
            );
        }

        const steps = [3, 6, 9, 12, 18, 24, 36, 48, 60];
        const options = [];
        for (let i = 0; i < steps.length; i++) {
            if (steps[i] <= maxMonths) {
                options.push(steps[i]);
            }
        }
        if (options.length === 0 || options[options.length - 1] < maxMonths) {
            options.push(maxMonths);
        }

        $.each(options, function (i, m) {
            sel.append(`<option value="${m}">${m} months</option>`);
        });

        // keep current selection if still valid, otherwise use default or max
        if (options.indexOf(currentVal) >= 0) {
            sel.val(currentVal);
        } else if (options.indexOf(defaultMonths) >= 0) {
            sel.val(defaultMonths);
        } else {
            sel.val(options[options.length - 1]);
        }
        $("#period_months").val(sel.val());

        if (cb) cb();
    });
}

function showEntityDevelopers(entity) {
    const popover = $("#entity-popover");
    const list = $("#entity-popover-list");
    const title = $("#entity-popover-title");

    title.text(entity);
    list.empty().append('<li>Loading...</li>');
    popover.addClass("open");

    $.get(`/data/entity/developers?e=${encodeURIComponent(entity)}`, function (data) {
        list.empty();
        if (!data.developers || data.developers.length === 0) {
            list.append('<li>No contributors found</li>');
            return;
        }
        $.each(data.developers, function (i, dev) {
            list.append(`<li><a href="https://github.com/${dev.username}" target="_blank">${dev.username}</a></li>`);
        });
        const escaped = entity.replace(/'/g, "\\'");
        list.append(`<li class="entity-popover-hint">Wrong affiliation? Fix locally:<br><code>devpulse import substitutions --type entity --old '${escaped}' --new 'CORRECT'</code><br>Or update the source: <a href="https://github.com/cncf/gitdm" target="_blank">cncf/gitdm</a></li>`);
    }).fail(function () {
        list.empty().append('<li>Failed to load contributors</li>');
    });
}

function loadReviewLatencyChart(url) {
    $.get(url, function (data) {
        if (reviewLatencyChart) reviewLatencyChart.destroy();
        reviewLatencyChart = new Chart($("#review-latency-chart")[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: data.months,
                datasets: [{
                    label: 'Avg Hours',
                    data: data.avg_hours,
                    backgroundColor: colors[2],
                    borderWidth: 1,
                    yAxisID: 'y',
                    order: 2
                }, {
                    label: 'Count',
                    type: 'line',
                    data: data.count,
                    borderColor: colors[5],
                    borderWidth: 3,
                    fill: false,
                    yAxisID: 'y1',
                    order: 1,
                    tension: 0.3
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: true } },
                scales: {
                    x: { ticks: { font: { size: 14 } } },
                    y: { beginAtZero: true, position: 'left', ticks: { font: { size: 14 } },
                        title: { display: true, text: 'Avg Hours' } },
                    y1: { beginAtZero: true, position: 'right', grid: { drawOnChartArea: false },
                        ticks: { precision: 0, font: { size: 14 } },
                        title: { display: true, text: 'Count' } }
                }
            }
        });
    });
}

function loadChangeFailureRateChart(url) {
    $.get(url, function (data) {
        if (changeFailureRateChart) changeFailureRateChart.destroy();
        changeFailureRateChart = new Chart($("#change-failure-rate-chart")[0].getContext("2d"), {
            type: 'line',
            data: {
                labels: data.months,
                datasets: [{
                    label: 'Failure Rate %',
                    data: data.rate,
                    borderColor: colors[3],
                    backgroundColor: colors[3] + '33',
                    fill: true,
                    tension: 0.3,
                    pointRadius: 3
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: true } },
                scales: {
                    x: { ticks: { font: { size: 14 } } },
                    y: { beginAtZero: true, ticks: { font: { size: 14 },
                        callback: function(v) { return v + '%'; } },
                        title: { display: true, text: 'Failure Rate' } }
                }
            }
        });
    });
}

function loadPRSizeChart(url) {
    $.get(url, function (data) {
        if (!data.months || data.months.length === 0) {
            return;
        }
        if (prSizeChart) prSizeChart.destroy();
        prSizeChart = new Chart($("#pr-size-chart")[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: data.months,
                datasets: [{
                    label: 'S (<50)',
                    data: data.small,
                    backgroundColor: colors[1]
                }, {
                    label: 'M (50-250)',
                    data: data.medium,
                    backgroundColor: colors[0]
                }, {
                    label: 'L (250-1K)',
                    data: data.large,
                    backgroundColor: colors[2]
                }, {
                    label: 'XL (>1K)',
                    data: data.xlarge,
                    backgroundColor: colors[3]
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: true } },
                scales: {
                    x: { stacked: true, ticks: { font: { size: 14 } } },
                    y: { stacked: true, beginAtZero: true, ticks: { precision: 0, font: { size: 14 } } }
                }
            }
        });
    });
}

function loadContributorFunnelChart(url) {
    $.get(url, function (data) {
        if (contributorFunnelChart) contributorFunnelChart.destroy();
        contributorFunnelChart = new Chart($("#contributor-funnel-chart")[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: data.months,
                datasets: [{
                    label: 'First Comment',
                    data: data.first_comment,
                    backgroundColor: colors[0]
                }, {
                    label: 'First PR',
                    data: data.first_pr,
                    backgroundColor: colors[1]
                }, {
                    label: 'First Merge',
                    data: data.first_merge,
                    backgroundColor: colors[4]
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: true } },
                scales: {
                    x: { ticks: { font: { size: 14 } } },
                    y: { beginAtZero: true, ticks: { precision: 0, font: { size: 14 } } }
                }
            }
        });
    });
}

function loadContributorProfileChart(url) {
    $.get(url, function (data) {
        if (contributorProfileChart) contributorProfileChart.destroy();
        if (data && data.reputation != null) {
            $("#contributor-score").text('Reputation Score: ' + data.reputation.toFixed(2));
        } else {
            $("#contributor-score").text('');
        }
        if (!data || !data.metrics) return;

        var barColor = colors[0];
        var avgColor = barColor.replace('rgb', 'rgba').replace(')', ', 0.35)');
        if (barColor.startsWith('#')) {
            var r = parseInt(barColor.slice(1,3), 16);
            var g = parseInt(barColor.slice(3,5), 16);
            var b = parseInt(barColor.slice(5,7), 16);
            avgColor = 'rgba(' + r + ',' + g + ',' + b + ',0.35)';
            barColor = 'rgba(' + r + ',' + g + ',' + b + ',1)';
        }

        contributorProfileChart = new Chart($("#contributor-profile-chart")[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: data.metrics,
                datasets: [
                    {
                        label: 'Contributor',
                        data: data.values,
                        backgroundColor: barColor,
                        borderRadius: 3
                    },
                    {
                        label: 'Average',
                        data: data.averages.map(function(v) { return Math.round(v * 10) / 10; }),
                        backgroundColor: avgColor,
                        borderRadius: 3
                    }
                ]
            },
            options: {
                indexAxis: 'y',
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { position: 'top' }
                },
                scales: {
                    x: { beginAtZero: true, grid: { display: false } },
                    y: { grid: { display: false } }
                }
            }
        });
    });
}

function initContributorSearch(q) {
    var $input = $("#contributor-search");
    var $suggestions = $("#contributor-suggestions");
    var knownUsers = [];
    var debounceTimer;

    // Pre-populate from already-loaded developer data
    $.get('/data/developer?' + q, function(data) {
        if (data && data.labels) {
            knownUsers = data.labels.filter(function(l) { return l !== 'ALL OTHERS'; });
        }
    });

    function showSuggestions(list) {
        $suggestions.empty();
        if (!list.length) {
            $suggestions.removeClass('visible');
            return;
        }
        list.forEach(function(name) {
            $suggestions.append($('<li>').text(name));
        });
        $suggestions.addClass('visible');
    }

    function selectUser(username) {
        $input.val(username);
        $suggestions.removeClass('visible');
        loadContributorProfileChart('/data/insights/contributor-profile?u=' + encodeURIComponent(username) + '&' + q);
    }

    $input.on('input', function() {
        var val = $input.val().trim().toLowerCase();
        if (val.length < 2) {
            $suggestions.removeClass('visible');
            return;
        }

        var local = knownUsers.filter(function(u) {
            return u.toLowerCase().indexOf(val) !== -1;
        });

        if (local.length > 0) {
            showSuggestions(local.slice(0, 10));
        }

        clearTimeout(debounceTimer);
        debounceTimer = setTimeout(function() {
            $.get('/data/developer/search?q=' + encodeURIComponent(val) + '&' + q, function(results) {
                if (!results || !results.length) {
                    if (!local.length) $suggestions.removeClass('visible');
                    return;
                }
                var merged = local.slice();
                results.forEach(function(u) {
                    if (merged.indexOf(u) === -1) merged.push(u);
                });
                showSuggestions(merged.slice(0, 10));
            });
        }, 250);
    });

    $suggestions.on('click', 'li', function() {
        selectUser($(this).text());
    });

    $input.on('keydown', function(e) {
        var $items = $suggestions.find('li');
        var $active = $items.filter('.active');
        if (e.key === 'ArrowDown') {
            e.preventDefault();
            if (!$active.length) { $items.first().addClass('active'); }
            else { $active.removeClass('active').next().addClass('active'); }
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            if ($active.length) { $active.removeClass('active').prev().addClass('active'); }
        } else if (e.key === 'Enter') {
            e.preventDefault();
            if ($active.length) { selectUser($active.text()); }
            else if (!$input.val().trim()) {
                if (contributorProfileChart) { contributorProfileChart.destroy(); contributorProfileChart = null; }
                $("#contributor-score").text('');
            }
        } else if (e.key === 'Escape') {
            $suggestions.removeClass('visible');
        }
    });

    $(document).on('click', function(e) {
        if (!$(e.target).closest('.contributor-search-wrap').length) {
            $suggestions.removeClass('visible');
        }
    });
}

function loadContributorMomentumChart(url) {
    $.get(url, function (data) {
        if (contributorMomentumChart) contributorMomentumChart.destroy();
        contributorMomentumChart = new Chart($("#contributor-momentum-chart")[0].getContext("2d"), {
            type: 'line',
            data: {
                labels: data.months,
                datasets: [{
                    label: 'Active (3mo rolling)',
                    data: data.active,
                    borderColor: colors[0],
                    backgroundColor: colors[0] + '33',
                    fill: true,
                    tension: 0.3,
                    pointRadius: 3
                }, {
                    label: 'Delta',
                    type: 'bar',
                    data: data.delta,
                    backgroundColor: data.delta.map(d => d >= 0 ? colors[1] + '88' : colors[3] + '88'),
                    borderWidth: 0,
                    yAxisID: 'y1',
                    order: 2
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: true } },
                scales: {
                    x: { ticks: { font: { size: 14 } } },
                    y: { beginAtZero: true, position: 'left', ticks: { precision: 0, font: { size: 14 } },
                        title: { display: true, text: 'Active Contributors' } },
                    y1: { position: 'right', grid: { drawOnChartArea: false },
                        ticks: { precision: 0, font: { size: 14 } },
                        title: { display: true, text: 'Delta' } }
                }
            }
        });
    });
}
