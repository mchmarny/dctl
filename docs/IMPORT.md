# dctl - import

> Assumes you have already [authenticated](../README.md)

The `dctl` CLI comes with an embedded [SQLite](https://www.sqlite.org/index.html) database. The following import operations are currently supported: 

* `events` - Imports GitHub repo event data (PRs, comments, issues, etc)
* `affiliations` - Updates imported developer entity/identity with CNCF and GitHub data
* `names` - Updates imported developer names with Apache Foundation data
* `updates` - Update all previously imported org, repos, and affiliations

## Import GitHub Events

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

## Update Developer Name and Entity Affiliation

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

## Update Developer Full Name

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


## Disclaimer

This is my personal project and it does not represent my employer. I take no responsibility for issues caused by this code. I do my best to ensure that everything works, but if something goes wrong, my apologies is all you will get.
