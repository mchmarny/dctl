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

`dctl` is dual-purpose app that can either be used as a `CLI` to import and query data, or as a `server` to launch a local app that can be accessed in your browser. Start by launching the CLI: 

```shell
dctl
```

You should see the CLI version and a short summary along with the usage options: 

* `auth` - Authenticate to GitHub to obtain an access token
* `import` - List data import operations
* `query` - List data query operations
* `server` - Start local HTTP server

### Authentication 

To avid the low rate limits for unauthenticated queries against GitHub API, `dctl` uses OAuth token. To obtain the token you will have first authenticate:

> `dctl` doesn't ask for any scopes, so the resulting token has only access to already public data

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

### Import Data 

> Assumes you have already authenticated

The `dctl` CLI comes with an embedded [SQLite](https://www.sqlite.org/index.html) database. The following import operations are currently supported: 

* `events` - Imports GitHub repo event data (PRs, comments, issues, etc)
* `affiliations` - Updates imported developer entity/identity with CNCF and GitHub data
* `names` - Updates imported developer names with Apache Foundation data
* `updates` - Update all previously imported org, repos, and affiliations

#### Import GitHub Events

> `dctl` will need an access to your [GitHub access token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token). Either create an environment variable `GITHUB_ACCESS_TOKEN` to hold that token or provide it each time using the `--token` flag. 

```shell
dctl import events --org <organization> --repo <repository>
```

> By default, `dctl` will download data for the last 6 months. Provide `--months` flag to download less or more data.

When completed, `dctl` will return a summary of the import: 

```json
{
    "org": "tektoncd",
    "repo": "dashboard",
    "duration": "4.4883105s",
    "imported": {
        "issue_comment":61,
        "issue_request":42,
        "pr_comment":55,
        "pr_request":100
    }
}
```

To get a more immediate feedback during import use the debug flag:

```shell
dctl --debug import events --org <organization> --repo <repository>
```

#### Update Developer Name and Entity Affiliation

> Assumes you have already authenticated

Developers on GitHub often don't include their company or organization affiliation, and when they do, there use all kind of creative ways of spelling it (you'd be surprized how many different IBMs and Googles are out there). To clean this data up, `dctl` provides two different operations:

* `affiliations` - Updates imported developer entity/identity with CNCF and GitHub data
* `names` - Updates imported developer names with Apache Foundation data

To update affiliations using [CNCF developer affiliation files](https://github.com/cncf/gitdm):

```shell
dctl import affiliation
```

> Alternatively you can provide the `--url` parameter to import a specific `developers_affiliations.txt` file 

When completed, `dctl` will output the results (in this example, out of the `3756` unique developers that were already imported into the local database, `459` were updated based on the `38984` CNCF affiliations): 

```json
{
    "duration": "1m4.576478333s",
    "db_devs": 3756,
    "cncf_devs": 38984,
    "mapped_devs": 459
}
```

Just like before, to get a more immediate feedback during import use the --debug flag.

#### Update Developer Full Name

Similarly, you can use the [Apache Foundation](https://www.apache.org/foundation/members.html) developer data to update developer full name (AF's data is used only when the local data has no developer full name):

```shell
dctl import names
```

Like with the affiliation, when done, `dctl` will return the results (in this example, out of the `3201` unique developers that were already imported into the local database, `3` were updated based on the `8337` AF names): 

```json
{
    "duration": "740.209µs",
    "db_devs": 3201,
    "af_devs": 8337,
    "mapped_devs": 3
}
```

### View Data

Once the data has been imported, the easiest way to view it is to start a local server: 

```shell
dctl server
```

The result should be the information with the address:

```shell
INFO    server started on 127.0.0.1:8080
```

At this point you can use your browser to navigate to [127.0.0.1:8080](http://127.0.0.1:8080) to view the data. 

> You can change the port on which the server starts by providing the `--port` flag. 

### Query Data 

> Assumes you have already authenticated

The imported data is also available as `JSON` via `dctl` query:

```shell
dctl query
```

Commands: 

* `developers` - List developers
* `developer` - Get specific CNCF developer details, identities and associated entities
* `entities` - List entities (companies or organizations with which users are affiliated)
* `entity` - Get specific CNCF entity and its associated developers
* `repositories` - List GitHub org/user repositories
* `events` - List GitHub repository events


### Query Developer

Query for developer usernames and their last update info: 

```shell
dctl query developers --like marn
```

```json
[
    {
        "username": "mchmarny",
        "update_date": "2022-05-13"
    },
    ...    
]
```

> You can use the `--limit` flag to indicate the maximum number of result that should be returned (default: 100)

You can also query for details of a single developer: 

```shell
dctl query developer --name mchmarny
```

```json
{
    "username": "mchmarny",
    "update_date": "2022-05-13",
    "id": 175854,
    "avatar_url": "https://avatars.githubusercontent.com/u/175854?v=4",
    "profile_url": "https://github.com/mchmarny",
    "organizations": [
        {
            "url": "https://api.github.com/orgs/knative",
            "name":"knative"
        },
        ...
    ]
}
```

### Query Entities

Query for entity names and the number of repositories that have events: 

```shell
dctl query entities --like goog
```

```json
[
    {
        "name": "GOOGLE",
        "count": 23
    }
]
```

> You can use the `--limit` flag to indicate the maximum number of result that should be returned (default: 100)

You can also get all the details for specific entity: 

```shell
dctl query entity --name GOOGLE
```

```json
{
    "entity": "GOOGLE",
    "developer_count": 23,
    "developers": [
        {
            "username": "mchmarny",
            "entity": "GOOGLE",
            "update_date": "2022-05-14"
        },
    ]
}
```



### Query Repositories

Query for organization repositories: 

```shell
dctl query repositories --org knative
```

```json
[
    {
        "name": "serving",
        "full_name": "knative/serving",
        "description": "Kubernetes-based, scale-to-zero, request-driven compute",
        "url": "https://github.com/knative/serving"
    },
    ...
]
```

> You can use the `--limit` flag to indicate the maximum number of result that should be returned (default: 100)


### Query Events

Query events provides a number of filter options: 

* `org` - Name of the GitHub organization or user
* `repo` - Name of the GitHub repository
* `since` - Event since date (YYYY-MM-DD)
* `author` - Event author (GitHub username)
* `type` - Event type (pr_request, issue_request, pr_comment, issue_comment)
* `limit` - Limits number of result returned (default: 500)

> Given the possible amount data, the `--org` and `--repo` flags are required

```shell
dctl query events --org knative --repo serving
```

```json
[
    {
        "id": 1235445267,
        "org": "knative",
        "repo": "serving",
        "username": "phunghaduong99",
        "type": "issue_request",
        "date": "2022-05-13"
    },    
    {
        "id": 935056755,
        "org": "knative",
        "repo": "serving",
        "username": "dprotaso",
        "type": "pr_request",
        "date": "2022-05-12"
    },
    ...
]
```


## Query DB Directly (SQL)

For more specialized queries you can also query the local database. The imported data is stored in your home directory, inside of the `.dctl` directory.

```shell
qlite3 ~/.dctl/data.db
```

DB Schema

> The script to create DB schema is located in [pkg/data/sql/ddl.sql](pkg/data/sql/ddl.sql)

### Table: `developer` (PK: `username`)

| `Columns`     | `Type`    | `Nullable` |
| ------------- | --------- | ---------- |
| username      | `TEXT`    | `false`    |
| update_date   | `TEXT`    | `false`    |
| id            | `INTEGER` | `false`    |
| full_name     | `TEXT`    | `true`     |
| email         | `TEXT`    | `true`     |
| avatar_url    | `TEXT`    | `true`     |
| profile_url   | `TEXT`    | `true`     |
| entity        | `TEXT`    | `true`     |
| location      | `TEXT`    | `true`     |

### Table: `event` (PK: `id`, `org`, `repo`, `username`, `event_type`, `event_date`)

| `Columns`  | `Type`    | `Nullable` |
| ---------- | --------- | ---------- |
| id         | `INTEGER` | `false`    |
| org        | `TEXT`    | `false`    |
| repo       | `TEXT`    | `false`    |
| username   | `TEXT`    | `false`    |
| event_type | `TEXT`    | `false`    |
| event_date | `TEXT`    | `false`    |


## Disclaimer

This is my personal project and it does not represent my employer. I take no responsibility for issues caused by this code. I do my best to ensure that everything works, but if something goes wrong, my apologies is all you will get.
