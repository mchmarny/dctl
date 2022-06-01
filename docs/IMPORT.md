# dctl - import

The following data import operations are currently supported: 

* `events` - Imports GitHub repo event data (PRs, comments, issues, etc)
* `affiliations` - Updates imported developer entity/identity with CNCF and GitHub data
* `substitutions` - Create a global data substitute (e.g. standardize entity name)
* `updates` - Update all previously imported org, repos, and affiliations 

The `dctl` CLI comes with an embedded [sqlite](https://www.sqlite.org/index.html) database. All imported data is persisted locally so all your queries are fast and subsequent imports only download the new data. 

## Import GitHub Events

> Assumes you have already [authenticated](../README.md)

```shell
dctl import events --org <organization> --repo <repository>
```

> By default, `dctl` will download data for the last 6 months. Provide `--months` flag to download less or more data.

When completed, `dctl` will return a summary of the import: 

```json
{
  "org": "knative",
  "repos": [
    "serving"
  ],
  "imported": {
    "knative/serving/issue": 5,
    "knative/serving/issue_comment": 2,
    "knative/serving/pr": 100,
    "knative/serving/pr_review": 79
  },
  "duration": "4.794015292s"
}
```

To import data for all repos in a specific organization simply omit the `--repo` flag:

```shell
dctl import events --org <organization>
```

To get a more immediate feedback during import use the debug flag:

```shell
dctl --debug import events --org <organization> --repo <repository>
```

## Update Developer Entity Affiliation

> Assumes you have already authenticated and imported data

Developers on GitHub often don't include their company or organization affiliation, and when they do, they tend to do it in all kinds of creative ways (you'd be surprized how many different IBMs and Googles are out there). To clean this data up, `dctl` provides:

* `affiliations` - Updates imported developer entity/identity with CNCF and GitHub data

To update affiliations using [CNCF developer affiliation files](https://github.com/cncf/gitdm):

```shell
dctl import affiliations
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

## Standardize imported data using substitutions

To  Standardize imported data (e.g. standardize entity name) you can use the substitute operation to create global data substitutions. For example, to replace all instances of `INTERNATIONAL BUSINESS MACHINES` with `IBM` you would execute the following operation: 

```shell
dctl import substitutions --type entity --old "INTERNATIONAL BUSINESS MACHINES" --new IBM
```

> Note, these substitutions will be applied to the already imported data as well as saved and apply after each new import and update operation.

The response will look something like this:

```json
{
  "prop": "entity",
  "old": "INTERNATIONAL BUSINESS MACHINES",
  "new": "IBM",
  "rows": 0
}
```

## Update all previously imported data

Once you configured the GitHub organizations and repositories for which you want to track metrics, you just need to run the `updates` command and `dctl` will automatically update the data, reconcile affiliations, and apply the substitutions. 

```shell
dctl import updates
```

> Just like with all the other operations you can include the `--debug` flag to get more immediate feedback on the update progress.

The response will look something like this:


```json
{
  "duration": "1m16.1696975s",
  "imported": {
    "knative/serving/issue": 11,
    "knative/serving/issue_comment": 38,
    "knative/serving/pr": 100,
    "knative/serving/pr_review": 83,
    ...
  },
  "updated": {
    "duration": "47.540827625s",
    "db_devs": 795,
    "cncf_devs": 38988,
    "mapped_devs": 313
  },
  "substituted": [
    {
      "prop": "entity",
      "old": "GOOGLE LLC",
      "new": "GOOGLE",
      "rows": 0
    },
    ...
  ]
}
```

## Disclaimer

This is my personal project and it does not represent my employer. I take no responsibility for issues caused by this code. I do my best to ensure that everything works, but if something goes wrong, my apologies is all you will get.
