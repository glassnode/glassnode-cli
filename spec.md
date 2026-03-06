## Implementation

### 1. cli naming, should be short but descriptive:

 - gn - the name of the cli

### 2. Authentication

- using env var, eg `export GLASSNODE_API_KEY=your-key`
- or using args, eg `--api-key your-key`
- lets also consider .gn folder in the home directory to store the api key

### 3. Commands

- follows mostly the proposal from Can
- sticks to service / resource like kubetcl / aws cli naming convention instead of POSIX / unix-style commands (eg ls)
- top level commands are asset, metric and config
- metric command has subcommands:
    - list - list metric paths
    - get - get metric data
    - describe - returns metric metadata
- asset command has subcommands:
    - list - list assets
    - describe - returns asset metadata
- config command has subcommands:
    - set with key=value pairs
    - get with key (all for all keys)


### 4. Arguments:

- short and long arguments are allowed (-a and --asset).
- JSON can be added, I dont think its that much of an effort however it wont make much sense especially given the low amount of commands we have

### 5. Language:

- Golang - easy to cross compile

### 6. Distribution:

- code hosted on github public repo instead of gitlab public repo, better discoverability by the LLMs
- binaries distributed via github releases, links to the CLI on the glassnode website, just like [cli.github.com](http://cli.github.com/)
- github actions to build and release the CLI with support for the following platforms:
    - mac os arm
    - mac os amd
    - linux amd64
    - linux arm64
    - windows amd64
- push skills to hubs (eg https://www.skillhub.club/docs/cli#push) from the CI

### 7. Extra

So far we talked about the cli usage only in terms of API but maybe we can also think of providing extra functionality for:

- creating alerts
- retrieving information about the current plan - access tiers, data packages, usage
