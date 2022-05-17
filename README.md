# dctl

`dctl` is an open source project created to provide a quick insight into the activity of a single repo or an entire GitHub organization across the two main dimensions:
            
* Volume of developer events over time (PR, issue, and their comments)
* Contributions by developer entity affiliation during time period (company or organization)

![](docs/img/screenshot.png)

> Hope you find this tool helpful. [Let me know](https://twitter.com/mchmarny) if you have any questions.

## Why

One of the core principles on which `dctl` is built is that open source contributions are more than just PRs. And, while GitHub does provide repo-level activity charts and Grafana has a [plugin for GitHub](https://grafana.com/grafana/plugins/grafana-github-datasource/) (e.g. [CNCF Grafana Dashboard](https://k8s.devstats.cncf.io/)), there isn't really anything out there that's both, free and simple to use that provides that data.

## How

`dctl` imports all contribution metadata for a specific repo(s) using the [GitHub API](https://docs.github.com/en/rest), and augments that data with developer affiliations from sources like [CNCF](https://github.com/cncf/gitdm) and [Apache Foundation](https://www.apache.org/foundation/members.html). More about importing data [here](docs/IMPORT.md].

Once downloaded, `dctl` exposes that data using a local UI with option to drill-downs different aspects of the project activity (screenshot above). The instructions on how to start the integrated server and access the UI in your browser are located [here](docs/SERVER.md).

`dctl` can also be used to query the imported data in CLI and output JSON payloads for subsequent postprocessing in another tool (e.g. [jq](https://stedolan.github.io/jq/)). More about the CLI query option [here](docs/QUERY.md)

Whichever way you decide to use `dctl`, you will be able to use time period, contribution type, and developer name filters to further scope your data and identify specific trends with direct links to the original detail in GitHub for additional context. And, since all this data is cached locally in [sqlite](https://www.sqlite.org/index.html) DB, you can even use another tool to further customized your queries using SQL without the need to re-download data. More about that [here](docs/QUERY.md)

## Usage 

`dctl` is a dual-purpose utility that can be either used as a `CLI` to [authentication](#authentication), [import](docs/IMPORT.md) data, and [query](docs/QUERY.md) the data, or as a [server](docs/SERVER.md) to launch a local app that provides UI to access the imported data in your browser. 

* [authentication](#authentication)
* [import](docs/IMPORT.md)
* [query](docs/QUERY.md)
* [server](docs/SERVER.md)

## Authentication 

To get access to the higher GitHub API rate limits, `dctl` uses OAuth authentication to obtain a token:

> `dctl` doesn't ask for any access scopes, so the resulting token has only access to already public data

```shell
dctl auth
```

The result should look something like this: 

```shell
1). Copy this code: E123-4567
2). Navigate to this URL in your browser to authenticate: https://github.com/login/device
3). Hit enter to complete the process:
```

Once you enter the provided code in the GitHub UI prompt and hit enter, `dctl` will persist the token in your home directory for all subsequent queries. Should you need to, you can revoke that token in your [GitHub Settings](https://docs.github.com/en/developers/apps/managing-oauth-apps/deleting-an-oauth-app). 

## Disclaimer

This is my personal project and it does not represent my employer. I take no responsibility for issues caused by this code. I do my best to ensure that everything works, but if something goes wrong, my apologies is all you will get.
