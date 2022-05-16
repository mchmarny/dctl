const colors = [
    'rgb(87, 164, 177)',
    'rgb(176, 216, 148)',
    'rgb(250, 222, 137)',
    'rgb(253, 186, 187)',
    'rgb(114, 90, 76)',
    'rgb(177, 148, 87)',
    'rgb(137, 137, 250)',
    'rgb(187, 137, 253)',
    'rgb(76, 114, 90)',
    'rgb(87, 177, 148)',
    'rgb(137, 250, 137)',
    'rgb(187, 253, 137)',
    'rgb(114, 90, 76)',
    'rgb(177, 148, 87)',
    'rgb(137, 137, 250)',
    'rgb(76, 100, 114)'
];

const searchCriteria = {
    "from": null,
    "to": null,
    "event_type": null,
    "org": null,
    "repo": null,
    "user": null,
    "entity": null,
    "page": 1,
    "page_size": 10,
    init: function() {
        let origValues = {};
        for (let prop in this) {
            if (this.hasOwnProperty(prop) && prop != "origValues") {
                origValues[prop] = this[prop];
            }
        }
        this.origValues = origValues;
    },
    reset: function() {
        for (let prop in this.origValues) {
            this[prop] = this.origValues[prop];
        }
    }
}

let autocomplete_cache = {};
let timeEventsChart;
let leftChart;
let rightChart;
let searchItem;

$(function () {
    // On page load
    const urlParams = new URLSearchParams(window.location.search);
    console.log(window.location);
    console.log(urlParams);

    /* GLOBALs */
    $(".init-hide").hide();
    $(window).resize(function () {
        const scrollWidth = $('.tbl-content').width() - $('.tbl-content table').width();
        $('.tbl-header').css({'padding-right': scrollWidth });
    });

    /* TOGGLE HEADER STATE */
    $(".admin-menu .collapse-btn").click(function() {
        $("body").toggleClass("collapsed");
        $(".admin-menu").attr("aria-expanded") == "true" 
            ? $(".admin-menu").attr("aria-expanded", "false") 
            : $(".admin-menu").attr("aria-expanded", "true");
        $(".collapse-btn").attr("aria-label") == "collapse menu" 
            ? $(".collapse-btn").attr("aria-label", "expand menu") 
            : $(".collapse-btn").attr("aria-label", "collapse menu");
    });

    /* TOGGLE MOBILE MENU */
    $(".toggle-mob-menu").click(function() {
        $("body").toggleClass("mob-menu-opened");
        $(".admin-menu").attr("aria-expanded") == "true" 
            ? $(".admin-menu").attr("aria-expanded", "false") 
            : $(".admin-menu").attr("aria-expanded", "true");
        $(".collapse-btn").attr("aria-label") == "open menu" 
            ? $(".collapse-btn").attr("aria-label", "close menu") 
            : $(".collapse-btn").attr("aria-label", "open menu");
    });

    // Change view based on the clicked link
    $(".nav-link").click(function(e) {
        e.preventDefault();
        const nav = $(this).data("nav");
        console.log(`nav: ${nav}`);
        
        // if not home go there 
        if (window.location.pathname == '/about') {
            window.location.href = `/?nav=` + nav;
            return false;
        }

        loadView(nav);
        return false;
    });

    // Only on home page
    if ($("#search-bar").length) {
        console.log("search form found");
        // initialize search criteria to default values
        searchCriteria.init();

        // initialize autocomplete and load the full data view for orgs 
        if (urlParams.has('nav')) {
            const nav = urlParams.get('nav');
            console.log(`nav: ${nav}`);
            loadView(nav);
        } else {
            loadView("org");
        }
    }
});


function loadView(view) {
    resetSearch();

    const months = $("#period_months").val();
    const searchBar = $("#search-bar").attr("placeholder", `select ${view}...`).val("");
    const searchTerm = $(".header-term").html("All imported events");
    
    resetCharts();
    let org = "", repo = "", entity = "";

    setupSearchAutocomplete(searchBar, `/data/query?v=${view}&q=`, function(item) {
        resetSearch();

        searchItem = item;
        searchTerm.html(item.text);

        // destroy previous charts
        resetCharts();
        
        // parse what the item mean 
        switch(view) {
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
            default:
              console.log(`unknown view: ${view}, defaulting to org`);
          }

          // submit search based on the view selection
          submitSearch();

        // re-reload charts
        loadTimeSeriesChart(`/data/type?m=${months}&o=${org}&r=${repo}&e=${entity}`, onTimeSeriesChartSelect);
        loadLeftChart(`/data/entity?m=${months}&o=${org}&r=${repo}&e=${entity}`, onLeftChartSelect);
        loadRightChart(`/data/developer?m=${months}&o=${org}&r=${repo}&e=${entity}`, onRightChartSelect);
    });
    loadTimeSeriesChart(`/data/type?m=${months}&o=${org}&r=${repo}&e=${entity}`, onTimeSeriesChartSelect);
    loadLeftChart(`/data/entity?m=${months}&o=${org}&r=${repo}&e=${entity}`, onLeftChartSelect);
    loadRightChart(`/data/developer?m=${months}&o=${org}&r=${repo}&e=${entity}`, onRightChartSelect);

    $("#prev-page").click(function(e) {
        e.preventDefault();
        if (searchCriteria.page > 1) {
            searchCriteria.page--;
            submitSearch();
        }
    });
    $("#next-page").click(function(e) {
        e.preventDefault();
        searchCriteria.page++;
        submitSearch();
    });
}

function resetSearch() {
    searchItem = null; 
    $("#result-table-content").empty();
    $(".init-hide").hide();
    searchCriteria.reset();
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
}

function onTimeSeriesChartSelect(label, val) {
    searchCriteria.from = label + "-01";
    searchCriteria.to = label + "-31";
    if (val != "Mean") {
        searchCriteria.event_type = val;
    }
    submitSearch();
}

function onLeftChartSelect(label) {
    searchCriteria.entity = label;
    submitSearch();
}

function onRightChartSelect(label) {
    searchCriteria.user = label;
    submitSearch();
}

function submitSearch() {
    const table = $("#result-table-content").empty();
    const criteria = JSON.stringify(searchCriteria);
    console.log(criteria);
    $.post("/data/search", criteria).done(function(data){
        console.log(data);
        displaySearchResults(table, data);
    }).fail(function(response) {
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
        $(".init-hide").show();
        return;
    }
    $.each(data, function(key, item) {
        $("<tr>")
            .append(`<td>${item.event_date}</td>`)
            .append(`<td><a href="#" class="gh-link">${item.org}/${item.repo}</a></td>`)
            .append(`<td>${item.event_type}</td>`)
            .append(`<td><a href="#" class="gh-link">${item.username}</a> ${parseOptional(item.full_name, " - ")}</td>`)
            .append(`<td>${parseOptional(item.entity)}</td>`)
            .appendTo(table); 
    });
    $(".gh-link").click(function(e) {
        e.preventDefault();
        openGitHubTab($(this).html());
    });
    $(".init-hide").show();
    return false;
}

function openGitHubTab(val) {
    window.open("https://github.com/" + val, '_blank');
}

function handleResponseError(response) {
    console.log(response);
    if (response.status == 400) {
        if (response.responseJSON.message) {
            $("#error-dialog p").html(response.responseJSON.message);
            $("#error-dialog").dialog({
                modal: true,
                buttons: {
                    Ok: function () {
                        $(this).dialog("close");
                    }
                }
            });
            return false;
        }
        alert("Bad request, please check your input.");
        return false;
    }
    alert("Server error, please try again later.");
    return false;
}

function setupSearchAutocomplete(sel, url, fn) {
    $(sel).on("input", function() {
        const val = $(this).val();
        if (val.length < 1) {
            resetSearch();
        }
        return false;
    });

    $(sel).autocomplete({
        minLength: 1,
        source: function (request, response) {
            const term = request.term;
            if (term in autocomplete_cache) {
                response(autocomplete_cache[term]);
                return;
            }

            $.getJSON(url + term, request, function (data, status, xhr) {
                autocomplete_cache[term] = data;
                response(data);
            });
        },
        select: function (event, ui) {
            $(sel).val(ui.item.text);
            if (fn) {
                fn(ui.item);
            }
            return false;
        }
    }).autocomplete("instance")._renderItem = function (ul, item) {
        return $("<li>")
            .data("item", item)
            .append(`<div class="ac-item">${item.text}</div>`)
            .appendTo(ul);
    }
    return false;
}


function loadTimeSeriesChart(url, fn) {
    $.get(url, function (data) {
        console.log(data);
        timeEventsChart = new Chart($("#time-series-chart")[0].getContext("2d"), {
            type: 'bar',
            data: {
                labels: data.dates,
                datasets: [{
                    label: 'PR',
                    data: data.pr_request,
                    backgroundColor: colors[0],
                    borderWidth: 1,
                    order: 2
                },{
                    label: 'PR-Comments',
                    data: data.pr_comment,
                    backgroundColor: colors[1],
                    borderWidth: 1,
                    order: 3
                },{
                    label: 'Issues',
                    data: data.issue_request,
                    backgroundColor: colors[2],
                    borderWidth: 1,
                    order: 4
                },{
                    label: 'Issue-Comments',
                    data: data.issue_comment,
                    backgroundColor: colors[3],
                    borderWidth: 1,
                    order: 5
                },{
                    label: 'Mean',
                    type: 'line',
                    fill: false,
                    data: data.avg,
                    borderColor: colors[4],
                    order: 1,
                    borderWidth: 5,
                    showLine: true,
                    tension: 1
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
                        console.log(timeEventsChart.data.datasets);
                        const label = timeEventsChart.data.labels[item[0].index];
                        const val = timeEventsChart.data.datasets[item[0].datasetIndex].label;
                        console.log(`time series chart selected label: '${label}', value: '${val}`);
                        if (fn) {
                            fn(label, val);
                        }
                        return false;
                    }
                    return false;
                }
            }
        }); // end eventChart
    });
}


function loadLeftChart(url, fn) {
    $.get(url, function(data) {
        console.log(data);
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
                        console.log(`left chart selected label: '${label}`);
                        if (fn) {
                            fn(label);
                        }
                        return false;
                    }
                    return false;
                }
            }
        }); // end eventChart
    });
}

function loadRightChart(url, fn) {
    $.get(url, function(data) {
        console.log(data);
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
                        console.log(`right chart selected label: '${label}`);
                        if (fn) {
                            fn(label);
                        }
                        return false;
                    }
                    return false;
                }
            }
        }); // end eventChart
    });
}