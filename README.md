crosswalk
======

Simple tool for syncing and running your application in the cloud.

## Components

1. Agent
2. Command Line

### Agent
The agent runs on the machine you would like to develop on. It receives file changes from your computer and runs your code.

```
./agent --dir /some/dir
```

### Command Line
The command line watches for file changes and sends the diff to the agent.
```
cli --addr 162.243.64.10 --start "node app.js" --test "npm test" --build "npm install" --path "."
```
