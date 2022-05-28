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
* `type` - Event type (pr, issue, pr_comment, issue_comment)
* `author` - Event author (GitHub username)
* `since` - Event since date (YYYY-MM-DD)
* `label` - GitHub label (like query on issues and PRs)
* `mention` GitHub mentions (like query on @username in body of the event or its assignments)
* `limit` - Limits number of result returned (default: 500)
 

```shell
dctl query events --org knative --repo serving
```

```json
[
  {
    "event_id": 378946614,
    "event_org": "knative",
    "event_repo": "serving",
    "event_date": "2018-11-08",
    "event_type": "issue",
    "event_url": "https://github.com/knative/serving/pull/2437",
    "event_mention": "mattmoor",
    "event_labels": "size/l,lgtm,approved",
    "dev_id": 16194785,
    "dev_update_date": "2022-05-28",
    "dev_username": "k4leung4",
    "dev_avatar_url": "https://avatars.githubusercontent.com/u/16194785?v=4",
    "dev_profile_url": "https://github.com/k4leung4"
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
| event_url  | `TEXT`    | `false`    |
| mention    | `TEXT`    | `false`    |
| labels     | `TEXT`    | `false`    |

## Disclaimer

This is my personal project and it does not represent my employer. I take no responsibility for issues caused by this code. I do my best to ensure that everything works, but if something goes wrong, my apologies is all you will get.
