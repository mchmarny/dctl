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
let reputationChartURL;
let forksAndActivityChart;
let searchItem;

const searchPrefixes = ['org', 'repo', 'entity'];

function parseSearchInput(raw) {
    const match = raw.match(/^(org|repo|entity):(.*)$/i);
    if (match) {
        return { scope: match[1].toLowerCase(), query: match[2].trimStart() };
    }
    return { scope: 'org', query: raw };
}

$(function () {
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

    $("#reputation-popover-close").click(function () {
        $("#reputation-popover").removeClass("open");
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

    if ($("#search-bar").length) {
        searchCriteria.init();
        initUnifiedSearch();
        initSearchFilters();
        initPeriodSelector();
        updatePeriodOptions("", "", function () {
            loadAllCharts($("#period_months").val(), "", "", "");
        });
    }
});

function loadAllCharts(months, org, repo, entity) {
    const onLeftExclude = function () {
        leftChart.destroy();
        const x = leftChartExcludes.join("|");
        loadLeftChart(`/data/entity?m=${months}&o=${org}&r=${repo}&e=${entity}&x=${x}`, onLeftChartSelect, onLeftExclude);
    };
    const onRightExclude = function () {
        rightChart.destroy();
        const x = rightChartExcludes.join("|");
        loadRightChart(`/data/developer?m=${months}&o=${org}&r=${repo}&e=${entity}&x=${x}`, onRightChartSelect, onRightExclude);
    };

    loadTimeSeriesChart(`/data/type?m=${months}&o=${org}&r=${repo}&e=${entity}`, onTimeSeriesChartSelect);
    loadLeftChart(`/data/entity?m=${months}&o=${org}&r=${repo}&e=${entity}`, onLeftChartSelect, onLeftExclude);
    loadRightChart(`/data/developer?m=${months}&o=${org}&r=${repo}&e=${entity}`, onRightChartSelect, onRightExclude);
    loadInsightsSummary(`/data/insights/summary?m=${months}&o=${org}&r=${repo}&e=${entity}`);
    loadRetentionChart(`/data/insights/retention?m=${months}&o=${org}&r=${repo}&e=${entity}`);
    loadPRRatioChart(`/data/insights/pr-ratio?m=${months}&o=${org}&r=${repo}&e=${entity}`);
    loadVelocityChart(`/data/insights/time-to-merge?m=${months}&o=${org}&r=${repo}&e=${entity}`, 'time-to-merge-chart', 'timeToMerge');
    loadVelocityChart(`/data/insights/time-to-close?m=${months}&o=${org}&r=${repo}&e=${entity}`, 'time-to-close-chart', 'timeToClose');
    loadForksAndActivityChart(`/data/insights/forks-and-activity?m=${months}&o=${org}&r=${repo}&e=${entity}`);
    loadRepoMeta(`/data/insights/repo-meta?o=${org}&r=${repo}`);
    loadReleaseCadenceChart(`/data/insights/release-cadence?m=${months}&o=${org}&r=${repo}`);
    loadReleaseDownloadsChart(`/data/insights/release-downloads?m=${months}&o=${org}&r=${repo}`);
    loadReleaseDownloadsByTagChart(`/data/insights/release-downloads-by-tag?m=${months}&o=${org}&r=${repo}`);
    loadReputationChart(`/data/insights/reputation?m=${months}&o=${org}&r=${repo}&e=${entity}`);
}

function applySelection(scope, item) {
    resetSearch();
    autocomplete_cache = {};
    leftChartExcludes = [];
    rightChartExcludes = [];

    searchItem = item;
    $(".header-term").html(item.value);

    resetCharts();

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

    submitSearch();
    updatePeriodOptions(org, repo, function () {
        loadAllCharts($("#period_months").val(), org, repo, entity);
    });
}

function initUnifiedSearch() {
    const sel = $("#search-bar");
    const dropdown = $("#ac-dropdown");
    let activeIndex = -1;
    let currentItems = [];
    let currentScope = 'org';

    function showDropdown(items) {
        currentItems = items;
        activeIndex = -1;
        dropdown.empty();
        if (items.length === 0) {
            dropdown.removeClass("open");
            return;
        }
        $.each(items, function (i, item) {
            $(`<div class="ac-item">${item.text}</div>`)
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
        sel.val(`${currentScope}:${item.value}`);
        hideDropdown();
        applySelection(currentScope, item);
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
    if (reputationChart) {
        reputationChart.destroy();
    }
    if (forksAndActivityChart) {
        forksAndActivityChart.destroy();
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
    searchCriteria.user = label;
    syncFiltersToInputs();
    submitSearch();
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
                onClick: (evt, item) => {
                    if (item.length) {
                        const label = rightChart.data.labels[item[0].index];
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

function loadInsightsSummary(url) {
    $.get(url, function (data) {
        $("#bus-factor-val").text(data.bus_factor);
        $("#pony-factor-val").text(data.pony_factor);
    });
}

function loadRetentionChart(url) {
    $.get(url, function (data) {
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

function loadVelocityChart(url, canvasId, key) {
    $.get(url, function (data) {
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
    });
}

function loadReleaseCadenceChart(url) {
    $.get(url, function (data) {
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

function reputationBarColors(values) {
    return values.map(function (v) {
        if (v >= 0.7) return '#2da44e';
        if (v >= 0.4) return '#bf8700';
        return '#cf222e';
    });
}

function loadReputationChart(url) {
    reputationChartURL = url;
    $.get(url, function (data) {
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
                    borderWidth: 1
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
                        ticks: { font: { size: 14 } }
                    }
                },
                onClick: (evt, item) => {
                    if (item.length) {
                        const username = reputationChart.data.labels[item[0].index];
                        showDeepReputation(username);
                    }
                }
            }
        });
    });
}

function showDeepReputation(username) {
    const popover = $("#reputation-popover");
    const list = $("#reputation-popover-list");
    const title = $("#reputation-popover-title");

    title.text(username);
    list.empty().append('<li>Computing full score...</li>');
    popover.addClass("open");

    $.get(`/data/insights/reputation/user?u=${encodeURIComponent(username)}`, function (data) {
        list.empty();
        const color = data.reputation >= 0.7 ? '#2da44e' : (data.reputation >= 0.4 ? '#bf8700' : '#cf222e');
        const label = data.reputation >= 0.7 ? 'High' : (data.reputation >= 0.4 ? 'Medium' : 'Low');
        const cached = data.deep ? '' : ' (cached)';
        list.append(`<li><b>Score:</b> <span style="color:${color};font-weight:bold;">${data.reputation.toFixed(2)} (${label})</span>${cached}</li>`);
        if (data.signals) {
            const s = data.signals;
            list.append(`<li><b>Account Age:</b> ${Math.round(s.age_days / 365)}y (${s.age_days}d)</li>`);
            list.append(`<li><b>2FA Enabled:</b> ${s.strong_auth ? 'Yes' : 'No'}</li>`);
            list.append(`<li><b>Org Member:</b> ${s.org_member ? 'Yes' : 'No'}</li>`);
            list.append(`<li><b>Followers:</b> ${s.followers} &middot; <b>Following:</b> ${s.following}</li>`);
            list.append(`<li><b>Repos:</b> ${s.public_repos} public, ${s.private_repos} private</li>`);
            list.append(`<li><b>Events:</b> ${s.commits} of ${s.total_commits} total</li>`);
            list.append(`<li><b>Last Active:</b> ${s.last_commit_days}d ago</li>`);
            list.append(`<li><b>Suspended:</b> ${s.suspended ? 'Yes' : 'No'}</li>`);
        }
        list.append(`<li><a href="https://github.com/${username}" target="_blank">View on GitHub</a></li>`);

        // refresh the chart so updated scores are reflected
        if (reputationChartURL) {
            if (reputationChart) { reputationChart.destroy(); }
            loadReputationChart(reputationChartURL);
        }
    }).fail(function () {
        list.empty().append('<li>Failed to compute score</li>');
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
