# Github Stats
A script to get repo metric from a github account such as lines of code or files per language.

Uses HTTP package and wait groups from Go to download zip archives in parallel for the lines of code metric.

Execute the program with `go run main.go`. Configurations should be in a `config.json` file in the same directory.

Sample `config.json` file:
```
{
    "token": "<your-github-token>",
    "users": ["username"]
    "repos: ["user/repo"]
    "includeExts": ["ts", "go", "rust", ["cpp", "hpp"]]
    "excludeDirs": ["build", "target"]
}
```

The `token` requires "repo" privileges to run the script. Create a token at https://github.com/settings/tokens

The `include` variable specifies a space separated list of file extensions to include. You can `group`
extensions into one using an array instead of a string like so `["cpp", "hpp"]`. In that case, `cpp` and `hpp` files are added to the same metric
groupings. 

The `exclude` variable specifies directories to exclude. This is useful for dynamic languages where the
artefacts of a project use the same extensions as the source.

## Example
![Example](https://github.com/user-attachments/assets/681dc614-10ca-4686-bcd8-1708f7a336b2)