# Deep-scoring reputation

Deep-score contributor reputation using GitHub API signals (profile age, org membership, PR history, forked repos, etc.). Basic reputation is computed automatically during `import` — this command adds richer scoring for the lowest-reputation contributors.

> All commands assume you have already [authenticated](../README.md#1-authenticate).

## Score lowest contributors in an org

```shell
devpulse score --org <org>
```

By default, deep-scores the 5 lowest-reputation contributors.

## Scope to a specific repo

```shell
devpulse score --org <org> --repo <repo>
```

## Adjust count

```shell
devpulse score --org <org> --count 20
```

Deep scoring makes multiple GitHub API calls per user. Run incrementally to stay within rate limits — each invocation scores the next batch of lowest-reputation contributors.

## Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--org` | GitHub organization or user | (required) |
| `--repo` | Repository name | all repos in org |
| `--count` | Number of lowest-reputation contributors to deep-score | 5 |
| `--format` | Output format: `json` or `yaml` | json |
| `--debug` | Enable verbose logging | false |
| `--log-json` | Output logs in JSON format | false |
