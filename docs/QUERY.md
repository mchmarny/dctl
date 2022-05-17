# dctl - query

> Assumes you have already [authenticated](../README.md)

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


## Developer

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

## Entities

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



## Repositories

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


## Events

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

> The script to create DB schema is located in [pkg/data/sql/ddl.sql](../pkg/data/sql/ddl.sql)

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
