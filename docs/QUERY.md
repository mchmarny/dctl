# dctl - query

> Assumes you have already [authenticated](../README.md) and [imported](../IMPORT.md) data.

The imported data is also available as `JSON` via `dctl` query:

```shell
dctl query
```

There are four types of query operations:

* `developer` - List developer operations
* `entity` - List entity operations
* `org` - List GitHub org/user operations
* `events` - List GitHub events

## Developer

The developer query provides two operations: 

* `list` - List developers
* `detail` - Get specific developer details, identities and associated entities

To query the developer list you provide the `--like` flag that can be any part of the developer username or full name:

```shell
dctl query developer list --like mark
```

The response will look something like this:

```json
[
  {
    "username": "mchmarny",
    "entity": "GOOGLE"
  },
  ...    
]
```

> You can use the `--limit` flag to indicate the maximum number of result that should be returned (default: 100)

You can also query for details of a single developer: 

```shell
dctl query developer detail --name mchmarny
```

```json
{
  "username": "mchmarny",
  "full_name": "Mark Chmarny",
  "email": "mark@chmarny.com",
  "avatar_url": "https://avatars.githubusercontent.com/u/175854?v=4",
  "profile_url": "https://github.com/mchmarny",
  "current_entity": "GOOGLE",
  "organizations": [
    {
      "url": "https://api.github.com/orgs/knative",
      "name":"knative"
    }
    ...
  ]
}
```

## Entities

Just like with developer, `dctl` provides two operations for entity:

* `list` - List entities (companies or organizations with which users are affiliated)
* `detail` - Get specific entity and its associated developers


To query the entity list you provide the `--like` flag that can be any part of the entity name:

```shell
dctl query entity list --like OO
```

The response will look something like this:

```json
[
    {
        "name": "GOOGLE",
        "count": 23
    }
    ...
]
```

> You can use the `--limit` flag to indicate the maximum number of result that should be returned (default: 100)

You can also get all the details for specific entity: 

```shell
dctl query entity detail --name GOOGLE
```

```json
{
    "entity": "GOOGLE",
    "developer_count": 23,
    "developers": [
        {
            "username": "mchmarny",
            "entity": "GOOGLE"
        },
        ...
    ]
}
```

## Repositories

Query for organization repositories: 

```shell
dctl query org repos --org knative
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
* `type` - Event type (pr, pr_review, issue, issue_comment, fork)
* `author` - Event author (GitHub username)
* `since` - Event since date (YYYY-MM-DD)
* `label` - GitHub label (like query on issues and PRs)
* `mention` GitHub mentions (like query on @username in body of the event or its assignments)
* `limit` - Limits number of result returned (default: 500)

For example, to query for all issue events from the Knative Serving repository since Jan 1, that mention `@mattmoor` you would execute the following query:

```shell
dctl query events --org knative --repo serving --since 2022-01-01 --type pr --mention mattmoor
```

```json
[
  {
    "event": {
      "org": "knative",
      "repo": "serving",
      "date": "2022-02-25",
      "type": "pr",
      "url": "https://github.com/knative/serving/pull/12668",
      "mentions": "julz,mattmoor",
      "labels": "area/api,size/m,lgtm,approved",
    },
    "developer": {
      "username": "dprotaso",
      "full_name": "Dave Protasowski",
      "avatar": "https://avatars.githubusercontent.com/u/18562?v=4",
      "url": "https://github.com/dprotaso",
      "entity": "VMWARE",
    }
  },  
  ...
]
```

You can also pipe the `dctl` output to something like `jq` for further processing. For example, to get the count of all the PR events since specific date you would: 

```shell
dctl query events --org knative --repo serving --since 2022-01-01 --type pr | jq '. | length'
```

## Query DB Directly (SQL)

For more specialized queries you can also query the local database. The imported data is stored in your home directory, inside of the `.dctl` directory.

```shell
qlite3 ~/.dctl/data.db
```

### DB Schema

> The script that is used to create DB schema is located in [pkg/data/sql/ddl.sql](../pkg/data/sql/ddl.sql)

The two main tables in `dctl` schema are `developer` and `event`:

#### Table: `developer` (PK: `username`)

| `Columns`     | `Type`    | `Nullable` |
| ------------- | --------- | ---------- |
| username      | `TEXT`    | `false`    |
| full_name     | `TEXT`    | `true`     |
| email         | `TEXT`    | `true`     |
| avatar_url    | `TEXT`    | `true`     |
| profile_url   | `TEXT`    | `true`     |
| entity        | `TEXT`    | `true`     |

#### Table: `event` (PK: `org`, `repo`, `username`, `type`, `date`)

| `Columns`  | `Type`    | `Nullable` |
| ---------- | --------- | ---------- |
| org        | `TEXT`    | `false`    |
| repo       | `TEXT`    | `false`    |
| username   | `TEXT`    | `false`    |
| type       | `TEXT`    | `false`    |
| date       | `TEXT`    | `false`    |
| url        | `TEXT`    | `false`    |
| mentions   | `TEXT`    | `false`    |
| labels     | `TEXT`    | `false`    |



## Disclaimer

This is my personal project and it does not represent my employer. I take no responsibility for issues caused by this code. I do my best to ensure that everything works, but if something goes wrong, my apologies is all you will get.
