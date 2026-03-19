# Deep-scoring reputation

Deep-score contributor reputation using GitHub API signals (profile age, org membership, PR history, forked repos, etc.). Basic (shallow) reputation is computed automatically during `import` — this command adds richer scoring for the lowest-reputation contributors.

Shallow scoring only uses local DB signals (commit count, last commit date). Deep scoring calls the GitHub API to gather additional signals (profile age, followers, org membership, PR merge history, forked repos, etc.) and produces a more accurate score.

Once a contributor has been deep-scored, shallow scoring will not overwrite their score — only the deep scoring cycle refreshes deep scores.

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

## Stale threshold

Control how recently a contributor must have been scored to skip re-scoring:

```shell
devpulse score --org <org> --stale 1w
```

Accepts Go duration syntax (`72h`) plus shorthand `d` (days) and `w` (weeks). Default: `3d` (72 hours).

Deep scoring makes multiple GitHub API calls per user. Run incrementally to stay within rate limits — each invocation scores the next batch of lowest-reputation contributors.

## Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--org` | GitHub organization or user | (required) |
| `--repo` | Repository name | all repos in org |
| `--count` | Number of lowest-reputation contributors to deep-score | 5 |
| `--stale` | Duration before a score is considered stale (e.g. `72h`, `3d`, `1w`) | `3d` |
| `--format` | Output format: `json` or `yaml` | json |
| `--debug` | Enable verbose logging | false |
| `--log-json` | Output logs in JSON format | false |
