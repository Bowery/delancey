crosswalk
======

Simple tool for syncing and running your application in the cloud.

## Components

1. Agent
2. Command Line

### Agent
The agent runs on the machine you would like to develop on. It receives file changes from your computer and runs your code.

### Command Line
The command line watches for file changes and sends the diff to the agent.

## Development

Using Vagrant set up a "remote server" which the agent will run on.
```
$ vagrant up --provider=vmware_fusion
```

SSH into the machine and run the agent.
```
$ vagrant ssh server
$ cd gocode/src/Bowery/crosswalk/agent
$ go run main.go
```

Outside the "remote server" run the client
```
$ go run main.go
```

You will be prompted to give an address and commands. Input the IP of the "remote server."

Good to go!
