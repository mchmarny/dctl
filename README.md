# dctl

`dctl` is an open source project created to provide a quick insight into the activity of a single repo or an entire GitHub organization across the two main dimensions:
            
* Volume of developer events over time (PR, issue, and their comments)
* Contributions by developer entity affiliation during time period (company or organization)

![](docs/img/screenshot.png)

While GitHub does provide repo-level activity charts and Grafana has [plugin for GitHub](https://grafana.com/grafana/plugins/grafana-github-datasource/) (e.g. [CNCF Grafana Dashboard](https://k8s.devstats.cncf.io/)), there isn't really anything that's both, free and simple to use that provides quick answers to these questions.

`dctl` downloads locally all event metadata for set of repos indicated by you using [GitHub API](https://docs.github.com/en/rest), augments it with developer affiliations from sources like [CNCF](https://github.com/cncf/gitdm) and [Apache Foundation](https://www.apache.org/foundation/members.html), and exposes easy to use drill-downs in locally hosted UI. `dctl` can also be used to query the imported dat and output JSON payloads with results for subsequent postprocessing in another tool. Either way, you can use time period, contribution type, and developer name filters to further scope your data and identify specific trends or navigate directly to the original detail in GitHub for additional context.
        
Additionally, since all this data is cached locally, you can even use SQL to even further customized your queries without the  need to re-download data. See below for more details on how to do that. 

Hope you find this tool helpful. [Let me know](https://twitter.com/mchmarny) if you have any questions.

## Usage 

`dctl` is dual-purpose app that can either be used as a `CLI` to import and query data, or as a `server` to launch a local app that provides UI to access and query the imported data in your browser: 

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
