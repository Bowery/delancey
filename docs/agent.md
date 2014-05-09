# Crosswalk Agent
The agent runs on the server where you would like to run your application. It is responsible for handling requests for the [command-line interface](/docs/cli) to update files and restarting the application.

## Arguments
```
Usage: crosswalk [options]

Options:
  --addr         Address of Agent.
  --start        Command to start application.
  --test         Command to test application.
  --build        Command to build application. It's always run before the Start Command.
  --path         Path to sync files from.
```

## Running as a Daemon With Upstart
If you're using a linux distribution that supports upstart. You should create a `crosswalk.conf` file that looks something like:
```
description "Crosswalk Agent"

start on runlevel [2345]
stop on runlevel [!2345]

exec /usr/bin/crosswalk --password SUP3RS3CUR3 --dir /application
```
Then reload your upstart configurations with:
```
sudo initctl reload-configuration
sudo service crosswalk restart
``


## HTTP API
The crosswalk agent is has a very simple HTTP API that you can easily write applications to integrate with.

### POST /
Creates a new application. It accepts a tar file that it will extract, run build, test, & start.

### PUT /
Updates a file with the new file, then runs build, test, & start

### GET /ping
Sends 200 OK if it is alive
